package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var qualityCategoryCode = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type AnalyticsRepository struct {
	store     *Store
	analytics config.AnalyticsConfig
}

func NewAnalyticsRepository(
	store *Store,
	analytics config.AnalyticsConfig,
) *AnalyticsRepository {
	return &AnalyticsRepository{store: store, analytics: analytics}
}

var _ analytics.QualityReader = (*AnalyticsRepository)(nil)

func (r *AnalyticsRepository) Publish(
	ctx context.Context,
	publication analytics.Publication,
) (int64, bool, error) {
	fingerprint, err := analytics.Fingerprint(publication)
	if err != nil {
		return 0, false, analytics.ErrInvalidPublication
	}
	if version, found, lookupErr := r.findPublication(ctx, fingerprint); found || lookupErr != nil {
		return version, found, lookupErr
	}
	return r.publishNew(ctx, publication, fingerprint)
}

func (r *AnalyticsRepository) publishNew(
	ctx context.Context,
	publication analytics.Publication,
	fingerprint string,
) (int64, bool, error) {
	publication, err := r.preparePublication(publication)
	if err != nil {
		return 0, false, err
	}
	if err := r.insertPublicationRun(ctx, publication, fingerprint); err != nil {
		return r.resolveConcurrentPublication(ctx, fingerprint, err)
	}
	return r.stageAndPublish(ctx, publication, fingerprint)
}

// Any failure after the run row exists must mark that run failed, otherwise a
// retry would find a stale "processing" run and refuse to proceed.
func (r *AnalyticsRepository) stageAndPublish(
	ctx context.Context,
	publication analytics.Publication,
	fingerprint string,
) (int64, bool, error) {
	if err := r.stagePublication(ctx, publication); err != nil {
		r.failPublicationRun(ctx, publication.RunID)
		return 0, false, err
	}
	version, err := r.publishTransaction(ctx, publication, fingerprint)
	if err != nil {
		r.failPublicationRun(ctx, publication.RunID)
		return 0, false, err
	}
	return version, false, nil
}

func (r *AnalyticsRepository) stagePublication(
	ctx context.Context,
	publication analytics.Publication,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	if _, err := r.store.queries.DeleteStagedMetricCellsForRun(
		ctx, idToPG(publication.RunID),
	); err != nil {
		return ErrUnavailable
	}
	for _, cell := range publication.Cells {
		params, err := stagedCellParams(publication.RunID, cell)
		if err != nil {
			return ErrUnavailable
		}
		if err := r.store.queries.InsertStagedMetricCell(ctx, params); err != nil {
			return ErrUnavailable
		}
	}
	return nil
}

func stagedCellParams(
	runID uuid.UUID,
	cell analytics.PublicationCell,
) (generated.InsertStagedMetricCellParams, error) {
	exactValue, err := optionalNumeric(cell.ExactValue)
	if err != nil {
		return generated.InsertStagedMetricCellParams{}, err
	}
	exactLower, err := optionalNumeric(cell.ExactLower)
	if err != nil {
		return generated.InsertStagedMetricCellParams{}, err
	}
	exactCentral, err := optionalNumeric(cell.ExactCentral)
	if err != nil {
		return generated.InsertStagedMetricCellParams{}, err
	}
	exactUpper, err := optionalNumeric(cell.ExactUpper)
	if err != nil {
		return generated.InsertStagedMetricCellParams{}, err
	}
	return generated.InsertStagedMetricCellParams{
		PublicationRunID: idToPG(runID), CellKey: cell.CellKey,
		MetricCode: cell.MetricCode, PeriodSelector: cell.PeriodSelector,
		PeriodStart: dateToPG(cell.PeriodStart), PeriodEnd: dateToPG(cell.PeriodEnd),
		DimensionCode: cell.DimensionCode, CategoryCode: cell.CategoryCode,
		Kind: cell.Kind, ExactValue: exactValue, ExactLower: exactLower,
		ExactCentral: exactCentral, ExactUpper: exactUpper,
		SampleSize: cell.SampleSize, AccommodationCount: cell.AccommodationCount,
		ProtectionStatus: string(cell.Status),
	}, nil
}

func optionalNumeric(value *float64) (pgtype.Numeric, error) {
	if value == nil {
		return pgtype.Numeric{}, nil
	}
	return numericFromFloat(*value)
}

func (r *AnalyticsRepository) findPublication(
	ctx context.Context,
	fingerprint string,
) (int64, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	row, err := r.store.queries.GetPublicationByFingerprint(ctx, fingerprint)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, ErrUnavailable
	}
	return row.PublicationVersion, true, nil
}

func (r *AnalyticsRepository) preparePublication(
	publication analytics.Publication,
) (analytics.Publication, error) {
	if publication.RunID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return publication, ErrUnavailable
		}
		publication.RunID = id
	}
	if publication.RunID.Version() != 7 {
		return publication, analytics.ErrInvalidPublication
	}
	if publication.PublishedAt.IsZero() {
		publication.PublishedAt = r.store.currentTime()
	}
	publication.PublishedAt = publication.PublishedAt.UTC()
	return publication, nil
}

func (r *AnalyticsRepository) insertPublicationRun(
	ctx context.Context,
	publication analytics.Publication,
	fingerprint string,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	err := r.store.queries.InsertPublicationRun(ctx, generated.InsertPublicationRunParams{
		ID: idToPG(publication.RunID), BuildFingerprint: fingerprint,
		AsOfOn:               dateToPG(publication.AsOfOn),
		PrivacyPolicyVersion: publication.PrivacyPolicyVersion,
		MethodologyVersion:   publication.MethodologyVersion,
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *AnalyticsRepository) resolveConcurrentPublication(
	ctx context.Context,
	fingerprint string,
	insertErr error,
) (int64, bool, error) {
	if !isUniqueViolation(insertErr) {
		return 0, false, ErrUnavailable
	}
	version, found, err := r.findPublication(ctx, fingerprint)
	if err != nil || !found {
		return 0, false, ErrUnavailable
	}
	return version, true, nil
}

func (r *AnalyticsRepository) publishTransaction(
	ctx context.Context,
	publication analytics.Publication,
	fingerprint string,
) (int64, error) {
	if r.store.pool == nil {
		return 0, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	tx, err := r.store.pool.Begin(ctx)
	if err != nil {
		return 0, ErrUnavailable
	}
	queries := generated.New(r.store.pool).WithTx(tx)
	version, err := publishSnapshot(ctx, queries, publication, fingerprint)
	if err != nil {
		_ = tx.Rollback(ctx)
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, ErrUnavailable
	}
	return version, nil
}

func publishSnapshot(
	ctx context.Context,
	queries generated.Querier,
	publication analytics.Publication,
	fingerprint string,
) (int64, error) {
	version, err := queries.InsertNextPublication(ctx, publicationParams(publication, fingerprint))
	if err != nil {
		return 0, ErrUnavailable
	}
	if err := insertPublishedCells(ctx, queries, version, publication.Cells); err != nil {
		return 0, err
	}
	if err := promotePublication(ctx, queries, version, publication); err != nil {
		return 0, err
	}
	return version, nil
}

func insertPublishedCells(
	ctx context.Context,
	queries generated.Querier,
	version int64,
	cells []analytics.PublicationCell,
) error {
	for _, cell := range cells {
		if err := queries.InsertPublishedMetricCell(ctx, publishedCellParams(version, cell)); err != nil {
			return ErrUnavailable
		}
	}
	return nil
}

func promotePublication(
	ctx context.Context,
	queries generated.Querier,
	version int64,
	publication analytics.Publication,
) error {
	promoted, err := queries.PromoteCurrentPublication(ctx, version)
	if err != nil || promoted != 1 {
		return ErrUnavailable
	}
	completed, err := queries.CompletePublicationRun(ctx, generated.CompletePublicationRunParams{
		Status: "published", CompletedAt: pgTime(publication.PublishedAt),
		ID: idToPG(publication.RunID),
	})
	if err != nil || completed != 1 {
		return ErrUnavailable
	}
	return nil
}

func publicationParams(
	publication analytics.Publication,
	fingerprint string,
) generated.InsertNextPublicationParams {
	return generated.InsertNextPublicationParams{
		BuildFingerprint: fingerprint, AsOfOn: dateToPG(publication.AsOfOn),
		DataMode:             publication.DataMode,
		PrivacyPolicyVersion: publication.PrivacyPolicyVersion,
		MethodologyVersion:   publication.MethodologyVersion,
		CoverageStatus:       publication.CoverageStatus,
		CoverageRatioPercent: publication.CoverageRatioPercent,
		PublishedAt:          pgTime(publication.PublishedAt),
	}
}

func publishedCellParams(
	version int64,
	cell analytics.PublicationCell,
) generated.InsertPublishedMetricCellParams {
	return generated.InsertPublishedMetricCellParams{
		PublicationVersion: version, CellKey: cell.CellKey,
		MetricCode: cell.MetricCode, PeriodSelector: cell.PeriodSelector,
		PeriodStart: dateToPG(cell.PeriodStart), PeriodEnd: dateToPG(cell.PeriodEnd),
		Unit: cell.Unit, DimensionCode: cell.DimensionCode,
		CategoryCode: cell.CategoryCode, Kind: cell.Kind, Status: string(cell.Status),
		PublishedValue: cell.PublishedValue, PublishedLower: cell.PublishedLower,
		PublishedCentral: cell.PublishedCentral, PublishedUpper: cell.PublishedUpper,
	}
}

func (r *AnalyticsRepository) failPublicationRun(ctx context.Context, runID uuid.UUID) {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	code := "publication-failed"
	_, _ = r.store.queries.CompletePublicationRun(ctx, generated.CompletePublicationRunParams{
		Status: "failed", CompletedAt: pgTime(r.store.currentTime()),
		ErrorCode: &code, ID: idToPG(runID),
	})
}

func (r *AnalyticsRepository) Quality(
	ctx context.Context,
	window string,
) (analytics.QualitySnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	rows, err := r.store.queries.ListCurrentQualityRows(ctx, window)
	if err != nil || len(rows) == 0 {
		return analytics.QualitySnapshot{}, analytics.ErrPublicUnavailable
	}
	return qualitySnapshot(rows)
}

func qualitySnapshot(
	rows []generated.AnalyticsCurrentQuality,
) (analytics.QualitySnapshot, error) {
	first := rows[0]
	if !first.UpdatedAt.Valid || first.WindowCode == "" {
		return analytics.QualitySnapshot{}, analytics.ErrPublicUnavailable
	}
	coverage, err := qualityCoverage(rows)
	if err != nil {
		return analytics.QualitySnapshot{}, err
	}
	return analytics.QualitySnapshot{
		Window: first.WindowCode, UpdatedAt: first.UpdatedAt.Time.UTC().Format(time.RFC3339),
		IncompleteStays:          availableCount(first.IncompleteStays),
		OverduePlannedDepartures: availableCount(first.OverduePlannedDepartures),
		SilentAccommodations:     availableCount(first.SilentAccommodations),
		AggregationFailures:      availableCount(first.AggregationFailures),
		SuspectedDuplicates: optionalCount(
			first.SuspectedDuplicates, first.SuspectedDuplicatesReason,
		),
		FNRHFailures:       optionalCount(first.FnrhFailures, first.FnrhFailuresReason),
		CoverageByCategory: coverage,
	}, nil
}

func availableCount(value int32) analytics.QualityCount {
	return analytics.QualityCount{Status: "available", Value: &value}
}

func optionalCount(value *int32, reason string) analytics.QualityCount {
	if value == nil {
		return analytics.QualityCount{Status: "not_available", ReasonCode: reason}
	}
	return availableCount(*value)
}

func qualityCoverage(
	rows []generated.AnalyticsCurrentQuality,
) ([]analytics.QualityCoverage, error) {
	result := make([]analytics.QualityCoverage, 0, len(rows))
	for _, row := range rows {
		if row.CategoryCode == nil && row.CoverageStatus == nil {
			continue
		}
		coverage, err := qualityCoverageRow(row)
		if err != nil {
			return nil, err
		}
		result = append(result, coverage)
	}
	return result, nil
}

func qualityCoverageRow(
	row generated.AnalyticsCurrentQuality,
) (analytics.QualityCoverage, error) {
	if row.CategoryCode == nil || row.CoverageStatus == nil {
		return analytics.QualityCoverage{}, analytics.ErrPublicUnavailable
	}
	if !qualityCategoryCode.MatchString(*row.CategoryCode) {
		return analytics.QualityCoverage{}, analytics.ErrPublicUnavailable
	}
	result := analytics.QualityCoverage{
		CategoryCode: *row.CategoryCode, Status: *row.CoverageStatus,
	}
	return withCoverageRatio(result, row.CoverageRatio)
}

// Only an "available" coverage carries a ratio, and that ratio must be a
// fraction; anything else is a contract break rather than a missing value.
func coverageFraction(ratio pgtype.Numeric) (float64, error) {
	value, err := numericFloat(ratio)
	if err != nil || value < 0 || value > 1 {
		return 0, analytics.ErrPublicUnavailable
	}
	return value, nil
}

func withCoverageRatio(
	result analytics.QualityCoverage,
	ratio pgtype.Numeric,
) (analytics.QualityCoverage, error) {
	if result.Status == "not_available" {
		return result, nil
	}
	if result.Status != "available" {
		return result, analytics.ErrPublicUnavailable
	}
	value, err := coverageFraction(ratio)
	if err != nil {
		return result, err
	}
	result.Ratio = &value
	return result, nil
}

func numericFloat(value pgtype.Numeric) (float64, error) {
	converted, err := value.Float64Value()
	if err != nil || !converted.Valid {
		return 0, analytics.ErrPublicUnavailable
	}
	return converted.Float64, nil
}

func (r *AnalyticsRepository) Reconcile(
	ctx context.Context,
	kind analytics.ReconciliationKind,
	asOf stay.CivilDate,
) (bool, error) {
	known := kind == analytics.ReconciliationIncremental ||
		kind == analytics.ReconciliationFull
	if !known || r.store.pool == nil {
		return false, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	return r.reconcileInTransaction(ctx, kind, asOf)
}

func (r *AnalyticsRepository) reconcileInTransaction(
	ctx context.Context,
	kind analytics.ReconciliationKind,
	asOf stay.CivilDate,
) (bool, error) {
	tx, err := r.store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return false, ErrUnavailable
	}
	queries := generated.New(r.store.pool).WithTx(tx)
	changed, err := r.reconcileTransaction(ctx, queries, kind, asOf)
	if err != nil {
		_ = tx.Rollback(ctx)
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, ErrUnavailable
	}
	return changed, nil
}

func (r *AnalyticsRepository) reconcileTransaction(
	ctx context.Context,
	queries generated.Querier,
	kind analytics.ReconciliationKind,
	asOf stay.CivilDate,
) (bool, error) {
	sources, err := loadReconciliationSources(ctx, queries, kind)
	if err != nil {
		return false, err
	}
	fingerprint := reconciliationFingerprint(kind, asOf, sources)
	alreadyRan, err := reconciliationAlreadyRan(ctx, queries, fingerprint)
	if err != nil || alreadyRan {
		return false, err
	}
	return r.runReconciliation(ctx, queries, kind, asOf, sources, fingerprint)
}

func (r *AnalyticsRepository) runReconciliation(
	ctx context.Context,
	queries generated.Querier,
	kind analytics.ReconciliationKind,
	asOf stay.CivilDate,
	sources []reconciliationSource,
	fingerprint string,
) (bool, error) {
	runID, err := uuid.NewV7()
	if err != nil {
		return false, ErrUnavailable
	}
	if err := startReconciliation(ctx, queries, runID, kind, asOf, fingerprint); err != nil {
		return false, err
	}
	if err := r.reconcilePresenceSources(ctx, queries, sources, asOf); err != nil {
		return false, err
	}
	if err := completeReconciliation(ctx, queries, runID, r.store.currentTime()); err != nil {
		return false, err
	}
	return true, nil
}

func reconciliationAlreadyRan(
	ctx context.Context,
	queries generated.Querier,
	fingerprint string,
) (bool, error) {
	if _, err := queries.GetReconciliationRunByFingerprint(ctx, fingerprint); err == nil {
		return true, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return false, ErrUnavailable
	}
	return false, nil
}

func (r *AnalyticsRepository) reconcilePresenceSources(
	ctx context.Context,
	queries generated.Querier,
	sources []reconciliationSource,
	asOf stay.CivilDate,
) error {
	for _, source := range sources {
		if err := r.reconcilePresenceSource(ctx, queries, source, asOf); err != nil {
			return err
		}
	}
	return nil
}

func completeReconciliation(
	ctx context.Context,
	queries generated.Querier,
	runID uuid.UUID,
	completedAt time.Time,
) error {
	completed, err := queries.CompleteReconciliationRun(
		ctx,
		generated.CompleteReconciliationRunParams{
			Status: "succeeded", CompletedAt: pgTime(completedAt), ID: idToPG(runID),
		},
	)
	if err != nil || completed != 1 {
		return ErrUnavailable
	}
	return nil
}

type reconciliationSource struct {
	presence analytics.PresenceSource
	existing []generated.AnalyticsPresenceDay
}

func loadReconciliationSources(
	ctx context.Context,
	queries generated.Querier,
	_ analytics.ReconciliationKind,
) ([]reconciliationSource, error) {
	rows, err := queries.ListPresenceReconciliationStays(ctx)
	if err != nil {
		return nil, ErrUnavailable
	}
	return fullReconciliationSources(ctx, queries, rows)
}

func fullReconciliationSources(
	ctx context.Context,
	queries generated.Querier,
	rows []generated.ListPresenceReconciliationStaysRow,
) ([]reconciliationSource, error) {
	result := make([]reconciliationSource, 0, len(rows))
	for _, row := range rows {
		source := analytics.PresenceSource{
			StayID: uuidFromPG(row.ID), Status: stay.Status(row.Status),
			Approval:         stay.ApprovalStateFromColumn(row.ApprovalState),
			PlannedArrival:   civilDateFromPG(row.PlannedArrivalOn),
			PlannedDeparture: civilDateFromPG(row.PlannedDepartureOn),
			CheckedInAt:      timePointer(row.CheckedInAt), CheckedOutAt: timePointer(row.CheckedOutAt),
			Version: row.ExpectedVersion,
		}
		if err := addPresenceVisitors(ctx, queries, &source); err != nil {
			return nil, err
		}
		existing, err := queries.ListPresenceDaysForStay(ctx, row.ID)
		if err != nil {
			return nil, ErrUnavailable
		}
		result = append(result, reconciliationSource{
			presence: source, existing: existing,
		})
	}
	return result, nil
}

func addPresenceVisitors(
	ctx context.Context,
	queries generated.Querier,
	source *analytics.PresenceSource,
) error {
	if !presenceEligible(source.Status, source.Approval) {
		return nil
	}
	rows, err := queries.ListStayVisitorsForPresence(ctx, idToPG(source.StayID))
	if err != nil {
		return ErrUnavailable
	}
	source.VisitorIDs = make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		id := uuidFromPG(row.ID)
		if id == uuid.Nil {
			return ErrUnavailable
		}
		source.VisitorIDs = append(source.VisitorIDs, id)
	}
	return nil
}

// presenceEligible is the second of the three mandatory filter points, and the
// choke point addPresenceVisitors and incompleteStayCount share. The status
// alone is not enough: a pending self-registration sits in pre_registered — the
// feature adds no value to core.stay_status — and would accrue forecast
// person-days that reach public_data.metric_cells without anybody approving it.
func presenceEligible(status stay.Status, approval stay.ApprovalState) bool {
	if !approval.Countable() {
		return false
	}
	return status == stay.StatusPreRegistered ||
		status == stay.StatusCheckedIn ||
		status == stay.StatusCheckedOut
}

func startReconciliation(
	ctx context.Context,
	queries generated.Querier,
	runID uuid.UUID,
	kind analytics.ReconciliationKind,
	asOf stay.CivilDate,
	fingerprint string,
) error {
	_, err := queries.InsertReconciliationRun(ctx, generated.InsertReconciliationRunParams{
		ID: idToPG(runID), RunKind: string(kind), AsOfOn: dateToPG(asOf.String()),
		SourceFingerprint: fingerprint,
	})
	if err != nil {
		return ErrUnavailable
	}
	updated, err := queries.MarkReconciliationRunRunning(ctx, idToPG(runID))
	if err != nil || updated != 1 {
		return ErrUnavailable
	}
	return nil
}

func (r *AnalyticsRepository) reconcilePresenceSource(
	ctx context.Context,
	queries generated.Querier,
	source reconciliationSource,
	asOf stay.CivilDate,
) error {
	desired := []analytics.PresenceFact{}
	if presenceEligible(source.presence.Status, source.presence.Approval) {
		var err error
		desired, err = analytics.MaterializePresence(
			source.presence, asOf, r.analytics.PreRegisteredWeight,
		)
		if err != nil {
			return ErrUnavailable
		}
	}
	return replacePresenceFacts(
		ctx, queries, source.presence.Version, source.existing, desired,
	)
}

type presenceFactKey struct {
	visitorID uuid.UUID
	date      string
}

func replacePresenceFacts(
	ctx context.Context,
	queries generated.Querier,
	version int64,
	existing []generated.AnalyticsPresenceDay,
	desired []analytics.PresenceFact,
) error {
	desiredKeys := make(map[presenceFactKey]struct{}, len(desired))
	for _, fact := range desired {
		key := presenceFactKey{visitorID: fact.VisitorID, date: fact.PresenceOn.String()}
		desiredKeys[key] = struct{}{}
		if err := upsertPresenceFact(ctx, queries, fact); err != nil {
			return err
		}
	}
	return deleteObsoletePresence(ctx, queries, version, existing, desiredKeys)
}

func deleteObsoletePresence(
	ctx context.Context,
	queries generated.Querier,
	version int64,
	existing []generated.AnalyticsPresenceDay,
	desiredKeys map[presenceFactKey]struct{},
) error {
	for _, fact := range existing {
		key := presenceFactKey{visitorID: uuidFromPG(fact.VisitorID), date: dateString(fact.PresenceOn)}
		_, keep := desiredKeys[key]
		if keep || fact.SourceStayVersion > version {
			continue
		}
		if err := deletePresenceFact(ctx, queries, fact, version); err != nil {
			return err
		}
	}
	return nil
}

func upsertPresenceFact(
	ctx context.Context,
	queries generated.Querier,
	fact analytics.PresenceFact,
) error {
	weight, err := numericFromFloat(fact.Weight)
	if err != nil {
		return ErrUnavailable
	}
	_, err = queries.UpsertPresenceDay(ctx, generated.UpsertPresenceDayParams{
		StayID: idToPG(fact.StayID), VisitorID: idToPG(fact.VisitorID),
		PresenceOn: dateToPG(fact.PresenceOn.String()), Kind: string(fact.Kind),
		Weight: weight, SourceStayVersion: fact.SourceStayVersion,
		AsOfOn: dateToPG(fact.AsOfOn.String()), UpdatedAt: pgTime(time.Now().UTC()),
	})
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

func deletePresenceFact(
	ctx context.Context,
	queries generated.Querier,
	fact generated.AnalyticsPresenceDay,
	version int64,
) error {
	_, err := queries.DeletePresenceDay(ctx, generated.DeletePresenceDayParams{
		StayID: fact.StayID, VisitorID: fact.VisitorID,
		PresenceOn: fact.PresenceOn, SourceStayVersion: version,
	})
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

func reconciliationFingerprint(
	kind analytics.ReconciliationKind,
	asOf stay.CivilDate,
	sources []reconciliationSource,
) string {
	var canonical strings.Builder
	canonical.WriteString(string(kind))
	canonical.WriteByte('|')
	canonical.WriteString(asOf.String())
	for _, source := range sources {
		writeReconciliationSource(&canonical, source)
	}
	sum := sha256.Sum256([]byte(canonical.String()))
	return hex.EncodeToString(sum[:])
}

func writeReconciliationSource(
	canonical *strings.Builder,
	source reconciliationSource,
) {
	writePresenceSource(canonical, source.presence)
	for _, fact := range source.existing {
		writePresenceFact(canonical, fact)
	}
}

func writePresenceSource(canonical *strings.Builder, source analytics.PresenceSource) {
	canonical.WriteByte('|')
	canonical.WriteString(source.StayID.String())
	canonical.WriteByte('|')
	canonical.WriteString(string(source.Status))
	canonical.WriteByte('|')
	canonical.WriteString(source.PlannedArrival.String())
	canonical.WriteByte('|')
	canonical.WriteString(source.PlannedDeparture.String())
	canonical.WriteByte('|')
	canonical.WriteString(strconv.FormatInt(source.Version, 10))
	writeOptionalTime(canonical, source.CheckedInAt)
	writeOptionalTime(canonical, source.CheckedOutAt)
	visitors := append([]uuid.UUID(nil), source.VisitorIDs...)
	sort.Slice(visitors, func(first, second int) bool {
		return visitors[first].String() < visitors[second].String()
	})
	for _, visitorID := range visitors {
		canonical.WriteByte('|')
		canonical.WriteString(visitorID.String())
	}
}

func writeOptionalTime(canonical *strings.Builder, value *time.Time) {
	canonical.WriteByte('|')
	if value != nil {
		canonical.WriteString(value.UTC().Format(time.RFC3339Nano))
	}
}

func writePresenceFact(
	canonical *strings.Builder,
	fact generated.AnalyticsPresenceDay,
) {
	weight, _ := numericFloat(fact.Weight)
	values := []string{
		uuidFromPG(fact.VisitorID).String(),
		dateString(fact.PresenceOn),
		fact.Kind,
		strconv.FormatFloat(weight, 'f', 6, 64),
		strconv.FormatInt(fact.SourceStayVersion, 10),
		dateString(fact.AsOfOn),
	}
	for _, value := range values {
		canonical.WriteByte('|')
		canonical.WriteString(value)
	}
}

func civilDateFromPG(value pgtype.Date) stay.CivilDate {
	parsed, _ := stay.ParseCivilDate(dateString(value))
	return parsed
}

func uuidFromPG(value pgtype.UUID) uuid.UUID {
	if !value.Valid {
		return uuid.Nil
	}
	return uuid.UUID(value.Bytes)
}

func numericFromFloat(value float64) (pgtype.Numeric, error) {
	var result pgtype.Numeric
	err := result.Scan(strconv.FormatFloat(value, 'f', 6, 64))
	return result, err
}

type publicationInputs struct {
	facts       []generated.ListPresenceFactsForWindowRow
	coverage    []generated.ListActiveAccommodationCoverageRow
	preferences []generated.ListEligiblePreferenceCountsRow
	sources     []generated.ListPresenceReconciliationStaysRow
	incomplete  int32
	overdue     int32
}

func (r *AnalyticsRepository) BuildAndPublish(
	ctx context.Context,
	asOf stay.CivilDate,
) (version int64, replayed bool, resultErr error) {
	defer func() {
		if resultErr != nil {
			_ = r.recordAggregationFailure(context.WithoutCancel(ctx))
		}
	}()
	inputs, err := r.loadPublicationInputs(ctx, asOf)
	if err != nil {
		return 0, false, err
	}
	cells, coverage, categoryCoverage, err := r.buildPublication(asOf, inputs)
	if err != nil {
		return 0, false, err
	}
	version, replayed, err = r.publishAndRecord(ctx, asOf, cells, coverage, inputs, categoryCoverage)
	if err != nil {
		return 0, false, err
	}
	return version, replayed, nil
}

func (r *AnalyticsRepository) publishAndRecord(
	ctx context.Context,
	asOf stay.CivilDate,
	cells []analytics.PublicationCell,
	coverage analytics.Coverage,
	inputs publicationInputs,
	categoryCoverage []categoryCoverage,
) (int64, bool, error) {
	version, replayed, err := r.Publish(ctx, r.newPublication(asOf, cells, coverage))
	if err != nil {
		return 0, false, err
	}
	if err := r.recordQuality(ctx, inputs, categoryCoverage); err != nil {
		return 0, false, err
	}
	return version, replayed, nil
}

func (r *AnalyticsRepository) newPublication(
	asOf stay.CivilDate,
	cells []analytics.PublicationCell,
	coverage analytics.Coverage,
) analytics.Publication {
	publication := analytics.Publication{
		AsOfOn: asOf.String(), DataMode: r.analytics.DataMode,
		PrivacyPolicyVersion: r.analytics.PrivacyPolicyVersion,
		MethodologyVersion:   r.analytics.MethodologyVersion,
		CoverageStatus:       string(coverage.Status), Cells: cells,
	}
	if coverage.Ratio != nil {
		value := int32(*coverage.Ratio)
		publication.CoverageRatioPercent = &value
	}
	return publication
}

func (r *AnalyticsRepository) recordAggregationFailure(ctx context.Context) error {
	id, err := uuid.NewV7()
	if err != nil {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	result, err := r.store.queries.RecordAggregationFailureQualitySnapshot(
		ctx,
		generated.RecordAggregationFailureQualitySnapshotParams{
			SnapshotID: idToPG(id),
			UpdatedAt:  pgTime(r.store.currentTime()),
		},
	)
	if err != nil || !recordedFailure(result, id) {
		return ErrUnavailable
	}
	return nil
}

func recordedFailure(
	result generated.RecordAggregationFailureQualitySnapshotRow,
	id uuid.UUID,
) bool {
	return uuidFromPG(result.ID) == id &&
		result.AggregationFailures >= 1 &&
		result.CoverageCount >= 0
}

func (r *AnalyticsRepository) loadPublicationInputs(
	ctx context.Context,
	asOf stay.CivilDate,
) (publicationInputs, error) {
	if r.store.pool == nil {
		return publicationInputs{}, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	tx, err := r.store.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return publicationInputs{}, ErrUnavailable
	}
	queries := generated.New(r.store.pool).WithTx(tx)
	inputs, err := r.readPublicationInputs(ctx, queries, asOf)
	if err != nil {
		_ = tx.Rollback(ctx)
		return publicationInputs{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return publicationInputs{}, ErrUnavailable
	}
	return inputs, nil
}

func (r *AnalyticsRepository) readPublicationInputs(
	ctx context.Context,
	queries generated.Querier,
	asOf stay.CivilDate,
) (publicationInputs, error) {
	if err := r.validateMetricCatalog(ctx, queries); err != nil {
		return publicationInputs{}, err
	}
	windows, err := r.readPublicationWindows(ctx, queries, asOf)
	if err != nil {
		return publicationInputs{}, err
	}
	facts, coverage, preferences := windows.facts, windows.coverage, windows.preferences
	sources, err := queries.ListPresenceReconciliationStays(ctx)
	if err != nil {
		return publicationInputs{}, ErrUnavailable
	}
	incomplete, err := incompleteStayCount(ctx, queries, sources)
	if err != nil {
		return publicationInputs{}, err
	}
	return publicationInputs{
		facts: facts, coverage: coverage, preferences: preferences, sources: sources,
		incomplete: incomplete, overdue: overdueStayCount(sources, asOf),
	}, nil
}

func preferenceCountParams(
	policyVersion string,
	asOf stay.CivilDate,
) generated.ListEligiblePreferenceCountsParams {
	start, end := lastCompleteMonth(asOf)
	return generated.ListEligiblePreferenceCountsParams{
		PrivacyPolicyVersion: policyVersion,
		PeriodStart:          pgTime(start.Start()),
		PeriodEnd:            pgTime(end.Start()),
	}
}

func (r *AnalyticsRepository) validateMetricCatalog(
	ctx context.Context,
	queries generated.Querier,
) error {
	rows, err := queries.ListActiveMetricCatalog(ctx, r.analytics.PrivacyPolicyVersion)
	if err != nil || len(rows) != 3 {
		return ErrUnavailable
	}
	return r.matchMetricCatalog(rows, expectedMetricShapes())
}

func expectedMetricShapes() map[string]string {
	return map[string]string{
		"presence|recent_30_days":               "none|person_day",
		"presence|next_30_days":                 "none|person_day",
		"first_visit_share|last_complete_month": "visit_profile|survey_response",
	}
}

// Every expected metric must appear exactly once with the expected shape; a
// leftover expectation means the catalogue drifted from the contract.
func (r *AnalyticsRepository) matchMetricCatalog(
	rows []generated.AnalyticsMetricCatalog,
	expected map[string]string,
) error {
	for _, row := range rows {
		key := row.MetricCode + "|" + row.PeriodSelector
		if !r.validMetricCatalogRow(row, expected[key]) {
			return ErrUnavailable
		}
		delete(expected, key)
	}
	if len(expected) != 0 {
		return ErrUnavailable
	}
	return nil
}

func (r *AnalyticsRepository) validMetricCatalogRow(
	row generated.AnalyticsMetricCatalog,
	expectedShape string,
) bool {
	shape := row.DimensionCode + "|" + row.Unit
	return expectedShape != "" && expectedShape == shape &&
		row.Active && row.PrivacyPolicyVersion == r.analytics.PrivacyPolicyVersion &&
		r.matchesCellThresholds(row)
}

func (r *AnalyticsRepository) matchesCellThresholds(
	row generated.AnalyticsMetricCatalog,
) bool {
	return row.MinimumPublicCell == int32(r.analytics.PrimaryCellThreshold) &&
		row.MinimumReportingAccommodations ==
			int32(r.analytics.MinimumReportingAccommodations)
}

func incompleteStayCount(
	ctx context.Context,
	queries generated.Querier,
	sources []generated.ListPresenceReconciliationStaysRow,
) (int32, error) {
	var count int32
	for _, source := range sources {
		approval := stay.ApprovalStateFromColumn(source.ApprovalState)
		if !presenceEligible(stay.Status(source.Status), approval) {
			continue
		}
		rows, err := queries.ListStayVisitorsForPresence(ctx, source.ID)
		if err != nil {
			return 0, ErrUnavailable
		}
		if len(rows) == 0 {
			count++
		}
	}
	return count, nil
}

type publicationWindows struct {
	facts       []generated.ListPresenceFactsForWindowRow
	coverage    []generated.ListActiveAccommodationCoverageRow
	preferences []generated.ListEligiblePreferenceCountsRow
}

// The three windowed reads share one transaction so the publication observes a
// single consistent snapshot.
func (r *AnalyticsRepository) readPublicationWindows(
	ctx context.Context,
	queries generated.Querier,
	asOf stay.CivilDate,
) (publicationWindows, error) {
	facts, err := queries.ListPresenceFactsForWindow(
		ctx,
		generated.ListPresenceFactsForWindowParams{
			WindowStart: dateToPG(asOf.AddDays(-56).String()),
			WindowEnd:   dateToPG(asOf.AddDays(31).String()),
		},
	)
	if err != nil {
		return publicationWindows{}, ErrUnavailable
	}
	coverage, err := queries.ListActiveAccommodationCoverage(
		ctx,
		generated.ListActiveAccommodationCoverageParams{
			WindowStart: dateToPG(asOf.AddDays(-29).String()),
			WindowEnd:   dateToPG(asOf.AddDays(1).String()),
		},
	)
	if err != nil {
		return publicationWindows{}, ErrUnavailable
	}
	preferences, err := queries.ListEligiblePreferenceCounts(
		ctx, preferenceCountParams(r.analytics.PrivacyPolicyVersion, asOf),
	)
	if err != nil {
		return publicationWindows{}, ErrUnavailable
	}
	return publicationWindows{facts: facts, coverage: coverage, preferences: preferences}, nil
}

func overdueStayCount(
	sources []generated.ListPresenceReconciliationStaysRow,
	asOf stay.CivilDate,
) int32 {
	var count int32
	for _, source := range sources {
		if overdueStay(source, asOf) {
			count++
		}
	}
	return count
}

// A stay is overdue when it is still open past its planned departure.
func overdueStay(
	source generated.ListPresenceReconciliationStaysRow,
	asOf stay.CivilDate,
) bool {
	status := stay.Status(source.Status)
	if status != stay.StatusPreRegistered && status != stay.StatusCheckedIn {
		return false
	}
	departure := civilDateFromPG(source.PlannedDepartureOn)
	return departure.Before(asOf) || departure.Equal(asOf)
}

type factAggregate struct {
	value          float64
	visitors       map[uuid.UUID]struct{}
	accommodations map[uuid.UUID]struct{}
}

type categoryCoverage struct {
	category string
	coverage analytics.Coverage
}

func (r *AnalyticsRepository) buildPublication(
	asOf stay.CivilDate,
	inputs publicationInputs,
) ([]analytics.PublicationCell, analytics.Coverage, []categoryCoverage, error) {
	aggregates, err := aggregatePresenceFacts(inputs.facts)
	if err != nil {
		return nil, analytics.Coverage{}, nil, err
	}
	observed, err := r.buildObservedCells(asOf, aggregates)
	if err != nil {
		return nil, analytics.Coverage{}, nil, err
	}
	forecast, err := r.buildForecastCells(asOf, aggregates)
	if err != nil {
		return nil, analytics.Coverage{}, nil, err
	}
	preferences, err := r.buildPreferenceCells(asOf, inputs.preferences)
	if err != nil {
		return nil, analytics.Coverage{}, nil, err
	}
	coverage, categories := r.buildCoverage(inputs.coverage)
	cells := append(observed, forecast...)
	cells = append(cells, preferences...)
	sort.Slice(cells, func(first, second int) bool {
		return cells[first].CellKey < cells[second].CellKey
	})
	return cells, coverage, categories, nil
}

func aggregatePresenceFacts(
	rows []generated.ListPresenceFactsForWindowRow,
) (map[string]*factAggregate, error) {
	result := make(map[string]*factAggregate)
	for _, row := range rows {
		if row.AccommodationStatus != generated.CoreAccommodationStatusActive {
			continue
		}
		if err := addPresenceAggregate(result, row); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func addPresenceAggregate(
	result map[string]*factAggregate,
	row generated.ListPresenceFactsForWindowRow,
) error {
	value, err := presenceWeight(row.Weight)
	if err != nil {
		return err
	}
	visitorID := uuidFromPG(row.VisitorID)
	accommodationID := uuidFromPG(row.AccommodationID)
	if visitorID == uuid.Nil || accommodationID == uuid.Nil || !row.PresenceOn.Valid {
		return ErrUnavailable
	}
	aggregate := ensureAggregate(result, aggregateKey(dateString(row.PresenceOn), row.Kind))
	aggregate.value += value
	aggregate.visitors[visitorID] = struct{}{}
	aggregate.accommodations[accommodationID] = struct{}{}
	return nil
}

// A presence weight is a fraction of one person-day; anything outside (0,1] is
// a corrupt fact rather than a value to clamp.
func presenceWeight(raw pgtype.Numeric) (float64, error) {
	value, err := numericFloat(raw)
	if err != nil || value <= 0 || value > 1 {
		return 0, ErrUnavailable
	}
	return value, nil
}

func ensureAggregate(result map[string]*factAggregate, key string) *factAggregate {
	aggregate := result[key]
	if aggregate == nil {
		aggregate = &factAggregate{
			visitors:       make(map[uuid.UUID]struct{}),
			accommodations: make(map[uuid.UUID]struct{}),
		}
		result[key] = aggregate
	}
	return aggregate
}

func aggregateKey(date, kind string) string {
	return date + "|" + kind
}

func (r *AnalyticsRepository) buildObservedCells(
	asOf stay.CivilDate,
	aggregates map[string]*factAggregate,
) ([]analytics.PublicationCell, error) {
	source := make([]analytics.Cell, 0, 30)
	dates := make([]stay.CivilDate, 0, 30)
	for offset := -29; offset <= 0; offset++ {
		date := asOf.AddDays(offset)
		dates = append(dates, date)
		aggregate := aggregateOrEmpty(aggregates, date, "observed")
		source = append(source, protectionCell(date.String(), "recent_30_days", aggregate))
	}
	protected, err := analytics.ProtectCells(source, r.presencePolicy())
	if err != nil {
		return nil, ErrUnavailable
	}
	result := make([]analytics.PublicationCell, 0, len(source))
	for index, cell := range source {
		status := protected[index].Status
		if cell.SampleSize == 0 {
			status = analytics.CellUnavailable
		}
		result = append(result, observedPublicationCell(
			dates[index], cell, status, protected[index].PublishedValue,
		))
	}
	return result, nil
}

func (r *AnalyticsRepository) buildForecastCells(
	asOf stay.CivilDate,
	aggregates map[string]*factAggregate,
) ([]analytics.PublicationCell, error) {
	source := make([]analytics.Cell, 0, 30)
	values := make([]analytics.Forecast, 0, 30)
	dates := make([]stay.CivilDate, 0, 30)
	for lead := 1; lead <= 30; lead++ {
		date := asOf.AddDays(lead)
		aggregate := aggregateOrEmpty(aggregates, date, "forecast")
		forecast, err := analytics.ExplainableBaseline(
			aggregate.value,
			eligiblePreviousWeekdayValues(date, aggregates, r.presencePolicy()),
			lead,
		)
		if err != nil {
			return nil, ErrUnavailable
		}
		dates = append(dates, date)
		values = append(values, forecast)
		cell := protectionCell(date.String(), "next_30_days", aggregate)
		cell.RawValue = forecast.Central
		source = append(source, cell)
	}
	protected, err := analytics.ProtectCells(source, r.presencePolicy())
	if err != nil {
		return nil, ErrUnavailable
	}
	return r.forecastPublicationCells(dates, source, values, protected)
}

func (r *AnalyticsRepository) forecastPublicationCells(
	dates []stay.CivilDate,
	source []analytics.Cell,
	values []analytics.Forecast,
	protected []analytics.ProtectedCell,
) ([]analytics.PublicationCell, error) {
	result := make([]analytics.PublicationCell, 0, len(source))
	for index, cell := range source {
		status := protected[index].Status
		if cell.SampleSize == 0 {
			status = analytics.CellUnavailable
		}
		publicationCell, err := r.forecastPublicationCell(
			dates[index], cell, values[index], status,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, publicationCell)
	}
	return result, nil
}

func (r *AnalyticsRepository) forecastPublicationCell(
	date stay.CivilDate,
	source analytics.Cell,
	forecast analytics.Forecast,
	status analytics.CellStatus,
) (analytics.PublicationCell, error) {
	cell := basePublicationCell(
		date, "next_30_days", "presence", "person_day", "none", "none", "forecast",
		status, source,
	)
	if status != analytics.CellPublished {
		return cell, nil
	}
	rounded, err := analytics.RoundForecast(
		forecast.Lower, forecast.Central, forecast.Upper, r.analytics.RoundingBase,
	)
	if err != nil {
		return analytics.PublicationCell{}, ErrUnavailable
	}
	cell.ExactLower = floatPointer(forecast.Lower)
	cell.ExactCentral = floatPointer(forecast.Central)
	cell.ExactUpper = floatPointer(forecast.Upper)
	cell.PublishedLower = analyticsInt32Pointer(rounded.Lower)
	cell.PublishedCentral = analyticsInt32Pointer(rounded.Central)
	cell.PublishedUpper = analyticsInt32Pointer(rounded.Upper)
	return cell, nil
}

func observedPublicationCell(
	date stay.CivilDate,
	source analytics.Cell,
	status analytics.CellStatus,
	published *int,
) analytics.PublicationCell {
	cell := basePublicationCell(
		date, "recent_30_days", "presence", "person_day", "none", "none", "observed",
		status, source,
	)
	if status == analytics.CellPublished && published != nil {
		cell.ExactValue = floatPointer(source.RawValue)
		cell.PublishedValue = analyticsInt32Pointer(*published)
	}
	return cell
}

func basePublicationCell(
	date stay.CivilDate,
	selector, metric, unit, dimension, category, kind string,
	status analytics.CellStatus,
	source analytics.Cell,
) analytics.PublicationCell {
	end := date.AddDays(1)
	return analytics.PublicationCell{
		CellKey:    metricCellKey(metric, selector, date.String(), dimension, category, kind),
		MetricCode: metric, PeriodSelector: selector,
		PeriodStart: date.String(), PeriodEnd: end.String(), Unit: unit,
		DimensionCode: dimension, CategoryCode: category, Kind: kind, Status: status,
		SampleSize:         int32(source.SampleSize),
		AccommodationCount: int32(source.AccommodationCount),
	}
}

func protectionCell(key, family string, aggregate factAggregate) analytics.Cell {
	return analytics.Cell{
		Key: key, Family: family, RawValue: aggregate.value,
		SampleSize:         len(aggregate.visitors),
		AccommodationCount: len(aggregate.accommodations),
	}
}

func aggregateOrEmpty(
	aggregates map[string]*factAggregate,
	date stay.CivilDate,
	kind string,
) factAggregate {
	value := aggregates[aggregateKey(date.String(), kind)]
	if value == nil {
		return factAggregate{
			visitors:       make(map[uuid.UUID]struct{}),
			accommodations: make(map[uuid.UUID]struct{}),
		}
	}
	return *value
}

func eligiblePreviousWeekdayValues(
	date stay.CivilDate,
	aggregates map[string]*factAggregate,
	policy analytics.Policy,
) []float64 {
	result := make([]float64, 0, 8)
	for weeks := 1; weeks <= 8; weeks++ {
		key := aggregateKey(date.AddDays(-7*weeks).String(), "observed")
		aggregate := aggregates[key]
		if eligibleHistoricalAggregate(aggregate, policy) {
			result = append(result, aggregate.value)
		}
	}
	return result
}

func eligibleHistoricalAggregate(
	aggregate *factAggregate,
	policy analytics.Policy,
) bool {
	return aggregate != nil &&
		len(aggregate.visitors) >= policy.PrimaryThreshold &&
		len(aggregate.accommodations) >= policy.MinimumReportingAccommodations
}

func (r *AnalyticsRepository) presencePolicy() analytics.Policy {
	return analytics.Policy{
		PrimaryThreshold:               r.analytics.PrimaryCellThreshold,
		MinimumReportingAccommodations: r.analytics.MinimumReportingAccommodations,
		RoundingBase:                   r.analytics.RoundingBase,
	}
}

func (r *AnalyticsRepository) buildPreferenceCells(
	asOf stay.CivilDate,
	rows []generated.ListEligiblePreferenceCountsRow,
) ([]analytics.PublicationCell, error) {
	counts, err := r.preferenceCounts(rows)
	if err != nil {
		return nil, err
	}
	threshold, err := r.effectivePreferenceThreshold(counts)
	if err != nil {
		return nil, err
	}
	categories := []string{"first_visit", "returning"}
	source := preferenceSourceCells(counts, categories)
	protected, err := analytics.ProtectCells(source, analytics.Policy{
		PrimaryThreshold:               threshold,
		MinimumReportingAccommodations: r.analytics.MinimumReportingAccommodations,
		RoundingBase:                   5,
	})
	if err != nil {
		return nil, ErrUnavailable
	}
	start, end := lastCompleteMonth(asOf)
	result := make([]analytics.PublicationCell, 0, len(categories))
	for index, category := range categories {
		status := protected[index].Status
		cell := preferencePublicationCell(
			start, end, category, source[index], status, protected[index].PublishedValue,
		)
		result = append(result, cell)
	}
	return result, nil
}

type preferenceCount struct {
	sample         int64
	accommodations int64
	minimum        int32
	present        bool
}

func (r *AnalyticsRepository) preferenceCounts(
	rows []generated.ListEligiblePreferenceCountsRow,
) (map[string]preferenceCount, error) {
	result := map[string]preferenceCount{
		"first_visit": {}, "returning": {},
	}
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if err := r.addPreferenceCount(result, seen, row); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func preferenceSourceCells(
	counts map[string]preferenceCount,
	categories []string,
) []analytics.Cell {
	total := counts["first_visit"].sample + counts["returning"].sample
	source := make([]analytics.Cell, 0, len(categories))
	for _, category := range categories {
		count := counts[category]
		share := 0.0
		if total > 0 {
			share = 100 * float64(count.sample) / float64(total)
		}
		source = append(source, analytics.Cell{
			Key: category, Family: "first_visit_share",
			RawValue: share, SampleSize: int(count.sample),
			AccommodationCount: int(count.accommodations),
		})
	}
	return source
}

func (r *AnalyticsRepository) addPreferenceCount(
	result map[string]preferenceCount,
	seen map[string]struct{},
	row generated.ListEligiblePreferenceCountsRow,
) error {
	if !r.acceptablePreferenceRow(row, result, seen) {
		return ErrUnavailable
	}
	seen[row.CategoryCode] = struct{}{}
	result[row.CategoryCode] = preferenceCount{
		sample: row.SampleSize, accommodations: row.AccommodationCount,
		minimum: row.MinimumPublicCell, present: true,
	}
	return nil
}

// A preference row must match the active policy, name a known category exactly
// once, and carry counts inside the int32 range the schema stores.
func (r *AnalyticsRepository) acceptablePreferenceRow(
	row generated.ListEligiblePreferenceCountsRow,
	result map[string]preferenceCount,
	seen map[string]struct{},
) bool {
	if row.PrivacyPolicyVersion != r.analytics.PrivacyPolicyVersion ||
		row.MetricCode != "first_visit_share" {
		return false
	}
	if !knownOnce(row.CategoryCode, result, seen) {
		return false
	}
	return validCount(row.SampleSize) && validCount(row.AccommodationCount)
}

func knownOnce(
	category string,
	result map[string]preferenceCount,
	seen map[string]struct{},
) bool {
	if _, allowed := result[category]; !allowed {
		return false
	}
	_, duplicate := seen[category]
	return !duplicate
}

func (r *AnalyticsRepository) effectivePreferenceThreshold(
	counts map[string]preferenceCount,
) (int, error) {
	result := r.analytics.PrimaryCellThreshold
	for _, category := range []string{"first_visit", "returning"} {
		count, exists := counts[category]
		if !r.usableThreshold(count, exists) {
			return 0, ErrUnavailable
		}
		result = max(result, int(count.minimum))
	}
	return result, nil
}

// The published threshold is never weaker than the configured one; a question
// may only raise its own minimum cell.
func (r *AnalyticsRepository) usableThreshold(count preferenceCount, exists bool) bool {
	return exists && count.present &&
		count.minimum >= int32(r.analytics.PrimaryCellThreshold)
}

func validCount(value int64) bool {
	return value >= 0 && value <= 1<<31-1
}

func lastCompleteMonth(asOf stay.CivilDate) (stay.CivilDate, stay.CivilDate) {
	current := asOf.Start()
	endTime := time.Date(
		current.Year(), current.Month(), 1, 0, 0, 0, 0, current.Location(),
	)
	startTime := endTime.AddDate(0, -1, 0)
	start, _ := stay.ParseCivilDate(startTime.Format("2006-01-02"))
	end, _ := stay.ParseCivilDate(endTime.Format("2006-01-02"))
	return start, end
}

func preferencePublicationCell(
	start, end stay.CivilDate,
	category string,
	source analytics.Cell,
	status analytics.CellStatus,
	published *int,
) analytics.PublicationCell {
	cell := analytics.PublicationCell{
		CellKey: metricCellKey(
			"first_visit_share", "last_complete_month", start.String(),
			"visit_profile", category, "preference",
		),
		MetricCode: "first_visit_share", PeriodSelector: "last_complete_month",
		PeriodStart: start.String(), PeriodEnd: end.String(), Unit: "survey_response",
		DimensionCode: "visit_profile", CategoryCode: category,
		Kind: "preference", Status: status,
		SampleSize:         int32(source.SampleSize),
		AccommodationCount: int32(source.AccommodationCount),
	}
	if status == analytics.CellPublished && published != nil {
		cell.ExactValue = floatPointer(source.RawValue)
		cell.PublishedValue = analyticsInt32Pointer(*published)
	}
	return cell
}

func (r *AnalyticsRepository) buildCoverage(
	rows []generated.ListActiveAccommodationCoverageRow,
) (analytics.Coverage, []categoryCoverage) {
	now := r.store.currentTime()
	global := coverageForRows(rows, now, r.analytics.MinimumReportingAccommodations)
	grouped := make(map[string][]generated.ListActiveAccommodationCoverageRow)
	for _, row := range rows {
		grouped[row.Category] = append(grouped[row.Category], row)
	}
	categories := make([]string, 0, len(grouped))
	for category := range grouped {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	result := make([]categoryCoverage, 0, len(categories))
	for _, category := range categories {
		result = append(result, categoryCoverage{
			category: category,
			coverage: coverageForRows(
				grouped[category], now, r.analytics.MinimumReportingAccommodations,
			),
		})
	}
	return global, result
}

func coverageForRows(
	rows []generated.ListActiveAccommodationCoverageRow,
	now time.Time,
	minimumReporting int,
) analytics.Coverage {
	source := make([]analytics.AccommodationCoverage, 0, len(rows))
	for _, row := range rows {
		capacity := 0
		if row.Capacity != nil {
			capacity = int(*row.Capacity)
		}
		lastReportedAt := time.Time{}
		if row.LastReportedAt.Valid {
			lastReportedAt = row.LastReportedAt.Time.UTC()
		}
		source = append(source, analytics.AccommodationCoverage{
			Active: true, Capacity: capacity, LastReportedAt: lastReportedAt,
		})
	}
	return analytics.ComputeCoverage(source, now, 30*24*time.Hour, minimumReporting)
}

func metricCellKey(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func floatPointer(value float64) *float64 {
	return &value
}

func analyticsInt32Pointer(value int) *int32 {
	result := int32(value)
	return &result
}

func (r *AnalyticsRepository) recordQuality(
	ctx context.Context,
	inputs publicationInputs,
	categories []categoryCoverage,
) error {
	now := r.store.currentTime()
	id, err := uuid.NewV7()
	if err != nil {
		return ErrUnavailable
	}
	return r.store.inTransaction(ctx, func(queries generated.Querier) error {
		if err := queries.InsertQualitySnapshot(ctx, generated.InsertQualitySnapshotParams{
			ID: idToPG(id), WindowCode: "last_30_days", UpdatedAt: pgTime(now),
			IncompleteStays:           inputs.incomplete,
			OverduePlannedDepartures:  inputs.overdue,
			SilentAccommodations:      silentAccommodationCount(inputs.coverage, now),
			AggregationFailures:       0,
			SuspectedDuplicatesReason: "pseudonym_not_approved",
			FnrhFailuresReason:        "not_implemented",
		}); err != nil {
			return ErrUnavailable
		}
		for _, category := range categories {
			if err := insertQualityCoverage(ctx, queries, id, category); err != nil {
				return err
			}
		}
		return nil
	})
}

func silentAccommodationCount(
	rows []generated.ListActiveAccommodationCoverageRow,
	now time.Time,
) int32 {
	cutoff := now.Add(-30 * 24 * time.Hour)
	var count int32
	for _, row := range rows {
		if !row.LastReportedAt.Valid || row.LastReportedAt.Time.Before(cutoff) {
			count++
		}
	}
	return count
}

// A withheld coverage is stored as "not_available" with no ratio, never as a
// zero that a reader could mistake for a measurement.
func coverageColumns(coverage analytics.Coverage) (string, pgtype.Numeric, error) {
	if coverage.Status != analytics.CoveragePublished || coverage.Ratio == nil {
		return "not_available", pgtype.Numeric{}, nil
	}
	ratio, err := numericFromFloat(float64(*coverage.Ratio) / 100)
	if err != nil {
		return "", pgtype.Numeric{}, ErrUnavailable
	}
	return "available", ratio, nil
}

func insertQualityCoverage(
	ctx context.Context,
	queries generated.Querier,
	snapshotID uuid.UUID,
	category categoryCoverage,
) error {
	if !qualityCategoryCode.MatchString(category.category) {
		return ErrUnavailable
	}
	status, ratio, err := coverageColumns(category.coverage)
	if err != nil {
		return err
	}
	if err := queries.InsertQualityCoverage(ctx, generated.InsertQualityCoverageParams{
		QualitySnapshotID: idToPG(snapshotID), CategoryCode: category.category,
		Status: status, Ratio: ratio,
	}); err != nil {
		return ErrUnavailable
	}
	return nil
}
