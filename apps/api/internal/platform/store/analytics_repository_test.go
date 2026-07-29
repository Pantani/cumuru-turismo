package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type publicationQueries struct {
	generated.Querier
	calls      []string
	failCellAt int
	cellCalls  int
}

type presenceQueries struct {
	generated.Querier
	upserts int
	deletes int
}

type reconciliationSelectionQueries struct {
	generated.Querier
	authoritativeCalls int
	eligibleOnlyCalls  int
}

type metricCatalogQueries struct {
	generated.Querier
	readCalls int
}

type aggregationFailureQueries struct {
	generated.Querier
	recordCalls               int
	aggregationFailures       int32
	currentPublicationVersion int64
	recordErr                 error
}

func (f *aggregationFailureQueries) RecordAggregationFailureQualitySnapshot(
	_ context.Context,
	params generated.RecordAggregationFailureQualitySnapshotParams,
) (generated.RecordAggregationFailureQualitySnapshotRow, error) {
	f.recordCalls++
	if f.recordErr != nil {
		return generated.RecordAggregationFailureQualitySnapshotRow{}, f.recordErr
	}
	f.aggregationFailures++
	return generated.RecordAggregationFailureQualitySnapshotRow{
		ID:                  params.SnapshotID,
		AggregationFailures: f.aggregationFailures,
		CoverageCount:       2,
	}, nil
}

func (f *metricCatalogQueries) ListActiveMetricCatalog(
	context.Context,
	string,
) ([]generated.AnalyticsMetricCatalog, error) {
	f.readCalls++
	return []generated.AnalyticsMetricCatalog{
		{
			PrivacyPolicyVersion: "prototype-v1", MetricCode: "presence",
			PeriodSelector: "recent_30_days", DimensionCode: "none", Unit: "person_day",
			MinimumPublicCell: 10, MinimumReportingAccommodations: 3, Active: true,
		},
		{
			PrivacyPolicyVersion: "prototype-v1", MetricCode: "presence",
			PeriodSelector: "next_30_days", DimensionCode: "none", Unit: "person_day",
			MinimumPublicCell: 10, MinimumReportingAccommodations: 3, Active: true,
		},
		{
			PrivacyPolicyVersion: "prototype-v1", MetricCode: "first_visit_share",
			PeriodSelector: "last_complete_month", DimensionCode: "visit_profile",
			Unit:              "survey_response",
			MinimumPublicCell: 10, MinimumReportingAccommodations: 3, Active: true,
		},
	}, nil
}

func (f *reconciliationSelectionQueries) ListPresenceReconciliationStays(
	context.Context,
) ([]generated.ListPresenceReconciliationStaysRow, error) {
	f.authoritativeCalls++
	return []generated.ListPresenceReconciliationStaysRow{{
		ID:              idToPG(uuid.MustParse("019f0000-0000-7000-8000-000000000001")),
		Status:          generated.CoreStayStatusCancelled,
		ExpectedVersion: 4,
	}}, nil
}

func (f *reconciliationSelectionQueries) ListPresenceSourceStays(
	context.Context,
) ([]generated.ListPresenceSourceStaysRow, error) {
	f.eligibleOnlyCalls++
	return nil, nil
}

func (f *reconciliationSelectionQueries) ListPresenceDaysForStay(
	context.Context,
	pgtype.UUID,
) ([]generated.AnalyticsPresenceDay, error) {
	return nil, nil
}

func (f *presenceQueries) UpsertPresenceDay(
	context.Context,
	generated.UpsertPresenceDayParams,
) (int64, error) {
	f.upserts++
	return 1, nil
}

func (f *presenceQueries) DeletePresenceDay(
	context.Context,
	generated.DeletePresenceDayParams,
) (int64, error) {
	f.deletes++
	return 1, nil
}

func (f *publicationQueries) InsertNextPublication(
	context.Context,
	generated.InsertNextPublicationParams,
) (int64, error) {
	f.calls = append(f.calls, "publication")
	return 4, nil
}

func (f *publicationQueries) InsertPublishedMetricCell(
	context.Context,
	generated.InsertPublishedMetricCellParams,
) error {
	f.calls = append(f.calls, "cell")
	f.cellCalls++
	if f.failCellAt == f.cellCalls {
		return errors.New("cell insert failed")
	}
	return nil
}

func (f *publicationQueries) PromoteCurrentPublication(context.Context, int64) (int64, error) {
	f.calls = append(f.calls, "promote")
	return 1, nil
}

func (f *publicationQueries) CompletePublicationRun(
	context.Context,
	generated.CompletePublicationRunParams,
) (int64, error) {
	f.calls = append(f.calls, "complete")
	return 1, nil
}

func TestPublishSnapshotPromotesOnlyAfterAllCells(t *testing.T) {
	t.Parallel()

	queries := &publicationQueries{}
	publication := analytics.Publication{
		RunID:  uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		AsOfOn: "2026-07-28", DataMode: "prototype_fixtures",
		PrivacyPolicyVersion: "prototype-v1",
		MethodologyVersion:   "explainable-baseline-v1",
		CoverageStatus:       "protected",
		PublishedAt:          time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC),
		Cells: []analytics.PublicationCell{
			{CellKey: "a", Status: analytics.CellProtected},
			{CellKey: "b", Status: analytics.CellProtected},
		},
	}

	version, err := publishSnapshot(context.Background(), queries, publication, "fingerprint")
	if err != nil {
		t.Fatalf("publishSnapshot() error = %v", err)
	}
	if version != 4 {
		t.Fatalf("version = %d", version)
	}
	want := []string{"publication", "cell", "cell", "promote", "complete"}
	if len(queries.calls) != len(want) {
		t.Fatalf("calls = %v", queries.calls)
	}
	for index := range want {
		if queries.calls[index] != want[index] {
			t.Fatalf("calls = %v", queries.calls)
		}
	}
}

func TestPublishSnapshotPreservesLastValidPointerOnPartialFailure(t *testing.T) {
	t.Parallel()

	queries := &publicationQueries{failCellAt: 2}
	publication := analytics.Publication{
		RunID:       uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		PublishedAt: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC),
		Cells: []analytics.PublicationCell{
			{CellKey: "a", Status: analytics.CellProtected},
			{CellKey: "b", Status: analytics.CellProtected},
		},
	}

	if _, err := publishSnapshot(
		context.Background(), queries, publication, "fingerprint",
	); err == nil {
		t.Fatal("publishSnapshot() error = nil")
	}
	for _, call := range queries.calls {
		if call == "promote" || call == "complete" {
			t.Fatalf("partial publication reached %q: %v", call, queries.calls)
		}
	}
}

func TestOptionalQualityCountsExposeExplicitNotAvailable(t *testing.T) {
	t.Parallel()

	got := optionalCount(nil, "phase_not_implemented")
	if got.Status != "not_available" || got.ReasonCode != "phase_not_implemented" {
		t.Fatalf("optionalCount() = %#v", got)
	}
}

func TestReplacePresenceFactsDeletesOnlyObsoleteVersionedFacts(t *testing.T) {
	t.Parallel()

	stayID := uuid.MustParse("019f0000-0000-7000-8000-000000000001")
	visitorID := uuid.MustParse("019f0000-0000-7000-8000-000000000002")
	queries := &presenceQueries{}
	existing := []generated.AnalyticsPresenceDay{{
		StayID: idToPG(stayID), VisitorID: idToPG(visitorID),
		PresenceOn: dateToPG("2026-07-28"), SourceStayVersion: 2,
	}}

	err := replacePresenceFacts(context.Background(), queries, 3, existing, nil)
	if err != nil {
		t.Fatalf("replacePresenceFacts() error = %v", err)
	}
	if queries.deletes != 1 || queries.upserts != 0 {
		t.Fatalf("upserts=%d deletes=%d", queries.upserts, queries.deletes)
	}
}

func TestReconciliationFingerprintIgnoresVisitorOrder(t *testing.T) {
	t.Parallel()

	firstID := uuid.MustParse("019f0000-0000-7000-8000-000000000001")
	secondID := uuid.MustParse("019f0000-0000-7000-8000-000000000002")
	source := reconciliationSource{presence: analytics.PresenceSource{
		StayID: firstID, Status: "pre_registered",
		PlannedArrival:   stay.MustCivilDate("2026-07-28"),
		PlannedDeparture: stay.MustCivilDate("2026-07-29"),
		Version:          1, VisitorIDs: []uuid.UUID{firstID, secondID},
	}}
	reversed := source
	reversed.presence.VisitorIDs = []uuid.UUID{secondID, firstID}
	asOf := stay.MustCivilDate("2026-07-28")

	first := reconciliationFingerprint(analytics.ReconciliationFull, asOf, []reconciliationSource{source})
	second := reconciliationFingerprint(analytics.ReconciliationFull, asOf, []reconciliationSource{reversed})
	if first != second {
		t.Fatalf("fingerprints differ: %q != %q", first, second)
	}

	weight, err := numericFromFloat(1)
	if err != nil {
		t.Fatal(err)
	}
	withDrift := source
	withDrift.existing = []generated.AnalyticsPresenceDay{{
		VisitorID: idToPG(secondID), PresenceOn: dateToPG("2026-07-28"),
		Kind: "observed", Weight: weight, SourceStayVersion: 1,
		AsOfOn: dateToPG("2026-07-28"),
	}}
	driftFingerprint := reconciliationFingerprint(
		analytics.ReconciliationFull, asOf, []reconciliationSource{withDrift},
	)
	if driftFingerprint == first {
		t.Fatal("existing fact drift did not change reconciliation fingerprint")
	}
}

func TestIncrementalCleanupDeletesCancelledAndNoShowFacts(t *testing.T) {
	t.Parallel()

	assertIncrementalCleanup(t, stay.StatusCancelled)
	assertIncrementalCleanup(t, stay.StatusNoShow)
}

func assertIncrementalCleanup(t *testing.T, status stay.Status) {
	t.Helper()
	stayID := uuid.MustParse("019f0000-0000-7000-8000-000000000001")
	visitorID := uuid.MustParse("019f0000-0000-7000-8000-000000000002")
	queries := &presenceQueries{}
	existing := []generated.AnalyticsPresenceDay{{
		StayID: idToPG(stayID), VisitorID: idToPG(visitorID),
		PresenceOn: dateToPG("2026-07-28"), SourceStayVersion: 3,
	}}

	if err := replacePresenceFacts(
		context.Background(), queries, 4, existing, nil,
	); err != nil {
		t.Fatalf("%s incremental cleanup error = %v", status, err)
	}
	if queries.deletes != 1 {
		t.Fatalf("%s deletes = %d", status, queries.deletes)
	}
	if err := replacePresenceFacts(
		context.Background(), queries, 4, nil, nil,
	); err != nil {
		t.Fatalf("%s incremental repeat error = %v", status, err)
	}
	if queries.deletes != 1 {
		t.Fatalf("%s repeat was not a no-op: deletes=%d", status, queries.deletes)
	}
}

func TestIncrementalSelectionIncludesStaleIneligibleStays(t *testing.T) {
	t.Parallel()

	queries := &reconciliationSelectionQueries{}
	sources, err := loadReconciliationSources(
		context.Background(), queries, analytics.ReconciliationIncremental,
	)
	if err != nil {
		t.Fatalf("loadReconciliationSources() error = %v", err)
	}
	if len(sources) != 1 || sources[0].presence.Status != stay.StatusCancelled {
		t.Fatalf("sources = %#v", sources)
	}
	if queries.authoritativeCalls != 1 || queries.eligibleOnlyCalls != 0 {
		t.Fatalf(
			"authoritative=%d eligible-only=%d",
			queries.authoritativeCalls,
			queries.eligibleOnlyCalls,
		)
	}
}

func TestPreferenceCountPeriodUsesPreviousCivilMonthInBahia(t *testing.T) {
	t.Parallel()

	params := preferenceCountParams(
		"prototype-v1",
		stay.MustCivilDate("2026-07-28"),
	)
	if !params.PeriodStart.Valid || !params.PeriodEnd.Valid {
		t.Fatalf("params = %#v", params)
	}
	if got := params.PeriodStart.Time.UTC().Format(time.RFC3339); got != "2026-06-01T03:00:00Z" {
		t.Fatalf("period start = %s", got)
	}
	if got := params.PeriodEnd.Time.UTC().Format(time.RFC3339); got != "2026-07-01T03:00:00Z" {
		t.Fatalf("period end = %s", got)
	}
}

func TestPreferenceThresholdUsesHistoricalMaximumAndComplementaryProtection(t *testing.T) {
	t.Parallel()

	repository := &AnalyticsRepository{phase4: config.Phase4Config{
		PrivacyPolicyVersion:           "prototype-v1",
		PrimaryCellThreshold:           10,
		MinimumReportingAccommodations: 3,
	}}
	asOf := stay.MustCivilDate("2026-07-28")
	rows := []generated.ListEligiblePreferenceCountsRow{
		{
			PrivacyPolicyVersion: "prototype-v1", MetricCode: "first_visit_share",
			CategoryCode: "first_visit", SampleSize: 20, AccommodationCount: 3,
			MinimumPublicCell: 20,
		},
		{
			PrivacyPolicyVersion: "prototype-v1", MetricCode: "first_visit_share",
			CategoryCode: "returning", SampleSize: 19, AccommodationCount: 3,
			MinimumPublicCell: 10,
		},
	}
	cells, err := repository.buildPreferenceCells(asOf, rows)
	if err != nil {
		t.Fatalf("buildPreferenceCells() error = %v", err)
	}
	if cells[0].Status != analytics.CellProtected ||
		cells[1].Status != analytics.CellProtected {
		t.Fatalf("19/20 complementary cells = %#v", cells)
	}
	rows[1].SampleSize = 20
	cells, err = repository.buildPreferenceCells(asOf, rows)
	if err != nil {
		t.Fatalf("buildPreferenceCells(20/20) error = %v", err)
	}
	if cells[0].Status != analytics.CellPublished ||
		cells[1].Status != analytics.CellPublished {
		t.Fatalf("20/20 cells = %#v", cells)
	}
}

func TestPreferenceThresholdAllowsZeroCountAndProtectsFamily(t *testing.T) {
	t.Parallel()

	repository := &AnalyticsRepository{phase4: config.Phase4Config{
		PrivacyPolicyVersion:           "prototype-v1",
		PrimaryCellThreshold:           10,
		MinimumReportingAccommodations: 3,
	}}
	asOf := stay.MustCivilDate("2026-07-28")
	rows := []generated.ListEligiblePreferenceCountsRow{
		{
			PrivacyPolicyVersion: "prototype-v1", MetricCode: "first_visit_share",
			CategoryCode: "first_visit", SampleSize: 19, AccommodationCount: 3,
			MinimumPublicCell: 20,
		},
		{
			PrivacyPolicyVersion: "prototype-v1", MetricCode: "first_visit_share",
			CategoryCode: "returning", SampleSize: 0, AccommodationCount: 3,
			MinimumPublicCell: 20,
		},
	}
	cells, err := repository.buildPreferenceCells(asOf, rows)
	if err != nil {
		t.Fatalf("buildPreferenceCells(19/0) error = %v", err)
	}
	if cells[0].Status != analytics.CellProtected ||
		cells[1].Status != analytics.CellProtected {
		t.Fatalf("19/0 complementary cells = %#v", cells)
	}
}

func TestPreferenceThresholdFailsClosedWhenAggregateIsInvalid(t *testing.T) {
	t.Parallel()

	repository := &AnalyticsRepository{phase4: config.Phase4Config{
		PrivacyPolicyVersion:           "prototype-v1",
		PrimaryCellThreshold:           10,
		MinimumReportingAccommodations: 3,
	}}
	_, err := repository.buildPreferenceCells(
		stay.MustCivilDate("2026-07-28"),
		[]generated.ListEligiblePreferenceCountsRow{{
			PrivacyPolicyVersion: "prototype-v1", MetricCode: "first_visit_share",
			CategoryCode: "first_visit", SampleSize: 20, AccommodationCount: 3,
			MinimumPublicCell: 9,
		}},
	)
	if err == nil {
		t.Fatal("buildPreferenceCells() error = nil")
	}

	_, err = repository.buildPreferenceCells(
		stay.MustCivilDate("2026-07-28"),
		[]generated.ListEligiblePreferenceCountsRow{{
			PrivacyPolicyVersion: "prototype-v1", MetricCode: "first_visit_share",
			CategoryCode: "first_visit", SampleSize: 20, AccommodationCount: 3,
			MinimumPublicCell: 10,
		}},
	)
	if err == nil {
		t.Fatal("buildPreferenceCells(missing returning) error = nil")
	}
}

func TestPublicationCatalogValidationIsReadOnly(t *testing.T) {
	t.Parallel()

	queries := &metricCatalogQueries{}
	repository := &AnalyticsRepository{phase4: config.Phase4Config{
		PrivacyPolicyVersion:           "prototype-v1",
		PrimaryCellThreshold:           10,
		MinimumReportingAccommodations: 3,
	}}
	if err := repository.validateMetricCatalog(context.Background(), queries); err != nil {
		t.Fatalf("validateMetricCatalog() error = %v", err)
	}
	if queries.readCalls != 1 {
		t.Fatalf("catalog read calls = %d", queries.readCalls)
	}
}

func TestBuildFailureRecordsQualityAndPreservesLastValidPublication(t *testing.T) {
	t.Parallel()

	queries := &aggregationFailureQueries{currentPublicationVersion: 7}
	database := New(queries, time.Second)
	database.now = func() time.Time {
		return time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	}
	repository := NewAnalyticsRepository(database, config.Phase4Config{})

	_, _, err := repository.BuildAndPublish(
		context.Background(),
		stay.MustCivilDate("2026-07-28"),
	)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("BuildAndPublish() error = %v", err)
	}
	if queries.recordCalls != 1 || queries.aggregationFailures != 1 {
		t.Fatalf(
			"quality failure calls/count = %d/%d",
			queries.recordCalls,
			queries.aggregationFailures,
		)
	}
	if queries.currentPublicationVersion != 7 {
		t.Fatalf("current publication = %d", queries.currentPublicationVersion)
	}

	_, _, err = repository.BuildAndPublish(
		context.Background(),
		stay.MustCivilDate("2026-07-28"),
	)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("BuildAndPublish(second) error = %v", err)
	}
	if queries.recordCalls != 2 || queries.aggregationFailures != 2 {
		t.Fatalf(
			"incremented quality calls/count = %d/%d",
			queries.recordCalls,
			queries.aggregationFailures,
		)
	}
}

func TestQualityRecordingFailureDoesNotReplaceBuildError(t *testing.T) {
	t.Parallel()

	queries := &aggregationFailureQueries{
		recordErr: errors.New("quality snapshot unavailable"),
	}
	database := New(queries, time.Second)
	repository := NewAnalyticsRepository(database, config.Phase4Config{})

	_, _, err := repository.BuildAndPublish(
		context.Background(),
		stay.MustCivilDate("2026-07-28"),
	)
	if !errors.Is(err, ErrUnavailable) ||
		strings.Contains(err.Error(), "quality snapshot unavailable") {
		t.Fatalf("BuildAndPublish() error = %v", err)
	}
	if queries.recordCalls != 1 {
		t.Fatalf("quality record calls = %d", queries.recordCalls)
	}
}

func TestObservedPublicationProtectsNineAndPublishesTenFromThreeAccommodations(
	t *testing.T,
) {
	t.Parallel()

	asOf := stay.MustCivilDate("2026-07-28")
	repository := &AnalyticsRepository{phase4: config.Phase4Config{
		PrimaryCellThreshold:           10,
		MinimumReportingAccommodations: 3,
		RoundingBase:                   10,
	}}
	nine := observedAggregate(9, 3)
	cells, err := repository.buildObservedCells(asOf, map[string]*factAggregate{
		aggregateKey(asOf.String(), "observed"): &nine,
	})
	if err != nil {
		t.Fatalf("buildObservedCells(nine) error = %v", err)
	}
	if cells[len(cells)-1].Status != analytics.CellProtected {
		t.Fatalf("nine status = %s", cells[len(cells)-1].Status)
	}

	ten := observedAggregate(10, 3)
	cells, err = repository.buildObservedCells(asOf, map[string]*factAggregate{
		aggregateKey(asOf.String(), "observed"): &ten,
	})
	if err != nil {
		t.Fatalf("buildObservedCells(ten) error = %v", err)
	}
	today := cells[len(cells)-1]
	if today.Status != analytics.CellPublished ||
		today.PublishedValue == nil || *today.PublishedValue != 10 {
		t.Fatalf("ten cell = %#v", today)
	}
}

func TestCoverageDenominatorIncludesActiveAccommodationsWithoutFacts(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	reported := pgtype.Timestamptz{Time: now, Valid: true}
	capacity10, capacity20, capacity30 := int32(10), int32(20), int32(30)
	got := coverageForRows([]generated.ListActiveAccommodationCoverageRow{
		{Capacity: &capacity10, LastReportedAt: reported},
		{Capacity: &capacity20},
		{Capacity: &capacity30},
	}, now, 1)
	if got.Status != analytics.CoveragePublished || got.Ratio == nil || *got.Ratio != 15 {
		t.Fatalf("coverage = %#v", got)
	}
}

func TestForecastHistoryExcludesSubThresholdWeeks(t *testing.T) {
	t.Parallel()

	date := stay.MustCivilDate("2026-08-15")
	policy := analytics.Policy{
		PrimaryThreshold: 10, MinimumReportingAccommodations: 3, RoundingBase: 10,
	}
	subThreshold := make(map[string]*factAggregate)
	eligible := make(map[string]*factAggregate)
	for week := 1; week <= 8; week++ {
		small := observedAggregate(9, 1)
		enough := observedAggregate(10, 3)
		key := aggregateKey(date.AddDays(-7*week).String(), "observed")
		subThreshold[key] = &small
		eligible[key] = &enough
	}
	excluded := eligiblePreviousWeekdayValues(date, subThreshold, policy)
	fallback, err := analytics.ExplainableBaseline(10, excluded, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(excluded) != 0 || !fallback.Fallback {
		t.Fatalf("sub-threshold history len=%d forecast=%#v", len(excluded), fallback)
	}
	included := eligiblePreviousWeekdayValues(date, eligible, policy)
	baseline, err := analytics.ExplainableBaseline(10, included, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(included) != 8 || baseline.Fallback {
		t.Fatalf("eligible history len=%d forecast=%#v", len(included), baseline)
	}
}

func observedAggregate(visitors, accommodations int) factAggregate {
	result := factAggregate{
		value:          float64(visitors),
		visitors:       make(map[uuid.UUID]struct{}, visitors),
		accommodations: make(map[uuid.UUID]struct{}, accommodations),
	}
	for index := 0; index < visitors; index++ {
		id := uuid.MustParse(
			"019f0000-0000-7000-8000-" + fmt.Sprintf("%012d", index+1),
		)
		result.visitors[id] = struct{}{}
	}
	for index := 0; index < accommodations; index++ {
		id := uuid.MustParse(
			"019f0000-0000-7001-8000-" + fmt.Sprintf("%012d", index+1),
		)
		result.accommodations[id] = struct{}{}
	}
	return result
}
