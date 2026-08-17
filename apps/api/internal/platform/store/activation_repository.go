package store

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/activation"
	"github.com/Pantani/cumuru/apps/api/internal/audit"
	"github.com/Pantani/cumuru/apps/api/internal/platform/idempotency"
	"github.com/Pantani/cumuru/apps/api/internal/platform/outbox"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	activationContextScope = "activation_context"
	activationSubmitScope  = "activation_submit"
)

// ActivationRepository is separate from StayRepository because it writes to
// auth and to core.memberships, and because the phase must be switchable off as
// a whole: with the phase disabled nothing constructs it and the routes are not
// registered at all.
type ActivationRepository struct {
	store *Store
	codec *stay.InviteCodec
}

var _ activation.Repository = (*ActivationRepository)(nil)

func NewActivationRepository(store *Store) (*ActivationRepository, error) {
	codec, err := stay.NewInviteCodec(stay.InviteKeyring{
		CurrentVersion: store.phase2.InviteKeys.CurrentVersion,
		Keys:           store.phase2.InviteKeys.Keys,
	})
	if err != nil {
		return nil, err
	}
	return &ActivationRepository{store: store, codec: codec}, nil
}

// activationReplayPayload keeps the identifier and the key version, never the
// token: the URL is rebuilt from the keyring on an exact replay, so the
// idempotency table never holds a working capability.
type activationReplayPayload struct {
	CapabilityID uuid.UUID `json:"capability_id"`
	AccountID    uuid.UUID `json:"account_id"`
	KeyVersion   string    `json:"key_version"`
	ExpiresAt    time.Time `json:"expires_at"`
	Version      int64     `json:"version"`
}

func (r *ActivationRepository) Create(
	ctx context.Context,
	command activation.CreateCommand,
) (result activation.Created, replayed bool, err error) {
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		idempotent, runErr := r.store.runIdempotent(
			ctx, q, activationIdempotency(command, now),
			func() (storedMutation, error) {
				return r.issueActivation(ctx, q, command, now)
			},
		)
		if runErr != nil {
			return activationMutationError(runErr)
		}
		result, runErr = r.decodeActivation(idempotent.response.body)
		replayed = idempotent.replayed
		return runErr
	})
	return result, replayed, err
}

func activationIdempotency(
	command activation.CreateCommand,
	now time.Time,
) idempotencySpec {
	return idempotencySpec{
		actorValue: actorValue(command.Actor.Issuer, command.Actor.Subject),
		operation:  idempotency.OperationCreateActivation,
		resourceID: command.AccommodationID, key: command.IdempotencyKey,
		request: struct {
			AccommodationID uuid.UUID `json:"accommodation_id"`
			Email           string    `json:"email"`
			DisplayName     string    `json:"display_name"`
			ExpectedVersion int64     `json:"expected_version"`
		}{
			command.AccommodationID, NormalizeEmail(command.Email),
			command.DisplayName, command.ExpectedVersion,
		},
		now: now,
	}
}

func (r *ActivationRepository) issueActivation(
	ctx context.Context,
	q generated.Querier,
	command activation.CreateCommand,
	now time.Time,
) (storedMutation, error) {
	target, err := accommodationGuard(
		ctx, q, command.Actor, command.AccommodationID,
		command.ExpectedVersion, accommodation.OperationIssueActivation,
	)
	if err != nil {
		return storedMutation{}, activationDomainError(err)
	}
	account, err := r.resolveActivationAccount(ctx, q, command)
	if err != nil {
		return storedMutation{}, err
	}
	payload, err := r.issueCapability(ctx, q, command, account, now)
	if err != nil {
		return storedMutation{}, err
	}
	if err := r.recordIssue(ctx, q, command, target, payload.CapabilityID, now); err != nil {
		return storedMutation{}, err
	}
	return jsonMutation(201, payload.CapabilityID, payload, map[string]string{
		"Location": "/api/v1/accommodations/" + command.AccommodationID.String() + "/activation",
		"ETag":     entityTag(payload.Version),
	})
}

// resolveActivationAccount is what makes re-issuing work. The contract and the
// ADR-041 describe a re-issue as the same account receiving a new capability,
// not a second account: creating one violates accounts_email_idx and aborts the
// whole transaction, which is how the endpoint came to answer 409 on every
// second call.
func (r *ActivationRepository) resolveActivationAccount(
	ctx context.Context,
	q generated.Querier,
	command activation.CreateCommand,
) (uuid.UUID, error) {
	row, err := q.FindAuthAccountByEmail(ctx, NormalizeEmail(command.Email))
	if errors.Is(err, pgx.ErrNoRows) {
		return r.provisionAccount(ctx, q, command)
	}
	if err != nil {
		return uuid.Nil, activation.ErrUnavailable
	}
	return r.reusePendingAccount(ctx, q, command, row)
}

// reusePendingAccount is the security boundary of this endpoint.
//
// accounts_email_idx is unique **globally**, not per accommodation, so a lookup
// by e-mail can land on an account that belongs to somebody else. The earlier
// version then added a membership for the issuing accommodation and preserved
// the existing ones, which handed whoever received the new link access to every
// accommodation that account already reached — including other organizations.
//
// Two rules close it, and the second is the one that matters:
//
//  1. the account must still be pending. An activated account keeps its
//     credential: accounts_credential_state_valid ties a missing hash to
//     pending_activation, so a downgrade would either violate the CHECK or
//     destroy a live credential;
//  2. the account must **already** be a member of the accommodation issuing the
//     capability, and reuse never adds a membership. That makes the reachable
//     set of an account monotonically non-increasing: provisioning creates
//     exactly one membership and reuse creates none, so a capability can only
//     ever grant the accommodation that provisioned the account — which
//     accommodationGuard already proved the issuer controls.
//
// Enumerating the account's memberships would be the obvious check and is the
// wrong one: ListActiveTenantMemberships filters a.status = 'active', so a
// membership in a suspended accommodation is invisible to it, and absence of
// evidence would read as evidence of absence.
//
// Both refusals answer the same ErrConflict on purpose. A caller that could tell
// "belongs to another accommodation" from "already activated" would learn the
// state of somebody else's account.
func (r *ActivationRepository) reusePendingAccount(
	ctx context.Context,
	q generated.Querier,
	command activation.CreateCommand,
	row generated.FindAuthAccountByEmailRow,
) (uuid.UUID, error) {
	if row.Status != accountStatusPendingActivation {
		return uuid.Nil, activation.ErrConflict
	}
	accountID := idFromPG(row.ID)
	if err := r.requireExistingMembership(ctx, q, command, accountID); err != nil {
		return uuid.Nil, err
	}
	return accountID, nil
}

func (r *ActivationRepository) requireExistingMembership(
	ctx context.Context,
	q generated.Querier,
	command activation.CreateCommand,
	accountID uuid.UUID,
) error {
	_, err := q.GetAccessibleAccommodation(ctx, generated.GetAccessibleAccommodationParams{
		AccommodationID: idToPG(command.AccommodationID),
		OidcIssuer:      access.LocalSessionIssuer, OidcSubject: accountID.String(),
	})
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return activation.ErrConflict
	}
	return activation.ErrUnavailable
}

// The account is born without a hash, in pending_activation, which is exactly
// what accounts_credential_state_valid ties together. It cannot authenticate
// until the activation writes the password.
func (r *ActivationRepository) provisionAccount(
	ctx context.Context,
	q generated.Querier,
	command activation.CreateCommand,
) (uuid.UUID, error) {
	accountID, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, activation.ErrUnavailable
	}
	row, err := q.CreateActivationAccount(ctx, generated.CreateActivationAccountParams{
		AccountID: idToPG(accountID), Email: NormalizeEmail(command.Email),
		// The account carries the scopes from birth but cannot use them until the
		// password is set, because a pending account does not authenticate.
		DisplayName: command.DisplayName, Scopes: activation.Scopes(),
	})
	if err != nil {
		return uuid.Nil, activationWriteError(err)
	}
	created := idFromPG(row.ID)
	return created, r.linkMembership(ctx, q, command, created)
}

// The membership binds the pending account to the accommodation as a manager.
// The query is gated on status = 'active' because issue_activation only exists
// on an active accommodation.
func (r *ActivationRepository) linkMembership(
	ctx context.Context,
	q generated.Querier,
	command activation.CreateCommand,
	accountID uuid.UUID,
) error {
	membershipID, err := uuid.NewV7()
	if err != nil {
		return activation.ErrUnavailable
	}
	_, err = q.CreateActivationManagerMembership(
		ctx, generated.CreateActivationManagerMembershipParams{
			MembershipID:     idToPG(membershipID),
			TargetOidcIssuer: access.LocalSessionIssuer, TargetOidcSubject: accountID.String(),
			AccommodationID: idToPG(command.AccommodationID),
			OidcIssuer:      command.Actor.Issuer, OidcSubject: command.Actor.Subject,
		},
	)
	if err != nil {
		return activationWriteError(err)
	}
	return nil
}

// Re-issuing revokes the previous capability in the same transaction, and the
// partial unique index makes two open ones impossible (N-43).
func (r *ActivationRepository) issueCapability(
	ctx context.Context,
	q generated.Querier,
	command activation.CreateCommand,
	accountID uuid.UUID,
	now time.Time,
) (activationReplayPayload, error) {
	if _, err := q.RevokeOpenActivationCapabilities(
		ctx, generated.RevokeOpenActivationCapabilitiesParams{
			RevokedAt: timeToPG(now), AccountID: idToPG(accountID),
		},
	); err != nil {
		return activationReplayPayload{}, activation.ErrUnavailable
	}
	capabilityID, err := uuid.NewV7()
	if err != nil {
		return activationReplayPayload{}, activation.ErrUnavailable
	}
	return r.writeCapability(ctx, q, command, accountID, capabilityID, now)
}

func (r *ActivationRepository) writeCapability(
	ctx context.Context,
	q generated.Querier,
	command activation.CreateCommand,
	accountID uuid.UUID,
	capabilityID uuid.UUID,
	now time.Time,
) (activationReplayPayload, error) {
	token, keyVersion, err := r.codec.Issue(
		stay.PurposeAccommodationActivation, capabilityID,
	)
	if err != nil {
		return activationReplayPayload{}, activation.ErrUnavailable
	}
	digest, err := r.codec.StorageDigest(token, keyVersion)
	if err != nil {
		return activationReplayPayload{}, activation.ErrUnavailable
	}
	expiresAt := now.Add(r.store.phase2.InviteTTL)
	row, err := q.CreateActivationCapability(ctx, generated.CreateActivationCapabilityParams{
		CapabilityID: idToPG(capabilityID), AccountID: idToPG(accountID),
		AccommodationID: idToPG(command.AccommodationID), TokenHmac: digest,
		TokenKeyVersion: keyVersion, ExpiresAt: timeToPG(expiresAt),
	})
	if err != nil {
		return activationReplayPayload{}, activationWriteError(err)
	}
	return activationReplayPayload{
		CapabilityID: idFromPG(row.ID), AccountID: accountID,
		KeyVersion: keyVersion, ExpiresAt: expiresAt, Version: 1,
	}, nil
}

func (r *ActivationRepository) decodeActivation(
	body []byte,
) (activation.Created, error) {
	var payload activationReplayPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return activation.Created{}, activation.ErrUnavailable
	}
	token, err := r.codec.Reconstruct(
		stay.PurposeAccommodationActivation, payload.CapabilityID, payload.KeyVersion,
	)
	if err != nil {
		return activation.Created{}, activation.ErrUnavailable
	}
	return activation.Created{
		ActivationID: payload.CapabilityID, AccountID: payload.AccountID,
		URL:       fragmentURL(r.store.phase7.ActivationURL, token),
		ExpiresAt: payload.ExpiresAt, Version: payload.Version,
	}, nil
}

// Same reason as the poster: re-issuing does not bump the accommodation
// version, so the capability is the aggregate and each re-issue mints a new one.
func (r *ActivationRepository) recordIssue(
	ctx context.Context,
	q generated.Querier,
	command activation.CreateCommand,
	target generated.GetAccessibleAccommodationRow,
	capabilityID uuid.UUID,
	now time.Time,
) error {
	return r.store.recordEvents(ctx, q, eventSpec{
		actorType: audit.ActorUser, actorIssuer: command.Actor.Issuer,
		actorSubject: command.Actor.Subject, organization: idFromPG(target.OrganizationID),
		action: audit.ActionActivationIssued, entityType: audit.EntityAccommodation,
		entityID: command.AccommodationID, aggregateID: capabilityID,
		requestID: command.RequestID, version: 1,
		aggregateType: outbox.AggregateAccommodationActivation,
		eventType:     outbox.EventAccommodationActivationIssued, now: now,
	})
}

func (r *ActivationRepository) Context(
	ctx context.Context,
	request activation.Request,
) (result activation.Context, err error) {
	now := r.store.currentTime()
	if err := r.limitActivation(
		ctx, activationContextScope, request.Token, request.RateSubject,
		r.store.phase7.ActivationContextRateLimit, now,
	); err != nil {
		return result, err
	}
	err = r.store.inReadOnlyTransaction(ctx, func(q generated.Querier) error {
		row, readErr := r.resolveCapability(ctx, q, request.Token, now)
		if readErr != nil {
			return readErr
		}
		result = activationContext(row)
		return nil
	})
	return result, err
}

func activationContext(row generated.GetActivationCapabilityRow) activation.Context {
	return activation.Context{
		AccommodationName: row.AccommodationName, DisplayName: row.DisplayName,
		ExpiresAt: row.ExpiresAt.Time.UTC(),
		PasswordPolicy: activation.PasswordPolicy{
			MinLength: access.MinPasswordLength, MaxLength: access.MaxPasswordLength,
		},
	}
}

// Complete writes the password and flips the account to active in the same
// transaction that consumes the capability. The zero-row case of the consume is
// the whole single-use guarantee, including under a concurrent race (N-42).
func (r *ActivationRepository) Complete(
	ctx context.Context,
	command activation.CompleteCommand,
) error {
	now := r.store.currentTime()
	if err := r.limitActivation(
		ctx, activationSubmitScope, command.Token, command.RateSubject,
		r.store.phase7.ActivationSubmitRateLimit, now,
	); err != nil {
		return err
	}
	hash, err := access.NewPasswordHasher().Hash(command.Password)
	if err != nil {
		return activation.ErrInvalidInput
	}
	return r.store.inTransaction(ctx, func(q generated.Querier) error {
		return r.completeActivation(ctx, q, command, hash, now)
	})
}

func (r *ActivationRepository) completeActivation(
	ctx context.Context,
	q generated.Querier,
	command activation.CompleteCommand,
	hash string,
	now time.Time,
) error {
	row, err := r.resolveCapability(ctx, q, command.Token, now)
	if err != nil {
		return err
	}
	consumed, err := q.ConsumeActivationCapability(
		ctx, generated.ConsumeActivationCapabilityParams{
			ConsumedAt: timeToPG(now), TokenHmac: row.TokenHmac,
		},
	)
	if err != nil {
		return activation.ErrNotFound
	}
	written, err := q.ActivateAccountPassword(ctx, generated.ActivateAccountPasswordParams{
		PasswordHash: &hash, ChangedAt: timeToPG(now), AccountID: consumed.AccountID,
	})
	if err != nil || written != 1 {
		return activation.ErrNotFound
	}
	return r.recordCompletion(ctx, q, command, consumed, now)
}

// The actor is the account that just activated itself, resolved through the
// membership the issue step created. That is the honest record: the person who
// held the link set the password, and no synthetic subject is invented.
func (r *ActivationRepository) recordCompletion(
	ctx context.Context,
	q generated.Querier,
	command activation.CompleteCommand,
	consumed generated.ConsumeActivationCapabilityRow,
	now time.Time,
) error {
	subject := idFromPG(consumed.AccountID).String()
	target, err := q.GetAccessibleAccommodation(
		ctx, generated.GetAccessibleAccommodationParams{
			AccommodationID: consumed.AccommodationID,
			OidcIssuer:      access.LocalSessionIssuer, OidcSubject: subject,
		},
	)
	if err != nil {
		return activation.ErrUnavailable
	}
	return r.store.recordEvents(ctx, q, eventSpec{
		actorType: audit.ActorUser, actorIssuer: access.LocalSessionIssuer,
		actorSubject: subject, organization: idFromPG(target.OrganizationID),
		action: audit.ActionActivationCompleted, entityType: audit.EntityAccommodation,
		entityID:    idFromPG(consumed.AccommodationID),
		aggregateID: idFromPG(consumed.ID), requestID: command.RequestID, version: 1,
		aggregateType: outbox.AggregateAccommodationActivation,
		eventType:     outbox.EventAccommodationActivationCompleted, now: now,
	})
}

// Every failure — absent, wrong, expired, consumed, revoked, or belonging to an
// accommodation that is no longer active — answers the same not-found.
func (r *ActivationRepository) resolveCapability(
	ctx context.Context,
	q generated.Querier,
	token string,
	now time.Time,
) (generated.GetActivationCapabilityRow, error) {
	capabilityID, err := capabilityInviteID(token)
	if err != nil {
		return generated.GetActivationCapabilityRow{}, activation.ErrNotFound
	}
	row, err := q.GetActivationCapability(ctx, idToPG(capabilityID))
	if err != nil {
		return generated.GetActivationCapabilityRow{}, activation.ErrNotFound
	}
	if !r.matchingCapability(token, capabilityID, row) || !usableCapability(row, now) {
		return generated.GetActivationCapabilityRow{}, activation.ErrNotFound
	}
	return row, nil
}

func (r *ActivationRepository) matchingCapability(
	token string,
	capabilityID uuid.UUID,
	row generated.GetActivationCapabilityRow,
) bool {
	verified, err := r.codec.Verify(
		stay.PurposeAccommodationActivation, token, row.TokenKeyVersion,
	)
	if err != nil || verified != capabilityID {
		return false
	}
	digest, err := r.codec.StorageDigest(token, row.TokenKeyVersion)
	return err == nil && hmac.Equal(digest, row.TokenHmac)
}

func usableCapability(row generated.GetActivationCapabilityRow, now time.Time) bool {
	if row.ConsumedAt.Valid || row.RevokedAt.Valid {
		return false
	}
	if !row.ExpiresAt.Valid || !row.ExpiresAt.Time.After(now) {
		return false
	}
	return accommodation.Status(row.AccommodationStatus) == accommodation.StatusActive
}

func (r *ActivationRepository) limitActivation(
	ctx context.Context,
	scope string,
	token string,
	subject string,
	limit int,
	now time.Time,
) error {
	key, ok := r.store.phase2.RateLimitKeys.Key(r.store.phase2.RateLimitKeys.CurrentVersion)
	if !ok {
		return activation.ErrUnavailable
	}
	window := now.Truncate(r.store.phase2.RateLimitWindow)
	limited, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	row, err := r.store.queries.IncrementRateLimit(limited, generated.IncrementRateLimitParams{
		Scope: scope, SubjectHmac: rateLimitDigest(key, scope, token, subject),
		SubjectKeyVersion: r.store.phase2.RateLimitKeys.CurrentVersion,
		WindowStartedAt:   timeToPG(window),
		ExpiresAt:         timeToPG(window.Add(2 * r.store.phase2.RateLimitWindow)),
	})
	if err != nil {
		return activation.ErrUnavailable
	}
	if row.RequestCount > int32(limit) {
		return stay.ErrRateLimited
	}
	return nil
}

// activationDomainError translates the shared accommodation guard, which speaks
// in stay errors, into the activation vocabulary. Without it a forbidden
// activation would surface as a stay error and the transport would map it by
// accident rather than by decision.
func activationDomainError(err error) error {
	switch {
	case errors.Is(err, stay.ErrNotFound):
		return activation.ErrNotFound
	case errors.Is(err, stay.ErrForbidden):
		return activation.ErrForbidden
	case errors.Is(err, stay.ErrPreconditionFailed):
		return activation.ErrPreconditionFailed
	case errors.Is(err, stay.ErrConflict):
		return activation.ErrConflict
	default:
		return activation.ErrUnavailable
	}
}

// A duplicate e-mail is a conflict on a unique index, not an internal error.
func activationWriteError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return activation.ErrNotFound
	}
	if isUniqueViolation(err) {
		return activation.ErrConflict
	}
	return activation.ErrUnavailable
}

func activationMutationError(err error) error {
	if errors.Is(err, errIdempotencyConflict) {
		return activation.ErrConflict
	}
	return err
}
