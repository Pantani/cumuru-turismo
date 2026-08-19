-- name: AssumePublicRuntimeRole :exec
SET ROLE public_runtime;

-- name: SetPublicRuntimeSearchPath :exec
SET search_path = pg_catalog, public_data;

-- name: ValidatePublicRuntimeSession :one
-- `external` entra nesta lista pelo lado NEGATIVO (ADR-045, emenda ao
-- ADR-030): a varredura só detecta grant indevido no que ela varre, então sem
-- `external` aqui um SELECT concedido por engano a `public_runtime` em
-- `external.*` passaria despercebido. `expected_schema_usage` continua só com
-- `public_data`, porque o papel público não recebe USAGE em `external`.
WITH application_schemas AS (
  SELECT 'identity'::text AS schema_name
  UNION ALL SELECT 'core'::text
  UNION ALL SELECT 'survey'::text
  UNION ALL SELECT 'analytics'::text
  UNION ALL SELECT 'public_data'::text
  UNION ALL SELECT 'platform'::text
  UNION ALL SELECT 'external'::text
),
expected_select AS (
  SELECT
    'public_data'::text AS schema_name,
    'current_summary'::text AS relation_name
  UNION ALL SELECT 'public_data'::text, 'current_presence'::text
  UNION ALL SELECT 'public_data'::text, 'current_preferences'::text
  UNION ALL SELECT 'public_data'::text, 'current_methodology'::text
  UNION ALL SELECT 'public_data'::text, 'current_external_context'::text
  UNION ALL SELECT 'public_data'::text, 'current_external_sources'::text
),
checked_roles AS (
  SELECT current_user::text AS role_name
  UNION
  SELECT session_user::text
),
reachable_roles AS (
  SELECT role.rolname
  FROM pg_catalog.pg_roles AS role
  WHERE role.rolname <> session_user
    AND pg_catalog.pg_has_role(session_user, role.rolname, 'MEMBER')
),
expected_schema_usage AS (
  SELECT
    checked.role_name,
    'public_data'::text AS schema_name
  FROM checked_roles AS checked
),
expected_role_select AS (
  SELECT
    checked.role_name,
    expected.schema_name,
    expected.relation_name
  FROM checked_roles AS checked
  CROSS JOIN expected_select AS expected
),
actual_schema_usage AS (
  SELECT checked.role_name, candidate.schema_name
  FROM checked_roles AS checked
  CROSS JOIN application_schemas AS candidate
  WHERE pg_catalog.has_schema_privilege(
    checked.role_name,
    candidate.schema_name,
    'USAGE'
  )
),
actual_select AS (
  SELECT
    checked.role_name,
    namespace.nspname::text AS schema_name,
    relation.relname::text AS relation_name
  FROM checked_roles AS checked
  CROSS JOIN pg_catalog.pg_class AS relation
  JOIN pg_catalog.pg_namespace AS namespace
    ON namespace.oid = relation.relnamespace
  JOIN application_schemas AS candidate
    ON candidate.schema_name = namespace.nspname
  WHERE relation.relkind IN ('r', 'p', 'v', 'm', 'f', 'S')
    AND (
      pg_catalog.has_table_privilege(
        checked.role_name,
        relation.oid,
        'SELECT'
      )
      OR pg_catalog.has_any_column_privilege(
        checked.role_name,
        relation.oid,
        'SELECT'
      )
    )
),
application_relations AS (
  SELECT
    namespace.nspname::text AS schema_name,
    relation.relname::text AS relation_name,
    relation.oid
  FROM pg_catalog.pg_class AS relation
  JOIN pg_catalog.pg_namespace AS namespace
    ON namespace.oid = relation.relnamespace
  JOIN application_schemas AS candidate
    ON candidate.schema_name = namespace.nspname
  WHERE relation.relkind IN ('r', 'p', 'v', 'm', 'f', 'S')
),
application_functions AS (
  SELECT routine.oid
  FROM pg_catalog.pg_proc AS routine
  JOIN pg_catalog.pg_namespace AS namespace
    ON namespace.oid = routine.pronamespace
  JOIN application_schemas AS candidate
    ON candidate.schema_name = namespace.nspname
),
role_attributes AS (
  SELECT
    role.rolname,
    role.rolsuper,
    role.rolcreaterole,
    role.rolcreatedb,
    role.rolreplication,
    role.rolbypassrls
  FROM pg_catalog.pg_roles AS role
  JOIN checked_roles AS checked ON checked.role_name = role.rolname
)
SELECT
  current_user::text AS current_user_name,
  session_user::text AS session_user_name,
  pg_catalog.current_setting('search_path') AS search_path
WHERE current_user = 'public_runtime'
  AND session_user <> current_user
  AND (SELECT count(*) FROM role_attributes) = 2
  AND NOT EXISTS (
    SELECT 1
    FROM role_attributes AS role
    WHERE role.rolsuper
      OR role.rolcreaterole
      OR role.rolcreatedb
      OR role.rolreplication
      OR role.rolbypassrls
  )
  AND pg_catalog.pg_has_role(session_user, 'public_runtime', 'MEMBER')
  AND NOT EXISTS (
    SELECT rolname FROM reachable_roles
    EXCEPT
    SELECT 'public_runtime'::text
  )
  AND NOT EXISTS (
    SELECT 'public_runtime'::text
    EXCEPT
    SELECT rolname FROM reachable_roles
  )
  AND pg_catalog.current_setting('search_path') = 'pg_catalog, public_data'
  AND pg_catalog.has_database_privilege(
    current_user,
    pg_catalog.current_database(),
    'CONNECT'
  )
  AND pg_catalog.has_database_privilege(
    session_user,
    pg_catalog.current_database(),
    'CONNECT'
  )
  AND NOT pg_catalog.has_database_privilege(
    current_user,
    pg_catalog.current_database(),
    'CREATE'
  )
  AND NOT pg_catalog.has_database_privilege(
    current_user,
    pg_catalog.current_database(),
    'TEMPORARY'
  )
  AND NOT pg_catalog.has_database_privilege(
    session_user,
    pg_catalog.current_database(),
    'CREATE'
  )
  AND NOT pg_catalog.has_database_privilege(
    session_user,
    pg_catalog.current_database(),
    'TEMPORARY'
  )
  AND NOT EXISTS (
    SELECT role_name, schema_name FROM actual_schema_usage
    EXCEPT
    SELECT role_name, schema_name FROM expected_schema_usage
  )
  AND NOT EXISTS (
    SELECT role_name, schema_name FROM expected_schema_usage
    EXCEPT
    SELECT role_name, schema_name FROM actual_schema_usage
  )
  AND NOT EXISTS (
    SELECT role_name, schema_name, relation_name FROM actual_select
    EXCEPT
    SELECT role_name, schema_name, relation_name FROM expected_role_select
  )
  AND NOT EXISTS (
    SELECT role_name, schema_name, relation_name FROM expected_role_select
    EXCEPT
    SELECT role_name, schema_name, relation_name FROM actual_select
  )
  AND NOT EXISTS (
    SELECT 1
    FROM checked_roles AS checked
    CROSS JOIN application_schemas AS candidate
    WHERE pg_catalog.has_schema_privilege(
      checked.role_name,
      candidate.schema_name,
      'CREATE'
    )
  )
  AND NOT EXISTS (
    SELECT 1
    FROM checked_roles AS checked
    CROSS JOIN application_relations AS relation
    WHERE pg_catalog.has_table_privilege(
        checked.role_name, relation.oid, 'INSERT'
      )
      OR pg_catalog.has_table_privilege(
        checked.role_name, relation.oid, 'UPDATE'
      )
      OR pg_catalog.has_table_privilege(
        checked.role_name, relation.oid, 'DELETE'
      )
      OR pg_catalog.has_table_privilege(
        checked.role_name, relation.oid, 'TRUNCATE'
      )
      OR pg_catalog.has_table_privilege(
        checked.role_name, relation.oid, 'REFERENCES'
      )
      OR pg_catalog.has_table_privilege(
        checked.role_name, relation.oid, 'TRIGGER'
      )
      OR pg_catalog.has_any_column_privilege(
        checked.role_name, relation.oid, 'INSERT'
      )
      OR pg_catalog.has_any_column_privilege(
        checked.role_name, relation.oid, 'UPDATE'
      )
      OR pg_catalog.has_any_column_privilege(
        checked.role_name, relation.oid, 'REFERENCES'
      )
  )
  AND NOT EXISTS (
    SELECT 1
    FROM checked_roles AS checked
    CROSS JOIN application_functions AS routine
    WHERE pg_catalog.has_function_privilege(
      checked.role_name,
      routine.oid,
      'EXECUTE'
    )
  );

-- name: ListCurrentSummaryCells :many
SELECT
  period_selector,
  period_start,
  period_end,
  unit,
  kind,
  status,
  published_value,
  published_lower,
  published_central,
  published_upper,
  as_of_on,
  data_mode,
  privacy_policy_version,
  methodology_version,
  coverage_status,
  coverage_ratio_percent,
  published_at
FROM public_data.current_summary
ORDER BY period_selector, period_start, kind;

-- name: ListCurrentPresenceCells :many
SELECT
  period_selector,
  period_start,
  period_end,
  unit,
  kind,
  status,
  published_value,
  published_lower,
  published_central,
  published_upper,
  as_of_on,
  data_mode,
  privacy_policy_version,
  methodology_version,
  coverage_status,
  coverage_ratio_percent,
  published_at
FROM public_data.current_presence
WHERE period_selector = sqlc.arg(period_selector)
ORDER BY period_start, kind;

-- name: ListCurrentPresenceCellsForRecentDays :many
-- A janela recente é medida contra o `as_of_on` da própria publicação: um
-- intervalo calculado no cliente e enviado pronto passaria a depender do
-- relógio de quem consulta, não da release publicada.
SELECT
  period_selector,
  period_start,
  period_end,
  unit,
  kind,
  status,
  published_value,
  published_lower,
  published_central,
  published_upper,
  as_of_on,
  data_mode,
  privacy_policy_version,
  methodology_version,
  coverage_status,
  coverage_ratio_percent,
  published_at
FROM public_data.current_presence
WHERE period_selector = sqlc.arg(period_selector)
  AND period_start > as_of_on - sqlc.arg(lookback_days)::integer
  AND period_start <= as_of_on
ORDER BY period_start, kind;

-- name: ListCurrentPresenceCellsInRange :many
SELECT
  period_selector,
  period_start,
  period_end,
  unit,
  kind,
  status,
  published_value,
  published_lower,
  published_central,
  published_upper,
  as_of_on,
  data_mode,
  privacy_policy_version,
  methodology_version,
  coverage_status,
  coverage_ratio_percent,
  published_at
FROM public_data.current_presence
WHERE period_selector = sqlc.arg(period_selector)
  AND period_start >= sqlc.arg(start_on)
  AND period_start < sqlc.arg(end_on)
ORDER BY period_start, kind;

-- name: ListCurrentPreferenceCells :many
SELECT
  period_selector,
  period_start,
  period_end,
  unit,
  dimension_code,
  category_code,
  status,
  share_percent,
  as_of_on,
  data_mode,
  privacy_policy_version,
  methodology_version,
  coverage_status,
  coverage_ratio_percent,
  published_at
FROM public_data.current_preferences
WHERE period_selector = sqlc.arg(period_selector)
ORDER BY dimension_code, category_code;

-- name: GetCurrentMethodology :one
SELECT
  as_of_on,
  data_mode,
  privacy_policy_version,
  methodology_version,
  coverage_status,
  coverage_ratio_percent,
  published_at,
  presence_interval,
  time_zone,
  observed_definition_code,
  forecast_definition_code,
  forecast_lower_percent,
  forecast_upper_percent,
  forecast_fallback_lower_percent,
  forecast_fallback_upper_percent,
  primary_threshold,
  minimum_reporting_accommodations,
  complementary_suppression,
  rounding_base,
  rounding_mode,
  presence_history_days,
  allowed_presence_windows,
  allowed_preference_periods
FROM public_data.current_methodology;

-- name: ListCurrentQualityRows :many
SELECT
  window_code,
  updated_at,
  incomplete_stays,
  overdue_planned_departures,
  silent_accommodations,
  aggregation_failures,
  suspected_duplicates,
  suspected_duplicates_reason,
  fnrh_failures,
  fnrh_failures_reason,
  category_code,
  coverage_status,
  coverage_ratio
FROM analytics.current_quality
WHERE window_code = sqlc.arg(window_code)
ORDER BY category_code;
