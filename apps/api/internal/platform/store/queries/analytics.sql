-- name: ListPresenceSourceStays :many
SELECT
  id,
  accommodation_id,
  status,
  planned_arrival_on,
  planned_departure_on,
  checked_in_at,
  checked_out_at,
  approval_state,
  version,
  updated_at
FROM core.stays
WHERE status IN ('pre_registered', 'checked_in', 'checked_out')
ORDER BY id;

-- name: ListPresenceReconciliationStays :many
SELECT
  stay.id,
  stay.accommodation_id,
  stay.status,
  stay.planned_arrival_on,
  stay.planned_departure_on,
  stay.checked_in_at,
  stay.checked_out_at,
  stay.approval_state,
  stay.version AS expected_version,
  stay.updated_at
FROM core.stays AS stay
-- O WHERE não muda de propósito. A elegibilidade por aprovação é decidida em
-- presenceEligible(), no Go, sobre esta projeção. Filtrar aqui deixaria os
-- fatos já materializados de uma estadia rejeitada órfãos: a cláusula EXISTS
-- abaixo precisa continuar trazendo a estadia agora inelegível justamente para
-- que o diff apague o que existia.
WHERE stay.status IN ('pre_registered', 'checked_in', 'checked_out')
  OR EXISTS (
    SELECT 1
    FROM analytics.presence_days AS fact
    WHERE fact.stay_id = stay.id
  )
ORDER BY stay.id;

-- name: ListStayVisitorsForPresence :many
SELECT id, stay_id
FROM core.visitors
WHERE stay_id = sqlc.arg(stay_id)
ORDER BY id;

-- name: UpsertPresenceDay :execrows
INSERT INTO analytics.presence_days (
  stay_id,
  visitor_id,
  presence_on,
  kind,
  weight,
  source_stay_version,
  as_of_on,
  updated_at
)
VALUES (
  sqlc.arg(stay_id),
  sqlc.arg(visitor_id),
  sqlc.arg(presence_on),
  sqlc.arg(kind),
  sqlc.arg(weight),
  sqlc.arg(source_stay_version),
  sqlc.arg(as_of_on),
  sqlc.arg(updated_at)
)
ON CONFLICT (stay_id, visitor_id, presence_on)
DO UPDATE SET
  kind = EXCLUDED.kind,
  weight = EXCLUDED.weight,
  source_stay_version = EXCLUDED.source_stay_version,
  as_of_on = EXCLUDED.as_of_on,
  updated_at = EXCLUDED.updated_at
WHERE analytics.presence_days.source_stay_version <= EXCLUDED.source_stay_version
  AND (
    analytics.presence_days.kind,
    analytics.presence_days.weight,
    analytics.presence_days.source_stay_version,
    analytics.presence_days.as_of_on
  ) IS DISTINCT FROM (
    EXCLUDED.kind,
    EXCLUDED.weight,
    EXCLUDED.source_stay_version,
    EXCLUDED.as_of_on
  );

-- name: ListPresenceDaysForStay :many
SELECT
  stay_id,
  visitor_id,
  presence_on,
  kind,
  weight,
  source_stay_version,
  as_of_on,
  updated_at
FROM analytics.presence_days
WHERE stay_id = sqlc.arg(stay_id)
ORDER BY visitor_id, presence_on;

-- name: DeletePresenceDay :execrows
DELETE FROM analytics.presence_days
WHERE stay_id = sqlc.arg(stay_id)
  AND visitor_id = sqlc.arg(visitor_id)
  AND presence_on = sqlc.arg(presence_on)
  AND source_stay_version <= sqlc.arg(source_stay_version);

-- name: ListPresenceFactsForWindow :many
SELECT
  presence.stay_id,
  presence.visitor_id,
  presence.presence_on,
  presence.kind,
  presence.weight,
  presence.source_stay_version,
  presence.as_of_on,
  presence.updated_at,
  stay.accommodation_id,
  accommodation.status AS accommodation_status,
  accommodation.category AS accommodation_category,
  accommodation.capacity AS accommodation_capacity,
  accommodation.updated_at AS accommodation_updated_at
FROM analytics.presence_days AS presence
JOIN core.stays AS stay ON stay.id = presence.stay_id
JOIN core.accommodations AS accommodation
  ON accommodation.id = stay.accommodation_id
WHERE presence.presence_on >= sqlc.arg(window_start)::date
  AND presence.presence_on < sqlc.arg(window_end)::date
ORDER BY
  presence.presence_on,
  stay.accommodation_id,
  presence.stay_id,
  presence.visitor_id;

-- name: ListActiveAccommodationCoverage :many
SELECT
  accommodation.id,
  accommodation.category,
  accommodation.capacity,
  (max(fact.updated_at) FILTER (
    WHERE fact.as_of_on >= sqlc.arg(window_start)::date
      AND fact.as_of_on < sqlc.arg(window_end)::date
  ))::timestamptz AS last_reported_at,
  (max(fact.presence_on) FILTER (
    WHERE fact.as_of_on >= sqlc.arg(window_start)::date
      AND fact.as_of_on < sqlc.arg(window_end)::date
  ))::date AS last_presence_on
FROM core.accommodations AS accommodation
LEFT JOIN core.stays AS stay
  ON stay.accommodation_id = accommodation.id
LEFT JOIN analytics.presence_days AS fact
  ON fact.stay_id = stay.id
WHERE accommodation.status = 'active'
GROUP BY
  accommodation.id,
  accommodation.category,
  accommodation.capacity
ORDER BY accommodation.id;

-- name: GetReconciliationRunByFingerprint :one
SELECT
  id,
  run_kind,
  as_of_on,
  source_fingerprint,
  status,
  started_at,
  completed_at,
  error_code
FROM analytics.reconciliation_runs
WHERE source_fingerprint = sqlc.arg(source_fingerprint);

-- name: InsertReconciliationRun :one
INSERT INTO analytics.reconciliation_runs (
  id,
  run_kind,
  as_of_on,
  source_fingerprint,
  status
)
VALUES (
  sqlc.arg(id),
  sqlc.arg(run_kind),
  sqlc.arg(as_of_on),
  sqlc.arg(source_fingerprint),
  'pending'
)
RETURNING
  id,
  run_kind,
  as_of_on,
  source_fingerprint,
  status,
  started_at,
  completed_at,
  error_code;

-- name: MarkReconciliationRunRunning :execrows
UPDATE analytics.reconciliation_runs
SET status = 'running'
WHERE id = sqlc.arg(id)
  AND status = 'pending';

-- name: CompleteReconciliationRun :execrows
UPDATE analytics.reconciliation_runs
SET
  status = sqlc.arg(status),
  completed_at = sqlc.arg(completed_at),
  error_code = sqlc.narg(error_code)
WHERE id = sqlc.arg(id)
  AND status IN ('pending', 'running');

-- name: ListActiveMetricCatalog :many
SELECT
  privacy_policy_version,
  metric_code,
  unit,
  period_selector,
  dimension_code,
  minimum_public_cell,
  minimum_reporting_accommodations,
  active
FROM analytics.metric_catalog
WHERE privacy_policy_version = sqlc.arg(privacy_policy_version)
  AND active
ORDER BY metric_code, period_selector, dimension_code;

-- name: ListMetricMappings :many
SELECT
  privacy_policy_version,
  metric_code,
  questionnaire_version_id,
  question_id,
  source_value,
  category_code
FROM analytics.metric_mappings
WHERE privacy_policy_version = sqlc.arg(privacy_policy_version)
  AND metric_code = sqlc.arg(metric_code)
ORDER BY questionnaire_version_id, question_id, source_value;

-- name: ListEligiblePreferenceCounts :many
WITH aggregate (
  privacy_policy_version,
  metric_code,
  category_code,
  sample_size,
  accommodation_count,
  minimum_public_cell
) AS (
  SELECT (result).*
  FROM analytics.aggregate_eligible_preferences(
    sqlc.arg(privacy_policy_version)::text,
    sqlc.arg(period_start)::timestamptz,
    sqlc.arg(period_end)::timestamptz
  ) AS result (
    privacy_policy_version text,
    metric_code text,
    category_code text,
    sample_size bigint,
    accommodation_count bigint,
    minimum_public_cell integer
  )
)
SELECT
  aggregate.privacy_policy_version::text AS privacy_policy_version,
  aggregate.metric_code::text AS metric_code,
  aggregate.category_code::text AS category_code,
  aggregate.sample_size::bigint AS sample_size,
  aggregate.accommodation_count::bigint AS accommodation_count,
  aggregate.minimum_public_cell::integer AS minimum_public_cell
FROM aggregate;

-- name: RecordAggregationFailureQualitySnapshot :one
WITH quality_failure_lock AS MATERIALIZED (
  SELECT pg_advisory_xact_lock(1129270869, 5)
),
previous_snapshot AS MATERIALIZED (
  SELECT
    snapshot.id,
    snapshot.incomplete_stays,
    snapshot.overdue_planned_departures,
    snapshot.silent_accommodations,
    snapshot.aggregation_failures,
    snapshot.suspected_duplicates,
    snapshot.suspected_duplicates_reason,
    snapshot.fnrh_failures,
    snapshot.fnrh_failures_reason
  FROM quality_failure_lock
  LEFT JOIN LATERAL (
    SELECT candidate.*
    FROM analytics.quality_snapshots AS candidate
    WHERE candidate.window_code = 'last_30_days'
    ORDER BY candidate.updated_at DESC, candidate.id DESC
    LIMIT 1
  ) AS snapshot ON true
),
inserted_snapshot AS (
  INSERT INTO analytics.quality_snapshots (
    id,
    window_code,
    updated_at,
    incomplete_stays,
    overdue_planned_departures,
    silent_accommodations,
    aggregation_failures,
    suspected_duplicates,
    suspected_duplicates_reason,
    fnrh_failures,
    fnrh_failures_reason
  )
  SELECT
    sqlc.arg(snapshot_id)::uuid,
    'last_30_days',
    sqlc.arg(updated_at)::timestamptz,
    coalesce(previous.incomplete_stays, 0),
    coalesce(previous.overdue_planned_departures, 0),
    coalesce(previous.silent_accommodations, 0),
    coalesce(previous.aggregation_failures, 0) + 1,
    previous.suspected_duplicates,
    coalesce(
      previous.suspected_duplicates_reason,
      'pseudonym_not_approved'
    ),
    previous.fnrh_failures,
    coalesce(previous.fnrh_failures_reason, 'phase_not_implemented')
  FROM previous_snapshot AS previous
  RETURNING id, aggregation_failures
),
copied_coverage AS (
  INSERT INTO analytics.quality_coverage (
    quality_snapshot_id,
    category_code,
    status,
    ratio
  )
  SELECT
    inserted.id,
    coverage.category_code,
    coverage.status,
    coverage.ratio
  FROM inserted_snapshot AS inserted
  JOIN previous_snapshot AS previous ON true
  JOIN analytics.quality_coverage AS coverage
    ON coverage.quality_snapshot_id = previous.id
  RETURNING quality_snapshot_id
)
SELECT
  inserted.id,
  inserted.aggregation_failures,
  count(copied.quality_snapshot_id)::integer AS coverage_count
FROM inserted_snapshot AS inserted
LEFT JOIN copied_coverage AS copied ON true
GROUP BY inserted.id, inserted.aggregation_failures;

-- name: DeleteStagedMetricCellsForRun :execrows
DELETE FROM analytics.staged_metric_cells
WHERE publication_run_id = sqlc.arg(publication_run_id);

-- name: InsertStagedMetricCell :exec
INSERT INTO analytics.staged_metric_cells (
  publication_run_id,
  cell_key,
  metric_code,
  period_selector,
  period_start,
  period_end,
  dimension_code,
  category_code,
  kind,
  exact_value,
  exact_lower,
  exact_central,
  exact_upper,
  sample_size,
  accommodation_count,
  protection_status
)
VALUES (
  sqlc.arg(publication_run_id),
  sqlc.arg(cell_key),
  sqlc.arg(metric_code),
  sqlc.arg(period_selector),
  sqlc.arg(period_start),
  sqlc.arg(period_end),
  sqlc.arg(dimension_code),
  sqlc.arg(category_code),
  sqlc.arg(kind),
  sqlc.narg(exact_value),
  sqlc.narg(exact_lower),
  sqlc.narg(exact_central),
  sqlc.narg(exact_upper),
  sqlc.arg(sample_size),
  sqlc.arg(accommodation_count),
  sqlc.arg(protection_status)
);

-- name: ListStagedMetricCellsForRun :many
SELECT
  publication_run_id,
  cell_key,
  metric_code,
  period_selector,
  period_start,
  period_end,
  dimension_code,
  category_code,
  kind,
  exact_value,
  exact_lower,
  exact_central,
  exact_upper,
  sample_size,
  accommodation_count,
  protection_status
FROM analytics.staged_metric_cells
WHERE publication_run_id = sqlc.arg(publication_run_id)
ORDER BY metric_code, period_selector, dimension_code, category_code, cell_key;

-- name: GetPublicationRunByFingerprint :one
SELECT
  id,
  build_fingerprint,
  as_of_on,
  privacy_policy_version,
  methodology_version,
  status,
  started_at,
  completed_at,
  error_code
FROM analytics.publication_runs
WHERE build_fingerprint = sqlc.arg(build_fingerprint);

-- name: InsertPublicationRun :exec
INSERT INTO analytics.publication_runs (
  id,
  build_fingerprint,
  as_of_on,
  privacy_policy_version,
  methodology_version,
  status
)
VALUES (
  sqlc.arg(id),
  sqlc.arg(build_fingerprint),
  sqlc.arg(as_of_on),
  sqlc.arg(privacy_policy_version),
  sqlc.arg(methodology_version),
  'building'
);

-- name: CompletePublicationRun :execrows
UPDATE analytics.publication_runs
SET
  status = sqlc.arg(status),
  completed_at = sqlc.arg(completed_at),
  error_code = sqlc.narg(error_code)
WHERE id = sqlc.arg(id)
  AND status = 'building';

-- name: GetPublicationByFingerprint :one
SELECT
  publication_version,
  build_fingerprint,
  as_of_on,
  data_mode,
  privacy_policy_version,
  methodology_version,
  coverage_status,
  coverage_ratio_percent,
  published_at
FROM public_data.publications
WHERE build_fingerprint = sqlc.arg(build_fingerprint);

-- name: InsertNextPublication :one
WITH publication_lock AS MATERIALIZED (
  SELECT pg_advisory_xact_lock(1129270869, 4)
),
next_version AS (
  SELECT (COALESCE(max(publication_version), 0) + 1)::bigint
    AS publication_version
  FROM public_data.publications
  CROSS JOIN publication_lock
)
INSERT INTO public_data.publications (
  publication_version,
  build_fingerprint,
  as_of_on,
  data_mode,
  privacy_policy_version,
  methodology_version,
  coverage_status,
  coverage_ratio_percent,
  published_at
)
SELECT
  next_version.publication_version,
  sqlc.arg(build_fingerprint),
  sqlc.arg(as_of_on),
  sqlc.arg(data_mode),
  sqlc.arg(privacy_policy_version),
  sqlc.arg(methodology_version),
  sqlc.arg(coverage_status),
  sqlc.narg(coverage_ratio_percent),
  sqlc.arg(published_at)
FROM next_version
RETURNING publication_version;

-- name: InsertPublishedMetricCell :exec
INSERT INTO public_data.metric_cells (
  publication_version,
  cell_key,
  metric_code,
  period_selector,
  period_start,
  period_end,
  unit,
  dimension_code,
  category_code,
  kind,
  status,
  published_value,
  published_lower,
  published_central,
  published_upper
)
VALUES (
  sqlc.arg(publication_version),
  sqlc.arg(cell_key),
  sqlc.arg(metric_code),
  sqlc.arg(period_selector),
  sqlc.arg(period_start),
  sqlc.arg(period_end),
  sqlc.arg(unit),
  sqlc.arg(dimension_code),
  sqlc.arg(category_code),
  sqlc.arg(kind),
  sqlc.arg(status),
  sqlc.narg(published_value),
  sqlc.narg(published_lower),
  sqlc.narg(published_central),
  sqlc.narg(published_upper)
);

-- name: GetCurrentPublicationVersion :one
SELECT publication_version
FROM public_data.current_publication
WHERE singleton;

-- name: PromoteCurrentPublication :execrows
INSERT INTO public_data.current_publication (
  singleton,
  publication_version
)
VALUES (true, sqlc.arg(publication_version))
ON CONFLICT (singleton)
DO UPDATE SET publication_version = EXCLUDED.publication_version
WHERE
  public_data.current_publication.publication_version
    <= EXCLUDED.publication_version;

-- name: InsertQualitySnapshot :exec
INSERT INTO analytics.quality_snapshots (
  id,
  window_code,
  updated_at,
  incomplete_stays,
  overdue_planned_departures,
  silent_accommodations,
  aggregation_failures,
  suspected_duplicates,
  suspected_duplicates_reason,
  fnrh_failures,
  fnrh_failures_reason
)
VALUES (
  sqlc.arg(id),
  sqlc.arg(window_code),
  sqlc.arg(updated_at),
  sqlc.arg(incomplete_stays),
  sqlc.arg(overdue_planned_departures),
  sqlc.arg(silent_accommodations),
  sqlc.arg(aggregation_failures),
  sqlc.narg(suspected_duplicates),
  sqlc.arg(suspected_duplicates_reason),
  sqlc.narg(fnrh_failures),
  sqlc.arg(fnrh_failures_reason)
);

-- name: InsertQualityCoverage :exec
INSERT INTO analytics.quality_coverage (
  quality_snapshot_id,
  category_code,
  status,
  ratio
)
VALUES (
  sqlc.arg(quality_snapshot_id),
  sqlc.arg(category_code),
  sqlc.arg(status),
  sqlc.narg(ratio)
);
