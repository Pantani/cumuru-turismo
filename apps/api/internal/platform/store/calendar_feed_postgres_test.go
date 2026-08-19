//go:build integration

// SQL inline via pgxpool é convenção deliberada nestes testes de integração
// (não migrar para sqlc); ver AGENTS.md, seção "Padrões de backend".
package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/calendarfeed"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// O caminho inteiro contra PostgreSQL real: a hospedagem cadastra o feed, o
// worker lê o que está vencido, aplica o que observou, e a confirmação cria a
// estadia na mesma transação.
//
// As duas pontas rodam como `cumuru_app`. Os grants por coluna do
// `worker_runtime` não são exercidos aqui — quem os prova é
// `deploy/scripts/test-migrations.sh`, que conecta com cada papel.
func TestCalendarFeedPostgreSQLRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	adminPool := openIntegrationPool(t, ctx, "CUMURU_TEST_ADMIN_DATABASE_URL")
	runtimePool := openIntegrationPool(t, ctx, "CUMURU_TEST_DATABASE_URL")
	requireRuntimeRole(t, ctx, runtimePool)
	requireCoreSchema(t, ctx, runtimePool)

	fixture := seedCalendarFeedFixture(t, ctx, adminPool, runtimePool)
	repository := store.NewCalendarFeedRepository(fixture.subject)

	feed := createCalendarFeedFixture(t, ctx, repository, fixture)
	assertCalendarSyncIsIdempotent(t, ctx, repository, fixture, feed)
	assertCalendarConfirmationCreatesTheStay(t, ctx, adminPool, repository, fixture)
}

type calendarFeedFixture struct {
	subject         *store.Store
	sealer          *calendarfeed.URLSealer
	accommodationID uuid.UUID
	actorSubject    string
	cleanup         *onboardingCleanup
}

func seedCalendarFeedFixture(
	t *testing.T,
	ctx context.Context,
	adminPool *pgxpool.Pool,
	runtimePool *pgxpool.Pool,
) calendarFeedFixture {
	t.Helper()
	subject := store.NewCore(runtimePool, 5*time.Second, integrationCoreConfig(t))
	service := accommodation.NewService(store.NewAccommodationRepository(subject))
	marker := "calendar-" + mustV7(t).String()
	cleanup := &onboardingCleanup{marker: marker}
	t.Cleanup(func() { cleanup.run(t, adminPool) })

	actorSubject := marker + "-operator"
	created, _, err := service.Create(ctx, accommodationOnboardingCommand(
		t, actorSubject, "Pousada calendário "+marker, marker+"-idem-000001",
	))
	if err != nil {
		t.Fatalf("Create() accommodation: %v", err)
	}
	cleanup.track(actorSubject, created)
	// t.Cleanup roda LIFO, então esta registra depois e roda antes da de
	// onboarding: sem ela a estadia criada pela confirmação segura a membership,
	// que segura a acomodação, que segura a organização.
	t.Cleanup(func() { cleanCalendarRows(t, adminPool, created.ID) })
	return calendarFeedFixture{
		subject: subject, sealer: integrationSealer(t),
		accommodationID: created.ID, actorSubject: actorSubject, cleanup: cleanup,
	}
}

func cleanCalendarRows(t *testing.T, adminPool *pgxpool.Pool, accommodationID uuid.UUID) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	statements := []string{
		`DELETE FROM platform.outbox_events WHERE aggregate_id IN (
			SELECT id FROM core.calendar_reservations
			WHERE feed_id IN (SELECT id FROM core.calendar_feeds WHERE accommodation_id = $1)
			UNION ALL SELECT id FROM core.calendar_feeds WHERE accommodation_id = $1
			UNION ALL SELECT id FROM core.stays WHERE accommodation_id = $1
		)`,
		`DELETE FROM core.calendar_reservations WHERE feed_id IN (
			SELECT id FROM core.calendar_feeds WHERE accommodation_id = $1
		)`,
		`DELETE FROM core.calendar_feeds WHERE accommodation_id = $1`,
		`DELETE FROM core.stays WHERE accommodation_id = $1`,
	}
	for _, statement := range statements {
		if _, err := adminPool.Exec(ctx, statement, accommodationID); err != nil {
			t.Errorf("cleanup calendar rows: %v", err)
		}
	}
}

func createCalendarFeedFixture(
	t *testing.T,
	ctx context.Context,
	repository *store.CalendarFeedRepository,
	fixture calendarFeedFixture,
) calendarfeed.Feed {
	t.Helper()
	service, err := calendarfeed.NewService(repository, fixture.sealer)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	feed, _, err := service.CreateFeed(ctx, calendarfeed.CreateFeedCommand{
		Actor: principal(fixture.actorSubject), AccommodationID: fixture.accommodationID,
		Provider: calendarfeed.ProviderBooking, Label: "Chalé 3",
		URL:            "https://ical.booking.com/v1/export?t=" + mustV7(t).String(),
		IdempotencyKey: fixture.cleanup.marker + "-feed-0001",
		RequestID:      "request-calendar-feed-test",
	})
	if err != nil {
		t.Fatalf("CreateFeed(): %v", err)
	}
	if feed.Status != calendarfeed.StatusActive || feed.LastSyncedAt != nil {
		t.Fatalf("CreateFeed() = %+v", feed)
	}
	return feed
}

// Toda sincronização revê o mesmo calendário, então o conflito é o caminho
// normal: duas passagens sobre o mesmo UID têm de deixar uma linha, não duas.
func assertCalendarSyncIsIdempotent(
	t *testing.T,
	ctx context.Context,
	repository *store.CalendarFeedRepository,
	fixture calendarFeedFixture,
	feed calendarfeed.Feed,
) {
	t.Helper()
	due, err := repository.DueFeeds(ctx, 10)
	if err != nil {
		t.Fatalf("DueFeeds(): %v", err)
	}
	if !containsFeed(due, feed.ID) {
		t.Fatalf("DueFeeds() did not return the feed just registered")
	}
	observed := observedFixture(t, fixture.sealer, feed.ID)
	for range 2 {
		if err := repository.ApplySync(ctx, calendarfeed.SyncResult{
			FeedID: feed.ID, Version: feed.Version,
			Outcome: calendarfeed.OutcomeOK, Observed: observed,
		}); err != nil {
			t.Fatalf("ApplySync(): %v", err)
		}
	}
	queue := listCalendarQueue(t, ctx, repository, fixture)
	if len(queue) != 1 {
		t.Fatalf("queue after two identical syncs has %d items", len(queue))
	}
	if queue[0].ArrivalOn != "2026-09-01" || queue[0].Kind != calendarfeed.KindReserved {
		t.Fatalf("queue item = %+v", queue[0])
	}
}

func assertCalendarConfirmationCreatesTheStay(
	t *testing.T,
	ctx context.Context,
	adminPool *pgxpool.Pool,
	repository *store.CalendarFeedRepository,
	fixture calendarFeedFixture,
) {
	t.Helper()
	queue := listCalendarQueue(t, ctx, repository, fixture)
	pending := queue[0]
	confirmed, _, err := repository.Confirm(ctx, calendarfeed.ConfirmCommand{
		Actor: principal(fixture.actorSubject), ReservationID: pending.ID,
		ExpectedVersion: pending.Version, ExpectedGuestCount: 4,
		ClientSubmissionID: mustV7(t),
		IdempotencyKey:     fixture.cleanup.marker + "-confirm-0001",
		RequestID:          "request-calendar-feed-test",
	})
	if err != nil {
		t.Fatalf("Confirm(): %v", err)
	}
	if confirmed.State != calendarfeed.StateConfirmed || confirmed.StayID == nil {
		t.Fatalf("Confirm() = %+v", confirmed)
	}
	assertStayMatchesReservation(t, ctx, adminPool, *confirmed.StayID)
}

// As datas da estadia têm de ser as do calendário e o número de pessoas o que a
// hospedagem informou: é a única parte que o arquivo não sabe.
func assertStayMatchesReservation(
	t *testing.T,
	ctx context.Context,
	adminPool *pgxpool.Pool,
	stayID uuid.UUID,
) {
	t.Helper()
	var arrival, departure time.Time
	var guests int32
	err := adminPool.QueryRow(ctx, `
		SELECT planned_arrival_on, planned_departure_on, expected_guest_count
		FROM core.stays WHERE id = $1
	`, stayID).Scan(&arrival, &departure, &guests)
	if err != nil {
		t.Fatalf("read created stay: %v", err)
	}
	if arrival.Format("2006-01-02") != "2026-09-01" {
		t.Fatalf("stay arrival = %s", arrival)
	}
	if departure.Format("2006-01-02") != "2026-09-03" || guests != 4 {
		t.Fatalf("stay departure = %s, guests = %d", departure, guests)
	}
}

func listCalendarQueue(
	t *testing.T,
	ctx context.Context,
	repository *store.CalendarFeedRepository,
	fixture calendarFeedFixture,
) []calendarfeed.Reservation {
	t.Helper()
	queue, err := repository.ListReservations(ctx, calendarfeed.ListReservationsRequest{
		Actor: principal(fixture.actorSubject), AccommodationID: fixture.accommodationID,
		State: calendarfeed.StatePending, Limit: 50,
	})
	if err != nil {
		t.Fatalf("ListReservations(): %v", err)
	}
	if len(queue) == 0 {
		t.Fatal("ListReservations() returned an empty queue")
	}
	return queue
}

func observedFixture(
	t *testing.T,
	sealer *calendarfeed.URLSealer,
	feedID uuid.UUID,
) []calendarfeed.Observed {
	t.Helper()
	uid, err := sealer.FingerprintUID(feedID.String(), "booking-fixture-0001")
	if err != nil {
		t.Fatalf("FingerprintUID(): %v", err)
	}
	return []calendarfeed.Observed{{
		UID: uid, ArrivalOn: "2026-09-01", DepartureOn: "2026-09-03",
		Kind: calendarfeed.KindReserved,
	}}
}

func containsFeed(feeds []calendarfeed.DueFeed, id uuid.UUID) bool {
	for _, feed := range feeds {
		if feed.ID == id {
			return true
		}
	}
	return false
}

func integrationSealer(t *testing.T) *calendarfeed.URLSealer {
	t.Helper()
	sealer, err := calendarfeed.NewURLSealer(
		calendarfeed.Keyring{
			CurrentVersion: "v1",
			Keys:           map[string][]byte{"v1": bytesRepeat('k', 32)},
		},
		calendarfeed.Keyring{
			CurrentVersion: "v1",
			Keys:           map[string][]byte{"v1": bytesRepeat('f', 32)},
		},
	)
	if err != nil {
		t.Fatalf("NewURLSealer(): %v", err)
	}
	return sealer
}
