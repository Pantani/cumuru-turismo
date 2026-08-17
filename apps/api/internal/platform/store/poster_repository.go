package store

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/audit"
	"github.com/Pantani/cumuru/apps/api/internal/platform/idempotency"
	"github.com/Pantani/cumuru/apps/api/internal/platform/outbox"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
)

// posterReplayPayload is what the idempotency record stores. It holds the
// identifier and the key version, never the token: the URL is rebuilt from the
// keyring on replay, exactly as the Fase 2 invite does, so a leaked idempotency
// table hands nobody a working poster.
type posterReplayPayload struct {
	InviteID   uuid.UUID `json:"invite_id"`
	KeyVersion string    `json:"key_version"`
	ExpiresAt  time.Time `json:"expires_at"`
	MaxUses    *int32    `json:"max_uses"`
	UseCount   int32     `json:"use_count"`
	Version    int64     `json:"version"`
}

// accommodationGuard resolves the accommodation the command targets and refuses
// everything the caller may not do: an unknown or foreign accommodation is
// not-found, a stale If-Match is 412, a status that forbids the operation is a
// conflict, and a non-manager is forbidden.
func accommodationGuard(
	ctx context.Context,
	q generated.Querier,
	actor access.Principal,
	accommodationID uuid.UUID,
	expectedVersion int64,
	operation accommodation.Operation,
) (generated.GetAccessibleAccommodationRow, error) {
	row, err := q.GetAccessibleAccommodation(ctx, generated.GetAccessibleAccommodationParams{
		AccommodationID: idToPG(accommodationID),
		OidcIssuer:      actor.Issuer, OidcSubject: actor.Subject,
	})
	if err != nil {
		return row, stayQueryError(err)
	}
	if expectedVersion > 0 && row.Version != expectedVersion {
		return row, stay.ErrPreconditionFailed
	}
	return row, allowedAccommodationCommand(row, operation)
}

func allowedAccommodationCommand(
	row generated.GetAccessibleAccommodationRow,
	operation accommodation.Operation,
) error {
	if !accommodation.Status(row.Status).Allows(operation) {
		return stay.ErrConflict
	}
	if accommodation.Role(row.ActorRole) != accommodation.RoleManager {
		return stay.ErrForbidden
	}
	return nil
}

func (r *StayRepository) CreateAccommodationInvite(
	ctx context.Context,
	command stay.AccommodationInviteCommand,
) (result stay.AccommodationInviteCreated, replayed bool, err error) {
	if !r.store.phase7.Enabled {
		return result, false, stay.ErrNotFound
	}
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		spec := posterIdempotency(command, now)
		idempotent, runErr := r.store.runIdempotent(ctx, q, spec, func() (storedMutation, error) {
			return r.rotatePoster(ctx, q, command, now)
		})
		if runErr != nil {
			return stayMutationError(runErr)
		}
		result, runErr = r.decodePoster(idempotent.response.body)
		replayed = idempotent.replayed
		return runErr
	})
	return result, replayed, err
}

func posterIdempotency(
	command stay.AccommodationInviteCommand,
	now time.Time,
) idempotencySpec {
	return idempotencySpec{
		actorValue: actorValue(command.Actor.Issuer, command.Actor.Subject),
		operation:  idempotency.OperationCreateAccommodationInvite,
		resourceID: command.AccommodationID, key: command.IdempotencyKey,
		request: struct {
			AccommodationID      uuid.UUID `json:"accommodation_id"`
			PrivacyNoticeVersion string    `json:"privacy_notice_version"`
			MaxUses              *int32    `json:"max_uses"`
			ExpectedVersion      int64     `json:"expected_version"`
		}{
			command.AccommodationID, command.PrivacyNoticeVersion,
			command.MaxUses, command.ExpectedVersion,
		},
		now: now,
	}
}

// rotatePoster revokes the active poster and mints a new invite_id in the same
// transaction. Bumping the key version on the same row would leave the old
// poster working for as long as the historical key stayed in the ring, because
// the verifier reads the version from the stored row — the rotation would be
// silently ineffective (T-05, N-14).
func (r *StayRepository) rotatePoster(
	ctx context.Context,
	q generated.Querier,
	command stay.AccommodationInviteCommand,
	now time.Time,
) (storedMutation, error) {
	target, err := accommodationGuard(
		ctx, q, command.Actor, command.AccommodationID,
		command.ExpectedVersion, accommodation.OperationIssueInvite,
	)
	if err != nil {
		return storedMutation{}, err
	}
	payload, err := r.replacePoster(ctx, q, command, now)
	if err != nil {
		return storedMutation{}, err
	}
	if err := r.store.recordPosterEvent(ctx, q, posterEvent{
		actor: command.Actor, organizationID: idFromPG(target.OrganizationID),
		accommodationID: command.AccommodationID, inviteID: payload.InviteID,
		requestID: command.RequestID, action: audit.ActionAccommodationInvited,
		event: outbox.EventAccommodationInvited, now: now,
	}); err != nil {
		return storedMutation{}, err
	}
	return jsonMutation(201, payload.InviteID, payload, map[string]string{
		"Location": "/api/v1/accommodations/" + command.AccommodationID.String() + "/invite",
		"ETag":     entityTag(payload.Version),
	})
}

func (r *StayRepository) replacePoster(
	ctx context.Context,
	q generated.Querier,
	command stay.AccommodationInviteCommand,
	now time.Time,
) (posterReplayPayload, error) {
	if _, err := q.RevokeActiveAccommodationInvites(
		ctx, revokePosterParams(command.Actor, command.AccommodationID, now),
	); err != nil {
		return posterReplayPayload{}, stay.ErrUnavailable
	}
	return r.insertPoster(ctx, q, command, now)
}

func revokePosterParams(
	actor access.Principal,
	accommodationID uuid.UUID,
	now time.Time,
) generated.RevokeActiveAccommodationInvitesParams {
	return generated.RevokeActiveAccommodationInvitesParams{
		RevokedAt: timeToPG(now), AccommodationID: idToPG(accommodationID),
		OidcIssuer: actor.Issuer, OidcSubject: actor.Subject,
	}
}

func (r *StayRepository) insertPoster(
	ctx context.Context,
	q generated.Querier,
	command stay.AccommodationInviteCommand,
	now time.Time,
) (posterReplayPayload, error) {
	inviteID, err := uuid.NewV7()
	if err != nil {
		return posterReplayPayload{}, stay.ErrUnavailable
	}
	digest, keyVersion, err := r.posterDigest(inviteID)
	if err != nil {
		return posterReplayPayload{}, err
	}
	row, err := q.CreateAccommodationInvite(ctx, posterParams(
		command, inviteID, digest, keyVersion, now.Add(r.store.phase2.InviteTTL),
	))
	if err != nil {
		return posterReplayPayload{}, stayMutationError(err)
	}
	return posterReplayPayload{
		InviteID: idFromPG(row.ID), KeyVersion: keyVersion,
		ExpiresAt: row.ExpiresAt.Time.UTC(), MaxUses: row.MaxUses,
		UseCount: row.UseCount, Version: 1,
	}, nil
}

func (r *StayRepository) posterDigest(inviteID uuid.UUID) ([]byte, string, error) {
	token, keyVersion, err := r.codec.Issue(
		stay.PurposeAccommodationSelfRegistration, inviteID,
	)
	if err != nil {
		return nil, "", stay.ErrUnavailable
	}
	digest, err := r.codec.StorageDigest(token, keyVersion)
	if err != nil {
		return nil, "", stay.ErrUnavailable
	}
	return digest, keyVersion, nil
}

func posterParams(
	command stay.AccommodationInviteCommand,
	inviteID uuid.UUID,
	digest []byte,
	keyVersion string,
	expiresAt time.Time,
) generated.CreateAccommodationInviteParams {
	return generated.CreateAccommodationInviteParams{
		InviteID: idToPG(inviteID), TokenHmac: digest, TokenKeyVersion: keyVersion,
		PrivacyNoticeVersion: command.PrivacyNoticeVersion,
		ExpiresAt:            timeToPG(expiresAt), MaxUses: command.MaxUses,
		AccommodationID: idToPG(command.AccommodationID),
		OidcIssuer:      command.Actor.Issuer, OidcSubject: command.Actor.Subject,
	}
}

func (r *StayRepository) decodePoster(
	body []byte,
) (stay.AccommodationInviteCreated, error) {
	var payload posterReplayPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return stay.AccommodationInviteCreated{}, stay.ErrUnavailable
	}
	token, err := r.codec.Reconstruct(
		stay.PurposeAccommodationSelfRegistration, payload.InviteID, payload.KeyVersion,
	)
	if err != nil {
		return stay.AccommodationInviteCreated{}, stay.ErrUnavailable
	}
	return stay.AccommodationInviteCreated{
		InviteID: payload.InviteID, URL: fragmentURL(
			r.store.phase7.SelfRegistrationURL, token,
		),
		ExpiresAt: payload.ExpiresAt, MaxUses: payload.MaxUses,
		UseCount: payload.UseCount, Version: payload.Version,
	}, nil
}

func (r *StayRepository) GetAccommodationInvite(
	ctx context.Context,
	actor access.Principal,
	accommodationID uuid.UUID,
) (result stay.AccommodationInviteStatus, err error) {
	if !r.store.phase7.Enabled {
		return result, stay.ErrNotFound
	}
	err = r.store.inReadOnlyTransaction(ctx, func(q generated.Querier) error {
		row, readErr := q.GetActiveAccommodationInvite(
			ctx, generated.GetActiveAccommodationInviteParams{
				AccommodationID: idToPG(accommodationID),
				OidcIssuer:      actor.Issuer, OidcSubject: actor.Subject,
			},
		)
		if readErr != nil {
			return stayQueryError(readErr)
		}
		result = posterStatus(row)
		return nil
	})
	return result, err
}

func posterStatus(
	row generated.GetActiveAccommodationInviteRow,
) stay.AccommodationInviteStatus {
	return stay.AccommodationInviteStatus{
		InviteID: idFromPG(row.ID), ExpiresAt: row.ExpiresAt.Time.UTC(),
		MaxUses: row.MaxUses, UseCount: row.UseCount,
		RevokedAt: timePointer(row.RevokedAt), Version: 1,
	}
}

func (r *StayRepository) RevokeAccommodationInvite(
	ctx context.Context,
	command stay.AccommodationInviteRevokeCommand,
) (result stay.AccommodationInviteStatus, replayed bool, err error) {
	if !r.store.phase7.Enabled {
		return result, false, stay.ErrNotFound
	}
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		idempotent, runErr := r.store.runIdempotent(
			ctx, q, revokePosterIdempotency(command, now),
			func() (storedMutation, error) {
				return r.revokePoster(ctx, q, command, now)
			},
		)
		if runErr != nil {
			return stayMutationError(runErr)
		}
		replayed = idempotent.replayed
		return decodePosterStatus(idempotent.response.body, &result)
	})
	return result, replayed, err
}

// Same reason as the self-registration payload: the contract forbids `version`
// in the poster status, so the API type hides it from the wire and the replay
// would decode it as zero.
type posterStatusReplayPayload struct {
	InviteID  uuid.UUID  `json:"invite_id"`
	ExpiresAt time.Time  `json:"expires_at"`
	MaxUses   *int32     `json:"max_uses"`
	UseCount  int32      `json:"use_count"`
	RevokedAt *time.Time `json:"revoked_at"`
	Version   int64      `json:"version"`
}

func posterStatusReplay(status stay.AccommodationInviteStatus) any {
	return posterStatusReplayPayload{
		InviteID: status.InviteID, ExpiresAt: status.ExpiresAt,
		MaxUses: status.MaxUses, UseCount: status.UseCount,
		RevokedAt: status.RevokedAt, Version: status.Version,
	}
}

func decodePosterStatus(body []byte, target *stay.AccommodationInviteStatus) error {
	var payload posterStatusReplayPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return stay.ErrUnavailable
	}
	*target = stay.AccommodationInviteStatus{
		InviteID: payload.InviteID, ExpiresAt: payload.ExpiresAt,
		MaxUses: payload.MaxUses, UseCount: payload.UseCount,
		RevokedAt: payload.RevokedAt, Version: payload.Version,
	}
	return nil
}

func revokePosterIdempotency(
	command stay.AccommodationInviteRevokeCommand,
	now time.Time,
) idempotencySpec {
	return idempotencySpec{
		actorValue: actorValue(command.Actor.Issuer, command.Actor.Subject),
		operation:  idempotency.OperationRevokeAccommodationInvite,
		resourceID: command.AccommodationID, key: command.IdempotencyKey,
		request: struct {
			AccommodationID uuid.UUID `json:"accommodation_id"`
			ExpectedVersion int64     `json:"expected_version"`
		}{command.AccommodationID, command.ExpectedVersion},
		now: now,
	}
}

// Revoking an already revoked poster answers 200, because the operation is
// idempotent by nature: the caller asked for a state, not for a transition.
func (r *StayRepository) revokePoster(
	ctx context.Context,
	q generated.Querier,
	command stay.AccommodationInviteRevokeCommand,
	now time.Time,
) (storedMutation, error) {
	target, err := accommodationGuard(
		ctx, q, command.Actor, command.AccommodationID,
		command.ExpectedVersion, accommodation.OperationIssueInvite,
	)
	if err != nil {
		return storedMutation{}, err
	}
	row, err := q.GetActiveAccommodationInvite(
		ctx, generated.GetActiveAccommodationInviteParams{
			AccommodationID: idToPG(command.AccommodationID),
			OidcIssuer:      command.Actor.Issuer, OidcSubject: command.Actor.Subject,
		},
	)
	if err != nil {
		return storedMutation{}, stayQueryError(err)
	}
	return r.writeRevocation(ctx, q, command, target, posterStatus(row), now)
}

func (r *StayRepository) writeRevocation(
	ctx context.Context,
	q generated.Querier,
	command stay.AccommodationInviteRevokeCommand,
	target generated.GetAccessibleAccommodationRow,
	status stay.AccommodationInviteStatus,
	now time.Time,
) (storedMutation, error) {
	if _, err := q.RevokeActiveAccommodationInvites(
		ctx, revokePosterParams(command.Actor, command.AccommodationID, now),
	); err != nil {
		return storedMutation{}, stay.ErrUnavailable
	}
	if err := r.store.recordPosterEvent(ctx, q, posterEvent{
		actor: command.Actor, organizationID: idFromPG(target.OrganizationID),
		accommodationID: command.AccommodationID, inviteID: status.InviteID,
		requestID: command.RequestID, action: audit.ActionAccommodationRevoked,
		event: outbox.EventAccommodationInviteRevoked, now: now,
	}); err != nil {
		return storedMutation{}, err
	}
	status.RevokedAt = &now
	return jsonMutation(200, status.InviteID, posterStatusReplay(status), map[string]string{
		"ETag": entityTag(status.Version),
	})
}

// posterEvent carries only identifiers. No token, no URL and no HMAC ever
// reaches audit, outbox, log, trace or metric.
type posterEvent struct {
	actor           access.Principal
	organizationID  uuid.UUID
	accommodationID uuid.UUID
	inviteID        uuid.UUID
	requestID       string
	action          audit.Action
	event           outbox.EventType
	now             time.Time
}

// The audit entity is the accommodation, because that is what the operator
// acted on. The outbox aggregate is the poster, because that is what changed —
// and because the accommodation version does not move when a poster is issued,
// filing it under the accommodation made every rotation collide on the outbox
// identity index and abort the transaction that was supposed to revoke the
// previous poster.
//
// Version one is right and stays right: each poster is minted once, and a
// rotation mints a new identifier rather than a new version of the old one.
func posterEventSpec(spec posterEvent) eventSpec {
	return eventSpec{
		actorType: audit.ActorUser, actorIssuer: spec.actor.Issuer,
		actorSubject: spec.actor.Subject, organization: spec.organizationID,
		action: spec.action, entityType: audit.EntityAccommodation,
		entityID: spec.accommodationID, aggregateID: spec.inviteID,
		requestID: spec.requestID, version: 1,
		aggregateType: outbox.AggregateAccommodationInvite,
		eventType:     spec.event, now: spec.now,
	}
}

func (s *Store) recordPosterEvent(
	ctx context.Context,
	q generated.Querier,
	spec posterEvent,
) error {
	return s.recordEvents(ctx, q, posterEventSpec(spec))
}

// resolvedPoster is the poster equivalent of resolvedCapability. Every way a
// probe could tell a bad token from a missing one answers not-found.
type resolvedPoster struct {
	inviteID uuid.UUID
	digest   []byte
	row      generated.GetAccommodationInviteForCapabilityRow
}

func (r *StayRepository) resolvePoster(
	ctx context.Context,
	q generated.Querier,
	token string,
	now time.Time,
) (resolvedPoster, error) {
	inviteID, err := capabilityInviteID(token)
	if err != nil {
		return resolvedPoster{}, stay.ErrNotFound
	}
	row, err := q.GetAccommodationInviteForCapability(ctx, idToPG(inviteID))
	if err != nil {
		return resolvedPoster{}, stayQueryError(err)
	}
	digest, err := r.verifiedPosterDigest(token, inviteID, row)
	if err != nil {
		return resolvedPoster{}, err
	}
	if !usablePoster(row, now) {
		return resolvedPoster{}, stay.ErrNotFound
	}
	return resolvedPoster{inviteID: inviteID, digest: digest, row: row}, nil
}

func (r *StayRepository) verifiedPosterDigest(
	token string,
	inviteID uuid.UUID,
	row generated.GetAccommodationInviteForCapabilityRow,
) ([]byte, error) {
	verifiedID, err := r.codec.Verify(
		stay.PurposeAccommodationSelfRegistration, token, row.TokenKeyVersion,
	)
	if err != nil || verifiedID != inviteID {
		return nil, stay.ErrNotFound
	}
	digest, err := r.codec.StorageDigest(token, row.TokenKeyVersion)
	if err != nil || !hmac.Equal(digest, row.TokenHmac) {
		return nil, stay.ErrNotFound
	}
	return digest, nil
}

// A null max_uses means unlimited, and the nil test comes before the
// comparison. The accommodation status is checked here too: a poster of a
// suspended or closed accommodation answers exactly like an unknown one (N-06).
func usablePoster(
	row generated.GetAccommodationInviteForCapabilityRow,
	now time.Time,
) bool {
	if !livePoster(row, now) {
		return false
	}
	return row.MaxUses == nil || row.UseCount < *row.MaxUses
}

func livePoster(
	row generated.GetAccommodationInviteForCapabilityRow,
	now time.Time,
) bool {
	if !row.ExpiresAt.Valid || !row.ExpiresAt.Time.After(now) || row.RevokedAt.Valid {
		return false
	}
	return accommodation.Status(row.AccommodationStatus) == accommodation.StatusActive
}

func decodeJSON(body []byte, target any) error {
	if err := json.Unmarshal(body, target); err != nil {
		return stay.ErrUnavailable
	}
	return nil
}
