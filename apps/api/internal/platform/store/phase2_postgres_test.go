//go:build integration

package store_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/idempotency"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPhase2PostgreSQLTenantReplayRollbackAndLastManager(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	adminPool := openIntegrationPool(t, ctx, "CUMURU_TEST_ADMIN_DATABASE_URL")
	runtimePool := openIntegrationPool(t, ctx, "CUMURU_TEST_DATABASE_URL")
	requireRuntimeRole(t, ctx, runtimePool)
	requirePhase2Schema(t, ctx, runtimePool)

	fixture := seedPhase2Fixture(t, ctx, adminPool)
	t.Cleanup(func() { cleanupPhase2Fixture(t, adminPool, fixture) })
	subject := store.NewPhase2(runtimePool, 5*time.Second, integrationPhase2Config(t))
	accommodationService := accommodation.NewService(store.NewAccommodationRepository(subject))
	stayRepository, err := store.NewStayRepository(subject)
	if err != nil {
		t.Fatalf("stay repository: %v", err)
	}
	stayService := stay.NewService(stayRepository)

	assertTenantIsolation(t, ctx, accommodationService, fixture)
	assertReplayAndRollback(t, ctx, runtimePool, accommodationService, fixture)
	assertConcurrentLastManager(t, ctx, runtimePool, accommodationService, fixture)
	assertStatusPolicies(t, ctx, accommodationService, stayService, fixture)
	assertMinimalPersistedStayMutations(t, ctx, runtimePool, stayService, fixture)
	assertExternalSubmissionID(t, ctx, stayService, fixture)
	assertInviteReplayAndRateIdentity(t, ctx, runtimePool, stayService, fixture)
	assertConcurrentInviteConsumption(t, ctx, runtimePool, subject, fixture)
	assertRuntimeCannotReadAppendOnlyTables(t, ctx, runtimePool)
}

func TestExpiredOperationalCleanupUsesWorkerRole(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	adminPool := openIntegrationPool(t, ctx, "CUMURU_TEST_ADMIN_DATABASE_URL")
	workerPool := openWorkerIntegrationPool(t, ctx)
	requireWorkerRole(t, ctx, workerPool)
	cutoff := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	seedExpiredCleanupFixture(t, ctx, adminPool, cutoff)
	t.Cleanup(func() { cleanupExpiredCleanupFixture(t, adminPool) })
	subject := store.NewPhase2(
		workerPool,
		5*time.Second,
		config.Phase2Config{},
	)

	first, err := subject.CleanupExpiredOperationalRecords(ctx, cutoff, 1)
	if err != nil {
		t.Fatalf("first cleanup: %v", err)
	}
	assertExpiredCleanupResult(t, first, 1, 1)
	assertExpiredCleanupCounts(
		t,
		readExpiredCleanupCounts(t, ctx, adminPool, cutoff),
		expiredCleanupCounts{
			expiredCompleted:  1,
			expiredProcessing: 1,
			validIdempotency:  2,
			expiredRateLimit:  1,
			validRateLimit:    2,
		},
	)

	second, err := subject.CleanupExpiredOperationalRecords(ctx, cutoff, 100)
	if err != nil {
		t.Fatalf("second cleanup: %v", err)
	}
	assertExpiredCleanupResult(t, second, 1, 1)
	third, err := subject.CleanupExpiredOperationalRecords(ctx, cutoff, 100)
	if err != nil {
		t.Fatalf("repeated cleanup: %v", err)
	}
	assertExpiredCleanupResult(t, third, 0, 0)
	assertExpiredCleanupCounts(
		t,
		readExpiredCleanupCounts(t, ctx, adminPool, cutoff),
		expiredCleanupCounts{
			expiredProcessing: 1,
			validIdempotency:  2,
			validRateLimit:    2,
		},
	)
}

type phase2Fixture struct {
	organizationIDs []uuid.UUID
	accommodationA  uuid.UUID
	accommodationB  uuid.UUID
	lockProperty    uuid.UUID
	closedProperty  uuid.UUID
	managerA        uuid.UUID
	managerB        uuid.UUID
	closedManager   uuid.UUID
	suspendedStay   uuid.UUID
	checkInStay     uuid.UUID
	checkOutStay    uuid.UUID
	cancelStay      uuid.UUID
	noShowStay      uuid.UUID
	groupStay       uuid.UUID
}

func seedPhase2Fixture(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
) phase2Fixture {
	t.Helper()
	fixture := phase2Fixture{
		organizationIDs: []uuid.UUID{mustV7(t), mustV7(t), mustV7(t), mustV7(t)},
		accommodationA:  mustV7(t), accommodationB: mustV7(t),
		lockProperty: mustV7(t), closedProperty: mustV7(t),
		managerA: mustV7(t), managerB: mustV7(t), closedManager: mustV7(t),
		suspendedStay: mustV7(t), checkInStay: mustV7(t), checkOutStay: mustV7(t),
		cancelStay: mustV7(t), noShowStay: mustV7(t), groupStay: mustV7(t),
	}
	for index, organizationID := range fixture.organizationIDs {
		_, err := pool.Exec(ctx,
			`INSERT INTO core.organizations (id, name) VALUES ($1, $2)`,
			organizationID, "Organização fictícia "+string(rune('A'+index)),
		)
		if err != nil {
			t.Fatalf("seed organization: %v", err)
		}
	}
	properties := []struct {
		id, organizationID uuid.UUID
		name, status       string
	}{
		{fixture.accommodationA, fixture.organizationIDs[0], "Hospedagem A", "active"},
		{fixture.accommodationB, fixture.organizationIDs[1], "Hospedagem B", "suspended"},
		{fixture.lockProperty, fixture.organizationIDs[2], "Hospedagem locks", "active"},
		{fixture.closedProperty, fixture.organizationIDs[3], "Hospedagem fechada", "closed"},
	}
	for _, property := range properties {
		_, err := pool.Exec(ctx,
			`INSERT INTO core.accommodations (id, organization_id, name, category, status)
			 VALUES ($1, $2, $3, 'pousada', $4)`,
			property.id, property.organizationID, property.name, property.status,
		)
		if err != nil {
			t.Fatalf("seed accommodation: %v", err)
		}
	}
	seedMembership(t, ctx, pool, mustV7(t), fixture.accommodationA, "manager-a")
	seedMembership(t, ctx, pool, mustV7(t), fixture.accommodationB, "manager-b")
	seedMembership(t, ctx, pool, fixture.managerA, fixture.lockProperty, "lock-a")
	seedMembership(t, ctx, pool, fixture.managerB, fixture.lockProperty, "lock-b")
	seedMembership(t, ctx, pool, fixture.closedManager, fixture.closedProperty, "closed-manager")
	seedPhase2Stays(t, ctx, pool, fixture)
	return fixture
}

func seedPhase2Stays(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	fixture phase2Fixture,
) {
	t.Helper()
	stays := []struct {
		id, accommodationID uuid.UUID
		status              string
		checkedInAt         any
		arrival             string
	}{
		{fixture.suspendedStay, fixture.accommodationB, "draft", nil, "2026-07-01"},
		{fixture.checkInStay, fixture.accommodationA, "pre_registered", nil, "2026-07-01"},
		{fixture.checkOutStay, fixture.accommodationA, "checked_in", time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC), "2026-07-01"},
		{fixture.cancelStay, fixture.accommodationA, "draft", nil, "2026-07-01"},
		{fixture.noShowStay, fixture.accommodationA, "invited", nil, "2026-07-01"},
		{fixture.groupStay, fixture.accommodationA, "draft", nil, "2026-07-01"},
	}
	for _, value := range stays {
		_, err := pool.Exec(ctx,
			`INSERT INTO core.stays
			 (id, accommodation_id, created_by_membership_id, status,
			  client_submission_id, planned_arrival_on, planned_departure_on,
			  expected_guest_count, checked_in_at)
			 SELECT $1, $2, m.id, $3, $4, $5::date, '2026-12-31'::date, 1, $6
			 FROM core.memberships m
			 WHERE m.accommodation_id=$2 AND m.active=true
			 ORDER BY m.id LIMIT 1`,
			value.id, value.accommodationID, value.status, mustV7(t),
			value.arrival, value.checkedInAt,
		)
		if err != nil {
			t.Fatalf("seed stay: %v", err)
		}
	}
	_, err := pool.Exec(ctx,
		`INSERT INTO core.visitors
		 (id, stay_id, client_id, role, age_band, residence_country)
		 VALUES ($1, $2, $3, 'responsible', '25_34', 'BR')`,
		mustV7(t), fixture.checkInStay, mustV7(t),
	)
	if err != nil {
		t.Fatalf("seed check-in visitor: %v", err)
	}
}

func seedMembership(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	id, accommodationID uuid.UUID,
	subject string,
) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO core.memberships
		 (id, accommodation_id, oidc_issuer, oidc_subject, role)
		 VALUES ($1, $2, 'https://issuer.invalid', $3, 'manager')`,
		id, accommodationID, subject,
	)
	if err != nil {
		t.Fatalf("seed membership: %v", err)
	}
}

func assertTenantIsolation(
	t *testing.T,
	ctx context.Context,
	service *accommodation.Service,
	fixture phase2Fixture,
) {
	t.Helper()
	page, err := service.List(ctx, principal("manager-a"), accommodation.PageRequest{Limit: 100})
	if err != nil {
		t.Fatalf("tenant A list: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != fixture.accommodationA {
		t.Fatalf("tenant A items = %#v", page.Items)
	}
	if _, err := service.Get(ctx, principal("manager-a"), fixture.accommodationB); err == nil {
		t.Fatal("tenant A read tenant B succeeded")
	}
}

func assertReplayAndRollback(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	service *accommodation.Service,
	fixture phase2Fixture,
) {
	t.Helper()
	command := accommodation.CreateMembershipCommand{
		Actor: principal("manager-a"), AccommodationID: fixture.accommodationA,
		TargetIssuer: "https://issuer.invalid", TargetSubject: "replay-target",
		Role: accommodation.RoleOperator, IdempotencyKey: "membership-replay-1234",
		RequestID: "request-replay-1234",
	}
	first, replayed, err := service.CreateMembership(ctx, command)
	if err != nil || replayed {
		t.Fatalf("first create = %#v, replay=%v, err=%v", first, replayed, err)
	}
	second, replayed, err := service.CreateMembership(ctx, command)
	if err != nil || !replayed || second.ID != first.ID {
		t.Fatalf("replay = %#v, replay=%v, err=%v", second, replayed, err)
	}
	rollback := command
	rollback.TargetSubject = "rollback-target"
	rollback.IdempotencyKey = "membership-rollback-1234"
	rollback.RequestID = "bad"
	if _, _, err := service.CreateMembership(ctx, rollback); err == nil {
		t.Fatal("failure injection succeeded")
	}
	assertCount(t, ctx, pool,
		`SELECT count(*) FROM core.memberships WHERE accommodation_id=$1 AND oidc_subject='rollback-target'`,
		fixture.accommodationA, 0,
	)
	assertCount(t, ctx, pool,
		`SELECT count(*) FROM platform.idempotency_records WHERE resource_id=$1`,
		fixture.accommodationA, 1,
	)
}

func assertConcurrentLastManager(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	service *accommodation.Service,
	fixture phase2Fixture,
) {
	t.Helper()
	commands := []accommodation.UpdateMembershipCommand{
		lastManagerCommand(fixture.lockProperty, fixture.managerB, "lock-a", "request-lock-a"),
		lastManagerCommand(fixture.lockProperty, fixture.managerA, "lock-b", "request-lock-b"),
	}
	start := make(chan struct{})
	results := make(chan error, len(commands))
	var group sync.WaitGroup
	for _, command := range commands {
		group.Add(1)
		go func(command accommodation.UpdateMembershipCommand) {
			defer group.Done()
			<-start
			_, err := service.UpdateMembership(ctx, command)
			results <- err
		}(command)
	}
	close(start)
	group.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful concurrent removals = %d, want 1", successes)
	}
	assertCount(t, ctx, pool,
		`SELECT count(*) FROM core.memberships
		 WHERE accommodation_id=$1 AND role='manager' AND active=true`,
		fixture.lockProperty, 1,
	)
}

func lastManagerCommand(
	accommodationID, membershipID uuid.UUID,
	actorSubject, requestID string,
) accommodation.UpdateMembershipCommand {
	return accommodation.UpdateMembershipCommand{
		Actor: principal(actorSubject), AccommodationID: accommodationID,
		MembershipID: membershipID, ExpectedVersion: 1,
		Patch:     accommodation.UpdateMembershipPatch{SetActive: true, Active: false},
		RequestID: requestID,
	}
}

func assertConcurrentInviteConsumption(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	subject *store.Store,
	fixture phase2Fixture,
) {
	t.Helper()
	repository, err := store.NewStayRepository(subject)
	if err != nil {
		t.Fatalf("stay repository: %v", err)
	}
	service := stay.NewService(repository)
	created, _, err := service.Create(ctx, stay.CreateCommand{
		Actor: principal("manager-a"), AccommodationID: fixture.accommodationA,
		ClientSubmissionID: mustV7(t), PlannedArrivalOn: "2026-08-10",
		PlannedDepartureOn: "2026-08-12", ExpectedGuestCount: 1,
		IdempotencyKey: "create-stay-integration-1234", RequestID: "request-stay-1234",
	})
	if err != nil {
		t.Fatalf("create stay: %v", err)
	}
	invite, _, err := service.CreateInvite(ctx, stay.InviteCommand{
		Actor: principal("manager-a"), StayID: created.ID,
		PrivacyNoticeVersion: "privacy-v1", ExpectedVersion: created.Version,
		IdempotencyKey: "create-invite-integration-1234", RequestID: "request-invite-1234",
	})
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}
	parsed, err := url.Parse(invite.URL)
	if err != nil {
		t.Fatalf("parse invite URL: %v", err)
	}
	token := path.Base(parsed.Path)
	commands := []stay.InviteGroupCommand{
		inviteGroupCommand(t, token, "invite-submit-a-1234", "request-submit-a"),
		inviteGroupCommand(t, token, "invite-submit-b-1234", "request-submit-b"),
	}
	successes := runInviteSubmissions(ctx, service, commands)
	if successes != 1 {
		t.Fatalf("successful invite submissions = %d, want 1", successes)
	}
	assertCount(t, ctx, pool,
		`SELECT count(*) FROM core.group_submissions WHERE stay_id=$1`,
		created.ID, 1,
	)
	assertCount(t, ctx, pool,
		`SELECT count(*) FROM core.visitors WHERE stay_id=$1`,
		created.ID, 1,
	)
	assertCount(t, ctx, pool,
		`SELECT count(*) FROM core.invites
		 WHERE stay_id=$1 AND use_count=1 AND max_uses=1`,
		created.ID, 1,
	)
}

func assertStatusPolicies(
	t *testing.T,
	ctx context.Context,
	accommodations *accommodation.Service,
	stays *stay.Service,
	fixture phase2Fixture,
) {
	t.Helper()
	if _, err := accommodations.Get(
		ctx,
		principal("closed-manager"),
		fixture.closedProperty,
	); err != nil {
		t.Fatalf("closed accommodation read: %v", err)
	}
	_, err := accommodations.Update(ctx, accommodation.UpdateCommand{
		Actor: principal("manager-b"), AccommodationID: fixture.accommodationB,
		ExpectedVersion: 1, Patch: accommodation.UpdatePatch{
			SetCategory: true, Category: "hotel",
		},
		RequestID: "request-suspended-update",
	})
	assertErrorIs(t, err, accommodation.ErrConflict, "suspended accommodation update")
	_, _, err = accommodations.CreateMembership(ctx, accommodation.CreateMembershipCommand{
		Actor: principal("closed-manager"), AccommodationID: fixture.closedProperty,
		TargetIssuer: "https://issuer.invalid", TargetSubject: "closed-target",
		Role: accommodation.RoleOperator, IdempotencyKey: "closed-membership-key-1234",
		RequestID: "request-closed-create",
	})
	assertErrorIs(t, err, accommodation.ErrConflict, "closed membership create")
	_, err = accommodations.UpdateMembership(ctx, accommodation.UpdateMembershipCommand{
		Actor: principal("closed-manager"), AccommodationID: fixture.closedProperty,
		MembershipID: fixture.closedManager, ExpectedVersion: 1,
		Patch:     accommodation.UpdateMembershipPatch{SetActive: true, Active: false},
		RequestID: "request-closed-update",
	})
	assertErrorIs(t, err, accommodation.ErrConflict, "closed membership update")
	_, err = stays.Update(ctx, stay.UpdateCommand{
		Actor: principal("manager-b"), StayID: fixture.suspendedStay,
		ExpectedVersion: 1,
		Patch: stay.UpdatePatch{
			SetExpectedGuestCount: true, ExpectedGuestCount: 2,
		},
		RequestID: "request-suspended-stay",
	})
	assertErrorIs(t, err, stay.ErrConflict, "suspended stay update")
}

func assertMinimalPersistedStayMutations(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	service *stay.Service,
	fixture phase2Fixture,
) {
	t.Helper()
	created, _, err := service.Create(ctx, stay.CreateCommand{
		Actor: principal("manager-a"), AccommodationID: fixture.accommodationA,
		ClientSubmissionID: mustV7(t), PlannedArrivalOn: "2026-09-01",
		PlannedDepartureOn: "2026-09-03", ExpectedGuestCount: 1,
		IdempotencyKey: "minimal-create-key-1234", RequestID: "request-minimal-create",
	})
	if err != nil {
		t.Fatalf("minimal create: %v", err)
	}
	assertStoredMutation(
		t, ctx, pool, idempotency.OperationCreateStay, fixture.accommodationA,
	)
	transitions := []struct {
		id      uuid.UUID
		kind    stay.TransitionKind
		key     string
		reason  string
		instant time.Time
	}{
		{fixture.checkInStay, stay.TransitionCheckIn, "minimal-checkin-key-1234", "", time.Date(2026, 7, 28, 11, 0, 0, 0, time.UTC)},
		{fixture.checkOutStay, stay.TransitionCheckOut, "minimal-checkout-key-1234", "", time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)},
		{fixture.cancelStay, stay.TransitionCancel, "minimal-cancel-key-1234", "guest_request", time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)},
		{fixture.noShowStay, stay.TransitionNoShow, "minimal-noshow-key-1234", "", time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)},
	}
	for _, transition := range transitions {
		_, _, err := service.Transition(ctx, stay.TransitionCommand{
			Actor: principal("manager-a"), StayID: transition.id,
			ExpectedVersion: 1, Kind: transition.kind,
			OccurredAt: transition.instant, ReasonCode: transition.reason,
			IdempotencyKey: transition.key, RequestID: "request-" + transition.key,
		})
		if err != nil {
			t.Fatalf("%s transition: %v", transition.kind, err)
		}
		assertStoredMutation(
			t, ctx, pool, transitionOperationForTest(transition.kind), transition.id,
		)
	}
	if created.ID == uuid.Nil {
		t.Fatal("create mutation returned nil ID")
	}
}

func transitionOperationForTest(kind stay.TransitionKind) idempotency.Operation {
	operations := map[stay.TransitionKind]idempotency.Operation{
		stay.TransitionCheckIn:  idempotency.OperationCheckIn,
		stay.TransitionCheckOut: idempotency.OperationCheckOut,
		stay.TransitionCancel:   idempotency.OperationCancel,
		stay.TransitionNoShow:   idempotency.OperationNoShow,
	}
	return operations[kind]
}

func assertStoredMutation(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	operation idempotency.Operation,
	resourceID uuid.UUID,
) {
	t.Helper()
	var body []byte
	err := pool.QueryRow(ctx,
		`SELECT response_body
		 FROM platform.idempotency_records
		 WHERE operation_key=$1 AND resource_id=$2 AND state='completed'`,
		string(operation), resourceID,
	).Scan(&body)
	if err != nil {
		t.Fatalf("stored mutation %s: %v", operation, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("stored mutation JSON: %v", err)
	}
	if len(payload) != 3 || payload["id"] == nil || payload["status"] == nil || payload["version"] == nil {
		t.Fatalf("stored mutation payload = %s", body)
	}
}

func assertExternalSubmissionID(
	t *testing.T,
	ctx context.Context,
	service *stay.Service,
	fixture phase2Fixture,
) {
	t.Helper()
	clientID := mustV7(t)
	result, _, err := service.SubmitAssistedGroup(ctx, stay.GroupCommand{
		Actor: principal("manager-a"), StayID: fixture.groupStay,
		ClientSubmissionID: clientID, PrivacyNoticeVersion: "privacy-v1",
		Visitors: []stay.Visitor{{
			ClientID: mustV7(t).String(), Role: stay.VisitorResponsible,
			AgeBand: stay.Age25To34, ResidenceCountry: "AR",
		}},
		ExpectedVersion: 1, IdempotencyKey: "external-submission-key-1234",
		RequestID: "request-external-submission",
	})
	if err != nil {
		t.Fatalf("assisted submission: %v", err)
	}
	if result.SubmissionID != clientID {
		t.Fatalf("submission_id = %s, want client_submission_id %s", result.SubmissionID, clientID)
	}
}

func assertInviteReplayAndRateIdentity(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	service *stay.Service,
	fixture phase2Fixture,
) {
	t.Helper()
	created, _, err := service.Create(ctx, stay.CreateCommand{
		Actor: principal("manager-a"), AccommodationID: fixture.accommodationA,
		ClientSubmissionID: mustV7(t), PlannedArrivalOn: "2026-10-01",
		PlannedDepartureOn: "2026-10-03", ExpectedGuestCount: 1,
		IdempotencyKey: "invite-replay-create-1234", RequestID: "request-invite-replay-create",
	})
	if err != nil {
		t.Fatalf("create invite replay stay: %v", err)
	}
	invite, _, err := service.CreateInvite(ctx, stay.InviteCommand{
		Actor: principal("manager-a"), StayID: created.ID,
		PrivacyNoticeVersion: "privacy-v1", ExpectedVersion: created.Version,
		IdempotencyKey: "invite-replay-issue-1234", RequestID: "request-invite-replay-issue",
	})
	if err != nil {
		t.Fatalf("create replay invite: %v", err)
	}
	token := inviteToken(t, invite.URL)
	command := inviteGroupCommand(t, token, "invite-replay-submit-1234", "request-invite-replay-submit")
	command.RateSubject = "203.0.113.0/24"
	first, replayed, err := service.SubmitInviteGroup(ctx, command)
	if err != nil || replayed {
		t.Fatalf("first invite submit = %#v, replay=%v, err=%v", first, replayed, err)
	}
	second, replayed, err := service.SubmitInviteGroup(ctx, command)
	if err != nil || !replayed || second != first {
		t.Fatalf("invite replay = %#v, replay=%v, err=%v", second, replayed, err)
	}
	newKey := command
	newKey.IdempotencyKey = "invite-consumed-new-key-1234"
	newKey.RequestID = "request-invite-consumed-new"
	_, _, err = service.SubmitInviteGroup(ctx, newKey)
	assertErrorIs(t, err, stay.ErrInviteConsumed, "consumed invite with new key")
	assertRateBucketsArePseudonymous(t, ctx, pool, token, command.RateSubject)
}

func inviteToken(t *testing.T, inviteURL string) string {
	t.Helper()
	parsed, err := url.Parse(inviteURL)
	if err != nil {
		t.Fatalf("parse invite URL: %v", err)
	}
	return path.Base(parsed.Path)
}

func assertRateBucketsArePseudonymous(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	token string,
	prefix string,
) {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT subject_hmac FROM platform.rate_limit_buckets`)
	if err != nil {
		t.Fatalf("read rate buckets: %v", err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var digest []byte
		if err := rows.Scan(&digest); err != nil {
			t.Fatalf("scan rate bucket: %v", err)
		}
		found = true
		if len(digest) != 32 || bytes.Contains(digest, []byte(token)) || bytes.Contains(digest, []byte(prefix)) {
			t.Fatalf("unsafe rate subject digest: %x", digest)
		}
	}
	if !found || rows.Err() != nil {
		t.Fatalf("rate buckets missing or invalid: %v", rows.Err())
	}
}

func assertRuntimeCannotReadAppendOnlyTables(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
) {
	t.Helper()
	for _, table := range []string{"platform.audit_events", "platform.outbox_events"} {
		var value int
		err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&value)
		var postgresError *pgconn.PgError
		if !errors.As(err, &postgresError) || postgresError.Code != "42501" {
			t.Errorf("runtime SELECT %s error = %v, want insufficient_privilege", table, err)
		}
	}
}

func assertErrorIs(t *testing.T, got, want error, operation string) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("%s error = %v, want %v", operation, got, want)
	}
}

func inviteGroupCommand(
	t *testing.T,
	token, key, requestID string,
) stay.InviteGroupCommand {
	t.Helper()
	return stay.InviteGroupCommand{
		Token: token, RateSubject: "127.0.0.1",
		ClientSubmissionID: mustV7(t), PrivacyNoticeVersion: "privacy-v1",
		Visitors: []stay.Visitor{{
			ClientID: mustV7(t).String(), Role: stay.VisitorResponsible,
			AgeBand: stay.Age25To34, ResidenceCountry: "AR",
		}},
		IdempotencyKey: key, RequestID: requestID,
	}
}

func runInviteSubmissions(
	ctx context.Context,
	service *stay.Service,
	commands []stay.InviteGroupCommand,
) int {
	start := make(chan struct{})
	results := make(chan error, len(commands))
	var group sync.WaitGroup
	for _, command := range commands {
		group.Add(1)
		go func(command stay.InviteGroupCommand) {
			defer group.Done()
			<-start
			_, _, err := service.SubmitInviteGroup(ctx, command)
			results <- err
		}(command)
	}
	close(start)
	group.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	return successes
}

func requirePhase2Schema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var name *string
	if err := pool.QueryRow(ctx, `SELECT to_regclass('platform.idempotency_records')::text`).Scan(&name); err != nil || name == nil {
		t.Fatalf("phase 2 migrations are required: %v", err)
	}
}

func openIntegrationPool(
	t *testing.T,
	ctx context.Context,
	field string,
) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv(field)
	if dsn == "" {
		t.Fatalf("%s is required for integration tests", field)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect PostgreSQL using %s: %v", field, err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func requireRuntimeRole(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var isRuntimeRole bool
	if err := pool.QueryRow(ctx, `SELECT current_user = 'cumuru_app'`).Scan(&isRuntimeRole); err != nil {
		t.Fatalf("read runtime current_user: %v", err)
	}
	if !isRuntimeRole {
		t.Fatal("runtime current_user is not cumuru_app")
	}
}

func openWorkerIntegrationPool(
	t *testing.T,
	ctx context.Context,
) *pgxpool.Pool {
	t.Helper()
	adminDSN := os.Getenv("CUMURU_TEST_ADMIN_DATABASE_URL")
	parsed, err := url.Parse(adminDSN)
	if err != nil || parsed.Host == "" {
		t.Fatalf("parse worker integration DSN: %v", err)
	}
	parsed.User = url.UserPassword(
		"cumuru_worker",
		"cumuru-local-worker-only",
	)
	pool, err := pgxpool.New(ctx, parsed.String())
	if err != nil {
		t.Fatalf("connect PostgreSQL as cumuru_worker: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func requireWorkerRole(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
) {
	t.Helper()
	var isWorker bool
	if err := pool.QueryRow(
		ctx,
		`SELECT current_user = 'cumuru_worker'`,
	).Scan(&isWorker); err != nil {
		t.Fatalf("read worker current_user: %v", err)
	}
	if !isWorker {
		t.Fatal("cleanup current_user is not cumuru_worker")
	}
}

type expiredCleanupCounts struct {
	expiredCompleted  int
	expiredProcessing int
	validIdempotency  int
	expiredRateLimit  int
	validRateLimit    int
}

func seedExpiredCleanupFixture(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	cutoff time.Time,
) {
	t.Helper()
	idempotencyRows := []struct {
		operation string
		state     string
		expiresAt time.Time
	}{
		{"cleanup_go_expired_1", "completed", cutoff.Add(-2 * time.Hour)},
		{"cleanup_go_expired_2", "completed", cutoff.Add(-time.Hour)},
		{"cleanup_go_processing", "processing", cutoff.Add(-time.Hour)},
		{"cleanup_go_equal", "completed", cutoff},
		{"cleanup_go_future", "completed", cutoff.Add(time.Hour)},
	}
	for _, row := range idempotencyRows {
		insertExpiredCleanupIdempotency(t, ctx, pool, cutoff, row)
	}
	rateLimitRows := []struct {
		scope     string
		expiresAt time.Time
	}{
		{"invite_context", cutoff.Add(-2 * time.Hour)},
		{"invite_submit", cutoff.Add(-time.Hour)},
		{"survey_submit", cutoff},
		{"invite_context", cutoff.Add(time.Hour)},
	}
	for index, row := range rateLimitRows {
		_, err := pool.Exec(
			ctx,
			`INSERT INTO platform.rate_limit_buckets (
			   scope, subject_hmac, subject_key_version, window_started_at,
			   request_count, expires_at
			 ) VALUES ($1, $2, 'f6-cleanup', $3, 1, $4)`,
			row.scope,
			[]byte("f6-cleanup-rate-"+row.scope+"-"+string(rune('a'+index))),
			cutoff.Add(-24*time.Hour-time.Duration(index)*time.Minute),
			row.expiresAt,
		)
		if err != nil {
			t.Fatalf("seed rate limit cleanup fixture: %v", err)
		}
	}
}

func insertExpiredCleanupIdempotency(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	cutoff time.Time,
	row struct {
		operation string
		state     string
		expiresAt time.Time
	},
) {
	t.Helper()
	var responseStatus *int32
	var responseHeaders any
	var completedAt *time.Time
	if row.state == "completed" {
		status := int32(200)
		completed := cutoff.Add(-23 * time.Hour)
		responseStatus = &status
		responseHeaders = `{}`
		completedAt = &completed
	}
	_, err := pool.Exec(
		ctx,
		`INSERT INTO platform.idempotency_records (
		   actor_key_hmac, actor_key_version, method, operation_key, resource_id,
		   idempotency_key_hmac, idempotency_key_version, request_hash, state,
		   response_status, response_headers, created_at, completed_at, expires_at
		 ) VALUES (
		   $1, 'f6-cleanup', 'POST', $2, $3, $4, 'f6-cleanup', $5, $6,
		   $7, $8, $9, $10, $11
		 )`,
		[]byte("f6-cleanup-actor"),
		row.operation,
		mustV7(t),
		[]byte("f6-cleanup-key-"+row.operation),
		[]byte("f6-cleanup-request"),
		row.state,
		responseStatus,
		responseHeaders,
		cutoff.Add(-24*time.Hour),
		completedAt,
		row.expiresAt,
	)
	if err != nil {
		t.Fatalf("seed idempotency cleanup fixture: %v", err)
	}
}

func readExpiredCleanupCounts(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	cutoff time.Time,
) expiredCleanupCounts {
	t.Helper()
	var result expiredCleanupCounts
	err := pool.QueryRow(
		ctx,
		`SELECT
		   count(*) FILTER (WHERE state='completed' AND expires_at < $1),
		   count(*) FILTER (WHERE state='processing' AND expires_at < $1),
		   count(*) FILTER (WHERE state='completed' AND expires_at >= $1)
		 FROM platform.idempotency_records
		 WHERE operation_key LIKE 'cleanup_go_%'`,
		cutoff,
	).Scan(
		&result.expiredCompleted,
		&result.expiredProcessing,
		&result.validIdempotency,
	)
	if err != nil {
		t.Fatalf("read idempotency cleanup fixture: %v", err)
	}
	err = pool.QueryRow(
		ctx,
		`SELECT
		   count(*) FILTER (WHERE expires_at < $1),
		   count(*) FILTER (WHERE expires_at >= $1)
		 FROM platform.rate_limit_buckets
		 WHERE subject_key_version='f6-cleanup'`,
		cutoff,
	).Scan(&result.expiredRateLimit, &result.validRateLimit)
	if err != nil {
		t.Fatalf("read rate limit cleanup fixture: %v", err)
	}
	return result
}

func assertExpiredCleanupResult(
	t *testing.T,
	result store.ExpiredRecordCleanupResult,
	idempotencyRecords int64,
	rateLimitBuckets int64,
) {
	t.Helper()
	if result.IdempotencyRecords != idempotencyRecords ||
		result.RateLimitBuckets != rateLimitBuckets {
		t.Fatalf("cleanup result = %#v", result)
	}
}

func assertExpiredCleanupCounts(
	t *testing.T,
	got expiredCleanupCounts,
	want expiredCleanupCounts,
) {
	t.Helper()
	if got != want {
		t.Fatalf("cleanup counts = %#v, want %#v", got, want)
	}
}

func cleanupExpiredCleanupFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := pool.Exec(
		ctx,
		`DELETE FROM platform.idempotency_records
		 WHERE operation_key LIKE 'cleanup_go_%'`,
	); err != nil {
		t.Errorf("cleanup idempotency fixture: %v", err)
	}
	if _, err := pool.Exec(
		ctx,
		`DELETE FROM platform.rate_limit_buckets
		 WHERE subject_key_version='f6-cleanup'`,
	); err != nil {
		t.Errorf("cleanup rate limit fixture: %v", err)
	}
}

func cleanupPhase2Fixture(t *testing.T, pool *pgxpool.Pool, fixture phase2Fixture) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	accommodations := []uuid.UUID{
		fixture.accommodationA,
		fixture.accommodationB,
		fixture.lockProperty,
		fixture.closedProperty,
	}
	statements := []string{
		`DELETE FROM platform.audit_events WHERE organization_id = ANY($1)`,
		`DELETE FROM platform.outbox_events WHERE aggregate_id IN (
		   SELECT id FROM core.memberships WHERE accommodation_id = ANY($1)
		   UNION SELECT id FROM core.stays WHERE accommodation_id = ANY($1)
		 )`,
		`DELETE FROM platform.idempotency_records WHERE resource_id = ANY($1)
		   OR resource_id IN (SELECT id FROM core.stays WHERE accommodation_id = ANY($1))
		   OR resource_id IN (
		     SELECT i.id FROM core.invites i JOIN core.stays s ON s.id=i.stay_id
		     WHERE s.accommodation_id = ANY($1)
		   )`,
		`DELETE FROM platform.rate_limit_buckets
		 WHERE subject_key_version='v1' AND $1::uuid[] IS NOT NULL`,
		`DELETE FROM core.visitors WHERE stay_id IN
		 (SELECT id FROM core.stays WHERE accommodation_id = ANY($1))`,
		`DELETE FROM core.group_submissions WHERE stay_id IN
		 (SELECT id FROM core.stays WHERE accommodation_id = ANY($1))`,
		`DELETE FROM core.invites WHERE stay_id IN
		 (SELECT id FROM core.stays WHERE accommodation_id = ANY($1))`,
		`DELETE FROM core.stays WHERE accommodation_id = ANY($1)`,
		`DELETE FROM core.memberships WHERE accommodation_id = ANY($1)`,
		`DELETE FROM core.accommodations WHERE id = ANY($1)`,
		`DELETE FROM core.organizations WHERE id = ANY($1)`,
	}
	arguments := [][]uuid.UUID{
		fixture.organizationIDs, accommodations, accommodations, accommodations,
		accommodations, accommodations, accommodations, accommodations,
		accommodations, accommodations, fixture.organizationIDs,
	}
	for index, statement := range statements {
		if _, err := pool.Exec(ctx, statement, arguments[index]); err != nil {
			t.Errorf("cleanup statement %d: %v", index, err)
		}
	}
}

func assertCount(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	query string,
	argument uuid.UUID,
	want int,
) {
	t.Helper()
	var got int
	if err := pool.QueryRow(ctx, query, argument).Scan(&got); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if got != want {
		t.Fatalf("count = %d, want %d", got, want)
	}
}

func integrationPhase2Config(t *testing.T) config.Phase2Config {
	t.Helper()
	baseURL := mustURL(t, "https://example.invalid/invites")
	return config.Phase2Config{
		InviteBaseURL: baseURL, InviteTTL: 72 * time.Hour,
		IdempotencyTTL: 30 * 24 * time.Hour, RateLimitWindow: time.Minute,
		InviteContextRateLimit: 30, InviteSubmitRateLimit: 10,
		CORSAllowedOrigins: []string{"https://example.invalid"},
		InviteKeys:         testKeyring('i'), ActorKeys: testKeyring('a'),
		IdempotencyKeys: testKeyring('d'), RateLimitKeys: testKeyring('r'),
		CursorKeys: testKeyring('c'),
	}
}

func testKeyring(fill byte) config.KeyringConfig {
	return config.KeyringConfig{
		CurrentVersion: "v1",
		Keys:           map[string][]byte{"v1": bytesRepeat(fill, 32)},
	}
}

func bytesRepeat(value byte, count int) []byte {
	result := make([]byte, count)
	for index := range result {
		result[index] = value
	}
	return result
}

func principal(subject string) access.Principal {
	return access.NewPrincipal("https://issuer.invalid", subject, nil)
}

func mustV7(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid v7: %v", err)
	}
	return id
}

func mustURL(t *testing.T, value string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	return parsed
}
