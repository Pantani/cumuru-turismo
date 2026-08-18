//go:build integration

package store_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The submission helper of this suite always plans 2026-12-10 → 2026-12-12, so
// a pre_registered stay accrues forecast days on the 10th and the 11th. asOf is
// the day before, which puts both inside the next_30_days window (leads 1 and 2)
// and outside the published observed history.
const (
	selfServicePresenceAsOf   = "2026-12-09"
	selfServiceFirstNightOn   = "2026-12-10"
	selfServiceSecondNightOn  = "2026-12-11"
	selfServicePlannedArrival = "2026-12-10"
	selfServicePlannedLeaving = "2026-12-12"
)

// N-36. The predicate presenceEligible() already has a table test. What had no
// proof is the projection around it: that an approved self-registration really
// materializes rows in analytics.presence_days, and that a stay which stops
// being approved really loses them. Both halves run the Go reconciliation
// against PostgreSQL; no row of presence_days is ever written or deleted by the
// test itself.
func TestSelfServiceApprovalMaterializesPresenceAndRejectionErasesIt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	suite := openPresenceIntegration(t, ctx)

	submission := submitSelfServiceSelfRegistration(t, ctx, suite.stays, suite.fixture)
	reconcilePresence(t, ctx, suite)
	assertPresenceDays(t, ctx, suite.admin, submission.stayID, nil)

	approved := approveSubmission(t, ctx, suite, submission)
	reconcilePresence(t, ctx, suite)
	assertPresenceDays(t, ctx, suite.admin, submission.stayID, expectedForecastPresence())

	// The stamp is turned back by SQL and the visitors are left in place on
	// purpose: presence_days cascades from core.visitors, so a real rejection
	// would erase the rows even if the Go projection never deleted anything.
	// Keeping the visitors makes the reconciliation the only possible author of
	// the deletion.
	revokeApproval(t, ctx, suite, submission.stayID, approved)
	reconcilePresence(t, ctx, suite)
	assertVisitorCount(t, ctx, suite.admin, submission.stayID, 2)
	assertPresenceDays(t, ctx, suite.admin, submission.stayID, nil)
}

// N-36, the product path. Rejecting through the repository is what a manager
// actually does, and the stay must never have reached presence_days at all.
func TestSelfServiceRejectedSubmissionNeverReachesPresenceDays(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	suite := openPresenceIntegration(t, ctx)

	submission := submitSelfServiceSelfRegistration(t, ctx, suite.stays, suite.fixture)
	if _, _, err := suite.stays.Reject(ctx, stay.RejectionCommand{
		Actor: principal(suite.fixture.subject), StayID: submission.stayID,
		ExpectedVersion: submission.version, ReasonCode: stay.RejectionNotAGuest,
		IdempotencyKey: "presence-reject-" + submission.stayID.String(),
		RequestID:      "request-presence-reject-" + submission.stayID.String(),
	}); err != nil {
		t.Fatalf("Reject() error = %v", err)
	}
	reconcilePresence(t, ctx, suite)
	assertPresenceDays(t, ctx, suite.admin, submission.stayID, nil)
}

// N-37. The aggregate function is already proved by execution; what was never
// proved is that the difference survives the whole publication pipeline and
// lands in public_data.metric_cells. The background fixture sits one visitor
// and one accommodation below both thresholds, so approving the pending stay is
// the single change that flips the two nights from protected to published.
func TestSelfServiceApprovalChangesPublishedMetricCells(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	suite := openPresenceIntegration(t, ctx)
	seedPreferenceCatalog(t, ctx, suite.admin)
	seedBackgroundPresence(t, ctx, suite.admin)

	submission := submitSelfServiceSelfRegistration(t, ctx, suite.stays, suite.fixture)
	reconcilePresence(t, ctx, suite)
	pending := metricCellSnapshot(t, ctx, suite.admin, publishMetricCells(t, ctx, suite))

	approveSubmission(t, ctx, suite, submission)
	reconcilePresence(t, ctx, suite)
	published := metricCellSnapshot(t, ctx, suite.admin, publishMetricCells(t, ctx, suite))

	assertMetricCellsDiffer(t, pending, published)
	assertNightCell(t, pending, selfServiceFirstNightOn, "protected", "-")
	assertNightCell(t, pending, selfServiceSecondNightOn, "protected", "-")
	assertNightCell(t, published, selfServiceFirstNightOn, "published", "10")
	assertNightCell(t, published, selfServiceSecondNightOn, "published", "10")
}

type presenceSuite struct {
	admin     *pgxpool.Pool
	worker    *pgxpool.Pool
	fixture   selfServiceFixture
	stays     *store.StayRepository
	analytics *store.AnalyticsRepository
}

func openPresenceIntegration(t *testing.T, ctx context.Context) presenceSuite {
	t.Helper()
	admin, runtime, fixture := openSelfServiceIntegration(t, ctx)
	worker := openWorkerIntegrationPool(t, ctx)
	requireWorkerRole(t, ctx, worker)
	// Before as well as after: a publication left behind by an earlier run has
	// the same build fingerprint, and Publish would answer "replayed" instead of
	// materializing the cells this test is about to compare.
	cleanupPublications(t, admin)
	t.Cleanup(func() { cleanupPublications(t, admin) })
	return presenceSuite{
		admin: admin, worker: worker, fixture: fixture,
		stays: newSelfServiceRepository(t, runtime),
		analytics: store.NewAnalyticsRepository(
			store.NewCore(worker, 30*time.Second, integrationCoreConfig(t)),
			integrationAnalyticsConfig(),
		),
	}
}

// The thresholds are frozen by config.AnalyticsConfig.validateThresholds; using
// anything else here would prove a policy the product does not run.
func integrationAnalyticsConfig() config.AnalyticsConfig {
	return config.AnalyticsConfig{
		Enabled:              true,
		PrivacyPolicyVersion: "prototype-v1",
		MethodologyVersion:   "explainable-baseline-v1",
		DataMode:             "prototype_fixtures",
		PrimaryCellThreshold: 10, MinimumReportingAccommodations: 3,
		RoundingBase: 10, PreRegisteredWeight: 0.80,
		IncrementalInterval: 15 * time.Minute, FullReconciliationInterval: 24 * time.Hour,
		PublicDatabaseTimeout: 3 * time.Second,
	}
}

// reconcilePresence insists the run actually happened. Without the changed flag
// a fingerprint replay would return silently and every "no rows" assertion
// below would pass against a projection that never executed.
func reconcilePresence(t *testing.T, ctx context.Context, suite presenceSuite) {
	t.Helper()
	changed, err := suite.analytics.Reconcile(
		ctx, analytics.ReconciliationFull, mustCivilDate(t, selfServicePresenceAsOf),
	)
	if err != nil {
		t.Fatalf("Reconcile() error = %v; %s", err, unprojectableStays(t, ctx, suite.admin))
	}
	if !changed {
		t.Fatal("Reconcile() reported no run; the projection did not execute")
	}
}

// The projection has no scope argument: one call walks every stay in the
// database. A single stay left in an impossible shape by another fixture stops
// it with a bare "database unavailable", so the diagnosis is attached to the
// failure instead of being rediscovered by hand.
func unprojectableStays(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
) string {
	t.Helper()
	rows, err := admin.Query(ctx,
		`SELECT id::text || ' checked_in_at=' || checked_in_at::text
		        || ' planned_departure_on=' || planned_departure_on::text
		   FROM core.stays
		  WHERE status = 'checked_in'
		    AND (checked_in_at IS NULL OR checked_in_at::date >= planned_departure_on)
		  ORDER BY id`,
	)
	if err != nil {
		t.Fatalf("diagnose the reconciliation input: %v", err)
	}
	defer rows.Close()
	offenders := collectStrings(t, rows)
	if len(offenders) == 0 {
		return "no stay in the database has an impossible presence interval"
	}
	return "stays with an impossible presence interval: " + strings.Join(offenders, "; ")
}

func publishMetricCells(t *testing.T, ctx context.Context, suite presenceSuite) int64 {
	t.Helper()
	version, replayed, err := suite.analytics.BuildAndPublish(
		ctx, mustCivilDate(t, selfServicePresenceAsOf),
	)
	if err != nil {
		t.Fatalf("BuildAndPublish() error = %v", err)
	}
	if replayed {
		t.Fatal("BuildAndPublish() replayed a previous publication; the inputs did not move")
	}
	return version
}

func approveSubmission(
	t *testing.T,
	ctx context.Context,
	suite presenceSuite,
	submission selfServiceSubmission,
) int64 {
	t.Helper()
	result, _, err := suite.stays.Approve(ctx, stay.ApprovalCommand{
		Actor: principal(suite.fixture.subject), StayID: submission.stayID,
		ExpectedVersion: submission.version,
		IdempotencyKey:  "presence-approve-" + submission.stayID.String(),
		RequestID:       "request-presence-approve-" + submission.stayID.String(),
	})
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if result.Version <= submission.version {
		t.Fatalf("approval version = %d, want a bump over %d", result.Version, submission.version)
	}
	return result.Version
}

// revokeApproval moves the stamp back without touching core.visitors, and bumps
// the version the way a real decision does so the fingerprint of the next
// reconciliation cannot collide with the previous one.
func revokeApproval(
	t *testing.T,
	ctx context.Context,
	suite presenceSuite,
	stayID uuid.UUID,
	version int64,
) {
	t.Helper()
	tag, err := suite.admin.Exec(ctx,
		`UPDATE core.stays
		    SET approval_state = 'rejected', approved_at = NULL,
		        approval_reason_code = 'not_a_guest', version = version + 1
		  WHERE id = $1 AND version = $2`,
		stayID, version,
	)
	if err != nil {
		t.Fatalf("revoke the approval stamp: %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("revoked rows = %d, want exactly the approved stay", tag.RowsAffected())
	}
}

// expectedForecastPresence spells out every column the projection is
// responsible for. Counting rows alone would survive a projection that wrote
// the wrong dates, the wrong kind or the wrong attendance weight.
func expectedForecastPresence() []string {
	return []string{
		selfServiceFirstNightOn + "|forecast|0.800000",
		selfServiceFirstNightOn + "|forecast|0.800000",
		selfServiceSecondNightOn + "|forecast|0.800000",
		selfServiceSecondNightOn + "|forecast|0.800000",
	}
}

func assertPresenceDays(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	stayID uuid.UUID,
	want []string,
) {
	t.Helper()
	got := readPresenceDays(t, ctx, admin, stayID)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("analytics.presence_days for the stay = %v, want %v", got, want)
	}
}

func readPresenceDays(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	stayID uuid.UUID,
) []string {
	t.Helper()
	rows, err := admin.Query(ctx,
		`SELECT to_char(presence_on, 'YYYY-MM-DD') || '|' || kind || '|' || weight::text
		   FROM analytics.presence_days
		  WHERE stay_id = $1
		  ORDER BY presence_on, visitor_id`,
		stayID,
	)
	if err != nil {
		t.Fatalf("read analytics.presence_days: %v", err)
	}
	defer rows.Close()
	return collectStrings(t, rows)
}

type stringRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func collectStrings(t *testing.T, rows stringRows) []string {
	t.Helper()
	result := make([]string, 0)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatalf("scan row: %v", err)
		}
		result = append(result, value)
	}
	if rows.Err() != nil {
		t.Fatalf("iterate rows: %v", rows.Err())
	}
	return result
}

// metricCellSnapshot reads the published surface itself, not an intermediate
// abstraction: the row set of public_data.metric_cells for one version.
func metricCellSnapshot(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	version int64,
) []string {
	t.Helper()
	rows, err := admin.Query(ctx,
		`SELECT period_selector || '|' || to_char(period_start, 'YYYY-MM-DD')
		        || '|' || kind || '|' || status
		        || '|' || coalesce(published_central::text, '-')
		        || '|' || coalesce(published_value::text, '-')
		   FROM public_data.metric_cells
		  WHERE publication_version = $1
		  ORDER BY period_selector, period_start, kind`,
		version,
	)
	if err != nil {
		t.Fatalf("read public_data.metric_cells: %v", err)
	}
	defer rows.Close()
	snapshot := collectStrings(t, rows)
	if len(snapshot) == 0 {
		t.Fatal("the publication produced no metric cell; the comparison would prove nothing")
	}
	return snapshot
}

func assertMetricCellsDiffer(t *testing.T, pending, published []string) {
	t.Helper()
	if strings.Join(pending, "\n") == strings.Join(published, "\n") {
		t.Fatal("public_data.metric_cells is identical with and without the approval")
	}
}

func assertNightCell(t *testing.T, snapshot []string, night, status, central string) {
	t.Helper()
	want := fmt.Sprintf("next_30_days|%s|forecast|%s|%s|-", night, status, central)
	for _, cell := range snapshot {
		if cell == want {
			return
		}
	}
	t.Fatalf("no metric cell %q in %v", want, snapshot)
}

func mustCivilDate(t *testing.T, value string) stay.CivilDate {
	t.Helper()
	parsed, err := stay.ParseCivilDate(value)
	if err != nil {
		t.Fatalf("parse civil date %q: %v", value, err)
	}
	return parsed
}

// effectivePreferenceThreshold refuses to publish unless both first_visit and
// returning come back from analytics.aggregate_eligible_preferences. The
// function reads FROM analytics.metric_mappings and LEFT JOINs everything else,
// so two mapping rows over one draft question are enough to make both
// categories appear. No survey response is seeded: the preference half must
// stay constant between the two publications, otherwise the difference in
// public_data.metric_cells would not be attributable to the approval.
func seedPreferenceCatalog(t *testing.T, ctx context.Context, admin *pgxpool.Pool) {
	t.Helper()
	questionnaireID, versionID, questionID := mustV7(t), mustV7(t), mustV7(t)
	t.Cleanup(func() { cleanupPreferenceCatalog(t, admin, versionID, questionnaireID) })
	seedQuestionnaireVersion(t, ctx, admin, questionnaireID, versionID)
	seedPreferenceQuestion(t, ctx, admin, versionID, questionID)
	seedPreferenceMappings(t, ctx, admin, versionID, questionID)
}

func seedQuestionnaireVersion(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	questionnaireID, versionID uuid.UUID,
) {
	t.Helper()
	if _, err := admin.Exec(ctx,
		`INSERT INTO survey.questionnaires (id, stable_key, name)
		 VALUES ($1, $2, 'Perfil fictício de presença')`,
		questionnaireID,
		"perfil_presenca_"+strings.ReplaceAll(questionnaireID.String(), "-", ""),
	); err != nil {
		t.Fatalf("seed questionnaire: %v", err)
	}
	if _, err := admin.Exec(ctx,
		`INSERT INTO survey.questionnaire_versions (
		   id, questionnaire_id, version_number, status, title,
		   privacy_notice_version, last_editor_hmac, last_editor_key_version
		 ) VALUES ($1, $2, 1, 'draft', 'Perfil fictício de presença', '2026-01-01',
		           decode(repeat('e1', 32), 'hex'), 'v1')`,
		versionID, questionnaireID,
	); err != nil {
		t.Fatalf("seed questionnaire version: %v", err)
	}
}

func seedPreferenceQuestion(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	versionID, questionID uuid.UUID,
) {
	t.Helper()
	if _, err := admin.Exec(ctx,
		`INSERT INTO survey.questions (
		   id, questionnaire_version_id, stable_key, prompt, answer_type, required,
		   data_classification, purpose_code, retention_policy_code, analytics_key,
		   public_aggregation_allowed, minimum_public_cell, display_order
		 ) VALUES ($1, $2, 'primeira_visita', 'Primeira visita?', 'single_choice', true,
		           'operational', 'estatistica_publica', 'padrao', 'visit_profile',
		           true, 10, 1)`,
		questionID, versionID,
	); err != nil {
		t.Fatalf("seed preference question: %v", err)
	}
}

func seedPreferenceMappings(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	versionID, questionID uuid.UUID,
) {
	t.Helper()
	for _, category := range []string{"first_visit", "returning"} {
		if _, err := admin.Exec(ctx,
			`INSERT INTO analytics.metric_mappings (
			   privacy_policy_version, metric_code, questionnaire_version_id,
			   question_id, source_value, category_code
			 ) VALUES ('prototype-v1', 'first_visit_share', $1, $2, $3, $3)`,
			versionID, questionID, category,
		); err != nil {
			t.Fatalf("seed metric mapping %s: %v", category, err)
		}
	}
}

func cleanupPreferenceCatalog(
	t *testing.T,
	admin *pgxpool.Pool,
	versionID, questionnaireID uuid.UUID,
) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	statements := []string{
		`DELETE FROM analytics.metric_mappings WHERE questionnaire_version_id = $1`,
		`DELETE FROM survey.questions WHERE questionnaire_version_id = $1`,
		`DELETE FROM survey.questionnaire_versions WHERE id = $1`,
	}
	for _, statement := range statements {
		if _, err := admin.Exec(ctx, statement, versionID); err != nil {
			t.Fatalf("cleanup preference catalog: %v", err)
		}
	}
	if _, err := admin.Exec(ctx,
		`DELETE FROM survey.questionnaires WHERE id = $1`, questionnaireID,
	); err != nil {
		t.Fatalf("cleanup questionnaire: %v", err)
	}
}

// The background sits at eight visitors over two accommodations: below both
// thresholds. The pending self-registration carries two visitors in a third
// accommodation, so approving it is what pushes the night over k >= 10 and over
// three reporting accommodations.
const (
	backgroundAccommodations = 2
	backgroundVisitorsEach   = 4
)

func seedBackgroundPresence(t *testing.T, ctx context.Context, admin *pgxpool.Pool) {
	t.Helper()
	organizationID := mustV7(t)
	if _, err := admin.Exec(ctx,
		`INSERT INTO core.organizations (id, name) VALUES ($1, $2)`,
		organizationID, "Organização fictícia de fundo do autoatendimento",
	); err != nil {
		t.Fatalf("seed background organization: %v", err)
	}
	t.Cleanup(func() { cleanupBackgroundPresence(t, admin, organizationID) })
	for index := 0; index < backgroundAccommodations; index++ {
		seedBackgroundAccommodation(t, ctx, admin, organizationID, index)
	}
}

func seedBackgroundAccommodation(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	organizationID uuid.UUID,
	index int,
) {
	t.Helper()
	accommodationID, membershipID, stayID := mustV7(t), mustV7(t), mustV7(t)
	if _, err := admin.Exec(ctx,
		`INSERT INTO core.accommodations (id, organization_id, name, category, status, capacity)
		 VALUES ($1, $2, $3, 'formal_lodging', 'active', 20)`,
		accommodationID, organizationID, fmt.Sprintf("Hospedagem de fundo %d", index),
	); err != nil {
		t.Fatalf("seed background accommodation: %v", err)
	}
	seedMembership(t, ctx, admin, membershipID, accommodationID,
		"background-manager-"+membershipID.String())
	seedBackgroundStay(t, ctx, admin, accommodationID, membershipID, stayID)
	seedBackgroundVisitors(t, ctx, admin, stayID)
}

func seedBackgroundStay(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	accommodationID, membershipID, stayID uuid.UUID,
) {
	t.Helper()
	if _, err := admin.Exec(ctx,
		`INSERT INTO core.stays (
		   id, accommodation_id, created_by_membership_id, status, client_submission_id,
		   planned_arrival_on, planned_departure_on, expected_guest_count
		 ) VALUES ($1, $2, $3, 'pre_registered', $4, $5::date, $6::date, $7)`,
		stayID, accommodationID, membershipID, mustV7(t),
		selfServicePlannedArrival, selfServicePlannedLeaving, backgroundVisitorsEach,
	); err != nil {
		t.Fatalf("seed background stay: %v", err)
	}
}

func seedBackgroundVisitors(
	t *testing.T,
	ctx context.Context,
	admin *pgxpool.Pool,
	stayID uuid.UUID,
) {
	t.Helper()
	roles := []string{"responsible", "companion", "companion", "companion"}
	for _, role := range roles[:backgroundVisitorsEach] {
		if _, err := admin.Exec(ctx,
			`INSERT INTO core.visitors (id, stay_id, client_id, role, age_band, residence_country)
			 VALUES ($1, $2, $3, $4::core.visitor_role, '25_34', 'BR')`,
			mustV7(t), stayID, mustV7(t), role,
		); err != nil {
			t.Fatalf("seed background visitor: %v", err)
		}
	}
}

func cleanupBackgroundPresence(t *testing.T, admin *pgxpool.Pool, organizationID uuid.UUID) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, statement := range backgroundCleanupStatements() {
		if _, err := admin.Exec(ctx, statement, organizationID); err != nil {
			t.Fatalf("cleanup background presence: %v", err)
		}
	}
}

func backgroundCleanupStatements() []string {
	return []string{
		`DELETE FROM core.visitors WHERE stay_id IN (
		   SELECT s.id FROM core.stays AS s JOIN core.accommodations AS a
		     ON a.id = s.accommodation_id WHERE a.organization_id = $1)`,
		`DELETE FROM core.stays WHERE accommodation_id IN (
		   SELECT id FROM core.accommodations WHERE organization_id = $1)`,
		`DELETE FROM core.memberships WHERE accommodation_id IN (
		   SELECT id FROM core.accommodations WHERE organization_id = $1)`,
		`DELETE FROM core.accommodations WHERE organization_id = $1`,
		`DELETE FROM core.organizations WHERE id = $1`,
	}
}

// The publication surface is append-only for the worker, so the cleanup runs as
// migration_admin. Leaving versions behind would let a later run of this suite
// compare against a publication it did not create.
func cleanupPublications(t *testing.T, admin *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, statement := range publicationCleanupStatements() {
		if _, err := admin.Exec(ctx, statement); err != nil {
			t.Fatalf("cleanup publications: %v", err)
		}
	}
}

func publicationCleanupStatements() []string {
	return []string{
		`DELETE FROM public_data.current_publication`,
		`DELETE FROM public_data.metric_cells`,
		`DELETE FROM public_data.publications`,
		`DELETE FROM analytics.staged_metric_cells`,
		`DELETE FROM analytics.publication_runs`,
		`DELETE FROM analytics.quality_coverage`,
		`DELETE FROM analytics.quality_snapshots`,
		`DELETE FROM analytics.reconciliation_runs`,
	}
}
