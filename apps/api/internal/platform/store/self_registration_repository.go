package store

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/audit"
	"github.com/Pantani/cumuru/apps/api/internal/platform/idempotency"
	"github.com/Pantani/cumuru/apps/api/internal/platform/outbox"
	"github.com/Pantani/cumuru/apps/api/internal/platform/proofofwork"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	posterContextScope = "accommodation_invite_context"
	posterSubmitScope  = "accommodation_invite_submit"
)

// fragmentURL puts the capability in the fragment. A fragment is never sent to
// the server, so the token reaches no request line, no access log, no WAF and
// no CDN — the whole reason ADR-039 moved it out of the path.
func fragmentURL(base *url.URL, token string) string {
	if base == nil {
		return ""
	}
	value := *base
	value.Fragment = token
	return value.String()
}

// GetAccommodationInviteContext answers the poster context together with the
// proof-of-work challenge. The challenge rides on this route rather than on one
// of its own because the route already owns a rate limit bucket, and that
// bucket counter is the source of the adaptive difficulty.
func (r *StayRepository) GetAccommodationInviteContext(
	ctx context.Context,
	request stay.InviteRequest,
) (result stay.AccommodationInviteContext, err error) {
	if r.challenges == nil {
		return result, stay.ErrNotFound
	}
	now := r.store.currentTime()
	count, err := r.countedRateLimit(
		ctx, posterContextScope, request.Token, request.RateSubject,
		r.store.selfService.SelfServiceContextRateLimit, now,
	)
	if err != nil {
		return result, err
	}
	err = r.store.inReadOnlyTransaction(ctx, func(q generated.Querier) error {
		poster, resolveErr := r.resolvePoster(ctx, q, request.Token, now)
		if resolveErr != nil {
			return resolveErr
		}
		result, resolveErr = r.posterContext(poster, count, now)
		return resolveErr
	})
	return result, err
}

func (r *StayRepository) posterContext(
	poster resolvedPoster,
	requestCount int32,
	now time.Time,
) (stay.AccommodationInviteContext, error) {
	selfService := r.store.selfService
	difficulty := proofofwork.Difficulty(
		selfService.DifficultyBase, selfService.DifficultyCeiling,
		selfService.DifficultyRequestsPerBit, requestCount,
	)
	challenge, err := r.challenges.Issue(poster.inviteID, difficulty, now)
	if err != nil {
		return stay.AccommodationInviteContext{}, stay.ErrUnavailable
	}
	return stay.AccommodationInviteContext{
		AccommodationName:    poster.row.AccommodationName,
		PrivacyNoticeVersion: poster.row.PrivacyNoticeVersion,
		ProofOfWork: stay.ProofOfWorkChallenge{
			Algorithm: challenge.Algorithm, Challenge: challenge.Value,
			DifficultyBits: int(challenge.DifficultyBits),
			ExpiresAt:      challenge.ExpiresAt,
		},
	}, nil
}

func (r *StayRepository) SubmitSelfRegistration(
	ctx context.Context,
	command stay.SelfRegistrationCommand,
) (result stay.SelfRegistrationAccepted, replayed bool, err error) {
	if r.challenges == nil {
		return result, false, stay.ErrNotFound
	}
	now := r.store.currentTime()
	if _, err = r.countedRateLimit(
		ctx, posterSubmitScope, command.Token, command.RateSubject,
		r.store.selfService.SelfServiceSubmitRateLimit, now,
	); err != nil {
		return result, false, err
	}
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		poster, resolveErr := r.resolvePoster(ctx, q, command.Token, now)
		if resolveErr != nil {
			return resolveErr
		}
		return r.runSelfRegistration(ctx, q, command, poster, now, &result, &replayed)
	})
	return result, replayed, err
}

func (r *StayRepository) runSelfRegistration(
	ctx context.Context,
	q generated.Querier,
	command stay.SelfRegistrationCommand,
	poster resolvedPoster,
	now time.Time,
	result *stay.SelfRegistrationAccepted,
	replayed *bool,
) error {
	spec := selfRegistrationIdempotency(command, poster.inviteID, now)
	idempotent, err := r.store.runIdempotent(ctx, q, spec, func() (storedMutation, error) {
		return r.writeSelfRegistration(ctx, q, command, poster, now)
	})
	if err != nil {
		return stayMutationError(err)
	}
	*replayed = idempotent.replayed
	return decodeSelfRegistration(idempotent.response.body, result)
}

// spendChallenge is the step without which the proof of work is worth zero: one
// solution would otherwise be replayed for the whole TTL, and the abuser would
// pay for one submission and make a thousand. A conflict on the primary key is
// exactly the replay, and it answers like an invalid challenge so the endpoint
// is not an oracle (N-17).
func (r *StayRepository) spendChallenge(
	ctx context.Context,
	q generated.Querier,
	command stay.SelfRegistrationCommand,
	poster resolvedPoster,
	now time.Time,
) error {
	spend, err := r.challenges.Verify(
		command.ProofOfWork.Challenge, command.ProofOfWork.Solution,
		poster.inviteID, now,
	)
	if err != nil {
		return stay.ErrNotFound
	}
	written, err := q.SpendProofOfWorkChallenge(ctx, generated.SpendProofOfWorkChallengeParams{
		ChallengeHmac: spend.ChallengeHMAC, KeyVersion: spend.KeyVersion,
		ExpiresAt: timeToPG(spend.ExpiresAt),
	})
	if err != nil {
		return stay.ErrUnavailable
	}
	if written != 1 {
		return stay.ErrNotFound
	}
	return nil
}

func selfRegistrationIdempotency(
	command stay.SelfRegistrationCommand,
	inviteID uuid.UUID,
	now time.Time,
) idempotencySpec {
	return idempotencySpec{
		actorValue: inviteID.String(),
		operation:  idempotency.OperationSubmitSelfRegistration,
		resourceID: inviteID, key: command.IdempotencyKey,
		request: selfRegistrationRequestValue(command), now: now,
	}
}

// The hashed body carries only generalized values, because the channel accepts
// nothing else. The digest is keyed anyway: it is persisted twice and must not
// become a confirmation oracle over the data the rejection erases.
func selfRegistrationRequestValue(command stay.SelfRegistrationCommand) any {
	return struct {
		ClientSubmissionID   uuid.UUID                 `json:"client_submission_id"`
		PrivacyNoticeVersion string                    `json:"privacy_notice_version"`
		PlannedArrivalOn     string                    `json:"planned_arrival_on"`
		PlannedDepartureOn   string                    `json:"planned_departure_on"`
		Visitors             []stay.SelfServiceVisitor `json:"visitors"`
	}{
		command.ClientSubmissionID, command.PrivacyNoticeVersion,
		command.PlannedArrivalOn, command.PlannedDepartureOn, command.Visitors,
	}
}

// The challenge is spent inside the idempotent work, never before it. Spending
// first would break the legitimate retry: the client re-sends the same body with
// the same Idempotency-Key and the same solved challenge, and a spend outside
// the replay would answer not-found instead of replaying the stored response.
// Inside, a replay never reaches the spend, while the same challenge under a
// different key still does — which is exactly what N-17 asks for.
//
// A poster printed with an old privacy notice submitting against a new one is a
// 409, never a silent success. That is the realistic case, because the poster is
// physical and does not update itself when the notice changes.
func (r *StayRepository) writeSelfRegistration(
	ctx context.Context,
	q generated.Querier,
	command stay.SelfRegistrationCommand,
	poster resolvedPoster,
	now time.Time,
) (storedMutation, error) {
	if err := r.claimPoster(ctx, q, command, poster, now); err != nil {
		return storedMutation{}, err
	}
	accepted, err := r.insertSelfRegistration(ctx, q, command, poster, now)
	if err != nil {
		return storedMutation{}, err
	}
	if err := r.store.recordSelfRegistration(ctx, q, command, poster, accepted, now); err != nil {
		return storedMutation{}, err
	}
	return jsonMutation(200, accepted.StayID, selfRegistrationReplay(accepted), map[string]string{
		"ETag": entityTag(accepted.Version),
	})
}

// claimPoster is the serialization point of the open channel: the notice
// version, the toll and the use counter are settled before a single row of the
// submission exists.
func (r *StayRepository) claimPoster(
	ctx context.Context,
	q generated.Querier,
	command stay.SelfRegistrationCommand,
	poster resolvedPoster,
	now time.Time,
) error {
	if command.PrivacyNoticeVersion != poster.row.PrivacyNoticeVersion {
		return stay.ErrConflict
	}
	if err := r.spendChallenge(ctx, q, command, poster, now); err != nil {
		return err
	}
	_, err := q.ConsumeInvite(ctx, generated.ConsumeInviteParams{
		ConsumedAt: timeToPG(now), InviteID: idToPG(poster.inviteID),
		TokenHmac: poster.digest,
	})
	if err != nil {
		return consumedPosterError(err)
	}
	return nil
}

// selfRegistrationReplayPayload exists because the contract forbids `version`
// in the answer, so the API type hides it from the wire — and the idempotency
// record stores exactly what the wire would carry. Without a payload that keeps
// the version, the decode on replay returns zero and the ETag becomes "0".
type selfRegistrationReplayPayload struct {
	SubmissionID  uuid.UUID          `json:"submission_id"`
	StayID        uuid.UUID          `json:"stay_id"`
	Status        string             `json:"status"`
	StayStatus    stay.Status        `json:"stay_status"`
	ApprovalState stay.ApprovalState `json:"approval_state"`
	Version       int64              `json:"version"`
}

func selfRegistrationReplay(accepted stay.SelfRegistrationAccepted) any {
	return selfRegistrationReplayPayload{
		SubmissionID: accepted.SubmissionID, StayID: accepted.StayID,
		Status: accepted.Status, StayStatus: accepted.StayStatus,
		ApprovalState: accepted.ApprovalState, Version: accepted.Version,
	}
}

func decodeSelfRegistration(body []byte, target *stay.SelfRegistrationAccepted) error {
	var payload selfRegistrationReplayPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return stay.ErrUnavailable
	}
	*target = stay.SelfRegistrationAccepted{
		SubmissionID: payload.SubmissionID, StayID: payload.StayID,
		Status: payload.Status, StayStatus: payload.StayStatus,
		ApprovalState: payload.ApprovalState, Version: payload.Version,
	}
	return nil
}

func consumedPosterError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return stay.ErrInviteConsumed
	}
	return stay.ErrUnavailable
}

func (r *StayRepository) insertSelfRegistration(
	ctx context.Context,
	q generated.Querier,
	command stay.SelfRegistrationCommand,
	poster resolvedPoster,
	now time.Time,
) (stay.SelfRegistrationAccepted, error) {
	stayID, err := uuid.NewV7()
	if err != nil {
		return stay.SelfRegistrationAccepted{}, stay.ErrUnavailable
	}
	row, err := q.CreateSelfServiceStay(ctx, selfServiceStayParams(command, poster, stayID, now))
	if err != nil {
		return stay.SelfRegistrationAccepted{}, stayMutationError(err)
	}
	submissionID, err := r.insertSelfServiceSubmission(ctx, q, command, poster, stayID, now)
	if err != nil {
		return stay.SelfRegistrationAccepted{}, err
	}
	if err := insertSelfServiceVisitors(ctx, q, command, stayID); err != nil {
		return stay.SelfRegistrationAccepted{}, err
	}
	return stay.SelfRegistrationAccepted{
		SubmissionID: submissionID, StayID: idFromPG(row.ID), Status: "accepted",
		StayStatus:    stay.Status(row.Status),
		ApprovalState: stay.ApprovalStateFromColumn(row.ApprovalState),
		Version:       row.Version,
	}, nil
}

// expected_guest_count is len(visitors) by construction, which removes any
// divergence between what was declared and what was submitted, and satisfies
// stays_guest_count_valid without a second field to validate.
func selfServiceStayParams(
	command stay.SelfRegistrationCommand,
	poster resolvedPoster,
	stayID uuid.UUID,
	now time.Time,
) generated.CreateSelfServiceStayParams {
	return generated.CreateSelfServiceStayParams{
		StayID: idToPG(stayID), ClientSubmissionID: idToPG(command.ClientSubmissionID),
		PlannedArrivalOn:   dateToPG(command.PlannedArrivalOn),
		PlannedDepartureOn: dateToPG(command.PlannedDepartureOn),
		ExpectedGuestCount: int32(len(command.Visitors)),
		ApprovalExpiresAt:  timeToPG(now.Add(stay.PendingApprovalTTL)),
		InviteID:           idToPG(poster.inviteID), TokenHmac: poster.digest,
		Now: timeToPG(now),
	}
}

func (r *StayRepository) insertSelfServiceSubmission(
	ctx context.Context,
	q generated.Querier,
	command stay.SelfRegistrationCommand,
	poster resolvedPoster,
	stayID uuid.UUID,
	now time.Time,
) (uuid.UUID, error) {
	submissionID, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, stay.ErrUnavailable
	}
	hash, err := idempotency.RequestHash(r.hashKey, selfRegistrationRequestValue(command))
	if err != nil {
		return uuid.Nil, stay.ErrUnavailable
	}
	row, err := q.CreateSelfServiceGroupSubmission(
		ctx, generated.CreateSelfServiceGroupSubmissionParams{
			SubmissionID: idToPG(submissionID), StayID: idToPG(stayID),
			ClientSubmissionID: idToPG(command.ClientSubmissionID),
			RequestHash:        hash[:], SubmittedAt: timeToPG(now),
			InviteID: idToPG(poster.inviteID), TokenHmac: poster.digest,
		},
	)
	if err != nil {
		return uuid.Nil, stayMutationError(err)
	}
	return idFromPG(row.ID), nil
}

func insertSelfServiceVisitors(
	ctx context.Context,
	q generated.Querier,
	command stay.SelfRegistrationCommand,
	stayID uuid.UUID,
) error {
	for _, visitor := range command.Visitors {
		if err := insertSelfServiceVisitor(ctx, q, visitor, stayID); err != nil {
			return err
		}
	}
	return nil
}

func insertSelfServiceVisitor(
	ctx context.Context,
	q generated.Querier,
	visitor stay.SelfServiceVisitor,
	stayID uuid.UUID,
) error {
	visitorID, err := uuid.NewV7()
	if err != nil {
		return stay.ErrUnavailable
	}
	clientID, err := uuid.Parse(visitor.ClientID)
	if err != nil {
		return stay.ErrInvalidInput
	}
	_, err = q.InsertSelfServiceVisitor(ctx, generated.InsertSelfServiceVisitorParams{
		VisitorID: idToPG(visitorID), StayID: idToPG(stayID), ClientID: idToPG(clientID),
		VisitorRole: generated.CoreVisitorRole(visitor.Role),
		AgeBand:     string(visitor.AgeBand), ResidenceCountry: visitor.ResidenceCountry,
		ResidenceState:    optionalText(visitor.ResidenceState),
		ResidenceCityCode: optionalText(visitor.ResidenceCityCode),
	})
	if err != nil {
		return stayMutationError(err)
	}
	return nil
}

func optionalText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// The audit actor is the poster, not a person: nobody identified submitted
// this. Fabricating a membership would destroy the distinction between "an
// operator created it" and "nobody created it".
//
// No presence event is emitted here. A pending stay accrues nothing, and asking
// the worker to recompute presence for it would be work whose only correct
// result is the empty set. The approval emits it instead.
func (s *Store) recordSelfRegistration(
	ctx context.Context,
	q generated.Querier,
	command stay.SelfRegistrationCommand,
	poster resolvedPoster,
	accepted stay.SelfRegistrationAccepted,
	now time.Time,
) error {
	return s.recordEvents(ctx, q, eventSpec{
		actorType: audit.ActorInvite, actorIssuer: "urn:cumuru:invite",
		actorSubject: poster.inviteID.String(),
		organization: idFromPG(poster.row.OrganizationID),
		action:       audit.ActionStayGroupSubmitted, entityType: audit.EntityStay,
		entityID: accepted.StayID, requestID: command.RequestID,
		changedFields: []audit.ChangedField{audit.FieldStatus},
		version:       accepted.Version, aggregateType: outbox.AggregateStay,
		eventType: outbox.EventStayGroupSubmitted, now: now,
	})
}

// countedRateLimit returns the counter as well as the verdict, because the
// counter is what the adaptive difficulty is derived from. Tightening the
// bucket instead would deny service to guests behind CGNAT, where one /24 can
// cover thousands of subscribers (T-06).
func (r *StayRepository) countedRateLimit(
	ctx context.Context,
	scope string,
	token string,
	subject string,
	limit int,
	now time.Time,
) (int32, error) {
	key, ok := r.store.core.RateLimitKeys.Key(r.store.core.RateLimitKeys.CurrentVersion)
	if !ok {
		return 0, stay.ErrUnavailable
	}
	window := now.Truncate(r.store.core.RateLimitWindow)
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	row, err := r.store.queries.IncrementRateLimit(ctx, generated.IncrementRateLimitParams{
		Scope: scope, SubjectHmac: rateLimitDigest(key, scope, token, subject),
		SubjectKeyVersion: r.store.core.RateLimitKeys.CurrentVersion,
		WindowStartedAt:   timeToPG(window),
		ExpiresAt:         timeToPG(window.Add(2 * r.store.core.RateLimitWindow)),
	})
	if err != nil {
		return 0, stay.ErrUnavailable
	}
	if row.RequestCount > int32(limit) {
		return row.RequestCount, stay.ErrRateLimited
	}
	return row.RequestCount, nil
}
