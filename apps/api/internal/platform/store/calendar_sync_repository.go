package store

import (
	"context"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/calendarfeed"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/google/uuid"
)

// DueFeeds reads without holding a lock, because the caller is about to make an
// outbound request with the result and a row lock held across somebody else's
// network would be a transaction waiting on a stranger's clock. Two cycles
// overlapping simply redo the same reconciliation, which is idempotent.
func (r *CalendarFeedRepository) DueFeeds(
	ctx context.Context,
	limit int32,
) ([]calendarfeed.DueFeed, error) {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	now := r.store.currentTime()
	rows, err := r.store.queries.ListDueCalendarFeeds(ctx, generated.ListDueCalendarFeedsParams{
		Cutoff: timeToPG(now.Add(-calendarfeed.SyncInterval)), BatchSize: limit,
	})
	if err != nil {
		return nil, calendarfeed.ErrUnavailable
	}
	feeds := make([]calendarfeed.DueFeed, 0, len(rows))
	for _, row := range rows {
		feeds = append(feeds, dueFeedFromRow(row))
	}
	return feeds, nil
}

func dueFeedFromRow(row generated.ListDueCalendarFeedsRow) calendarfeed.DueFeed {
	return calendarfeed.DueFeed{
		ID:              idFromPG(row.ID),
		AccommodationID: idFromPG(row.AccommodationID),
		Provider:        calendarfeed.Provider(row.Provider),
		URL: calendarfeed.SealedURL{
			Ciphertext: row.UrlCiphertext,
			Nonce:      row.UrlNonce,
			KeyVersion: row.UrlKeyVersion,
		},
		Failures: row.ConsecutiveFailures,
		Version:  row.Version,
	}
}

// ApplySync writes the whole outcome of one feed in one transaction. A failure
// touches only the feed's own fields: the queue says nothing about a network
// error, and emptying it on every hiccup would make the screen unusable.
// O prazo é aplicado aqui, e não só na abertura da transação: a reconciliação
// percorre um evento por sentença, e um calendário de dois anos tem centenas —
// sem deadline nas consultas o ciclo seguraria trava pelo tempo que o banco
// levasse.
func (r *CalendarFeedRepository) ApplySync(
	ctx context.Context,
	result calendarfeed.SyncResult,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	now := r.store.currentTime()
	return r.store.inTransaction(ctx, func(q generated.Querier) error {
		if result.Outcome != calendarfeed.OutcomeOK {
			return r.markFeedFailed(ctx, q, result, now)
		}
		return r.reconcileFeed(ctx, q, result, now)
	})
}

func (r *CalendarFeedRepository) markFeedFailed(
	ctx context.Context,
	q generated.Querier,
	result calendarfeed.SyncResult,
	now time.Time,
) error {
	err := q.MarkCalendarFeedFailed(ctx, generated.MarkCalendarFeedFailedParams{
		FeedID: idToPG(result.FeedID), SyncedAt: timeToPG(now),
		Outcome: optionalText(string(result.Outcome)), Suspend: result.Suspend,
		ExpectedVersion: result.Version,
	})
	if err != nil {
		return calendarfeed.ErrUnavailable
	}
	return nil
}

// The order matters: everything the origin still shows is stamped with this
// cycle's instant first, and only then is what kept an older instant withdrawn.
// Reversing it would withdraw the whole queue on every run.
func (r *CalendarFeedRepository) reconcileFeed(
	ctx context.Context,
	q generated.Querier,
	result calendarfeed.SyncResult,
	now time.Time,
) error {
	for _, observed := range result.Observed {
		if err := r.applyObserved(ctx, q, result.FeedID, observed, now); err != nil {
			return err
		}
	}
	if err := r.withdrawUnseen(ctx, q, result.FeedID, now); err != nil {
		return err
	}
	if err := q.MarkCalendarFeedSynced(ctx, generated.MarkCalendarFeedSyncedParams{
		FeedID: idToPG(result.FeedID), SyncedAt: timeToPG(now),
		ExpectedVersion: result.Version,
	}); err != nil {
		return calendarfeed.ErrUnavailable
	}
	return nil
}

// applyObserved is one statement per event on purpose: the upsert already
// covers the reservation that vanished and came back, so a calendar with
// hundreds of events costs hundreds of round trips instead of twice that.
func (r *CalendarFeedRepository) applyObserved(
	ctx context.Context,
	q generated.Querier,
	feedID uuid.UUID,
	observed calendarfeed.Observed,
	now time.Time,
) error {
	id, err := uuid.NewV7()
	if err != nil {
		return calendarfeed.ErrUnavailable
	}
	if err := q.UpsertCalendarReservation(
		ctx, upsertReservationParams(id, feedID, observed, now),
	); err != nil {
		return calendarfeed.ErrUnavailable
	}
	return nil
}

func upsertReservationParams(
	id uuid.UUID,
	feedID uuid.UUID,
	observed calendarfeed.Observed,
	now time.Time,
) generated.UpsertCalendarReservationParams {
	return generated.UpsertCalendarReservationParams{
		ReservationID: idToPG(id), FeedID: idToPG(feedID),
		ExternalUidHmac:       observed.UID.Digest,
		ExternalUidKeyVersion: observed.UID.KeyVersion,
		ArrivalOn:             dateToPG(observed.ArrivalOn),
		DepartureOn:           dateToPG(observed.DepartureOn),
		Kind:                  string(observed.Kind),
		SeenAt:                timeToPG(now),
	}
}

func (r *CalendarFeedRepository) withdrawUnseen(
	ctx context.Context,
	q generated.Querier,
	feedID uuid.UUID,
	now time.Time,
) error {
	err := q.WithdrawUnseenCalendarReservations(
		ctx, generated.WithdrawUnseenCalendarReservationsParams{
			FeedID: idToPG(feedID), WithdrawnAt: timeToPG(now),
			CycleStartedAt: timeToPG(now),
		},
	)
	if err != nil {
		return calendarfeed.ErrUnavailable
	}
	return nil
}
