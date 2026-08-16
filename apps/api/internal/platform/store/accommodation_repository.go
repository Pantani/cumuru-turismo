package store

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/audit"
	"github.com/Pantani/cumuru/apps/api/internal/platform/idempotency"
	"github.com/Pantani/cumuru/apps/api/internal/platform/outbox"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var accommodationOnboardingResourceID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

type AccommodationRepository struct {
	store *Store
}

func NewAccommodationRepository(store *Store) *AccommodationRepository {
	return &AccommodationRepository{store: store}
}

var _ accommodation.Repository = (*AccommodationRepository)(nil)

func (r *AccommodationRepository) Create(
	ctx context.Context,
	command accommodation.CreateCommand,
) (result accommodation.Accommodation, replayed bool, err error) {
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		lockKey, lockErr := r.store.accommodationOnboardingLockKey(command.Actor)
		if lockErr != nil {
			return accommodation.ErrUnavailable
		}
		if lockErr = q.AcquireAccommodationOnboardingLock(ctx, lockKey); lockErr != nil {
			return accommodation.ErrUnavailable
		}
		idempotent, runErr := r.store.runIdempotent(
			ctx,
			q,
			accommodationIdempotencySpec(command, now),
			func() (storedMutation, error) {
				return r.createAccommodation(ctx, q, command)
			},
		)
		if runErr != nil {
			return accommodationMutationError(runErr)
		}
		if decodeErr := json.Unmarshal(idempotent.response.body, &result); decodeErr != nil {
			return accommodation.ErrUnavailable
		}
		replayed = idempotent.replayed
		return nil
	})
	return result, replayed, err
}

func (r *AccommodationRepository) createAccommodation(
	ctx context.Context,
	q generated.Querier,
	command accommodation.CreateCommand,
) (storedMutation, error) {
	scope, ids, err := prepareOnboarding(ctx, q, command)
	if err != nil {
		return storedMutation{}, err
	}
	row, err := insertOnboardingRecords(ctx, q, scope, ids, command)
	if err != nil {
		return storedMutation{}, err
	}
	created := accommodationFromOnboarding(row)
	if err := r.store.recordAccommodationCreate(ctx, q, command, created); err != nil {
		return storedMutation{}, err
	}
	return onboardingStoredMutation(created)
}

// The organization, the accommodation and the first manager membership are
// written together: a participant is never left with an accommodation nobody
// can operate.
func prepareOnboarding(
	ctx context.Context,
	q generated.Querier,
	command accommodation.CreateCommand,
) (accommodationOnboardingScope, accommodationOnboardingIDs, error) {
	var ids accommodationOnboardingIDs
	scope, err := resolveAccommodationOnboardingScope(ctx, q, command.Actor)
	if err != nil {
		return scope, ids, err
	}
	if err := ensureOnboardingSubmissionUnused(ctx, q, scope.organizationID, command); err != nil {
		return scope, ids, err
	}
	ids, err = newAccommodationOnboardingIDs(scope)
	if err != nil {
		return scope, ids, accommodation.ErrUnavailable
	}
	return scope, ids, nil
}

func insertOnboardingRecords(
	ctx context.Context,
	q generated.Querier,
	scope accommodationOnboardingScope,
	ids accommodationOnboardingIDs,
	command accommodation.CreateCommand,
) (generated.InsertOnboardingAccommodationRow, error) {
	var empty generated.InsertOnboardingAccommodationRow
	if err := insertOnboardingOrganization(ctx, q, scope, ids, command.Name); err != nil {
		return empty, accommodationMutationError(err)
	}
	row, err := q.InsertOnboardingAccommodation(ctx, generated.InsertOnboardingAccommodationParams{
		AccommodationID: pgUUID(ids.accommodationID), OrganizationID: pgUUID(ids.organizationID),
		Name: command.Name, Category: string(command.Category), Capacity: &command.Capacity,
		OnboardingSubmissionID: pgUUID(command.ClientSubmissionID),
	})
	if err != nil {
		return empty, accommodationMutationError(err)
	}
	if _, err := q.InsertOnboardingManagerMembership(ctx, generated.InsertOnboardingManagerMembershipParams{
		MembershipID: pgUUID(ids.membershipID), AccommodationID: pgUUID(ids.accommodationID),
		OidcIssuer: command.Actor.Issuer, OidcSubject: command.Actor.Subject,
	}); err != nil {
		return empty, accommodationMutationError(err)
	}
	return row, nil
}

type accommodationOnboardingScope struct {
	organizationID     uuid.UUID
	createOrganization bool
}

type accommodationOnboardingIDs struct {
	organizationID  uuid.UUID
	accommodationID uuid.UUID
	membershipID    uuid.UUID
}

func resolveAccommodationOnboardingScope(
	ctx context.Context,
	q generated.Querier,
	actor access.Principal,
) (accommodationOnboardingScope, error) {
	rows, err := q.ListAccommodationOnboardingOrganizations(
		ctx,
		generated.ListAccommodationOnboardingOrganizationsParams{
			OidcIssuer: actor.Issuer, OidcSubject: actor.Subject,
		},
	)
	if err != nil {
		return accommodationOnboardingScope{}, accommodation.ErrUnavailable
	}
	if len(rows) > 1 {
		return accommodationOnboardingScope{}, accommodation.ErrConflict
	}
	if len(rows) == 1 {
		return existingAccommodationOnboardingScope(rows[0])
	}
	id, err := uuid.NewV7()
	if err != nil {
		return accommodationOnboardingScope{}, accommodation.ErrUnavailable
	}
	return accommodationOnboardingScope{organizationID: id, createOrganization: true}, nil
}

func existingAccommodationOnboardingScope(
	row generated.ListAccommodationOnboardingOrganizationsRow,
) (accommodationOnboardingScope, error) {
	if !row.OrganizationID.Valid {
		return accommodationOnboardingScope{}, accommodation.ErrUnavailable
	}
	if !row.HasManagerMembership {
		return accommodationOnboardingScope{}, accommodation.ErrForbidden
	}
	return accommodationOnboardingScope{organizationID: uuid.UUID(row.OrganizationID.Bytes)}, nil
}

func ensureOnboardingSubmissionUnused(
	ctx context.Context,
	q generated.Querier,
	organizationID uuid.UUID,
	command accommodation.CreateCommand,
) error {
	_, err := q.FindOnboardedAccommodation(ctx, generated.FindOnboardedAccommodationParams{
		OrganizationID:         pgUUID(organizationID),
		OnboardingSubmissionID: pgUUID(command.ClientSubmissionID),
		OidcIssuer:             command.Actor.Issuer, OidcSubject: command.Actor.Subject,
	})
	if err == nil {
		return accommodation.ErrConflict
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return accommodation.ErrUnavailable
}

func newAccommodationOnboardingIDs(
	scope accommodationOnboardingScope,
) (accommodationOnboardingIDs, error) {
	accommodationID, err := uuid.NewV7()
	if err != nil {
		return accommodationOnboardingIDs{}, err
	}
	membershipID, err := uuid.NewV7()
	if err != nil {
		return accommodationOnboardingIDs{}, err
	}
	return accommodationOnboardingIDs{
		organizationID:  scope.organizationID,
		accommodationID: accommodationID,
		membershipID:    membershipID,
	}, nil
}

func insertOnboardingOrganization(
	ctx context.Context,
	q generated.Querier,
	scope accommodationOnboardingScope,
	ids accommodationOnboardingIDs,
	name string,
) error {
	if !scope.createOrganization {
		return nil
	}
	_, err := q.InsertOnboardingOrganization(ctx, generated.InsertOnboardingOrganizationParams{
		OrganizationID: pgUUID(ids.organizationID), Name: name,
	})
	return err
}

func onboardingStoredMutation(
	created accommodation.Accommodation,
) (storedMutation, error) {
	body, err := json.Marshal(created)
	if err != nil {
		return storedMutation{}, accommodation.ErrUnavailable
	}
	return storedMutation{
		status: 201, resourceID: created.ID, body: body,
		headers: map[string]string{
			"Location": "/api/v1/accommodations/" + created.ID.String(),
			"ETag":     entityTag(created.Version),
		},
	}, nil
}

func (r *AccommodationRepository) List(
	ctx context.Context,
	actor access.Principal,
	page accommodation.PageRequest,
) (accommodation.AccommodationPage, error) {
	s := r.store
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	rows, err := s.queries.ListAccessibleAccommodations(ctx, generated.ListAccessibleAccommodationsParams{
		OidcIssuer: actor.Issuer, OidcSubject: actor.Subject,
		CursorCreatedAt: optionalTime(page.CursorCreatedAt),
		CursorID:        pgUUID(page.CursorID), PageLimit: page.Limit + 1,
	})
	if err != nil {
		return accommodation.AccommodationPage{}, accommodation.ErrUnavailable
	}
	return accommodationPage(rows, page.Limit), nil
}

func (r *AccommodationRepository) Get(
	ctx context.Context,
	actor access.Principal,
	id uuid.UUID,
) (accommodation.Accommodation, error) {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	row, err := r.store.queries.GetAccessibleAccommodation(ctx, accommodationKey(id, actor))
	if errors.Is(err, pgx.ErrNoRows) {
		return accommodation.Accommodation{}, accommodation.ErrNotFound
	}
	if err != nil {
		return accommodation.Accommodation{}, accommodation.ErrUnavailable
	}
	return accommodationFromGet(row), nil
}

func (r *AccommodationRepository) Update(
	ctx context.Context,
	command accommodation.UpdateCommand,
) (result accommodation.Accommodation, err error) {
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		current, queryErr := q.GetAccessibleAccommodation(ctx, accommodationKey(command.AccommodationID, command.Actor))
		if queryErr != nil {
			return accommodationQueryError(queryErr)
		}
		if current.Version != command.ExpectedVersion {
			return accommodation.ErrPreconditionFailed
		}
		if policyErr := accommodationMutationPolicy(
			accommodation.Status(current.Status),
			accommodation.Role(current.ActorRole),
			accommodation.OperationUpdateAccommodation,
		); policyErr != nil {
			return policyErr
		}
		updated, queryErr := q.UpdateAccommodation(ctx, updateAccommodationParams(command, r.store.currentTime()))
		if queryErr != nil {
			return accommodationCommandError(ctx, q, command, queryErr)
		}
		result = accommodationFromUpdate(updated)
		return r.store.recordAccommodationMutation(ctx, q, command, current.OrganizationID, result)
	})
	return result, err
}

func (r *AccommodationRepository) ListMemberships(
	ctx context.Context,
	actor access.Principal,
	accommodationID uuid.UUID,
	page accommodation.PageRequest,
) (accommodation.MembershipPage, error) {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	rows, err := r.store.queries.ListAccommodationMemberships(ctx, generated.ListAccommodationMembershipsParams{
		AccommodationID: pgUUID(accommodationID),
		OidcIssuer:      actor.Issuer, OidcSubject: actor.Subject,
		CursorCreatedAt: optionalTime(page.CursorCreatedAt),
		CursorID:        pgUUID(page.CursorID), PageLimit: page.Limit + 1,
	})
	if err != nil {
		return accommodation.MembershipPage{}, accommodation.ErrUnavailable
	}
	if len(rows) == 0 {
		if _, err := r.Get(ctx, actor, accommodationID); err != nil {
			return accommodation.MembershipPage{}, err
		}
	}
	return membershipPage(rows, page.Limit), nil
}

func (r *AccommodationRepository) CreateMembership(
	ctx context.Context,
	command accommodation.CreateMembershipCommand,
) (result accommodation.MembershipCreated, replayed bool, err error) {
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		spec := membershipIdempotencySpec(command, now)
		idempotent, runErr := r.store.runIdempotent(ctx, q, spec, func() (storedMutation, error) {
			return r.createMembership(ctx, q, command, now)
		})
		if runErr != nil {
			return accommodationMutationError(runErr)
		}
		if decodeErr := json.Unmarshal(idempotent.response.body, &result); decodeErr != nil {
			return accommodation.ErrUnavailable
		}
		replayed = idempotent.replayed
		return nil
	})
	return result, replayed, err
}

// Only an active manager of an accommodation whose status still allows it may
// change its memberships.
func managerAccommodation(
	ctx context.Context,
	q generated.Querier,
	accommodationID uuid.UUID,
	actor access.Principal,
) (generated.GetAccessibleAccommodationRow, error) {
	current, err := q.GetAccessibleAccommodation(ctx, accommodationKey(accommodationID, actor))
	if err != nil || current.ActorRole != string(accommodation.RoleManager) {
		return current, accommodationQueryError(err)
	}
	policyErr := accommodationMutationPolicy(
		accommodation.Status(current.Status),
		accommodation.Role(current.ActorRole),
		accommodation.OperationManageMemberships,
	)
	return current, policyErr
}

func (r *AccommodationRepository) createMembership(
	ctx context.Context,
	q generated.Querier,
	command accommodation.CreateMembershipCommand,
	now time.Time,
) (storedMutation, error) {
	current, err := managerAccommodation(ctx, q, command.AccommodationID, command.Actor)
	if err != nil {
		return storedMutation{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return storedMutation{}, accommodation.ErrUnavailable
	}
	row, err := q.CreateAccommodationMembership(ctx, generated.CreateAccommodationMembershipParams{
		MembershipID: pgUUID(id), TargetOidcIssuer: command.TargetIssuer,
		TargetOidcSubject: command.TargetSubject, TargetRole: string(command.Role),
		AccommodationID: pgUUID(command.AccommodationID),
		OidcIssuer:      command.Actor.Issuer, OidcSubject: command.Actor.Subject,
	})
	if err != nil {
		return storedMutation{}, accommodationMutationError(err)
	}
	created := membershipCreated(row)
	if err := r.store.recordMembershipCreate(ctx, q, command, current.OrganizationID, created, now); err != nil {
		return storedMutation{}, err
	}
	return membershipStoredMutation(command.AccommodationID, created)
}

func membershipStoredMutation(
	accommodationID uuid.UUID,
	created accommodation.MembershipCreated,
) (storedMutation, error) {
	body, err := json.Marshal(created)
	if err != nil {
		return storedMutation{}, accommodation.ErrUnavailable
	}
	return storedMutation{
		status: 201, resourceID: created.ID, body: body,
		headers: map[string]string{
			"Location": "/api/v1/accommodations/" + accommodationID.String() +
				"/memberships/" + created.ID.String(),
			"ETag": entityTag(created.Version),
		},
	}, nil
}

func (r *AccommodationRepository) UpdateMembership(
	ctx context.Context,
	command accommodation.UpdateMembershipCommand,
) (result accommodation.Membership, err error) {
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		result, err = r.updateMembership(ctx, q, command)
		return err
	})
	return result, err
}

func (r *AccommodationRepository) updateMembership(
	ctx context.Context,
	q generated.Querier,
	command accommodation.UpdateMembershipCommand,
) (result accommodation.Membership, err error) {
	current, err := managerAccommodation(ctx, q, command.AccommodationID, command.Actor)
	if err != nil {
		return result, err
	}
	rows, err := q.LockMembershipSetForManager(ctx, generated.LockMembershipSetForManagerParams{
		AccommodationID: pgUUID(command.AccommodationID),
		OidcIssuer:      command.Actor.Issuer, OidcSubject: command.Actor.Subject,
	})
	if err != nil || len(rows) == 0 {
		return result, accommodationQueryError(err)
	}
	nextRole, nextActive, err := plannedMembership(rows, command)
	if err != nil {
		return result, err
	}
	err = r.updateLockedMembership(
		ctx, q, command, nextRole, nextActive, current.OrganizationID, &result,
	)
	return result, err
}

func (r *AccommodationRepository) updateLockedMembership(
	ctx context.Context,
	q generated.Querier,
	command accommodation.UpdateMembershipCommand,
	nextRole accommodation.Role,
	nextActive bool,
	organizationID pgtype.UUID,
	result *accommodation.Membership,
) error {
	now := r.store.currentTime()
	row, err := q.UpdateAccommodationMembership(ctx, generated.UpdateAccommodationMembershipParams{
		NextRole: string(nextRole), NextActive: nextActive, UpdatedAt: pgTime(now),
		MembershipID: pgUUID(command.MembershipID), AccommodationID: pgUUID(command.AccommodationID),
		ExpectedVersion: command.ExpectedVersion,
		OidcIssuer:      command.Actor.Issuer, OidcSubject: command.Actor.Subject,
	})
	if err != nil {
		return accommodationMutationError(err)
	}
	*result = membershipFromUpdate(row)
	return r.store.recordMembershipUpdate(ctx, q, command, organizationID, *result, now)
}

func accommodationPage(
	rows []generated.ListAccessibleAccommodationsRow,
	limit int32,
) accommodation.AccommodationPage {
	more := len(rows) > int(limit)
	if more {
		rows = rows[:limit]
	}
	items := make([]accommodation.Accommodation, 0, len(rows))
	for _, row := range rows {
		items = append(items, accommodationFromList(row))
	}
	var cursor *accommodation.PageCursor
	if more {
		last := items[len(items)-1]
		cursor = &accommodation.PageCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return accommodation.AccommodationPage{Items: items, NextCursor: cursor}
}

func membershipPage(
	rows []generated.ListAccommodationMembershipsRow,
	limit int32,
) accommodation.MembershipPage {
	more := len(rows) > int(limit)
	if more {
		rows = rows[:limit]
	}
	items := make([]accommodation.Membership, 0, len(rows))
	for _, row := range rows {
		items = append(items, membershipFromList(row))
	}
	var cursor *accommodation.PageCursor
	if more {
		last := items[len(items)-1]
		cursor = &accommodation.PageCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return accommodation.MembershipPage{Items: items, NextCursor: cursor}
}

func accommodationKey(id uuid.UUID, actor access.Principal) generated.GetAccessibleAccommodationParams {
	return generated.GetAccessibleAccommodationParams{
		AccommodationID: pgUUID(id), OidcIssuer: actor.Issuer, OidcSubject: actor.Subject,
	}
}

func updateAccommodationParams(
	command accommodation.UpdateCommand,
	now time.Time,
) generated.UpdateAccommodationParams {
	patch := command.Patch
	return generated.UpdateAccommodationParams{
		SetName: patch.SetName, Name: patch.Name,
		SetCategory: patch.SetCategory, Category: string(patch.Category),
		SetCapacity: patch.SetCapacity, Capacity: patch.Capacity,
		SetPublicAreaCode: patch.SetPublicAreaCode, PublicAreaCode: patch.PublicAreaCode,
		UpdatedAt: pgTime(now), AccommodationID: pgUUID(command.AccommodationID),
		ExpectedVersion: command.ExpectedVersion,
		OidcIssuer:      command.Actor.Issuer, OidcSubject: command.Actor.Subject,
	}
}

func accommodationIdempotencySpec(
	command accommodation.CreateCommand,
	now time.Time,
) idempotencySpec {
	request := struct {
		Name               string                 `json:"name"`
		Category           accommodation.Category `json:"category"`
		Capacity           int32                  `json:"capacity"`
		ClientSubmissionID uuid.UUID              `json:"client_submission_id"`
	}{command.Name, command.Category, command.Capacity, command.ClientSubmissionID}
	return idempotencySpec{
		actorValue: actorValue(command.Actor.Issuer, command.Actor.Subject),
		operation:  idempotency.OperationCreateAccommodation,
		resourceID: accommodationOnboardingResourceID,
		key:        command.IdempotencyKey,
		request:    request,
		now:        now,
	}
}

func (s *Store) accommodationOnboardingLockKey(actor access.Principal) (string, error) {
	key, ok := s.phase2.ActorKeys.Key(s.phase2.ActorKeys.CurrentVersion)
	if !ok {
		return "", accommodation.ErrUnavailable
	}
	digest := keyedDigest(
		key,
		"accommodation-onboarding-lock",
		actorValue(actor.Issuer, actor.Subject),
	)
	return hex.EncodeToString(digest), nil
}

// The whole membership set is locked so the last active manager cannot be
// demoted by two concurrent updates.
func plannedMembership(
	rows []generated.LockMembershipSetForManagerRow,
	command accommodation.UpdateMembershipCommand,
) (accommodation.Role, bool, error) {
	target, activeManagers, found := membershipTarget(rows, command.MembershipID)
	if !found {
		return "", false, accommodation.ErrNotFound
	}
	if target.Version != command.ExpectedVersion {
		return "", false, accommodation.ErrPreconditionFailed
	}
	nextRole, nextActive := nextMembership(target, command.Patch)
	change := accommodation.MembershipChange{
		CurrentRole: accommodation.Role(target.Role), CurrentActive: target.Active,
		NextRole: nextRole, NextActive: nextActive,
	}
	if err := change.Validate(activeManagers); err != nil {
		return "", false, accommodation.ErrConflict
	}
	return nextRole, nextActive, nil
}

func membershipTarget(
	rows []generated.LockMembershipSetForManagerRow,
	id uuid.UUID,
) (generated.LockMembershipSetForManagerRow, int, bool) {
	activeManagers := 0
	var target generated.LockMembershipSetForManagerRow
	found := false
	for _, row := range rows {
		if activeManager(row) {
			activeManagers++
		}
		if row.ID.Valid && uuid.UUID(row.ID.Bytes) == id {
			target, found = row, true
		}
	}
	return target, activeManagers, found
}

func activeManager(row generated.LockMembershipSetForManagerRow) bool {
	return row.Active && row.Role == string(accommodation.RoleManager)
}

func nextMembership(
	current generated.LockMembershipSetForManagerRow,
	patch accommodation.UpdateMembershipPatch,
) (accommodation.Role, bool) {
	role := accommodation.Role(current.Role)
	active := current.Active
	if patch.SetRole {
		role = patch.Role
	}
	if patch.SetActive {
		active = patch.Active
	}
	return role, active
}

func membershipIdempotencySpec(
	command accommodation.CreateMembershipCommand,
	now time.Time,
) idempotencySpec {
	request := struct {
		Issuer  string             `json:"oidc_issuer"`
		Subject string             `json:"oidc_subject"`
		Role    accommodation.Role `json:"role"`
	}{command.TargetIssuer, command.TargetSubject, command.Role}
	return idempotencySpec{
		actorValue: actorValue(command.Actor.Issuer, command.Actor.Subject),
		operation:  idempotency.OperationCreateMembership,
		resourceID: command.AccommodationID, key: command.IdempotencyKey,
		request: request, now: now,
	}
}

func accommodationQueryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) || err == nil {
		return accommodation.ErrNotFound
	}
	return accommodation.ErrUnavailable
}

func accommodationMutationPolicy(
	status accommodation.Status,
	role accommodation.Role,
	operation accommodation.Operation,
) error {
	if role != accommodation.RoleManager {
		return accommodation.ErrNotFound
	}
	if !status.Allows(operation) {
		return accommodation.ErrConflict
	}
	return nil
}

func accommodationCommandError(
	ctx context.Context,
	q generated.Querier,
	command accommodation.UpdateCommand,
	mutationErr error,
) error {
	if !errors.Is(mutationErr, pgx.ErrNoRows) {
		return accommodationMutationError(mutationErr)
	}
	current, err := q.GetAccessibleAccommodation(
		ctx,
		accommodationKey(command.AccommodationID, command.Actor),
	)
	if err != nil {
		return accommodationQueryError(err)
	}
	if current.Version != command.ExpectedVersion {
		return accommodation.ErrPreconditionFailed
	}
	policyErr := accommodationMutationPolicy(
		accommodation.Status(current.Status),
		accommodation.Role(current.ActorRole),
		accommodation.OperationUpdateAccommodation,
	)
	if policyErr != nil {
		return policyErr
	}
	return accommodation.ErrConflict
}

var knownAccommodationErrors = []error{
	accommodation.ErrNotFound, accommodation.ErrForbidden,
	accommodation.ErrPreconditionFailed, accommodation.ErrConflict,
}

func accommodationConflict(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) ||
		errors.Is(err, errIdempotencyConflict) ||
		isUniqueViolation(err)
}

func accommodationMutationError(err error) error {
	if errors.Is(err, idempotency.ErrProcessing) {
		return err
	}
	if accommodationConflict(err) {
		return accommodation.ErrConflict
	}
	if _, ok := firstKnownError(err, knownAccommodationErrors); ok {
		return err
	}
	return accommodation.ErrUnavailable
}

func optionalTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: !value.IsZero()}
}

func accommodationFromGet(row generated.GetAccessibleAccommodationRow) accommodation.Accommodation {
	return accommodation.Accommodation{
		ID: uuid.UUID(row.ID.Bytes), OrganizationID: uuid.UUID(row.OrganizationID.Bytes),
		Name: row.Name, Category: accommodation.Category(row.Category), Status: accommodation.Status(row.Status),
		CadasturID: row.CadasturID, Capacity: row.Capacity, PublicAreaCode: row.PublicAreaCode,
		Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func accommodationFromList(row generated.ListAccessibleAccommodationsRow) accommodation.Accommodation {
	return accommodation.Accommodation{
		ID: uuid.UUID(row.ID.Bytes), OrganizationID: uuid.UUID(row.OrganizationID.Bytes),
		Name: row.Name, Category: accommodation.Category(row.Category), Status: accommodation.Status(row.Status),
		CadasturID: row.CadasturID, Capacity: row.Capacity, PublicAreaCode: row.PublicAreaCode,
		Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func accommodationFromUpdate(row generated.UpdateAccommodationRow) accommodation.Accommodation {
	return accommodation.Accommodation{
		ID: uuid.UUID(row.ID.Bytes), OrganizationID: uuid.UUID(row.OrganizationID.Bytes),
		Name: row.Name, Category: accommodation.Category(row.Category), Status: accommodation.Status(row.Status),
		CadasturID: row.CadasturID, Capacity: row.Capacity, PublicAreaCode: row.PublicAreaCode,
		Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func accommodationFromOnboarding(
	row generated.InsertOnboardingAccommodationRow,
) accommodation.Accommodation {
	return accommodation.Accommodation{
		ID: uuid.UUID(row.ID.Bytes), OrganizationID: uuid.UUID(row.OrganizationID.Bytes),
		Name: row.Name, Category: accommodation.Category(row.Category),
		Status: accommodation.Status(row.Status), CadasturID: row.CadasturID,
		Capacity: row.Capacity, PublicAreaCode: row.PublicAreaCode,
		Version: row.Version, CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func membershipFromList(row generated.ListAccommodationMembershipsRow) accommodation.Membership {
	return accommodation.Membership{
		ID: uuid.UUID(row.ID.Bytes), AccommodationID: uuid.UUID(row.AccommodationID.Bytes),
		OIDCIssuer: row.OidcIssuer, OIDCSubject: row.OidcSubject,
		Role: accommodation.Role(row.Role), Active: row.Active, Version: row.Version,
		CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func membershipFromUpdate(row generated.UpdateAccommodationMembershipRow) accommodation.Membership {
	return accommodation.Membership{
		ID: uuid.UUID(row.ID.Bytes), AccommodationID: uuid.UUID(row.AccommodationID.Bytes),
		OIDCIssuer: row.OidcIssuer, OIDCSubject: row.OidcSubject,
		Role: accommodation.Role(row.Role), Active: row.Active, Version: row.Version,
		CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func membershipCreated(row generated.CreateAccommodationMembershipRow) accommodation.MembershipCreated {
	return accommodation.MembershipCreated{
		ID: uuid.UUID(row.ID.Bytes), AccommodationID: uuid.UUID(row.AccommodationID.Bytes),
		Role: accommodation.Role(row.Role), Active: row.Active, Version: row.Version,
		CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func (s *Store) recordAccommodationMutation(
	ctx context.Context,
	q generated.Querier,
	command accommodation.UpdateCommand,
	organizationID pgtype.UUID,
	result accommodation.Accommodation,
) error {
	return s.recordEvents(ctx, q, eventSpec{
		actorType: audit.ActorUser, actorIssuer: command.Actor.Issuer,
		actorSubject: command.Actor.Subject, organization: uuid.UUID(organizationID.Bytes),
		action: audit.ActionAccommodationUpdated, entityType: audit.EntityAccommodation,
		entityID: result.ID, requestID: command.RequestID,
		changedFields: accommodationChangedFields(command.Patch), version: result.Version,
		aggregateType: outbox.AggregateAccommodation,
		eventType:     outbox.EventAccommodationUpdated, now: result.UpdatedAt,
	})
}

func (s *Store) recordAccommodationCreate(
	ctx context.Context,
	q generated.Querier,
	command accommodation.CreateCommand,
	result accommodation.Accommodation,
) error {
	return s.recordEvents(ctx, q, eventSpec{
		actorType: audit.ActorUser, actorIssuer: command.Actor.Issuer,
		actorSubject: command.Actor.Subject, organization: result.OrganizationID,
		action: audit.ActionAccommodationCreated, entityType: audit.EntityAccommodation,
		entityID: result.ID, requestID: command.RequestID,
		changedFields: []audit.ChangedField{
			audit.FieldName,
			audit.FieldCategory,
			audit.FieldCapacity,
			audit.FieldStatus,
		},
		version: result.Version, aggregateType: outbox.AggregateAccommodation,
		eventType: outbox.EventAccommodationCreated, now: result.CreatedAt,
	})
}

func (s *Store) recordMembershipCreate(
	ctx context.Context,
	q generated.Querier,
	command accommodation.CreateMembershipCommand,
	organizationID pgtype.UUID,
	result accommodation.MembershipCreated,
	now time.Time,
) error {
	return s.recordEvents(ctx, q, eventSpec{
		actorType: audit.ActorUser, actorIssuer: command.Actor.Issuer,
		actorSubject: command.Actor.Subject, organization: uuid.UUID(organizationID.Bytes),
		action: audit.ActionMembershipCreated, entityType: audit.EntityMembership,
		entityID: result.ID, requestID: command.RequestID,
		changedFields: []audit.ChangedField{audit.FieldRole, audit.FieldActive},
		version:       result.Version, aggregateType: outbox.AggregateMembership,
		eventType: outbox.EventMembershipCreated, now: now,
	})
}

func (s *Store) recordMembershipUpdate(
	ctx context.Context,
	q generated.Querier,
	command accommodation.UpdateMembershipCommand,
	organizationID pgtype.UUID,
	result accommodation.Membership,
	now time.Time,
) error {
	return s.recordEvents(ctx, q, eventSpec{
		actorType: audit.ActorUser, actorIssuer: command.Actor.Issuer,
		actorSubject: command.Actor.Subject, organization: uuid.UUID(organizationID.Bytes),
		action: audit.ActionMembershipUpdated, entityType: audit.EntityMembership,
		entityID: result.ID, requestID: command.RequestID,
		changedFields: membershipChangedFields(command.Patch), version: result.Version,
		aggregateType: outbox.AggregateMembership,
		eventType:     outbox.EventMembershipUpdated, now: now,
	})
}

func accommodationChangedFields(patch accommodation.UpdatePatch) []audit.ChangedField {
	fields := make([]audit.ChangedField, 0, 4)
	if patch.SetName {
		fields = append(fields, audit.FieldName)
	}
	if patch.SetCategory {
		fields = append(fields, audit.FieldCategory)
	}
	if patch.SetCapacity {
		fields = append(fields, audit.FieldCapacity)
	}
	if patch.SetPublicAreaCode {
		fields = append(fields, audit.FieldPublicAreaCode)
	}
	return fields
}

func membershipChangedFields(patch accommodation.UpdateMembershipPatch) []audit.ChangedField {
	fields := make([]audit.ChangedField, 0, 2)
	if patch.SetRole {
		fields = append(fields, audit.FieldRole)
	}
	if patch.SetActive {
		fields = append(fields, audit.FieldActive)
	}
	return fields
}
