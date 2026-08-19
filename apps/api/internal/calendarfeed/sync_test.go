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

// O comentário de SyncDue promete que um feed quebrado não para os outros. Com
// a falha vindo do repositório, e não da busca, essa promessa só vale se o laço
// continuar.
func TestSyncDueKeepsGoingWhenOneFeedFailsToPersist(t *testing.T) {
	t.Parallel()

	sealer := testSealer(t)
	first := dueFeed(t, sealer)
	second := dueFeed(t, sealer)
	second.ID = uuid.MustParse("019f0000-0000-7000-8000-0000000000f2")
	repository := &syncRepository{
		feeds:     []calendarfeed.DueFeed{first, second},
		failFeeds: map[uuid.UUID]bool{first.ID: true},
	}
	synchronizer := newSynchronizer(t, repository, stubFetcher(bookingCalendar), sealer)

	count, err := synchronizer.SyncDue(context.Background(), 10)
	if err == nil {
		t.Fatal("SyncDue() error = nil, want the failed feed reported")
	}
	if count != 1 {
		t.Fatalf("SyncDue() = %d, want the second feed still processed", count)
	}
}

// Um corpo que é calendário mas tem evento quebrado precisa chegar como
// malformed: é o que distingue "arquivo estranho" de "site fora do ar".
func TestSyncDueReportsAMalformedEvent(t *testing.T) {
	t.Parallel()

	sealer := testSealer(t)
	repository := &syncRepository{feeds: []calendarfeed.DueFeed{dueFeed(t, sealer)}}
	broken := "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\n" +
		"DTSTART;VALUE=DATE:20260818\r\nDTEND;VALUE=DATE:20260815\r\n" +
		"UID:booking-0001\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	synchronizer := newSynchronizer(t, repository, stubFetcher(broken), sealer)

	if _, err := synchronizer.SyncDue(context.Background(), 10); err != nil {
		t.Fatalf("SyncDue() error = %v", err)
	}
	if repository.applied[0].Outcome != calendarfeed.OutcomeMalformed {
		t.Fatalf("ApplySync() outcome = %s", repository.applied[0].Outcome)
	}
}

// Abrir a URL falha por motivo local — chave rotacionada, versão ausente. O
// feed remoto está saudável, e contabilizar isso como falha dele suspenderia
// todos eles apontando o operador para um endereço que nunca quebrou.
func TestSyncDueDoesNotPenalizeAFeedForALocalKeyFailure(t *testing.T) {
	t.Parallel()

	feed := dueFeed(t, testSealer(t))
	other := calendarfeed.Keyring{
		CurrentVersion: "feed-v9",
		Keys:           map[string][]byte{"feed-v9": []byte("calendar-feed-other-key-32bytes!")},
	}
	rotated, err := calendarfeed.NewURLSealer(other, other)
	if err != nil {
		t.Fatalf("NewURLSealer() error = %v", err)
	}
	repository := &syncRepository{feeds: []calendarfeed.DueFeed{feed}}
	synchronizer := newSynchronizer(t, repository, stubFetcher(bookingCalendar), rotated)

	if _, err := synchronizer.SyncDue(context.Background(), 10); err == nil {
		t.Fatal("SyncDue() error = nil, want the key failure reported")
	}
	if len(repository.applied) != 0 {
		t.Fatalf("ApplySync() ran for a local key failure: %+v", repository.applied)
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
	feeds     []calendarfeed.DueFeed
	applied   []calendarfeed.SyncResult
	failFeeds map[uuid.UUID]bool
}

func (r *syncRepository) DueFeeds(context.Context, int32) ([]calendarfeed.DueFeed, error) {
	return r.feeds, nil
}

func (r *syncRepository) ApplySync(_ context.Context, result calendarfeed.SyncResult) error {
	if r.failFeeds[result.FeedID] {
		return errors.New("write conflict")
	}
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
