package calendarfeed

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// DueFeed is what the worker reads. It carries the sealed address and not the
// address: opening it is a deliberate step, taken once, right before the
// request that needs it.
type DueFeed struct {
	ID              uuid.UUID
	AccommodationID uuid.UUID
	Provider        Provider
	URL             SealedURL
	Failures        int32
	Version         int64
}

// Observed is one reservation as the origin shows it today. It has no identity
// and no guest count because the calendar has neither.
type Observed struct {
	UID         Fingerprint
	ArrivalOn   string
	DepartureOn string
	Kind        ReservationKind
}

// SyncResult is the whole outcome of one feed, applied in a single transaction.
//
// Reconciliation happens only when Outcome is OutcomeOK. Then Observed is the
// complete picture the origin shows, and everything pending that is absent from
// it is withdrawn from the queue — while nothing confirmed is touched, because a
// booking cancelled on the platform simply stops appearing, and that is not the
// same statement as "the stay did not happen" (ADR-044).
//
// On any other outcome Observed is empty and means "unknown", not "none": the
// queue is left exactly as it was, and only the feed's own failure fields move.
type SyncResult struct {
	FeedID   uuid.UUID
	Version  int64
	Outcome  SyncOutcome
	Observed []Observed
	Suspend  bool
}

type Fetcher interface {
	Fetch(ctx context.Context, url string) (string, error)
}

type SyncRepository interface {
	DueFeeds(ctx context.Context, limit int32) ([]DueFeed, error)
	ApplySync(ctx context.Context, result SyncResult) error
}

// Synchronizer is the worker half of the slice. It never writes a stay and
// never reads one: it reconciles the queue of observations with what the origin
// shows today.
type Synchronizer struct {
	repository SyncRepository
	fetcher    Fetcher
	sealer     *URLSealer
}

func NewSynchronizer(
	repository SyncRepository,
	fetcher Fetcher,
	sealer *URLSealer,
) (*Synchronizer, error) {
	if repository == nil || fetcher == nil || sealer == nil {
		return nil, ErrInvalidInput
	}
	return &Synchronizer{repository: repository, fetcher: fetcher, sealer: sealer}, nil
}

// SyncDue returns how many feeds it touched so the caller can decide whether a
// further batch is worth the budget.
func (s *Synchronizer) SyncDue(ctx context.Context, limit int32) (int, error) {
	feeds, err := s.repository.DueFeeds(ctx, limit)
	if err != nil {
		return 0, err
	}
	for _, feed := range feeds {
		if err := s.syncFeed(ctx, feed); err != nil {
			return 0, err
		}
	}
	return len(feeds), nil
}

// syncFeed swallows no failure: a feed that cannot be read produces a result
// that says so, which the lodging sees on its own screen. What it does refuse
// to do is let one broken feed stop the others, because the failure belongs to
// one address and not to the cycle.
func (s *Synchronizer) syncFeed(ctx context.Context, feed DueFeed) error {
	result := s.readFeed(ctx, feed)
	return s.repository.ApplySync(ctx, result)
}

func (s *Synchronizer) readFeed(ctx context.Context, feed DueFeed) SyncResult {
	observed, err := s.observe(ctx, feed)
	if err != nil {
		return failureResult(feed, err)
	}
	return SyncResult{
		FeedID:   feed.ID,
		Version:  feed.Version,
		Outcome:  OutcomeOK,
		Observed: observed,
	}
}

func (s *Synchronizer) observe(ctx context.Context, feed DueFeed) ([]Observed, error) {
	address, err := s.sealer.Open(feed.URL, feed.AccommodationID[:])
	if err != nil {
		return nil, err
	}
	body, err := s.fetcher.Fetch(ctx, address)
	if err != nil {
		return nil, err
	}
	events, err := ParseCalendar(body)
	if err != nil {
		return nil, err
	}
	return s.blindEvents(feed.ID, events)
}

func (s *Synchronizer) blindEvents(feedID uuid.UUID, events []Event) ([]Observed, error) {
	observed := make([]Observed, 0, len(events))
	for _, event := range events {
		uid, err := s.sealer.FingerprintUID(feedID.String(), event.UID)
		if err != nil {
			return nil, err
		}
		observed = append(observed, Observed{
			UID:         uid,
			ArrivalOn:   event.Arrival.String(),
			DepartureOn: event.Departure.String(),
			Kind:        event.Kind,
		})
	}
	return observed, nil
}

// failureResult keeps the previous queue untouched. A feed that failed says
// nothing about the reservations already imported, so withdrawing them on a
// network error would empty the screen every time the extranet hiccups.
func failureResult(feed DueFeed, err error) SyncResult {
	return SyncResult{
		FeedID:   feed.ID,
		Version:  feed.Version,
		Outcome:  classifyFailure(err),
		Observed: nil,
		Suspend:  feed.Failures+1 >= SuspendAfterFailures,
	}
}

func classifyFailure(err error) SyncOutcome {
	switch {
	case errors.Is(err, ErrNotCalendar):
		return OutcomeNotCalendar
	case errors.Is(err, ErrMalformed):
		return OutcomeMalformed
	default:
		return OutcomeUnreachable
	}
}
