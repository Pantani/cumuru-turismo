package calendarfeed_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/calendarfeed"
	"github.com/google/uuid"
)

var feedAccommodation = uuid.MustParse("019f0000-0000-7000-8000-000000000001")

func TestSyncDueBlindsEveryObservedReservation(t *testing.T) {
	t.Parallel()

	repository := &syncRepository{feeds: []calendarfeed.DueFeed{}}
	sealer := testSealer(t)
	repository.feeds = append(repository.feeds, dueFeed(t, sealer))
	synchronizer := newSynchronizer(t, repository, stubFetcher(bookingCalendar), sealer)

	count, err := synchronizer.SyncDue(context.Background(), 10)
	if err != nil || count != 1 {
		t.Fatalf("SyncDue() = %d, %v", count, err)
	}
	applied := repository.applied[0]
	if applied.Outcome != calendarfeed.OutcomeOK || len(applied.Observed) != 2 {
		t.Fatalf("ApplySync() got %+v", applied)
	}
	if len(applied.Observed[0].UID.Digest) != 32 {
		t.Fatalf("observed uid was not blinded: %+v", applied.Observed[0])
	}
	if applied.Observed[1].ArrivalOn != "2026-09-01" {
		t.Fatalf("observed arrival = %s", applied.Observed[1].ArrivalOn)
	}
}

// A network failure says nothing about the reservations already imported, so
// the queue has to survive it untouched — otherwise the screen empties itself
// every time the extranet hiccups.
func TestSyncDueKeepsTheQueueWhenTheFeedCannotBeRead(t *testing.T) {
	t.Parallel()

	sealer := testSealer(t)
	repository := &syncRepository{feeds: []calendarfeed.DueFeed{dueFeed(t, sealer)}}
	synchronizer := newSynchronizer(t, repository, failingFetcher{}, sealer)

	if _, err := synchronizer.SyncDue(context.Background(), 10); err != nil {
		t.Fatalf("SyncDue() error = %v", err)
	}
	applied := repository.applied[0]
	if applied.Outcome != calendarfeed.OutcomeUnreachable {
		t.Fatalf("ApplySync() outcome = %s", applied.Outcome)
	}
	if applied.Observed != nil {
		t.Fatalf("ApplySync() observed = %+v, want nil", applied.Observed)
	}
	if applied.Suspend {
		t.Fatal("ApplySync() suspended a feed on its first failure")
	}
}

// An expired feed URL redirects to a login page, and the lodging has to see
// that distinct from a dead host: one is fixed by pasting a new address, the
// other by waiting.
func TestSyncDueReportsAResponseThatIsNotACalendar(t *testing.T) {
	t.Parallel()

	sealer := testSealer(t)
	repository := &syncRepository{feeds: []calendarfeed.DueFeed{dueFeed(t, sealer)}}
	synchronizer := newSynchronizer(t, repository, stubFetcher("<html>Sign in</html>"), sealer)

	if _, err := synchronizer.SyncDue(context.Background(), 10); err != nil {
		t.Fatalf("SyncDue() error = %v", err)
	}
	if repository.applied[0].Outcome != calendarfeed.OutcomeNotCalendar {
		t.Fatalf("ApplySync() outcome = %s", repository.applied[0].Outcome)
	}
}

// Retrying a dead address forever turns the operator's mistake into our
// outbound traffic; the feed suspends and says so.
func TestSyncDueSuspendsAFeedThatKeepsFailing(t *testing.T) {
	t.Parallel()

	sealer := testSealer(t)
	feed := dueFeed(t, sealer)
	feed.Failures = calendarfeed.SuspendAfterFailures - 1
	repository := &syncRepository{feeds: []calendarfeed.DueFeed{feed}}
	synchronizer := newSynchronizer(t, repository, failingFetcher{}, sealer)

	if _, err := synchronizer.SyncDue(context.Background(), 10); err != nil {
		t.Fatalf("SyncDue() error = %v", err)
	}
	if !repository.applied[0].Suspend {
		t.Fatal("ApplySync() did not suspend after the threshold")
	}
}

func newSynchronizer(
	t *testing.T,
	repository calendarfeed.SyncRepository,
	fetcher calendarfeed.Fetcher,
	sealer *calendarfeed.URLSealer,
) *calendarfeed.Synchronizer {
	t.Helper()
	synchronizer, err := calendarfeed.NewSynchronizer(repository, fetcher, sealer)
	if err != nil {
		t.Fatalf("NewSynchronizer() error = %v", err)
	}
	return synchronizer
}

func dueFeed(t *testing.T, sealer *calendarfeed.URLSealer) calendarfeed.DueFeed {
	t.Helper()
	sealed, err := sealer.Seal(
		"https://ical.booking.com/v1/export?t=9f2a", feedAccommodation[:],
	)
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	return calendarfeed.DueFeed{
		ID:              uuid.MustParse("019f0000-0000-7000-8000-0000000000f1"),
		AccommodationID: feedAccommodation,
		Provider:        calendarfeed.ProviderBooking,
		URL:             sealed,
		Version:         1,
	}
}

type syncRepository struct {
	feeds   []calendarfeed.DueFeed
	applied []calendarfeed.SyncResult
}

func (r *syncRepository) DueFeeds(context.Context, int32) ([]calendarfeed.DueFeed, error) {
	return r.feeds, nil
}

func (r *syncRepository) ApplySync(_ context.Context, result calendarfeed.SyncResult) error {
	r.applied = append(r.applied, result)
	return nil
}

type stubFetcher string

func (f stubFetcher) Fetch(context.Context, string) (string, error) {
	return string(f), nil
}

type failingFetcher struct{}

func (failingFetcher) Fetch(context.Context, string) (string, error) {
	return "", errors.New("dial tcp: connection refused")
}
