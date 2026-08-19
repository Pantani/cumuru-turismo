package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/audit"
	"github.com/Pantani/cumuru/apps/api/internal/calendarfeed"
	"github.com/Pantani/cumuru/apps/api/internal/platform/idempotency"
	"github.com/Pantani/cumuru/apps/api/internal/platform/outbox"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// CalendarFeedRepository serves both halves of the slice: the application, which
// registers feeds and turns an observation into a stay, and the worker, which
// reconciles the queue with what the origin shows. They share a table and
// nothing else — the worker never writes stay_id, and the application never
// fetches a calendar.
type CalendarFeedRepository struct {
	store *Store
}

func NewCalendarFeedRepository(store *Store) *CalendarFeedRepository {
	return &CalendarFeedRepository{store: store}
}

var (
	_ calendarfeed.Repository     = (*CalendarFeedRepository)(nil)
	_ calendarfeed.SyncRepository = (*CalendarFeedRepository)(nil)
)

func (r *CalendarFeedRepository) CreateFeed(
	ctx context.Context,
	command calendarfeed.CreateFeedCommand,
	sealed calendarfeed.SealedURL,
	fingerprint calendarfeed.Fingerprint,
) (feed calendarfeed.Feed, replayed bool, err error) {
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		spec := createCalendarFeedIdempotency(command, fingerprint, now)
		idempotent, runErr := r.store.runIdempotent(ctx, q, spec, func() (storedMutation, error) {
			return r.writeCalendarFeed(ctx, q, command, sealed, fingerprint, now)
		})
		if runErr != nil {
			return calendarFeedMutationError(runErr)
		}
		replayed = idempotent.replayed
		return decodeCalendarJSON(idempotent.response.body, &feed)
	})
	return feed, replayed, err
}

// The hashed body carries the fingerprint and not the address: the digest is
// persisted, and an unkeyed hash of a bearer URL would outlive the row that
// holds it sealed.
func createCalendarFeedIdempotency(
	command calendarfeed.CreateFeedCommand,
	fingerprint calendarfeed.Fingerprint,
	now time.Time,
) idempotencySpec {
	return idempotencySpec{
		actorValue: actorValue(command.Actor.Issuer, command.Actor.Subject),
		operation:  idempotency.OperationCreateCalendarFeed,
		resourceID: command.AccommodationID, key: command.IdempotencyKey,
		request: struct {
			AccommodationID uuid.UUID `json:"accommodation_id"`
			Provider        string    `json:"provider"`
			Label           string    `json:"label"`
			URLFingerprint  []byte    `json:"url_fingerprint"`
		}{
			command.AccommodationID, string(command.Provider),
			command.Label, fingerprint.Digest,
		},
		now: now,
	}
}

func (r *CalendarFeedRepository) writeCalendarFeed(
	ctx context.Context,
	q generated.Querier,
	command calendarfeed.CreateFeedCommand,
	sealed calendarfeed.SealedURL,
	fingerprint calendarfeed.Fingerprint,
	now time.Time,
) (storedMutation, error) {
	property, err := q.GetAccessibleAccommodation(
		ctx, accommodationKey(command.AccommodationID, command.Actor),
	)
	if err != nil {
		return storedMutation{}, calendarFeedQueryError(err)
	}
	feed, err := r.insertCalendarFeed(ctx, q, command, sealed, fingerprint, now)
	if err != nil {
		return storedMutation{}, err
	}
	return r.finishFeedRegistration(ctx, q, command, property, feed, now)
}

func (r *CalendarFeedRepository) finishFeedRegistration(
	ctx context.Context,
	q generated.Querier,
	command calendarfeed.CreateFeedCommand,
	property generated.GetAccessibleAccommodationRow,
	feed calendarfeed.Feed,
	now time.Time,
) (storedMutation, error) {
	err := r.store.recordEvents(ctx, q, calendarFeedEvent(calendarEventInput{
		actor: command.Actor, organization: idFromPG(property.OrganizationID),
		action: audit.ActionCalendarFeedRegistered,
		event:  outbox.EventCalendarFeedRegistered,
		entity: feed.ID, version: feed.Version,
		requestID: command.RequestID, now: now,
	}))
	if err != nil {
		return storedMutation{}, err
	}
	return jsonMutationValue(201, feed.ID, feed, map[string]string{
		"ETag": entityTag(feed.Version),
	})
}

// A feed may only be registered against an accommodation whose status still
// allows stays: an address that will never produce one is outbound traffic with
// no destination.
func (r *CalendarFeedRepository) insertCalendarFeed(
	ctx context.Context,
	q generated.Querier,
	command calendarfeed.CreateFeedCommand,
	sealed calendarfeed.SealedURL,
	fingerprint calendarfeed.Fingerprint,
	now time.Time,
) (calendarfeed.Feed, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return calendarfeed.Feed{}, calendarfeed.ErrUnavailable
	}
	row, err := q.CreateCalendarFeed(ctx, generated.CreateCalendarFeedParams{
		FeedID: idToPG(id), AccommodationID: idToPG(command.AccommodationID),
		Provider: string(command.Provider), Label: command.Label,
		UrlCiphertext: sealed.Ciphertext, UrlNonce: sealed.Nonce,
		UrlKeyVersion:            sealed.KeyVersion,
		UrlFingerprint:           fingerprint.Digest,
		UrlFingerprintKeyVersion: fingerprint.KeyVersion,
		Now:                      timeToPG(now),
	})
	if err != nil {
		return calendarfeed.Feed{}, calendarFeedMutationError(err)
	}
	return feedFromCreate(row), nil
}

func (r *CalendarFeedRepository) ListFeeds(
	ctx context.Context,
	request calendarfeed.ListFeedsRequest,
) ([]calendarfeed.Feed, error) {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	if err := r.assertAccessible(ctx, request.AccommodationID, request.Actor); err != nil {
		return nil, err
	}
	rows, err := r.store.queries.ListCalendarFeeds(ctx, idToPG(request.AccommodationID))
	if err != nil {
		return nil, calendarfeed.ErrUnavailable
	}
	feeds := make([]calendarfeed.Feed, 0, len(rows))
	for _, row := range rows {
		feeds = append(feeds, feedFromList(row))
	}
	return feeds, nil
}

// assertAccessible is the authorization: membership on the accommodation, read
// from the same query every other slice uses, so a feed cannot be listed by
// naming somebody else's identifier.
func (r *CalendarFeedRepository) assertAccessible(
	ctx context.Context,
	accommodationID uuid.UUID,
	actor access.Principal,
) error {
	if _, err := r.store.queries.GetAccessibleAccommodation(
		ctx, accommodationKey(accommodationID, actor),
	); err != nil {
		return calendarFeedQueryError(err)
	}
	return nil
}

func (r *CalendarFeedRepository) RemoveFeed(
	ctx context.Context,
	command calendarfeed.RemoveFeedCommand,
) (feed calendarfeed.Feed, replayed bool, err error) {
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		spec := calendarDecisionIdempotency(
			command.Actor, command.FeedID, command.ExpectedVersion,
			command.IdempotencyKey, idempotency.OperationRemoveCalendarFeed, now,
		)
		idempotent, runErr := r.store.runIdempotent(ctx, q, spec, func() (storedMutation, error) {
			return r.removeCalendarFeed(ctx, q, command, now)
		})
		if runErr != nil {
			return calendarFeedMutationError(runErr)
		}
		replayed = idempotent.replayed
		return decodeCalendarJSON(idempotent.response.body, &feed)
	})
	return feed, replayed, err
}

func (r *CalendarFeedRepository) removeCalendarFeed(
	ctx context.Context,
	q generated.Querier,
	command calendarfeed.RemoveFeedCommand,
	now time.Time,
) (storedMutation, error) {
	locked, err := q.LockCalendarFeedForDecision(ctx, idToPG(command.FeedID))
	if err != nil {
		return storedMutation{}, calendarFeedQueryError(err)
	}
	property, err := q.GetAccessibleAccommodation(
		ctx, accommodationKey(idFromPG(locked.AccommodationID), command.Actor),
	)
	if err != nil {
		return storedMutation{}, calendarFeedQueryError(err)
	}
	return r.writeFeedRemoval(ctx, q, command, property, now)
}

func (r *CalendarFeedRepository) writeFeedRemoval(
	ctx context.Context,
	q generated.Querier,
	command calendarfeed.RemoveFeedCommand,
	property generated.GetAccessibleAccommodationRow,
	now time.Time,
) (storedMutation, error) {
	row, err := q.RemoveCalendarFeed(ctx, generated.RemoveCalendarFeedParams{
		FeedID: idToPG(command.FeedID), ExpectedVersion: command.ExpectedVersion,
		RemovedAt: timeToPG(now),
	})
	if err != nil {
		return storedMutation{}, calendarFeedMutationError(err)
	}
	feed := feedFromRemove(row)
	if err := r.store.recordEvents(ctx, q, calendarFeedEvent(calendarEventInput{
		actor: command.Actor, organization: idFromPG(property.OrganizationID),
		action: audit.ActionCalendarFeedRemoved,
		event:  outbox.EventCalendarFeedRemoved,
		entity: feed.ID, version: feed.Version,
		requestID: command.RequestID, now: now,
	})); err != nil {
		return storedMutation{}, err
	}
	return jsonMutationValue(200, feed.ID, feed, map[string]string{
		"ETag": entityTag(feed.Version),
	})
}

func (r *CalendarFeedRepository) ListReservations(
	ctx context.Context,
	request calendarfeed.ListReservationsRequest,
) ([]calendarfeed.Reservation, error) {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	if err := r.assertAccessible(ctx, request.AccommodationID, request.Actor); err != nil {
		return nil, err
	}
	rows, err := r.store.queries.ListCalendarReservations(
		ctx, listCalendarReservationsParams(request),
	)
	if err != nil {
		return nil, calendarfeed.ErrUnavailable
	}
	items := make([]calendarfeed.Reservation, 0, len(rows))
	for _, row := range rows {
		items = append(items, reservationFromList(row))
	}
	return items, nil
}

func listCalendarReservationsParams(
	request calendarfeed.ListReservationsRequest,
) generated.ListCalendarReservationsParams {
	var state *string
	if request.State != "" {
		value := string(request.State)
		state = &value
	}
	return generated.ListCalendarReservationsParams{
		AccommodationID: idToPG(request.AccommodationID),
		State:           state, PageLimit: request.Limit,
	}
}

// Confirm is the only place an observation becomes presence. The stay is
// created through the same statement the manual screen uses, in the same
// transaction that stamps the confirmation, so a confirmed row without a stay
// cannot exist and neither can a stay the queue does not point at.
func (r *CalendarFeedRepository) Confirm(
	ctx context.Context,
	command calendarfeed.ConfirmCommand,
) (reservation calendarfeed.Reservation, replayed bool, err error) {
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		spec := confirmCalendarReservationIdempotency(command, now)
		idempotent, runErr := r.store.runIdempotent(ctx, q, spec, func() (storedMutation, error) {
			return r.confirmReservation(ctx, q, command, now)
		})
		if runErr != nil {
			return calendarFeedMutationError(runErr)
		}
		replayed = idempotent.replayed
		return decodeCalendarJSON(idempotent.response.body, &reservation)
	})
	return reservation, replayed, err
}

func confirmCalendarReservationIdempotency(
	command calendarfeed.ConfirmCommand,
	now time.Time,
) idempotencySpec {
	return idempotencySpec{
		actorValue: actorValue(command.Actor.Issuer, command.Actor.Subject),
		operation:  idempotency.OperationConfirmCalendarReservation,
		resourceID: command.ReservationID, key: command.IdempotencyKey,
		request: struct {
			ReservationID      uuid.UUID `json:"reservation_id"`
			ExpectedVersion    int64     `json:"expected_version"`
			ExpectedGuestCount int32     `json:"expected_guest_count"`
			ClientSubmissionID uuid.UUID `json:"client_submission_id"`
		}{
			command.ReservationID, command.ExpectedVersion,
			command.ExpectedGuestCount, command.ClientSubmissionID,
		},
		now: now,
	}
}

func (r *CalendarFeedRepository) confirmReservation(
	ctx context.Context,
	q generated.Querier,
	command calendarfeed.ConfirmCommand,
	now time.Time,
) (storedMutation, error) {
	locked, err := q.LockCalendarReservationForDecision(ctx, idToPG(command.ReservationID))
	if err != nil {
		return storedMutation{}, calendarFeedQueryError(err)
	}
	property, err := q.GetAccessibleAccommodation(
		ctx, accommodationKey(idFromPG(locked.AccommodationID), command.Actor),
	)
	if err != nil {
		return storedMutation{}, calendarFeedQueryError(err)
	}
	stayID, err := r.createStayForReservation(ctx, q, command, locked, property)
	if err != nil {
		return storedMutation{}, err
	}
	return r.writeConfirmation(ctx, q, command, property, stayID, now)
}

func (r *CalendarFeedRepository) createStayForReservation(
	ctx context.Context,
	q generated.Querier,
	command calendarfeed.ConfirmCommand,
	locked generated.LockCalendarReservationForDecisionRow,
	property generated.GetAccessibleAccommodationRow,
) (uuid.UUID, error) {
	if !accommodation.Status(property.Status).Allows(accommodation.OperationCreateStay) {
		return uuid.Nil, calendarfeed.ErrConflict
	}
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, calendarfeed.ErrUnavailable
	}
	row, err := q.CreateStay(ctx, generated.CreateStayParams{
		StayID: idToPG(id), ClientSubmissionID: idToPG(command.ClientSubmissionID),
		PlannedArrivalOn: locked.ArrivalOn, PlannedDepartureOn: locked.DepartureOn,
		ExpectedGuestCount: command.ExpectedGuestCount,
		AccommodationID:    property.ID,
		OidcIssuer:         command.Actor.Issuer, OidcSubject: command.Actor.Subject,
	})
	if err != nil {
		return uuid.Nil, calendarFeedMutationError(err)
	}
	return idFromPG(row.ID), nil
}

// Two events, not one: the stay was created and the observation was confirmed
// are different facts, and collapsing them would leave the trail unable to say
// whether a stay was typed by hand or imported.
func (r *CalendarFeedRepository) writeConfirmation(
	ctx context.Context,
	q generated.Querier,
	command calendarfeed.ConfirmCommand,
	property generated.GetAccessibleAccommodationRow,
	stayID uuid.UUID,
	now time.Time,
) (storedMutation, error) {
	row, err := q.ConfirmCalendarReservation(ctx, generated.ConfirmCalendarReservationParams{
		ReservationID: idToPG(command.ReservationID), StayID: idToPG(stayID),
		ExpectedVersion: command.ExpectedVersion, DecidedAt: timeToPG(now),
	})
	if err != nil {
		return storedMutation{}, calendarFeedMutationError(err)
	}
	reservation := reservationFromConfirm(row)
	if err := r.recordConfirmationEvents(
		ctx, q, command, property, reservation, stayID, now,
	); err != nil {
		return storedMutation{}, err
	}
	return jsonMutationValue(200, reservation.ID, reservation, map[string]string{
		"ETag": entityTag(reservation.Version),
	})
}

func (r *CalendarFeedRepository) recordConfirmationEvents(
	ctx context.Context,
	q generated.Querier,
	command calendarfeed.ConfirmCommand,
	property generated.GetAccessibleAccommodationRow,
	reservation calendarfeed.Reservation,
	stayID uuid.UUID,
	now time.Time,
) error {
	organization := idFromPG(property.OrganizationID)
	if err := r.store.recordEvents(ctx, q, stayCreationEvent(calendarEventInput{
		actor: command.Actor, organization: organization,
		entity: stayID, version: 1, requestID: command.RequestID, now: now,
	})); err != nil {
		return err
	}
	return r.store.recordEvents(ctx, q, calendarReservationEvent(calendarEventInput{
		actor: command.Actor, organization: organization,
		action: audit.ActionCalendarReservationConfirmed,
		event:  outbox.EventCalendarReservationConfirmed,
		entity: reservation.ID, version: reservation.Version,
		requestID: command.RequestID, now: now,
	}))
}

func (r *CalendarFeedRepository) Dismiss(
	ctx context.Context,
	command calendarfeed.DismissCommand,
) (reservation calendarfeed.Reservation, replayed bool, err error) {
	now := r.store.currentTime()
	err = r.store.inTransaction(ctx, func(q generated.Querier) error {
		spec := calendarDecisionIdempotency(
			command.Actor, command.ReservationID, command.ExpectedVersion,
			command.IdempotencyKey, idempotency.OperationDismissCalendarReservation, now,
		)
		idempotent, runErr := r.store.runIdempotent(ctx, q, spec, func() (storedMutation, error) {
			return r.dismissReservation(ctx, q, command, now)
		})
		if runErr != nil {
			return calendarFeedMutationError(runErr)
		}
		replayed = idempotent.replayed
		return decodeCalendarJSON(idempotent.response.body, &reservation)
	})
	return reservation, replayed, err
}

func (r *CalendarFeedRepository) dismissReservation(
	ctx context.Context,
	q generated.Querier,
	command calendarfeed.DismissCommand,
	now time.Time,
) (storedMutation, error) {
	locked, err := q.LockCalendarReservationForDecision(ctx, idToPG(command.ReservationID))
	if err != nil {
		return storedMutation{}, calendarFeedQueryError(err)
	}
	property, err := q.GetAccessibleAccommodation(
		ctx, accommodationKey(idFromPG(locked.AccommodationID), command.Actor),
	)
	if err != nil {
		return storedMutation{}, calendarFeedQueryError(err)
	}
	return r.writeDismissal(ctx, q, command, property, now)
}

func (r *CalendarFeedRepository) writeDismissal(
	ctx context.Context,
	q generated.Querier,
	command calendarfeed.DismissCommand,
	property generated.GetAccessibleAccommodationRow,
	now time.Time,
) (storedMutation, error) {
	row, err := q.DismissCalendarReservation(ctx, generated.DismissCalendarReservationParams{
		ReservationID:   idToPG(command.ReservationID),
		ExpectedVersion: command.ExpectedVersion, DecidedAt: timeToPG(now),
	})
	if err != nil {
		return storedMutation{}, calendarFeedMutationError(err)
	}
	reservation := reservationFromDismiss(row)
	if err := r.store.recordEvents(ctx, q, calendarReservationEvent(calendarEventInput{
		actor: command.Actor, organization: idFromPG(property.OrganizationID),
		action: audit.ActionCalendarReservationDismissed,
		event:  outbox.EventCalendarReservationDismissed,
		entity: reservation.ID, version: reservation.Version,
		requestID: command.RequestID, now: now,
	})); err != nil {
		return storedMutation{}, err
	}
	return jsonMutationValue(200, reservation.ID, reservation, map[string]string{
		"ETag": entityTag(reservation.Version),
	})
}

func calendarDecisionIdempotency(
	actor access.Principal,
	resourceID uuid.UUID,
	expectedVersion int64,
	key string,
	operation idempotency.Operation,
	now time.Time,
) idempotencySpec {
	return idempotencySpec{
		actorValue: actorValue(actor.Issuer, actor.Subject),
		operation:  operation, resourceID: resourceID, key: key,
		request: struct {
			ResourceID      uuid.UUID `json:"resource_id"`
			ExpectedVersion int64     `json:"expected_version"`
		}{resourceID, expectedVersion},
		now: now,
	}
}

func jsonMutationValue(
	statusCode int,
	resourceID uuid.UUID,
	value any,
	headers map[string]string,
) (storedMutation, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return storedMutation{}, calendarfeed.ErrUnavailable
	}
	return storedMutation{
		status: statusCode, resourceID: resourceID, body: body, headers: headers,
	}, nil
}

func decodeCalendarJSON(body []byte, target any) error {
	if err := json.Unmarshal(body, target); err != nil {
		return calendarfeed.ErrUnavailable
	}
	return nil
}

type calendarEventInput struct {
	actor        access.Principal
	organization uuid.UUID
	action       audit.Action
	event        outbox.EventType
	entity       uuid.UUID
	version      int64
	requestID    string
	now          time.Time
}

func calendarFeedEvent(input calendarEventInput) eventSpec {
	return calendarEventSpec(input, audit.EntityCalendarFeed, outbox.AggregateCalendarFeed)
}

func calendarReservationEvent(input calendarEventInput) eventSpec {
	return calendarEventSpec(
		input, audit.EntityCalendarReservation, outbox.AggregateCalendarReservation,
	)
}

func stayCreationEvent(input calendarEventInput) eventSpec {
	input.action = audit.ActionStayCreated
	input.event = outbox.EventStayCreated
	return calendarEventSpec(input, audit.EntityStay, outbox.AggregateStay)
}

func calendarEventSpec(
	input calendarEventInput,
	entity audit.EntityType,
	aggregate outbox.AggregateType,
) eventSpec {
	return eventSpec{
		actorType: audit.ActorUser, actorIssuer: input.actor.Issuer,
		actorSubject: input.actor.Subject, organization: input.organization,
		action: input.action, entityType: entity, entityID: input.entity,
		requestID: input.requestID, version: input.version,
		aggregateType: aggregate, eventType: input.event,
		purpose: audit.PurposeFor(entity), now: input.now,
	}
}

func feedFromCreate(row generated.CreateCalendarFeedRow) calendarfeed.Feed {
	return calendarfeed.Feed{
		ID: idFromPG(row.ID), AccommodationID: idFromPG(row.AccommodationID),
		Provider: calendarfeed.Provider(row.Provider), Label: row.Label,
		Status:              calendarfeed.FeedStatus(row.Status),
		LastSyncedAt:        optionalInstant(row.LastSyncedAt),
		LastSyncOutcome:     optionalOutcome(row.LastSyncOutcome),
		ConsecutiveFailures: row.ConsecutiveFailures, Version: row.Version,
		CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

func feedFromList(row generated.ListCalendarFeedsRow) calendarfeed.Feed {
	return feedFromCreate(generated.CreateCalendarFeedRow(row))
}

func feedFromRemove(row generated.RemoveCalendarFeedRow) calendarfeed.Feed {
	return feedFromCreate(generated.CreateCalendarFeedRow(row))
}

func reservationFromList(row generated.ListCalendarReservationsRow) calendarfeed.Reservation {
	return calendarfeed.Reservation{
		ID: idFromPG(row.ID), FeedID: idFromPG(row.FeedID),
		ArrivalOn:   row.ArrivalOn.Time.Format(civilDateLayout),
		DepartureOn: row.DepartureOn.Time.Format(civilDateLayout),
		Kind:        calendarfeed.ReservationKind(row.Kind),
		State:       calendarfeed.ReservationState(row.State),
		StayID:      optionalID(row.StayID),
		FirstSeenAt: row.FirstSeenAt.Time.UTC(),
		LastSeenAt:  row.LastSeenAt.Time.UTC(), Version: row.Version,
	}
}

func reservationFromConfirm(row generated.ConfirmCalendarReservationRow) calendarfeed.Reservation {
	return reservationFromList(generated.ListCalendarReservationsRow(row))
}

func reservationFromDismiss(row generated.DismissCalendarReservationRow) calendarfeed.Reservation {
	return reservationFromList(generated.ListCalendarReservationsRow(row))
}

func optionalInstant(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	instant := value.Time.UTC()
	return &instant
}

func optionalOutcome(value *string) *calendarfeed.SyncOutcome {
	if value == nil {
		return nil
	}
	outcome := calendarfeed.SyncOutcome(*value)
	return &outcome
}

const civilDateLayout = "2006-01-02"

// calendarFeedQueryError keeps "absent" and "not mine" the same answer: the
// membership check is the authorization, and a distinct 403 would tell an
// outsider that somebody else's identifier exists.
func calendarFeedQueryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return calendarfeed.ErrNotFound
	}
	return calendarfeed.ErrUnavailable
}

func calendarFeedMutationError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, pgx.ErrNoRows):
		return calendarfeed.ErrConflict
	case isUniqueViolation(err):
		return calendarfeed.ErrConflict
	default:
		return calendarFeedDomainError(err)
	}
}

func calendarFeedDomainError(err error) error {
	if errors.Is(err, calendarfeed.ErrInvalidInput) ||
		errors.Is(err, calendarfeed.ErrNotFound) ||
		errors.Is(err, calendarfeed.ErrConflict) {
		return err
	}
	return calendarfeed.ErrUnavailable
}
