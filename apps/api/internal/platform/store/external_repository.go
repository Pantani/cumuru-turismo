package store

import (
	"context"
	"strconv"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/external"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ExternalIngestionRepository writes the layer under `external_runtime`. Every
// statement it issues lives inside the `external` schema; the role behind its
// pool cannot reach the protected series even if a future change asked it to.
type ExternalIngestionRepository struct {
	queries generated.Querier
	timeout time.Duration
}

// The timeout here is the ingestion budget, never DATABASE_TIMEOUT: that one
// sizes a request, and a cycle that walked a batch on a request clock would
// fail on the clock instead of on real trouble.
func NewExternalIngestionRepository(
	pool *pgxpool.Pool,
	timeout time.Duration,
) *ExternalIngestionRepository {
	return &ExternalIngestionRepository{
		queries: generated.New(pool),
		timeout: timeout,
	}
}

var _ external.Repository = (*ExternalIngestionRepository)(nil)

func (r *ExternalIngestionRepository) EnsureSource(
	ctx context.Context,
	source external.SourceRecord,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	return r.queries.UpsertExternalSource(
		ctx,
		generated.UpsertExternalSourceParams{
			SourceCode:           source.SourceCode,
			Publisher:            source.Publisher,
			LicenseCode:          source.LicenseCode,
			LicenseUrl:           source.LicenseURL,
			AttributionText:      source.AttributionText,
			TermsUrl:             source.TermsURL,
			CommercialUseAllowed: source.CommercialUseAllowed,
			Active:               source.Active,
		},
	)
}

func (r *ExternalIngestionRepository) EnsureSeries(
	ctx context.Context,
	series external.SeriesRecord,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	return r.queries.UpsertExternalSeries(ctx, externalSeriesParams(series))
}

func externalSeriesParams(
	series external.SeriesRecord,
) generated.UpsertExternalSeriesParams {
	return generated.UpsertExternalSeriesParams{
		SourceCode:            series.SourceCode,
		SeriesCode:            series.SeriesCode,
		CardCode:              externalOptionalText(series.CardCode),
		UnitCode:              series.UnitCode,
		PeriodKind:            series.PeriodKind,
		ValueKind:             series.ValueKind,
		DeclaredLag:           externalInterval(series.DeclaredLag),
		RetentionDays:         series.RetentionDays,
		PublicExposable:       series.PublicExposable,
		GeoScope:              series.GeoScope,
		DefinitionVersion:     series.DefinitionVersion,
		DataMode:              series.DataMode,
		Derived:               series.Derived,
		DerivationCode:        externalOptionalText(series.DerivationCode),
		UnavailableReasonCode: externalOptionalText(series.UnavailableReasonCode),
	}
}

func (r *ExternalIngestionRepository) StartRun(
	ctx context.Context,
	run external.RunStart,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	return r.queries.StartExternalFetchRun(
		ctx,
		generated.StartExternalFetchRunParams{
			ID:         pgtype.UUID{Bytes: run.ID, Valid: true},
			SourceCode: run.SourceCode,
			StartedAt:  externalTimestamp(run.StartedAt),
			Outcome:    run.Outcome,
		},
	)
}

func (r *ExternalIngestionRepository) FinishRun(
	ctx context.Context,
	result external.RunResult,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	return r.queries.FinishExternalFetchRun(
		ctx,
		generated.FinishExternalFetchRunParams{
			ID:                   pgtype.UUID{Bytes: result.ID, Valid: true},
			FinishedAt:           externalTimestamp(result.FinishedAt),
			Outcome:              result.Outcome,
			HttpStatus:           result.HTTPStatus,
			ObservationsWritten:  result.ObservationsWritten,
			BatchBudgetExhausted: result.BatchBudgetExhausted,
		},
	)
}

func (r *ExternalIngestionRepository) NextRevision(
	ctx context.Context,
	key external.ObservationKey,
) (int32, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	return r.queries.NextExternalObservationRevision(
		ctx,
		generated.NextExternalObservationRevisionParams{
			SourceCode:  key.SourceCode,
			SeriesCode:  key.SeriesCode,
			PeriodStart: externalTimestamp(key.PeriodStart),
		},
	)
}

func (r *ExternalIngestionRepository) InsertObservation(
	ctx context.Context,
	record external.ObservationRecord,
) (int64, error) {
	value, err := externalNumeric(record.Value)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	return r.queries.InsertExternalObservation(
		ctx, externalObservationParams(record, value),
	)
}

func externalObservationParams(
	record external.ObservationRecord,
	value pgtype.Numeric,
) generated.InsertExternalObservationParams {
	return generated.InsertExternalObservationParams{
		SourceCode:          record.Key.SourceCode,
		SeriesCode:          record.Key.SeriesCode,
		PeriodKind:          record.PeriodKind,
		PeriodStart:         externalTimestamp(record.Key.PeriodStart),
		PeriodEnd:           externalTimestamp(record.PeriodEnd),
		Revision:            record.Revision,
		ObservedValue:       value,
		QualityFlag:         record.QualityFlag,
		RetrievedAt:         externalTimestamp(record.RetrievedAt),
		SourceRevisionLabel: externalOptionalText(record.SourceRevisionLabel),
		PayloadDigest:       record.PayloadDigest,
		FetchRunID:          pgtype.UUID{Bytes: record.FetchRunID, Valid: true},
	}
}

func (r *ExternalIngestionRepository) DeleteExpiredObservations(
	ctx context.Context,
	reference time.Time,
	batch int32,
) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	return r.queries.DeleteExpiredExternalObservations(
		ctx,
		generated.DeleteExpiredExternalObservationsParams{
			ReferenceAt: externalTimestamp(reference),
			BatchLimit:  batch,
		},
	)
}

// ExternalContextRepository reads the layer through the public pool, which sees
// `public_data.current_external_context` and holds no privilege — not even
// USAGE — in the `external` schema. That is why `publicRuntimeSearchPath` did
// not have to change and why reading this layer cannot take the public API down
// at startup.
type ExternalContextRepository struct {
	queries generated.Querier
	timeout time.Duration
	now     func() time.Time
}

func NewExternalContextRepository(
	pool *pgxpool.Pool,
	timeout time.Duration,
) *ExternalContextRepository {
	return &ExternalContextRepository{
		queries: generated.New(pool),
		timeout: timeout,
		now:     func() time.Time { return time.Now().UTC() },
	}
}

var _ external.ContextReader = (*ExternalContextRepository)(nil)

func (r *ExternalContextRepository) Context(
	ctx context.Context,
) (external.PublicContext, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	rows, err := r.queries.ListCurrentExternalContext(ctx)
	if err != nil {
		return external.PublicContext{}, external.ErrContextUnavailable
	}
	sources, err := r.queries.ListCreditedExternalSources(ctx)
	if err != nil {
		return external.PublicContext{}, external.ErrContextUnavailable
	}
	return external.BuildDocument(
		externalContextRows(rows), externalCreditedSources(sources), r.now(),
	)
}

func externalContextRows(
	rows []generated.PublicDataCurrentExternalContext,
) []external.ContextRow {
	mapped := make([]external.ContextRow, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, externalContextRow(row))
	}
	return mapped
}

// The observation and the last run arrive through outer joins, so their columns
// are null exactly when the card has nothing to show. That is the unavailable
// branch of ADR-045 §7 and not an error: the row still carries the publisher,
// the licence and the attribution text.
func externalContextRow(
	row generated.PublicDataCurrentExternalContext,
) external.ContextRow {
	mapped := external.ContextRow{
		CardCode:              externalText(row.CardCode),
		SourceCode:            row.SourceCode,
		SeriesCode:            row.SeriesCode,
		UnitCode:              row.UnitCode,
		DataMode:              row.DataMode,
		Derived:               row.Derived,
		DerivationCode:        externalText(row.DerivationCode),
		UnavailableReasonCode: externalText(row.UnavailableReasonCode),
		DeclaredLagSeconds:    row.DeclaredLagSeconds,
		SourceRevisionLabel:   externalText(row.SourceRevisionLabel),
		LastFetchOutcome:      externalText(row.LastFetchOutcome),
	}
	externalApplyCredit(&mapped, row)
	externalApplyObservation(&mapped, row)
	return mapped
}

func externalApplyCredit(
	mapped *external.ContextRow,
	row generated.PublicDataCurrentExternalContext,
) {
	mapped.Publisher = row.Publisher
	mapped.LicenseCode = row.LicenseCode
	mapped.LicenseURL = row.LicenseUrl
	mapped.AttributionText = row.AttributionText
	mapped.TermsURL = row.TermsUrl
}

func externalApplyObservation(
	mapped *external.ContextRow,
	row generated.PublicDataCurrentExternalContext,
) {
	mapped.PeriodStart = externalInstant(row.PeriodStart)
	mapped.PeriodEnd = externalInstant(row.PeriodEnd)
	mapped.RetrievedAt = externalInstant(row.RetrievedAt)
	mapped.LastFetchFinishedAt = externalInstant(row.LastFetchFinishedAt)
	mapped.Value = externalOptionalFloat(row.ObservedValue)
	if row.Revision != nil {
		mapped.Revision = *row.Revision
	}
}

func externalCreditedSources(
	rows []generated.PublicDataCurrentExternalSource,
) []external.CreditedSource {
	sources := make([]external.CreditedSource, 0, len(rows))
	for _, row := range rows {
		sources = append(sources, external.CreditedSource{
			SourceCode:      row.SourceCode,
			Publisher:       row.Publisher,
			LicenseCode:     row.LicenseCode,
			LicenseURL:      row.LicenseUrl,
			AttributionText: row.AttributionText,
			TermsURL:        row.TermsUrl,
		})
	}
	return sources
}

func externalText(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func externalOptionalText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func externalInstant(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	instant := value.Time.UTC()
	return &instant
}

// A numeric that will not convert is a corrupt cell, not a zero. The point is
// dropped and the card falls back to the branch that says so, because inventing
// a value here would be the one thing ADR-045 §7 forbids outright.
func externalOptionalFloat(value pgtype.Numeric) *float64 {
	if !value.Valid {
		return nil
	}
	converted, err := value.Float64Value()
	if err != nil || !converted.Valid {
		return nil
	}
	return &converted.Float64
}

func externalTimestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: !value.IsZero()}
}

func externalInterval(value time.Duration) pgtype.Interval {
	return pgtype.Interval{
		Microseconds: value.Microseconds(),
		Valid:        true,
	}
}

// Full precision on purpose. The digest binds the value it stored, so a
// formatting step that silently rounded would make an unchanged fact look
// revised on the next cycle.
func externalNumeric(value float64) (pgtype.Numeric, error) {
	numeric := pgtype.Numeric{}
	err := numeric.Scan(strconv.FormatFloat(value, 'f', -1, 64))
	return numeric, err
}
