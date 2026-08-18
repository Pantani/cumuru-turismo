package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/accessrequest"
	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/audit"
	"github.com/Pantani/cumuru/apps/api/internal/platform/idempotency"
	"github.com/Pantani/cumuru/apps/api/internal/platform/outbox"
	"github.com/Pantani/cumuru/apps/api/internal/platform/proofofwork"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	accessRequestContextScope = "accommodation_access_request_context"
	accessRequestSubmitScope  = "accommodation_access_request_submit"
)

// accessRequestChallengeScope is the UUID this route's proof of work binds to.
// The issuer ties every challenge to an invite identifier, and here there is no
// prior invite: whoever asks does not have an account yet. The value is fixed
// and derived from the route rather than from whoever arrives, so it identifies
// nobody — all it does is stop a challenge minted for a pousada's wall poster
// from being spent on this form, and the other way round.
var accessRequestChallengeScope = uuid.MustParse("019f0000-0000-7000-8000-00000000a042")

// accessRequestAuditActor names the process, not a person. Nobody identified
// submitted the request: the channel is open and the declared contact is not
// verified. Fabricating an actor would destroy the distinction between
// "somebody on the inside registered it" and "it arrived from outside with no
// proof" — the same reason ADR-040 refused a system membership for the stay
// with no author.
const accessRequestAuditActor = "access-request-channel"

type AccessRequestRepository struct {
	store      *Store
	challenges *proofofwork.Issuer
}

// NewAccessRequestRepository fails closed like the poster's does: with the
// feature on and no usable key the repository refuses to exist, because a
// challenge signed with an empty key is a MAC anybody can forge.
func NewAccessRequestRepository(store *Store) (*AccessRequestRepository, error) {
	challenges, err := newChallengeIssuer(store.selfService)
	if err != nil {
		return nil, err
	}
	return &AccessRequestRepository{store: store, challenges: challenges}, nil
}

var _ accessrequest.Repository = (*AccessRequestRepository)(nil)

// Context issues the challenge on the route that already owns a rate limit
// bucket, because that bucket counter is the source of the adaptive difficulty.
// A separate issuing route would add a second bucket to protect while
// protecting nothing further.
func (r *AccessRequestRepository) Context(
	ctx context.Context,
	request accessrequest.ContextRequest,
) (accessrequest.Context, error) {
	if r.challenges == nil {
		return accessrequest.Context{}, accessrequest.ErrNotFound
	}
	now := r.store.currentTime()
	count, err := r.countedRateLimit(
		ctx, accessRequestContextScope, request.RateSubject,
		r.store.selfService.SelfServiceContextRateLimit, now,
	)
	if err != nil {
		return accessrequest.Context{}, err
	}
	settings := r.store.selfService
	difficulty := proofofwork.Difficulty(
		settings.DifficultyBase, settings.DifficultyCeiling,
		settings.DifficultyRequestsPerBit, count,
	)
	challenge, err := r.challenges.Issue(accessRequestChallengeScope, difficulty, now)
	if err != nil {
		return accessrequest.Context{}, accessrequest.ErrUnavailable
	}
	return accessRequestContext(challenge), nil
}

func accessRequestContext(challenge proofofwork.Challenge) accessrequest.Context {
	return accessrequest.Context{
		ProofOfWork: accessrequest.ProofOfWorkChallenge{
			Algorithm: challenge.Algorithm, Challenge: challenge.Value,
			DifficultyBits: int(challenge.DifficultyBits),
			ExpiresAt:      challenge.ExpiresAt,
		},
		PrivacyNoticeVersion: accessrequest.PrivacyNoticeVersion,
	}
}

func (r *AccessRequestRepository) Create(
	ctx context.Context,
	command accessrequest.CreateCommand,
) (created accessrequest.Created, replayed bool, err error) {
	if r.challenges == nil {
		return created, false, accessrequest.ErrNotFound
	}
	now := r.store.currentTime()
	if _, err = r.countedRateLimit(
		ctx, accessRequestSubmitScope, command.RateSubject,
		r.store.selfService.SelfServiceSubmitRateLimit, now,
	); err != nil {
		return created, false, err
	}
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		idempotent, runErr := r.store.runIdempotent(
			ctx, q, createAccessRequestIdempotency(command, now),
			func() (storedMutation, error) {
				return r.writeAccessRequest(ctx, q, command, now)
			},
		)
		if runErr != nil {
			return accessRequestMutationError(runErr)
		}
		replayed = idempotent.replayed
		return decodeAccessRequest(idempotent.response.body, &created)
	})
	return created, replayed, err
}

// The idempotency actor is the identifier the client generated: there is no
// authenticated principal, and using the network prefix would put two people
// behind the same CGNAT into one replay window.
func createAccessRequestIdempotency(
	command accessrequest.CreateCommand,
	now time.Time,
) idempotencySpec {
	return idempotencySpec{
		actorValue: command.ClientSubmissionID.String(),
		operation:  idempotency.OperationCreateAccessRequest,
		resourceID: command.ClientSubmissionID, key: command.IdempotencyKey,
		request: accessRequestRequestValue(command), now: now,
	}
}

// The body digest is keyed because it is persisted and outlives the erasure of
// the contact: without a key it would be possible to confirm by trial which
// e-mail asked for access after the rejection deleted the address.
func accessRequestRequestValue(command accessrequest.CreateCommand) any {
	return struct {
		ClientSubmissionID uuid.UUID `json:"client_submission_id"`
		AccommodationName  string    `json:"accommodation_name"`
		Category           string    `json:"category"`
		Capacity           int32     `json:"capacity"`
		ContactName        string    `json:"contact_name"`
		ContactEmail       string    `json:"contact_email"`
		ContactPhone       string    `json:"contact_phone"`
		CityLabel          string    `json:"city_label"`
		StateCode          string    `json:"state_code"`
	}{
		command.ClientSubmissionID, command.AccommodationName,
		string(command.Category), command.Capacity, command.ContactName,
		command.ContactEmail, command.ContactPhone, command.CityLabel,
		command.StateCode,
	}
}

// The challenge is spent inside the idempotent work, never before it. Spending
// outside would break the legitimate retry: the client re-sends the same body
// with the same key and the same solved challenge, and an external spend would
// answer a refusal instead of replaying the stored response. Inside, a replay
// never reaches the spend, while the same challenge under a different key still
// does.
func (r *AccessRequestRepository) writeAccessRequest(
	ctx context.Context,
	q generated.Querier,
	command accessrequest.CreateCommand,
	now time.Time,
) (storedMutation, error) {
	if command.PrivacyNoticeVersion != accessrequest.PrivacyNoticeVersion {
		return storedMutation{}, accessrequest.ErrConflict
	}
	if err := r.spendChallenge(ctx, q, command.ProofOfWork, now); err != nil {
		return storedMutation{}, err
	}
	created, err := r.insertAccessRequest(ctx, q, command, now)
	if err != nil {
		return storedMutation{}, err
	}
	if err := r.store.recordAccessRequestEvent(ctx, q, accessRequestEvent{
		requestID: created.ID, correlationID: command.RequestID,
		action: audit.ActionAccessRequestCreated,
		event:  outbox.EventAccessRequestCreated, version: 1, now: now,
	}); err != nil {
		return storedMutation{}, err
	}
	return accessRequestMutation(201, created.ID, created, nil)
}

// A collision on the partial unique index of pending e-mails is a business
// answer, not a technical failure: whoever re-sends because they got no reply
// receives a 409 rather than a second row in the queue.
func (r *AccessRequestRepository) insertAccessRequest(
	ctx context.Context,
	q generated.Querier,
	command accessrequest.CreateCommand,
	now time.Time,
) (accessrequest.Created, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return accessrequest.Created{}, accessrequest.ErrUnavailable
	}
	row, err := q.CreateAccommodationAccessRequest(
		ctx, createAccessRequestParams(command, id, now),
	)
	if err != nil {
		return accessrequest.Created{}, accessRequestMutationError(err)
	}
	return accessrequest.Created{
		ID: idFromPG(row.ID), CreatedAt: row.CreatedAt.Time.UTC(),
	}, nil
}

func createAccessRequestParams(
	command accessrequest.CreateCommand,
	id uuid.UUID,
	now time.Time,
) generated.CreateAccommodationAccessRequestParams {
	name := command.ContactName
	email := command.ContactEmail
	return generated.CreateAccommodationAccessRequestParams{
		RequestID: idToPG(id), AccommodationName: command.AccommodationName,
		Category: string(command.Category), Capacity: command.Capacity,
		ContactName: &name, ContactEmail: &email,
		ContactPhone: optionalText(command.ContactPhone),
		CityLabel:    command.CityLabel, StateCode: command.StateCode,
		ExpiresAt: timeToPG(now.Add(accessrequest.PendingTTL)), Now: timeToPG(now),
	}
}

// A conflict on the primary key of the nonce book is exactly the replay, and it
// answers like an invalid challenge so the route does not become an oracle.
func (r *AccessRequestRepository) spendChallenge(
	ctx context.Context,
	q generated.Querier,
	answer accessrequest.ProofOfWorkAnswer,
	now time.Time,
) error {
	spend, err := r.challenges.Verify(
		answer.Challenge, answer.Solution, accessRequestChallengeScope, now,
	)
	if err != nil {
		return accessrequest.ErrInvalidInput
	}
	written, err := q.SpendProofOfWorkChallenge(ctx, generated.SpendProofOfWorkChallengeParams{
		ChallengeHmac: spend.ChallengeHMAC, KeyVersion: spend.KeyVersion,
		ExpiresAt: timeToPG(spend.ExpiresAt),
	})
	if err != nil {
		return accessrequest.ErrUnavailable
	}
	if written != 1 {
		return accessrequest.ErrInvalidInput
	}
	return nil
}

func (r *AccessRequestRepository) List(
	ctx context.Context,
	page accessrequest.PageRequest,
) (accessrequest.Page, error) {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	rows, err := r.store.queries.ListAccommodationAccessRequests(
		ctx, listAccessRequestParams(page),
	)
	if err != nil {
		return accessrequest.Page{}, accessrequest.ErrUnavailable
	}
	return accessRequestPage(rows, page.Limit), nil
}

func listAccessRequestParams(
	page accessrequest.PageRequest,
) generated.ListAccommodationAccessRequestsParams {
	var state *string
	if page.State != "" {
		value := string(page.State)
		state = &value
	}
	return generated.ListAccommodationAccessRequestsParams{
		ApprovalState: state, CursorCreatedAt: optionalTime(page.CursorCreatedAt),
		CursorID: pgUUID(page.CursorID), PageLimit: page.Limit + 1,
	}
}

// The page asks for one item beyond the limit: the existence of that item is
// what tells whether a next page exists, without a separate count.
func accessRequestPage(
	rows []generated.ListAccommodationAccessRequestsRow,
	limit int32,
) accessrequest.Page {
	items := make([]accessrequest.Request, 0, len(rows))
	for _, row := range rows {
		// The bound is read off len(items) and demands a positive limit: the
		// service refuses anything outside 1..100, but this helper is called
		// directly by its own test and cannot see that refusal, and a limit of
		// zero would otherwise match on the first row and read items[-1].
		if limit > 0 && len(items) == int(limit) {
			last := items[len(items)-1]
			return accessrequest.Page{
				Items: items,
				NextCursor: &accessrequest.PageCursor{
					CreatedAt: last.CreatedAt, ID: last.ID,
				},
			}
		}
		items = append(items, accessRequestFromRow(row))
	}
	return accessrequest.Page{Items: items}
}

func (r *AccessRequestRepository) Approve(
	ctx context.Context,
	command accessrequest.ApprovalCommand,
) (result accessrequest.Request, replayed bool, err error) {
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		if lockErr := r.acquireOnboardingLock(ctx, q, command); lockErr != nil {
			return lockErr
		}
		spec := decisionAccessRequestIdempotency(
			command.Actor, command.AccessRequestID, command.ExpectedVersion,
			command.IdempotencyKey, idempotency.OperationApproveAccessRequest, "", now,
		)
		idempotent, runErr := r.store.runIdempotent(ctx, q, spec, func() (storedMutation, error) {
			return r.approveAccessRequest(ctx, q, command, now)
		})
		if runErr != nil {
			return accessRequestMutationError(runErr)
		}
		replayed = idempotent.replayed
		return decodeAccessRequest(idempotent.response.body, &result)
	})
	return result, replayed, err
}

// The onboarding lock is the same one POST /accommodations takes, because the
// approval goes down the same path: without it, two concurrent approvals by the
// same administrator would create two organizations for them.
func (r *AccessRequestRepository) acquireOnboardingLock(
	ctx context.Context,
	q generated.Querier,
	command accessrequest.ApprovalCommand,
) error {
	lockKey, err := r.store.accommodationOnboardingLockKey(command.Actor)
	if err != nil {
		return accessrequest.ErrUnavailable
	}
	if err := q.AcquireAccommodationOnboardingLock(ctx, lockKey); err != nil {
		return accessrequest.ErrUnavailable
	}
	return nil
}

func (r *AccessRequestRepository) Reject(
	ctx context.Context,
	command accessrequest.RejectionCommand,
) (result accessrequest.Request, replayed bool, err error) {
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		spec := decisionAccessRequestIdempotency(
			command.Actor, command.AccessRequestID, command.ExpectedVersion,
			command.IdempotencyKey, idempotency.OperationRejectAccessRequest,
			string(command.ReasonCode), now,
		)
		idempotent, runErr := r.store.runIdempotent(ctx, q, spec, func() (storedMutation, error) {
			return r.rejectAccessRequest(ctx, q, command, now)
		})
		if runErr != nil {
			return accessRequestMutationError(runErr)
		}
		replayed = idempotent.replayed
		return decodeAccessRequest(idempotent.response.body, &result)
	})
	return result, replayed, err
}

// The reason code is part of the hashed body, so the same key with a different
// reason is a conflict rather than a silent overwrite of the audited fact.
func decisionAccessRequestIdempotency(
	actor access.Principal,
	requestID uuid.UUID,
	expectedVersion int64,
	key string,
	operation idempotency.Operation,
	reasonCode string,
	now time.Time,
) idempotencySpec {
	return idempotencySpec{
		actorValue: actorValue(actor.Issuer, actor.Subject),
		operation:  operation, resourceID: requestID, key: key,
		request: struct {
			RequestID       uuid.UUID `json:"request_id"`
			ExpectedVersion int64     `json:"expected_version"`
			ReasonCode      string    `json:"reason_code"`
		}{requestID, expectedVersion, reasonCode},
		now: now,
	}
}

// The approval creates the accommodation and writes the state in the same
// transaction, which is why an approved request cannot exist without a matching
// record: the decision constraint refuses 'approved' with a null
// accommodation_id, and an aborted transaction undoes both writes together.
func (r *AccessRequestRepository) approveAccessRequest(
	ctx context.Context,
	q generated.Querier,
	command accessrequest.ApprovalCommand,
	now time.Time,
) (storedMutation, error) {
	locked, err := lockPendingAccessRequest(
		ctx, q, command.AccessRequestID, command.ExpectedVersion,
	)
	if err != nil {
		return storedMutation{}, err
	}
	created, err := r.createApprovedAccommodation(ctx, q, command, locked)
	if err != nil {
		return storedMutation{}, err
	}
	row, err := q.ApproveAccommodationAccessRequest(
		ctx, approveAccessRequestParams(command, created.ID, now),
	)
	if err != nil {
		return storedMutation{}, accessRequestDecisionError(err)
	}
	result := accessRequestFromRow(generated.ListAccommodationAccessRequestsRow(row))
	return r.finishDecision(ctx, q, command.RequestID, result, accessRequestEvent{
		action: audit.ActionAccessRequestApproved,
		event:  outbox.EventAccessRequestApproved, now: now,
	})
}

func approveAccessRequestParams(
	command accessrequest.ApprovalCommand,
	accommodationID uuid.UUID,
	now time.Time,
) generated.ApproveAccommodationAccessRequestParams {
	issuer := command.Actor.Issuer
	subject := command.Actor.Subject
	return generated.ApproveAccommodationAccessRequestParams{
		AccommodationID: idToPG(accommodationID), DecidedAt: timeToPG(now),
		OidcIssuer: &issuer, OidcSubject: &subject,
		RequestID:       idToPG(command.AccessRequestID),
		ExpectedVersion: command.ExpectedVersion,
	}
}

// The accommodation is born through the same path as POST /accommodations, with
// the same organization scope and the same 'active' status. This is not code
// economy: if the approval created a record in any other status, the
// administration would approve the request and then be unable to issue the
// activation, because issue_activation is only permitted in 'active'. The
// submission identifier is the request's own id, already a UUIDv7, so the
// record carries back which request originated it.
func (r *AccessRequestRepository) createApprovedAccommodation(
	ctx context.Context,
	q generated.Querier,
	command accessrequest.ApprovalCommand,
	locked generated.LockAccommodationAccessRequestForDecisionRow,
) (accommodation.Accommodation, error) {
	onboarding := accommodation.CreateCommand{
		Actor: command.Actor, Name: locked.AccommodationName,
		Category: accommodation.Category(locked.Category), Capacity: locked.Capacity,
		ClientSubmissionID: command.AccessRequestID,
		IdempotencyKey:     command.IdempotencyKey, RequestID: command.RequestID,
	}
	scope, ids, err := prepareOnboarding(ctx, q, onboarding)
	if err != nil {
		return accommodation.Accommodation{}, accessRequestOnboardingError(err)
	}
	row, err := insertOnboardingRecords(ctx, q, scope, ids, onboarding)
	if err != nil {
		return accommodation.Accommodation{}, accessRequestOnboardingError(err)
	}
	created := accommodationFromOnboarding(row)
	if err := r.store.recordAccommodationCreate(ctx, q, onboarding, created); err != nil {
		return accommodation.Accommodation{}, accessRequestOnboardingError(err)
	}
	return created, nil
}

// The rejection erases the contact in the same transaction that stamps the
// decision, and the erasure lives in the SQL itself: the decision constraint
// refuses a 'rejected' row still carrying contact details, so the purge cannot
// be forgotten by some path through the application.
func (r *AccessRequestRepository) rejectAccessRequest(
	ctx context.Context,
	q generated.Querier,
	command accessrequest.RejectionCommand,
	now time.Time,
) (storedMutation, error) {
	if _, err := lockPendingAccessRequest(
		ctx, q, command.AccessRequestID, command.ExpectedVersion,
	); err != nil {
		return storedMutation{}, err
	}
	reason := string(command.ReasonCode)
	issuer := command.Actor.Issuer
	subject := command.Actor.Subject
	row, err := q.RejectAccommodationAccessRequest(
		ctx, generated.RejectAccommodationAccessRequestParams{
			ReasonCode: &reason, DecidedAt: timeToPG(now),
			OidcIssuer: &issuer, OidcSubject: &subject,
			RequestID:       idToPG(command.AccessRequestID),
			ExpectedVersion: command.ExpectedVersion,
		},
	)
	if err != nil {
		return storedMutation{}, accessRequestDecisionError(err)
	}
	result := accessRequestFromRow(generated.ListAccommodationAccessRequestsRow(row))
	return r.finishDecision(ctx, q, command.RequestID, result, accessRequestEvent{
		action: audit.ActionAccessRequestRejected,
		event:  outbox.EventAccessRequestRejected, now: now,
	})
}

func (r *AccessRequestRepository) finishDecision(
	ctx context.Context,
	q generated.Querier,
	correlationID string,
	result accessrequest.Request,
	spec accessRequestEvent,
) (storedMutation, error) {
	spec.requestID = result.ID
	spec.correlationID = correlationID
	spec.version = result.Version
	if err := r.store.recordAccessRequestEvent(ctx, q, spec); err != nil {
		return storedMutation{}, err
	}
	return accessRequestMutation(200, result.ID, result, map[string]string{
		"ETag": entityTag(result.Version),
	})
}

// lockPendingAccessRequest separates three answers the happy path conflates: an
// absent request is a 404, an already decided one is a conflict — approving a
// rejected request and rejecting an approved one fail alike — and a diverging
// version is a failed precondition.
func lockPendingAccessRequest(
	ctx context.Context,
	q generated.Querier,
	id uuid.UUID,
	expectedVersion int64,
) (generated.LockAccommodationAccessRequestForDecisionRow, error) {
	locked, err := q.LockAccommodationAccessRequestForDecision(ctx, idToPG(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return locked, accessrequest.ErrNotFound
	}
	if err != nil {
		return locked, accessrequest.ErrUnavailable
	}
	if accessrequest.ApprovalState(locked.ApprovalState) != accessrequest.StatePending {
		return locked, accessrequest.ErrConflict
	}
	if locked.Version != expectedVersion {
		return locked, accessrequest.ErrPreconditionFailed
	}
	return locked, nil
}

// Zero rows after the lock means the row moved between the read and the write,
// which is a conflict and never a 500.
func accessRequestDecisionError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return accessrequest.ErrConflict
	}
	return accessRequestMutationError(err)
}

var knownAccessRequestErrors = []error{
	accessrequest.ErrNotFound, accessrequest.ErrConflict,
	accessrequest.ErrPreconditionFailed, accessrequest.ErrForbidden,
	accessrequest.ErrInvalidInput, accessrequest.ErrRateLimited,
}

func accessRequestMutationError(err error) error {
	if errors.Is(err, idempotency.ErrProcessing) {
		return err
	}
	if errors.Is(err, errIdempotencyConflict) || isUniqueViolation(err) {
		return accessrequest.ErrConflict
	}
	if known, ok := firstKnownError(err, knownAccessRequestErrors); ok {
		return known
	}
	return accessrequest.ErrUnavailable
}

// The onboarding path speaks accommodation's error vocabulary. The translation
// preserves what the administrator needs to tell apart — already belonging to
// more than one organization, or not being a manager of the one they have — and
// erases the rest.
func accessRequestOnboardingError(err error) error {
	switch {
	case errors.Is(err, accommodation.ErrForbidden):
		return accessrequest.ErrForbidden
	case errors.Is(err, accommodation.ErrConflict), isUniqueViolation(err):
		return accessrequest.ErrConflict
	default:
		return accessrequest.ErrUnavailable
	}
}

type accessRequestEvent struct {
	requestID     uuid.UUID
	correlationID string
	action        audit.Action
	event         outbox.EventType
	version       int64
	now           time.Time
}

// The audit event carries no organization because the request exists before any
// organization does, and a rejected one will never have one. It carries no
// reason text either: the reason is a closed code, and platform.audit_events is
// append-only.
func (s *Store) recordAccessRequestEvent(
	ctx context.Context,
	q generated.Querier,
	spec accessRequestEvent,
) error {
	return s.recordEvents(ctx, q, eventSpec{
		actorType: audit.ActorSystem, actorIssuer: systemActorIssuer,
		actorSubject: accessRequestAuditActor, action: spec.action,
		entityType: audit.EntityAccessRequest, entityID: spec.requestID,
		requestID:     spec.correlationID,
		changedFields: []audit.ChangedField{audit.FieldStatus},
		version:       spec.version, aggregateType: outbox.AggregateAccessRequest,
		eventType: spec.event, now: spec.now,
	})
}

// accessRequestFromRow takes the listing type, and the approval and rejection
// types are converted into it. The conversion only compiles while the three
// generated rows stay field-for-field identical, which is the point: the day one
// of them diverges, this stops compiling instead of mapping the wrong field.
func accessRequestFromRow(
	row generated.ListAccommodationAccessRequestsRow,
) accessrequest.Request {
	return accessrequest.Request{
		ID: idFromPG(row.ID), AccommodationName: row.AccommodationName,
		Category: accommodation.Category(row.Category), Capacity: row.Capacity,
		ContactName: row.ContactName, ContactEmail: row.ContactEmail,
		ContactPhone: row.ContactPhone, CityLabel: row.CityLabel,
		StateCode:           row.StateCode,
		ApprovalState:       accessrequest.ApprovalState(row.ApprovalState),
		ExpiresAt:           row.ExpiresAt.Time.UTC(),
		AccommodationID:     optionalID(row.AccommodationID),
		RejectionReasonCode: rejectionReason(row.RejectionReasonCode),
		Version:             row.Version, CreatedAt: row.CreatedAt.Time.UTC(),
		UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func optionalID(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	id := uuid.UUID(value.Bytes)
	return &id
}

func rejectionReason(value *string) *accessrequest.RejectionReason {
	if value == nil {
		return nil
	}
	reason := accessrequest.RejectionReason(*value)
	return &reason
}

func accessRequestMutation(
	statusCode int,
	resourceID uuid.UUID,
	value any,
	headers map[string]string,
) (storedMutation, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return storedMutation{}, accessrequest.ErrUnavailable
	}
	return storedMutation{
		status: statusCode, resourceID: resourceID, body: body, headers: headers,
	}, nil
}

func decodeAccessRequest(body []byte, target any) error {
	if err := json.Unmarshal(body, target); err != nil {
		return accessrequest.ErrUnavailable
	}
	return nil
}

// The bucket subject is only the network prefix: unlike the poster, this
// surface has no capability to compose the key with. It stays a prefix rather
// than an address because the pousada Wi-Fi and carrier CGNAT put many people
// behind the same /24.
func (r *AccessRequestRepository) countedRateLimit(
	ctx context.Context,
	scope string,
	subject string,
	limit int,
	now time.Time,
) (int32, error) {
	key, ok := r.store.core.RateLimitKeys.Key(r.store.core.RateLimitKeys.CurrentVersion)
	if !ok {
		return 0, accessrequest.ErrUnavailable
	}
	window := now.Truncate(r.store.core.RateLimitWindow)
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	row, err := r.store.queries.IncrementRateLimit(ctx, generated.IncrementRateLimitParams{
		Scope: scope, SubjectHmac: keyedDigest(key, "rate-limit:"+scope, subject),
		SubjectKeyVersion: r.store.core.RateLimitKeys.CurrentVersion,
		WindowStartedAt:   timeToPG(window),
		ExpiresAt:         timeToPG(window.Add(2 * r.store.core.RateLimitWindow)),
	})
	if err != nil {
		return 0, accessrequest.ErrUnavailable
	}
	if int64(row.RequestCount) > int64(limit) {
		return row.RequestCount, accessrequest.ErrRateLimited
	}
	return row.RequestCount, nil
}
