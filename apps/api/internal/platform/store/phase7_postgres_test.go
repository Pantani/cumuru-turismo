//go:build integration

package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/activation"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The schemas privacy.md §3.4 requires the sweep to cover. identity is listed
// even though ADR-020 left it without tables: the enumeration has to break the
// day somebody adds one.
var canaryScanSchemas = []string{
	"auth", "identity", "core", "survey", "analytics", "public_data", "platform",
}

type phase7Fixture struct {
	organizationID  uuid.UUID
	accommodationID uuid.UUID
	membershipID    uuid.UUID
	subject         string
	// name is the scan control: a value that legitimately survives every purge.
	// Without it, a sweep that finds nothing proves nothing — it could be
	// looking at the wrong tables, or at nothing at all.
	name string
}

// N-29. The order is the whole point: write, prove the sweep finds it, purge,
// prove the sweep no longer finds it. A sweep that only runs after the purge
// passes trivially when the data was never written or the scan is blind.
func TestPhase7RejectionErasesTheSelfRegistrationFromTheWholeDatabase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, runtime, fixture := openPhase7Integration(t, ctx)
	repository := newPhase7Repository(t, runtime)

	submission := submitPhase7SelfRegistration(t, ctx, repository, fixture)
	assertCanariesPresent(t, ctx, admin, fixture, submission)

	if _, _, err := repository.Reject(ctx, stay.RejectionCommand{
		Actor: principal(fixture.subject), StayID: submission.stayID,
		ExpectedVersion: submission.version, ReasonCode: stay.RejectionNotAGuest,
		IdempotencyKey: "phase7-reject-" + submission.stayID.String(),
		RequestID:      "request-" + submission.stayID.String(),
	}); err != nil {
		t.Fatalf("Reject() error = %v", err)
	}

	assertCanariesErased(t, ctx, admin, fixture, submission)
	assertAuditedFactSurvives(t, ctx, admin, submission.stayID, "stay.rejected")
}

// N-30. Erasing only on rejection would leave doing nothing as the cheapest way
// to keep a stranger's submission forever, so the expiry has to be proven by
// the same sweep and not by reading the code that calls the same helper.
func TestPhase7ExpiryErasesTheSelfRegistrationFromTheWholeDatabase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, runtime, fixture := openPhase7Integration(t, ctx)
	repository := newPhase7Repository(t, runtime)

	submission := submitPhase7SelfRegistration(t, ctx, repository, fixture)
	assertCanariesPresent(t, ctx, admin, fixture, submission)

	// The sweep reads the clock; moving the deadline is what makes the pending
	// row due without waiting 72 hours.
	if _, err := admin.Exec(ctx,
		`UPDATE core.stays SET approval_expires_at = now() - interval '1 hour'
		 WHERE id = $1`, submission.stayID,
	); err != nil {
		t.Fatalf("age the pending approval: %v", err)
	}
	expired, err := repository.ExpireApprovals(ctx)
	if err != nil {
		t.Fatalf("ExpireApprovals() error = %v", err)
	}
	if expired < 1 {
		t.Fatalf("ExpireApprovals() = %d, want at least the aged submission", expired)
	}

	assertCanariesErased(t, ctx, admin, fixture, submission)
	assertAuditedFactSurvives(t, ctx, admin, submission.stayID, "stay.approval_expired")
}

type phase7Submission struct {
	stayID       uuid.UUID
	submissionID uuid.UUID
	version      int64
	// visitorClientIDs are chosen by whoever submitted the form and are the
	// highest-entropy values the open channel accepts. Under ADR-040 the channel
	// takes no name and no document, so these are what a purge has to remove.
	visitorClientIDs []string
}

func openPhase7Integration(
	t *testing.T,
	ctx context.Context,
) (*pgxpool.Pool, *pgxpool.Pool, phase7Fixture) {
	t.Helper()
	admin := openIntegrationPool(t, ctx, "CUMURU_TEST_ADMIN_DATABASE_URL")
	runtime := openIntegrationPool(t, ctx, "CUMURU_TEST_DATABASE_URL")
	requireRuntimeRole(t, ctx, runtime)
	requirePhase7Schema(t, ctx, runtime)
	fixture := seedPhase7Fixture(t, ctx, admin)
	t.Cleanup(func() { cleanupPhase7Fixture(t, fixture, admin) })
	return admin, runtime, fixture
}

func requirePhase7Schema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var name *string
	err := pool.QueryRow(ctx,
		`SELECT to_regclass('platform.proof_of_work_spends')::text`,
	).Scan(&name)
	if err != nil || name == nil {
		t.Fatalf("phase 7 migrations are required: %v", err)
	}
}

func seedPhase7Fixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) phase7Fixture {
	t.Helper()
	fixture := phase7Fixture{
		organizationID: mustV7(t), accommodationID: mustV7(t),
		membershipID: mustV7(t), subject: "phase7-manager-" + mustV7(t).String(),
	}
	fixture.name = "Hospedagem canário " + mustV7(t).String()
	if _, err := pool.Exec(ctx,
		`INSERT INTO core.organizations (id, name) VALUES ($1, $2)`,
		fixture.organizationID, "Organização fictícia da Fase 7",
	); err != nil {
		t.Fatalf("seed organization: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO core.accommodations (id, organization_id, name, category, status)
		 VALUES ($1, $2, $3, 'formal_lodging', 'active')`,
		fixture.accommodationID, fixture.organizationID, fixture.name,
	); err != nil {
		t.Fatalf("seed accommodation: %v", err)
	}
	seedMembership(t, ctx, pool, fixture.membershipID, fixture.accommodationID, fixture.subject)
	return fixture
}

func newPhase7Repository(t *testing.T, pool *pgxpool.Pool) *store.StayRepository {
	t.Helper()
	built, err := store.NewPhase3(
		pool, 10*time.Second, integrationPhase2Config(t), config.Phase3Config{},
		store.WithPhase7Config(integrationPhase7Config(t)),
	)
	if err != nil {
		t.Fatalf("NewPhase3() error = %v", err)
	}
	repository, err := store.NewStayRepository(built)
	if err != nil {
		t.Fatalf("NewStayRepository() error = %v", err)
	}
	return repository
}

// The difficulty is pinned to one bit: this test proves erasure, not cost, and
// the cost curve is already proven by the unit tests of the issuer.
func integrationPhase7Config(t *testing.T) config.Phase7Config {
	t.Helper()
	return config.Phase7Config{
		Enabled:             true,
		SelfRegistrationURL: mustURL(t, "https://example.invalid/i"),
		ActivationURL:       mustURL(t, "https://example.invalid/ativacao"),
		ChallengeTTL:        10 * time.Minute,
		DifficultyBase:      1, DifficultyCeiling: 1, DifficultyRequestsPerBit: 1000,
		SelfServiceContextRateLimit: 1000, SelfServiceSubmitRateLimit: 1000,
		ActivationContextRateLimit: 1000, ActivationSubmitRateLimit: 1000,
		ExpirySweepBatchSize: 100,
		ProofOfWorkKeys:      testKeyring('p'),
	}
}

// submitPhase7SelfRegistration walks the real path end to end: the manager
// issues the poster, the open channel reads its context and the challenge, the
// challenge is solved with the client's own wire convention, and the submission
// lands. Seeding the rows directly would prove nothing about the code that
// writes them.
func submitPhase7SelfRegistration(
	t *testing.T,
	ctx context.Context,
	repository *store.StayRepository,
	fixture phase7Fixture,
) phase7Submission {
	t.Helper()
	token := issuePhase7Poster(t, ctx, repository, fixture)
	context7, err := repository.GetAccommodationInviteContext(ctx, stay.InviteRequest{
		Token: token, RateSubject: "203.0.113.0/24",
	})
	if err != nil {
		t.Fatalf("GetAccommodationInviteContext() error = %v", err)
	}
	command := phase7SelfRegistrationCommand(t, token, context7)
	accepted, _, err := repository.SubmitSelfRegistration(ctx, command)
	if err != nil {
		t.Fatalf("SubmitSelfRegistration() error = %v", err)
	}
	if accepted.ApprovalState != stay.ApprovalPending {
		t.Fatalf("approval state = %q, want pending", accepted.ApprovalState)
	}
	return phase7Submission{
		stayID: accepted.StayID, submissionID: accepted.SubmissionID,
		version: accepted.Version,
		visitorClientIDs: []string{
			command.Visitors[0].ClientID, command.Visitors[1].ClientID,
		},
	}
}

func issuePhase7Poster(
	t *testing.T,
	ctx context.Context,
	repository *store.StayRepository,
	fixture phase7Fixture,
) string {
	t.Helper()
	created, _, err := repository.CreateAccommodationInvite(ctx, stay.AccommodationInviteCommand{
		Actor: principal(fixture.subject), AccommodationID: fixture.accommodationID,
		PrivacyNoticeVersion: "2026-08", ExpectedVersion: 1,
		IdempotencyKey: "phase7-poster-" + fixture.accommodationID.String(),
		RequestID:      "request-poster-" + fixture.accommodationID.String(),
	})
	if err != nil {
		t.Fatalf("CreateAccommodationInvite() error = %v", err)
	}
	parsed, err := url.Parse(created.URL)
	if err != nil || parsed.Fragment == "" {
		t.Fatalf("poster URL = %q; the token must live in the fragment", created.URL)
	}
	return parsed.Fragment
}

func phase7SelfRegistrationCommand(
	t *testing.T,
	token string,
	poster stay.AccommodationInviteContext,
) stay.SelfRegistrationCommand {
	t.Helper()
	responsible := mustV7(t).String()
	companion := mustV7(t).String()
	// The correlation identifiers deliberately share nothing with the visitor
	// identifiers. The first draft derived them from the responsible visitor and
	// the sweep found that value in platform.audit_events.request_id after the
	// purge — the scan was right and the fixture was wrong.
	correlation := mustV7(t).String()
	return stay.SelfRegistrationCommand{
		Token: token, RateSubject: "203.0.113.0/24",
		ClientSubmissionID:   mustV7(t),
		PrivacyNoticeVersion: poster.PrivacyNoticeVersion,
		PlannedArrivalOn:     "2026-12-10", PlannedDepartureOn: "2026-12-12",
		Visitors: []stay.SelfServiceVisitor{
			{
				ClientID: responsible, Role: stay.VisitorResponsible,
				AgeBand: stay.Age25To34, ResidenceCountry: "BR",
				ResidenceState: "BA", ResidenceCityCode: "2925303",
			},
			{
				ClientID: companion, Role: stay.VisitorCompanion,
				AgeBand: stay.Age35To44, ResidenceCountry: "PT",
			},
		},
		ProofOfWork: stay.ProofOfWorkAnswer{
			Challenge: poster.ProofOfWork.Challenge,
			Solution: solvePhase7Challenge(
				t, poster.ProofOfWork.Challenge, poster.ProofOfWork.DifficultyBits,
			),
		},
		IdempotencyKey: "phase7-submit-" + correlation,
		RequestID:      "request-submit-" + correlation,
	}
}

// solvePhase7Challenge mirrors the browser: base64url of an 8-byte big-endian
// counter, digest over the concatenated UTF-8 of challenge and solution.
func solvePhase7Challenge(t *testing.T, challenge string, bits int) string {
	t.Helper()
	for counter := uint64(0); counter < 1<<24; counter++ {
		encoded := make([]byte, 8)
		binary.BigEndian.PutUint64(encoded, counter)
		solution := base64.RawURLEncoding.EncodeToString(encoded)
		digest := sha256.Sum256([]byte(challenge + solution))
		if phase7LeadingZeroBits(digest[:]) >= bits {
			return solution
		}
	}
	t.Fatalf("no solution found for %d bits", bits)
	return ""
}

func phase7LeadingZeroBits(digest []byte) int {
	bits := 0
	for _, value := range digest {
		for mask := byte(0x80); mask != 0; mask >>= 1 {
			if value&mask != 0 {
				return bits
			}
			bits++
		}
	}
	return bits
}

// assertCanariesPresent is the step that gives the sweep its weight. It proves
// the submitted values really landed and that the scan reaches the place they
// landed in; without it, the post-purge assertion is decoration.
func assertCanariesPresent(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	fixture phase7Fixture,
	submission phase7Submission,
) {
	t.Helper()
	control := scanDatabaseFor(t, ctx, admin, fixture.name)
	if len(control) == 0 {
		t.Fatal("the sweep did not find a value that certainly exists; it is blind")
	}
	for _, clientID := range submission.visitorClientIDs {
		hits := scanDatabaseFor(t, ctx, admin, clientID)
		if len(hits) == 0 {
			t.Fatalf("visitor %s was never stored, so erasing it proves nothing", clientID)
		}
		if !containsColumn(hits, "core.visitors.client_id") {
			t.Fatalf("visitor %s found only in %v, never in core.visitors", clientID, hits)
		}
	}
	assertVisitorCount(t, ctx, admin, submission.stayID, 2)
}

// assertCanariesErased sweeps every text, uuid, jsonb and bytea column of every
// schema — not the suspect tables. Sweeping everything is the point: the test
// has to break when a later phase adds a column that starts carrying the value.
func assertCanariesErased(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	fixture phase7Fixture,
	submission phase7Submission,
) {
	t.Helper()
	for _, clientID := range submission.visitorClientIDs {
		if hits := scanDatabaseFor(t, ctx, admin, clientID); len(hits) > 0 {
			t.Fatalf("visitor %s survived the purge in %v", clientID, hits)
		}
	}
	assertVisitorCount(t, ctx, admin, submission.stayID, 0)
	// The control must still be found, otherwise the empty result above would be
	// explained by a sweep that stopped working rather than by the purge.
	if control := scanDatabaseFor(t, ctx, admin, fixture.name); len(control) == 0 {
		t.Fatal("the sweep stopped finding the control; the empty result is not evidence")
	}
}

// The shell of the stay and the submission row survive on purpose: they carry no
// generalized attribute of anybody, only identifiers and the auditable fact that
// a submission happened.
func assertAuditedFactSurvives(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	stayID uuid.UUID,
	action string,
) {
	t.Helper()
	var events int
	if err := admin.QueryRow(ctx,
		`SELECT count(*) FROM platform.audit_events
		 WHERE entity_type = 'stay' AND entity_id = $1 AND action = $2`,
		stayID, action,
	).Scan(&events); err != nil {
		t.Fatalf("read audit events: %v", err)
	}
	if events != 1 {
		t.Fatalf("audit events for %s = %d, want exactly one", action, events)
	}
	var stays int
	if err := admin.QueryRow(ctx,
		`SELECT count(*) FROM core.stays WHERE id = $1 AND provenance = 'self_service'`,
		stayID,
	).Scan(&stays); err != nil {
		t.Fatalf("read stay shell: %v", err)
	}
	if stays != 1 {
		t.Fatalf("stay shell = %d, want the auditable shell to survive", stays)
	}
}

func assertVisitorCount(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	stayID uuid.UUID,
	want int,
) {
	t.Helper()
	var count int
	if err := admin.QueryRow(ctx,
		`SELECT count(*) FROM core.visitors WHERE stay_id = $1`, stayID,
	).Scan(&count); err != nil {
		t.Fatalf("count visitors: %v", err)
	}
	if count != want {
		t.Fatalf("visitors for the stay = %d, want %d", count, want)
	}
}

type scannedColumn struct {
	schema, table, column, dataType string
}

func (c scannedColumn) label() string {
	return c.schema + "." + c.table + "." + c.column
}

// expression casts to text, except for bytea, which would otherwise be compared
// as a hex literal and never match anything readable.
func (c scannedColumn) expression() string {
	quoted := pgx.Identifier{c.column}.Sanitize()
	if c.dataType == "bytea" {
		return "encode(" + quoted + ", 'escape')"
	}
	return quoted + "::text"
}

func scanDatabaseFor(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	value string,
) []string {
	t.Helper()
	hits := make([]string, 0)
	for _, column := range enumerateScannableColumns(t, ctx, admin) {
		if columnContains(t, ctx, admin, column, value) {
			hits = append(hits, column.label())
		}
	}
	return hits
}

func enumerateScannableColumns(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
) []scannedColumn {
	t.Helper()
	rows, err := admin.Query(ctx,
		`SELECT c.table_schema, c.table_name, c.column_name, c.data_type
		 FROM information_schema.columns AS c
		 JOIN information_schema.tables AS t
		   ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		 WHERE c.table_schema = ANY($1)
		   AND t.table_type = 'BASE TABLE'
		   AND c.data_type IN ('text','character varying','uuid','jsonb','bytea')
		 ORDER BY c.table_schema, c.table_name, c.column_name`,
		canaryScanSchemas,
	)
	if err != nil {
		t.Fatalf("enumerate columns: %v", err)
	}
	defer rows.Close()
	columns := make([]scannedColumn, 0)
	for rows.Next() {
		var column scannedColumn
		if err := rows.Scan(
			&column.schema, &column.table, &column.column, &column.dataType,
		); err != nil {
			t.Fatalf("scan column metadata: %v", err)
		}
		columns = append(columns, column)
	}
	if rows.Err() != nil {
		t.Fatalf("enumerate columns: %v", rows.Err())
	}
	requireScanReachesTheOpenChannel(t, columns)
	return columns
}

// The enumeration itself is guarded: a filter that silently excluded the tables
// the open channel writes would make every sweep pass.
func requireScanReachesTheOpenChannel(t *testing.T, columns []scannedColumn) {
	t.Helper()
	required := map[string]bool{
		"core.visitors.client_id":                   false,
		"core.group_submissions.request_hash":       false,
		"platform.idempotency_records.request_hash": false,
		"platform.audit_events.action":              false,
	}
	for _, column := range columns {
		if _, ok := required[column.label()]; ok {
			required[column.label()] = true
		}
	}
	for label, found := range required {
		if !found {
			t.Fatalf("the sweep does not reach %s; it cannot prove erasure", label)
		}
	}
}

func columnContains(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	column scannedColumn,
	value string,
) bool {
	t.Helper()
	relation := pgx.Identifier{column.schema, column.table}.Sanitize()
	query := fmt.Sprintf(
		`SELECT EXISTS (SELECT 1 FROM %s WHERE %s LIKE $1)`,
		relation, column.expression(),
	)
	var found bool
	if err := admin.QueryRow(ctx, query, "%"+value+"%").Scan(&found); err != nil {
		t.Fatalf("scan %s: %v", column.label(), err)
	}
	return found
}

func containsColumn(hits []string, label string) bool {
	for _, hit := range hits {
		if strings.EqualFold(hit, label) {
			return true
		}
	}
	return false
}

func cleanupPhase7Fixture(t *testing.T, fixture phase7Fixture, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	statements := []string{
		`DELETE FROM platform.audit_events WHERE entity_id IN
		   (SELECT id FROM core.stays WHERE accommodation_id = $1)
		   OR entity_id = $1`,
		`DELETE FROM platform.outbox_events WHERE aggregate_id IN
		   (SELECT id FROM core.stays WHERE accommodation_id = $1)
		   OR aggregate_id = $1`,
		`DELETE FROM platform.idempotency_records WHERE resource_id IN
		   (SELECT id FROM core.stays WHERE accommodation_id = $1)
		   OR resource_id = $1
		   OR resource_id IN (SELECT id FROM core.invites WHERE accommodation_id = $1)`,
		`DELETE FROM core.visitors WHERE stay_id IN
		   (SELECT id FROM core.stays WHERE accommodation_id = $1)`,
		`DELETE FROM core.group_submissions WHERE stay_id IN
		   (SELECT id FROM core.stays WHERE accommodation_id = $1)`,
		`DELETE FROM core.invites WHERE accommodation_id = $1`,
		`DELETE FROM core.stays WHERE accommodation_id = $1`,
		// D-10. Deleting only the capabilities scoped to this accommodation left
		// an account that spans two fixtures holding a live capability in the
		// other one, and activation_capabilities_account_id_fkey then refused the
		// account delete. Every A/B isolation test has exactly that shape, so the
		// helper has to clear the capabilities **of the accounts it is about to
		// delete**, wherever those capabilities live.
		`DELETE FROM auth.activation_capabilities
		   WHERE accommodation_id = $1
		      OR account_id IN (
		           SELECT m.oidc_subject::uuid FROM core.memberships m
		           WHERE m.accommodation_id = $1
		             AND m.oidc_issuer = 'https://auth.cumuru.local')`,
		`DELETE FROM auth.accounts WHERE id IN (
		   SELECT m.oidc_subject::uuid FROM core.memberships m
		   WHERE m.accommodation_id = $1 AND m.oidc_issuer = 'https://auth.cumuru.local')`,
		`DELETE FROM core.memberships WHERE accommodation_id = $1`,
		`DELETE FROM core.accommodations WHERE id = $1`,
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement, fixture.accommodationID); err != nil {
			t.Errorf("cleanup phase 7 fixture: %v", err)
		}
	}
	if _, err := pool.Exec(ctx,
		`DELETE FROM core.organizations WHERE id = $1`, fixture.organizationID,
	); err != nil {
		t.Errorf("cleanup phase 7 organization: %v", err)
	}
}

// N-17 through the real path, and N-34. The unit tests prove the nonce book is
// unique in SQL and that Verify refuses a tampered challenge; neither proves
// that a solved challenge cannot be spent twice through the code that submits.
// The second attempt carries a different idempotency key on purpose: the same
// key would be answered by the replay and would never reach the spend.
func TestPhase7SpentChallengeAndDecidedStayAreRefused(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, runtime, fixture := openPhase7Integration(t, ctx)
	_ = admin
	repository := newPhase7Repository(t, runtime)

	token := issuePhase7Poster(t, ctx, repository, fixture)
	poster, err := repository.GetAccommodationInviteContext(ctx, stay.InviteRequest{
		Token: token, RateSubject: "203.0.113.0/24",
	})
	if err != nil {
		t.Fatalf("GetAccommodationInviteContext() error = %v", err)
	}
	first := phase7SelfRegistrationCommand(t, token, poster)
	accepted, _, err := repository.SubmitSelfRegistration(ctx, first)
	if err != nil {
		t.Fatalf("SubmitSelfRegistration() error = %v", err)
	}
	if accepted.Version < 1 {
		t.Fatalf("accepted version = %d; the replay payload lost it", accepted.Version)
	}

	assertSpentChallengeIsRefused(t, ctx, repository, first)
	assertDecidedStayRefusesASecondDecision(t, ctx, repository, fixture, accepted)
}

func assertSpentChallengeIsRefused(
	t *testing.T,
	ctx context.Context,
	repository *store.StayRepository,
	first stay.SelfRegistrationCommand,
) {
	t.Helper()
	replay := first
	replay.ClientSubmissionID = mustV7(t)
	replay.IdempotencyKey = "phase7-replay-" + mustV7(t).String()
	replay.RequestID = "request-replay-" + mustV7(t).String()
	if _, _, err := repository.SubmitSelfRegistration(ctx, replay); err == nil {
		t.Fatal("the same solved challenge was spent twice")
	}
}

// Approving a stay that was just rejected, and rejecting one that was just
// approved, both have to fail on the stamp rather than on the version.
func assertDecidedStayRefusesASecondDecision(
	t *testing.T,
	ctx context.Context,
	repository *store.StayRepository,
	fixture phase7Fixture,
	accepted stay.SelfRegistrationAccepted,
) {
	t.Helper()
	approved, _, err := repository.Approve(ctx, stay.ApprovalCommand{
		Actor: principal(fixture.subject), StayID: accepted.StayID,
		ExpectedVersion: accepted.Version,
		IdempotencyKey:  "phase7-approve-" + accepted.StayID.String(),
		RequestID:       "request-approve-" + accepted.StayID.String(),
	})
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	_, _, err = repository.Reject(ctx, stay.RejectionCommand{
		Actor: principal(fixture.subject), StayID: accepted.StayID,
		ExpectedVersion: approved.Version, ReasonCode: stay.RejectionNotAGuest,
		IdempotencyKey: "phase7-reject-after-" + accepted.StayID.String(),
		RequestID:      "request-reject-after-" + accepted.StayID.String(),
	})
	if err == nil {
		t.Fatal("an approved stay accepted a rejection")
	}
}

// N-03 and N-14. The ADR-039 promises that rotation invalidates the previous
// poster. The observable contract has four parts and all four are asserted
// here, because the first three can pass while the fourth silently regresses:
// the second issue succeeds, exactly one active row remains, the old token
// stops resolving, and the new one works.
func TestPhase7PosterRotationReplacesThePreviousOne(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, runtime, fixture := openPhase7Integration(t, ctx)
	repository := newPhase7Repository(t, runtime)

	first := issuePhase7PosterWithKey(t, ctx, repository, fixture, "rotation-first")
	second := issuePhase7PosterWithKey(t, ctx, repository, fixture, "rotation-second")
	if first == second {
		t.Fatal("rotation reused the token; the wall poster was never replaced")
	}

	assertSingleActivePoster(t, ctx, admin, fixture)
	assertPosterResolves(t, ctx, repository, second, true)
	assertPosterResolves(t, ctx, repository, first, false)
}

// A rotation that answered the previous poster on a repeated key would mint a
// second wall poster for one operator intent. The idempotent replay has to win
// over the rotation, not the other way round.
func TestPhase7PosterRotationHonoursTheIdempotencyKey(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, runtime, fixture := openPhase7Integration(t, ctx)
	repository := newPhase7Repository(t, runtime)

	first := issuePhase7PosterWithKey(t, ctx, repository, fixture, "same-key")
	replay := issuePhase7PosterWithKey(t, ctx, repository, fixture, "same-key")
	if first != replay {
		t.Fatalf("the same idempotency key minted a second poster")
	}
	assertSingleActivePoster(t, ctx, admin, fixture)
	assertPosterResolves(t, ctx, repository, first, true)
}

func issuePhase7PosterWithKey(
	t *testing.T,
	ctx context.Context,
	repository *store.StayRepository,
	fixture phase7Fixture,
	key string,
) string {
	t.Helper()
	created, _, err := repository.CreateAccommodationInvite(ctx, stay.AccommodationInviteCommand{
		Actor: principal(fixture.subject), AccommodationID: fixture.accommodationID,
		PrivacyNoticeVersion: "2026-08", ExpectedVersion: 1,
		IdempotencyKey: "phase7-" + key + "-" + fixture.accommodationID.String(),
		RequestID:      "request-" + key + "-" + fixture.accommodationID.String(),
	})
	if err != nil {
		t.Fatalf("CreateAccommodationInvite(%s) error = %v", key, err)
	}
	parsed, err := url.Parse(created.URL)
	if err != nil || parsed.Fragment == "" {
		t.Fatalf("poster URL = %q", created.URL)
	}
	return parsed.Fragment
}

func assertSingleActivePoster(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	fixture phase7Fixture,
) {
	t.Helper()
	var active, total int
	err := admin.QueryRow(ctx,
		`SELECT count(*) FILTER (WHERE revoked_at IS NULL), count(*)
		 FROM core.invites
		 WHERE accommodation_id = $1 AND purpose = 'accommodation_self_registration'`,
		fixture.accommodationID,
	).Scan(&active, &total)
	if err != nil {
		t.Fatalf("count posters: %v", err)
	}
	if active != 1 {
		t.Fatalf("active posters = %d of %d, want exactly one", active, total)
	}
}

func assertPosterResolves(
	t *testing.T,
	ctx context.Context,
	repository *store.StayRepository,
	token string,
	want bool,
) {
	t.Helper()
	_, err := repository.GetAccommodationInviteContext(ctx, stay.InviteRequest{
		Token: token, RateSubject: "203.0.113.0/24",
	})
	if want && err != nil {
		t.Fatalf("the current poster does not resolve: %v", err)
	}
	if !want && err == nil {
		t.Fatal("the rotated-out poster still resolves")
	}
}

// N-43. The activation capability was corrected by analogy with the poster in
// R4 — same outbox aggregate defect — but nothing ever executed Create twice.
// This is that execution, in the shape of the poster rotation test.
//
// The five parts are asserted separately because "no error" would hide three of
// them: the count in particular, without which "the old one stopped working"
// would also pass if the re-issue had destroyed both.
func TestPhase7ActivationReissueReplacesThePreviousCapability(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, runtime, fixture := openPhase7Integration(t, ctx)
	repository := newPhase7ActivationRepository(t, runtime)
	email := "operadora-" + mustV7(t).String() + "@exemplo.invalid"

	first := issueActivationWithKey(t, ctx, repository, fixture, email, "reissue-first")
	second := issueActivationWithKey(t, ctx, repository, fixture, email, "reissue-second")
	if first == second {
		t.Fatal("the re-issue reused the token; the previous link was never replaced")
	}

	assertSingleOpenCapability(t, ctx, admin, fixture)
	assertCapabilityResolves(t, ctx, repository, second, true)
	assertCapabilityResolves(t, ctx, repository, first, false)
}

// The idempotent replay has to win over the re-issue: one operator intent must
// not mint two activation links.
func TestPhase7ActivationReissueHonoursTheIdempotencyKey(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, runtime, fixture := openPhase7Integration(t, ctx)
	repository := newPhase7ActivationRepository(t, runtime)
	email := "operadora-" + mustV7(t).String() + "@exemplo.invalid"

	first := issueActivationWithKey(t, ctx, repository, fixture, email, "same-key")
	replay := issueActivationWithKey(t, ctx, repository, fixture, email, "same-key")
	if first != replay {
		t.Fatal("the same idempotency key minted a second activation capability")
	}
	assertSingleOpenCapability(t, ctx, admin, fixture)
	assertCapabilityResolves(t, ctx, repository, first, true)
}

func newPhase7ActivationRepository(
	t *testing.T,
	pool *pgxpool.Pool,
) *store.ActivationRepository {
	t.Helper()
	built, err := store.NewPhase3(
		pool, 10*time.Second, integrationPhase2Config(t), config.Phase3Config{},
		store.WithPhase7Config(integrationPhase7Config(t)),
	)
	if err != nil {
		t.Fatalf("NewPhase3() error = %v", err)
	}
	repository, err := store.NewActivationRepository(built)
	if err != nil {
		t.Fatalf("NewActivationRepository() error = %v", err)
	}
	return repository
}

func issueActivationWithKey(
	t *testing.T,
	ctx context.Context,
	repository *store.ActivationRepository,
	fixture phase7Fixture,
	email string,
	key string,
) string {
	t.Helper()
	created, _, err := repository.Create(ctx, activation.CreateCommand{
		Actor: principal(fixture.subject), AccommodationID: fixture.accommodationID,
		Email: email, DisplayName: "Operadora fictícia", ExpectedVersion: 1,
		IdempotencyKey: "phase7-" + key + "-" + fixture.accommodationID.String(),
		RequestID:      "request-" + key + "-" + fixture.accommodationID.String(),
	})
	if err != nil {
		t.Fatalf("Create(%s) error = %v", key, err)
	}
	parsed, err := url.Parse(created.URL)
	if err != nil || parsed.Fragment == "" {
		t.Fatalf("activation URL = %q; the token must live in the fragment", created.URL)
	}
	return parsed.Fragment
}

// The count is what separates "the previous one was replaced" from "both were
// destroyed" and from "both are still open".
func assertSingleOpenCapability(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	fixture phase7Fixture,
) {
	t.Helper()
	var open, total int
	err := admin.QueryRow(ctx,
		`SELECT count(*) FILTER (WHERE consumed_at IS NULL AND revoked_at IS NULL),
		        count(*)
		 FROM auth.activation_capabilities WHERE accommodation_id = $1`,
		fixture.accommodationID,
	).Scan(&open, &total)
	if err != nil {
		t.Fatalf("count activation capabilities: %v", err)
	}
	if open != 1 {
		t.Fatalf("open capabilities = %d of %d, want exactly one", open, total)
	}
}

func assertCapabilityResolves(
	t *testing.T,
	ctx context.Context,
	repository *store.ActivationRepository,
	token string,
	want bool,
) {
	t.Helper()
	_, err := repository.Context(ctx, activation.Request{
		Token: token, RateSubject: "203.0.113.0/24",
	})
	if want && err != nil {
		t.Fatalf("the current activation capability does not resolve: %v", err)
	}
	if !want && err == nil {
		t.Fatal("the replaced activation capability still resolves")
	}
}

// An account that already activated keeps its credential. Re-issuing must
// refuse rather than downgrade it to pending or erase the password:
// accounts_credential_state_valid ties a missing hash to pending_activation, so
// a downgrade would either violate the CHECK or destroy a live credential.
//
// The assertions on status and hash are the point. "It returned an error" would
// also pass if the refusal happened after the damage.
func TestPhase7ActivationRefusesToDowngradeAnActivatedAccount(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, runtime, fixture := openPhase7Integration(t, ctx)
	repository := newPhase7ActivationRepository(t, runtime)
	email := "ativada-" + mustV7(t).String() + "@exemplo.invalid"

	token := issueActivationWithKey(t, ctx, repository, fixture, email, "before-activation")
	if err := repository.Complete(ctx, activation.CompleteCommand{
		Token: token, RateSubject: "203.0.113.0/24",
		Password: "uma-senha-bem-longa", RequestID: "request-complete-" + mustV7(t).String(),
	}); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	assertAccountCredential(t, ctx, admin, email, "active", true)

	_, _, err := repository.Create(ctx, activation.CreateCommand{
		Actor: principal(fixture.subject), AccommodationID: fixture.accommodationID,
		Email: email, DisplayName: "Operadora fictícia", ExpectedVersion: 1,
		IdempotencyKey: "phase7-after-activation-" + fixture.accommodationID.String(),
		RequestID:      "request-after-activation-" + fixture.accommodationID.String(),
	})
	if !errors.Is(err, activation.ErrConflict) {
		t.Fatalf("Create() after activation error = %v, want ErrConflict", err)
	}
	// The refusal has to leave the credential exactly as it was.
	assertAccountCredential(t, ctx, admin, email, "active", true)
}

func assertAccountCredential(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	email string,
	wantStatus string,
	wantHash bool,
) {
	t.Helper()
	var status string
	var hasHash bool
	err := admin.QueryRow(ctx,
		`SELECT status, password_hash IS NOT NULL FROM auth.accounts WHERE email = $1`,
		email,
	).Scan(&status, &hasHash)
	if err != nil {
		t.Fatalf("read account: %v", err)
	}
	if status != wantStatus || hasHash != wantHash {
		t.Fatalf("account = %s/hash:%t, want %s/hash:%t",
			status, hasHash, wantStatus, wantHash)
	}
}

// D-05, adopted from the QA probe that found it. accounts_email_idx is unique
// globally, not per accommodation, so a lookup by e-mail can land on an account
// that belongs to somebody else. The earlier reuse added a membership and kept
// the existing ones, handing whoever received the new link access to every
// accommodation that account already reached.
//
// The assertion is on reachability, not on the error: counting memberships is
// what distinguishes "refused" from "succeeded but harmless".
func TestPhase7ActivationReuseNeverCrossesAccommodations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, runtime, first := openPhase7Integration(t, ctx)
	second := seedPhase7Fixture(t, ctx, admin)
	t.Cleanup(func() { cleanupPhase7Fixture(t, second, admin) })

	repository := newPhase7ActivationRepository(t, runtime)
	email := "alvo-" + mustV7(t).String() + "@exemplo.invalid"

	issueActivationWithKey(t, ctx, repository, first, email, "cross-first")
	_, _, err := repository.Create(ctx, activation.CreateCommand{
		Actor: principal(second.subject), AccommodationID: second.accommodationID,
		Email: email, DisplayName: "Operadora fictícia", ExpectedVersion: 1,
		IdempotencyKey: "phase7-cross-second-" + second.accommodationID.String(),
		RequestID:      "request-cross-second-" + second.accommodationID.String(),
	})
	if !errors.Is(err, activation.ErrConflict) {
		t.Fatalf("Create() for a foreign accommodation error = %v, want ErrConflict", err)
	}
	assertAccountReaches(t, ctx, admin, email, first, second, 1)
}

// The refusals must be indistinguishable. A caller that could tell "belongs to
// another accommodation" from "already activated" would learn the state of
// somebody else's account from an endpoint it is allowed to call.
func TestPhase7ActivationRefusalsDoNotDiscloseAccountState(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, runtime, first := openPhase7Integration(t, ctx)
	second := seedPhase7Fixture(t, ctx, admin)
	t.Cleanup(func() { cleanupPhase7Fixture(t, second, admin) })
	repository := newPhase7ActivationRepository(t, runtime)

	foreign := "estranha-" + mustV7(t).String() + "@exemplo.invalid"
	issueActivationWithKey(t, ctx, repository, first, foreign, "disclosure-foreign")

	activated := "ativada-" + mustV7(t).String() + "@exemplo.invalid"
	token := issueActivationWithKey(t, ctx, repository, second, activated, "disclosure-active")
	if err := repository.Complete(ctx, activation.CompleteCommand{
		Token: token, RateSubject: "203.0.113.0/24",
		Password: "uma-senha-bem-longa", RequestID: "request-" + mustV7(t).String(),
	}); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	foreignErr := createActivationError(ctx, repository, second, foreign, "probe-foreign")
	activeErr := createActivationError(ctx, repository, second, activated, "probe-active")
	if foreignErr == nil || activeErr == nil {
		t.Fatalf("expected both probes refused; got %v and %v", foreignErr, activeErr)
	}
	if foreignErr.Error() != activeErr.Error() {
		t.Fatalf("refusals differ: %q versus %q", foreignErr, activeErr)
	}
}

func createActivationError(
	ctx context.Context,
	repository *store.ActivationRepository,
	fixture phase7Fixture,
	email string,
	key string,
) error {
	_, _, err := repository.Create(ctx, activation.CreateCommand{
		Actor: principal(fixture.subject), AccommodationID: fixture.accommodationID,
		Email: email, DisplayName: "Operadora fictícia", ExpectedVersion: 1,
		IdempotencyKey: "phase7-" + key + "-" + fixture.accommodationID.String(),
		RequestID:      "request-" + key + "-" + fixture.accommodationID.String(),
	})
	return err
}

func assertAccountReaches(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	email string,
	first phase7Fixture,
	second phase7Fixture,
	want int,
) {
	t.Helper()
	var reach int
	err := admin.QueryRow(ctx,
		`SELECT count(*) FROM core.memberships AS m
		 JOIN auth.accounts AS a ON a.id::text = m.oidc_subject
		 WHERE a.email = $1 AND m.active AND m.accommodation_id = ANY($2)`,
		email, []uuid.UUID{first.accommodationID, second.accommodationID},
	).Scan(&reach)
	if err != nil {
		t.Fatalf("count reachable accommodations: %v", err)
	}
	if reach != want {
		t.Fatalf("the account reaches %d accommodations, want %d", reach, want)
	}
}

// N-01. The poster carries no accommodation parameter: the target comes from the
// invite row, so a submission cannot be aimed elsewhere. What proves it is the
// count on the other side, not the success on this one — "the stay landed in A"
// and "no stay landed in B" are different facts and only the second is N-01.
func TestPhase7PosterOfOneAccommodationNeverReachesAnother(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	admin, runtime, first := openPhase7Integration(t, ctx)
	second := seedPhase7Fixture(t, ctx, admin)
	t.Cleanup(func() { cleanupPhase7Fixture(t, second, admin) })
	repository := newPhase7Repository(t, runtime)

	// Both accommodations publish a poster, so the only difference between them
	// is which token the submission carries.
	issuePhase7Poster(t, ctx, repository, second)
	submission := submitPhase7SelfRegistration(t, ctx, repository, first)

	assertStayCount(t, ctx, admin, first.accommodationID, 1)
	assertStayCount(t, ctx, admin, second.accommodationID, 0)
	assertStayBelongsTo(t, ctx, admin, submission.stayID, first.accommodationID)
}

// N-32. The queue is listStays with a filter, so its isolation is the Fase 2
// membership join. The listing deliberately carries **no** accommodation filter:
// if isolation came from the filter rather than from the join, this would still
// pass, and it must not.
func TestPhase7ApprovalQueueIsolatesAccommodations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	admin, runtime, first := openPhase7Integration(t, ctx)
	second := seedPhase7Fixture(t, ctx, admin)
	t.Cleanup(func() { cleanupPhase7Fixture(t, second, admin) })
	repository := newPhase7Repository(t, runtime)

	pending := submitPhase7SelfRegistration(t, ctx, repository, second)

	page, err := repository.List(ctx, principal(first.subject), stay.PageRequest{
		Limit: 100, ApprovalState: stay.ApprovalPending,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	for _, record := range page.Items {
		if record.ID == pending.stayID {
			t.Fatal("the manager of one accommodation read the queue of another")
		}
	}
	// And cannot decide it either: the lock refuses before any state changes.
	if _, _, err := repository.Approve(ctx, stay.ApprovalCommand{
		Actor: principal(first.subject), StayID: pending.stayID,
		ExpectedVersion: pending.version,
		IdempotencyKey:  "phase7-foreign-approve-" + pending.stayID.String(),
		RequestID:       "request-foreign-approve-" + pending.stayID.String(),
	}); err == nil {
		t.Fatal("the manager of one accommodation approved the queue of another")
	}
	assertApprovalState(t, ctx, admin, pending.stayID, "pending")
}

// N-22 and N-23 together, because they are the same counter seen from two
// angles. The limit is crossed for real: the stub that returned ErrRateLimited
// proved the HTTP shape and nothing about the limiter.
func TestPhase7RateLimitCrossesTheThresholdAndIsolatesAccommodations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	admin, runtime, first := openPhase7Integration(t, ctx)
	second := seedPhase7Fixture(t, ctx, admin)
	t.Cleanup(func() { cleanupPhase7Fixture(t, second, admin) })
	repository := newTightlyLimitedPhase7Repository(t, runtime)

	firstToken := issuePhase7Poster(t, ctx, repository, first)
	secondToken := issuePhase7Poster(t, ctx, repository, second)
	subject := "203.0.113." + mustV7(t).String()[:2] + "/24"

	// The threshold is two: the first two reads pass, the third is refused by
	// the limiter itself.
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err := repository.GetAccommodationInviteContext(ctx, stay.InviteRequest{
			Token: firstToken, RateSubject: subject,
		}); err != nil {
			t.Fatalf("read %d below the threshold: %v", attempt, err)
		}
	}
	_, err := repository.GetAccommodationInviteContext(ctx, stay.InviteRequest{
		Token: firstToken, RateSubject: subject,
	})
	if !errors.Is(err, stay.ErrRateLimited) {
		t.Fatalf("read above the threshold error = %v, want ErrRateLimited", err)
	}

	// N-23: the same client, over the limit on one poster, is untouched on the
	// other. The bucket is keyed by the capability, so two posters cannot share
	// one budget.
	if _, err := repository.GetAccommodationInviteContext(ctx, stay.InviteRequest{
		Token: secondToken, RateSubject: subject,
	}); err != nil {
		t.Fatalf("the second accommodation shared the exhausted bucket: %v", err)
	}
	assertDistinctBuckets(t, ctx, admin, subject)
}

func newTightlyLimitedPhase7Repository(
	t *testing.T,
	pool *pgxpool.Pool,
) *store.StayRepository {
	t.Helper()
	config7 := integrationPhase7Config(t)
	config7.SelfServiceContextRateLimit = 2
	built, err := store.NewPhase3(
		pool, 10*time.Second, integrationPhase2Config(t), config.Phase3Config{},
		store.WithPhase7Config(config7),
	)
	if err != nil {
		t.Fatalf("NewPhase3() error = %v", err)
	}
	repository, err := store.NewStayRepository(built)
	if err != nil {
		t.Fatalf("NewStayRepository() error = %v", err)
	}
	return repository
}

// Two buckets for one client is the observable form of "they do not share a
// budget"; a single row would mean the counter was keyed by the subject alone.
func assertDistinctBuckets(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	subject string,
) {
	t.Helper()
	var buckets int
	err := admin.QueryRow(ctx,
		`SELECT count(DISTINCT subject_hmac) FROM platform.rate_limit_buckets
		 WHERE scope = 'accommodation_invite_context'`,
	).Scan(&buckets)
	if err != nil {
		t.Fatalf("count rate limit buckets: %v", err)
	}
	if buckets < 2 {
		t.Fatalf("distinct buckets = %d, want one per poster", buckets)
	}
}

func assertStayCount(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	accommodationID uuid.UUID,
	want int,
) {
	t.Helper()
	var count int
	if err := admin.QueryRow(ctx,
		`SELECT count(*) FROM core.stays WHERE accommodation_id = $1`, accommodationID,
	).Scan(&count); err != nil {
		t.Fatalf("count stays: %v", err)
	}
	if count != want {
		t.Fatalf("stays in %s = %d, want %d", accommodationID, count, want)
	}
}

func assertStayBelongsTo(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	stayID uuid.UUID,
	accommodationID uuid.UUID,
) {
	t.Helper()
	var owner uuid.UUID
	if err := admin.QueryRow(ctx,
		`SELECT accommodation_id FROM core.stays WHERE id = $1`, stayID,
	).Scan(&owner); err != nil {
		t.Fatalf("read stay owner: %v", err)
	}
	if owner != accommodationID {
		t.Fatalf("stay landed in %s, want %s", owner, accommodationID)
	}
}

func assertApprovalState(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	stayID uuid.UUID,
	want string,
) {
	t.Helper()
	var state *string
	if err := admin.QueryRow(ctx,
		`SELECT approval_state FROM core.stays WHERE id = $1`, stayID,
	).Scan(&state); err != nil {
		t.Fatalf("read approval state: %v", err)
	}
	if state == nil || *state != want {
		t.Fatalf("approval state = %v, want %s", state, want)
	}
}
