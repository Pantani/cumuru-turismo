-- Consolidated from 000004_admin_seed_password_rotation.down.sql (second pre-launch wave).
BEGIN;

REVOKE UPDATE (
  password_hash,
  password_changed_at,
  password_must_change
) ON TABLE auth.accounts FROM app_runtime;

ALTER TABLE auth.accounts
  DROP COLUMN password_must_change;

COMMIT;

-- Consolidated from 000003_local_password_auth.down.sql (second pre-launch wave).
BEGIN;

DROP TABLE auth.sessions;
DROP TABLE auth.accounts;
DROP SCHEMA auth;

COMMIT;

-- Consolidated from 000002_accommodation_onboarding.down.sql (second pre-launch wave).
BEGIN;

REVOKE INSERT (
  id,
  name
) ON TABLE core.organizations
FROM app_runtime;

REVOKE INSERT (
  id,
  organization_id,
  name,
  category,
  status,
  capacity,
  onboarding_submission_id
) ON TABLE core.accommodations
FROM app_runtime;

GRANT UPDATE (cadastur_id) ON TABLE core.accommodations
TO app_runtime;

REVOKE SELECT (
  id,
  name,
  legal_name,
  created_at,
  updated_at,
  version
) ON TABLE core.organizations
FROM app_runtime, worker_runtime;

GRANT SELECT ON TABLE core.organizations
TO app_runtime, worker_runtime;

DROP INDEX core.accommodations_onboarding_submission_idx;

ALTER TABLE core.accommodations
  DROP CONSTRAINT accommodations_category_valid;

ALTER TABLE core.accommodations
  DROP COLUMN onboarding_submission_id;

COMMENT ON COLUMN core.accommodations.cadastur_id IS NULL;

COMMIT;

-- Consolidated from 000019_select_single_current_quality_snapshot.down.sql.
BEGIN;

CREATE OR REPLACE VIEW analytics.current_quality
WITH (security_barrier = true)
AS
SELECT
  snapshot.window_code,
  snapshot.updated_at,
  snapshot.incomplete_stays,
  snapshot.overdue_planned_departures,
  snapshot.silent_accommodations,
  snapshot.aggregation_failures,
  snapshot.suspected_duplicates,
  snapshot.suspected_duplicates_reason,
  snapshot.fnrh_failures,
  snapshot.fnrh_failures_reason,
  coverage.category_code,
  coverage.status AS coverage_status,
  coverage.ratio AS coverage_ratio
FROM analytics.quality_snapshots AS snapshot
LEFT JOIN analytics.quality_coverage AS coverage
  ON coverage.quality_snapshot_id = snapshot.id
WHERE snapshot.updated_at = (
  SELECT max(candidate.updated_at)
  FROM analytics.quality_snapshots AS candidate
  WHERE candidate.window_code = snapshot.window_code
);

COMMENT ON VIEW analytics.current_quality IS
  'Projeção agregada interna, sem IDs de hospedagem, estadia ou pessoa.';

COMMIT;

-- Consolidated from 000018_add_bounded_operational_cleanup.down.sql.
BEGIN;

REVOKE EXECUTE ON FUNCTION platform.cleanup_expired_operational_records(
  timestamptz,
  integer
) FROM worker_runtime;

DROP FUNCTION platform.cleanup_expired_operational_records(
  timestamptz,
  integer
);

REVOKE SELECT (
  actor_key_hmac,
  method,
  operation_key,
  resource_id,
  idempotency_key_hmac,
  state,
  expires_at
) ON TABLE platform.idempotency_records
FROM migration_admin;

REVOKE DELETE ON TABLE platform.idempotency_records
FROM migration_admin;

REVOKE UPDATE (expires_at) ON TABLE platform.idempotency_records
FROM migration_admin;

REVOKE SELECT (
  scope,
  subject_hmac,
  window_started_at,
  expires_at
) ON TABLE platform.rate_limit_buckets
FROM migration_admin;

REVOKE DELETE ON TABLE platform.rate_limit_buckets
FROM migration_admin;

REVOKE UPDATE (expires_at) ON TABLE platform.rate_limit_buckets
FROM migration_admin;

REVOKE USAGE ON SCHEMA platform FROM migration_admin;

GRANT DELETE ON TABLE
  platform.idempotency_records,
  platform.rate_limit_buckets
TO worker_runtime;

GRANT SELECT (expires_at) ON TABLE platform.idempotency_records
TO worker_runtime;

GRANT SELECT (expires_at) ON TABLE platform.rate_limit_buckets
TO worker_runtime;

COMMIT;

-- Consolidated from 000017_enforce_presence_selector_kind.down.sql.
BEGIN;

ALTER TABLE public_data.metric_cells
  DROP CONSTRAINT metric_cells_presence_selector_kind_valid;

COMMIT;

-- Consolidated from 000016_expose_forecast_fallback_bounds.down.sql.
BEGIN;

DROP VIEW public_data.current_methodology;

CREATE VIEW public_data.current_methodology
WITH (security_barrier = true)
AS
SELECT
  publication.as_of_on,
  publication.data_mode,
  publication.privacy_policy_version,
  publication.methodology_version,
  publication.coverage_status,
  publication.coverage_ratio_percent,
  publication.published_at,
  '[arrival,departure)'::text AS presence_interval,
  'America/Bahia'::text AS time_zone,
  'checked_presence_through_as_of'::text AS observed_definition_code,
  'explainable-baseline-v1'::text AS forecast_definition_code,
  85::integer AS forecast_lower_percent,
  115::integer AS forecast_upper_percent,
  10::integer AS primary_threshold,
  3::integer AS minimum_reporting_accommodations,
  true AS complementary_suppression,
  10::integer AS rounding_base,
  'stable-half-up'::text AS rounding_mode,
  ARRAY['recent_30_days', 'next_30_days']::text[]
    AS allowed_presence_windows,
  ARRAY['last_complete_month']::text[]
    AS allowed_preference_periods
FROM public_data.current_publication AS current
JOIN public_data.publications AS publication
  ON publication.publication_version = current.publication_version
WHERE current.singleton;

GRANT SELECT ON TABLE public_data.current_methodology TO public_runtime;

COMMIT;

-- Consolidated from 000015_secure_preference_aggregation.down.sql.
BEGIN;

REVOKE EXECUTE ON FUNCTION analytics.aggregate_eligible_preferences(
  text,
  timestamptz,
  timestamptz
) FROM worker_runtime;

DROP FUNCTION analytics.aggregate_eligible_preferences(
  text,
  timestamptz,
  timestamptz
);

REVOKE SELECT (
  privacy_policy_version,
  metric_code,
  questionnaire_version_id,
  question_id,
  source_value,
  category_code
) ON TABLE analytics.metric_mappings
FROM migration_admin;

REVOKE SELECT (
  id,
  questionnaire_version_id,
  answer_type,
  data_classification,
  purpose_code,
  analytics_key,
  public_aggregation_allowed,
  minimum_public_cell
) ON TABLE survey.questions
FROM migration_admin;

REVOKE SELECT (
  response_id,
  questionnaire_version_id,
  question_id,
  structured_value
) ON TABLE survey.answers
FROM migration_admin;

REVOKE SELECT (
  id,
  stay_id,
  questionnaire_version_id,
  participation,
  submitted_at
) ON TABLE survey.responses
FROM migration_admin;

REVOKE SELECT (
  response_id,
  questionnaire_version_id,
  purpose_code,
  granted
) ON TABLE survey.consent_decisions
FROM migration_admin;

REVOKE SELECT (id, accommodation_id) ON TABLE core.stays
FROM migration_admin;

REVOKE USAGE, CREATE ON SCHEMA analytics FROM migration_admin;
REVOKE USAGE ON SCHEMA survey, core FROM migration_admin;

GRANT SELECT (
  id,
  stay_id,
  questionnaire_version_id,
  participation,
  submitted_at
) ON TABLE survey.responses
TO worker_runtime;

GRANT SELECT (
  response_id,
  questionnaire_version_id,
  question_id,
  structured_value
) ON TABLE survey.answers
TO worker_runtime;

GRANT SELECT (
  id,
  questionnaire_version_id,
  answer_type,
  data_classification,
  purpose_code,
  analytics_key,
  public_aggregation_allowed,
  minimum_public_cell
) ON TABLE survey.questions
TO worker_runtime;

GRANT SELECT (
  response_id,
  questionnaire_version_id,
  purpose_code,
  notice_version,
  granted
) ON TABLE survey.consent_decisions
TO worker_runtime;

GRANT INSERT, UPDATE, DELETE ON TABLE
  analytics.metric_catalog,
  analytics.metric_mappings
TO worker_runtime;

COMMIT;

-- Consolidated from 000014_harden_public_runtime_session.down.sql.
BEGIN;

-- A reversão é deliberadamente monotônica para ACLs de objetos: não
-- reintroduz EXECUTE/SELECT implícitos que a migration removeu. Apenas restaura
-- TEMPORARY concedido por default antes desta versão, para permitir o ciclo
-- descartável 13 -> 14 -> 13 -> 14 sem ampliar acesso aos dados.
DO $$
BEGIN
  EXECUTE pg_catalog.format(
    'GRANT TEMPORARY ON DATABASE %I TO PUBLIC',
    pg_catalog.current_database()
  );
END
$$;

COMMIT;

-- Consolidated from 000013_apply_phase4_privileges.down.sql.
REVOKE ALL ON TABLE
  public_data.current_summary,
  public_data.current_presence,
  public_data.current_preferences,
  public_data.current_methodology
FROM public_runtime;

REVOKE ALL ON TABLE analytics.current_quality
FROM app_runtime;

REVOKE ALL ON TABLE
  analytics.presence_days,
  analytics.reconciliation_runs,
  analytics.metric_catalog,
  analytics.metric_mappings,
  analytics.publication_runs,
  analytics.staged_metric_cells,
  analytics.quality_snapshots,
  analytics.quality_coverage,
  public_data.publications,
  public_data.metric_cells,
  public_data.current_publication
FROM worker_runtime;

REVOKE SELECT (
  id,
  accommodation_id,
  status,
  planned_arrival_on,
  planned_departure_on,
  checked_in_at,
  checked_out_at,
  version,
  updated_at
) ON TABLE core.stays
FROM worker_runtime;

REVOKE SELECT (
  id,
  stay_id
) ON TABLE core.visitors
FROM worker_runtime;

REVOKE SELECT (
  id,
  status,
  category,
  capacity,
  updated_at,
  version
) ON TABLE core.accommodations
FROM worker_runtime;

REVOKE SELECT (
  id,
  stay_id,
  questionnaire_version_id,
  participation,
  submitted_at
) ON TABLE survey.responses
FROM worker_runtime;

REVOKE SELECT (
  response_id,
  questionnaire_version_id,
  question_id,
  structured_value
) ON TABLE survey.answers
FROM worker_runtime;

REVOKE SELECT (
  id,
  questionnaire_version_id,
  answer_type,
  data_classification,
  purpose_code,
  analytics_key,
  public_aggregation_allowed,
  minimum_public_cell
) ON TABLE survey.questions
FROM worker_runtime;

REVOKE SELECT (
  response_id,
  questionnaire_version_id,
  purpose_code,
  notice_version,
  granted
) ON TABLE survey.consent_decisions
FROM worker_runtime;

REVOKE USAGE ON SCHEMA analytics FROM app_runtime, worker_runtime;
REVOKE USAGE ON SCHEMA public_data FROM worker_runtime, public_runtime;

GRANT USAGE ON SCHEMA public_data TO public_runtime;

-- Consolidated from 000012_create_public_snapshots.down.sql.
DROP VIEW IF EXISTS analytics.current_quality;
DROP VIEW IF EXISTS public_data.current_methodology;
DROP VIEW IF EXISTS public_data.current_preferences;
DROP VIEW IF EXISTS public_data.current_summary;
DROP VIEW IF EXISTS public_data.current_presence;

DROP TABLE IF EXISTS analytics.quality_coverage;
DROP TABLE IF EXISTS analytics.quality_snapshots;
DROP TABLE IF EXISTS public_data.current_publication;
DROP TABLE IF EXISTS public_data.metric_cells;
DROP TABLE IF EXISTS public_data.publications;

-- Consolidated from 000011_create_analytics_domain.down.sql.
DROP TABLE IF EXISTS analytics.staged_metric_cells;
DROP TABLE IF EXISTS analytics.publication_runs;
DROP TABLE IF EXISTS analytics.metric_mappings;
DROP TABLE IF EXISTS analytics.metric_catalog;
DROP TABLE IF EXISTS analytics.reconciliation_runs;
DROP TABLE IF EXISTS analytics.presence_days;

ALTER DEFAULT PRIVILEGES IN SCHEMA public_data
  GRANT SELECT ON TABLES TO public_runtime;

-- Consolidated from 000010_apply_phase3_privileges.down.sql.
REVOKE ALL ON TABLE
  survey.questionnaires,
  survey.questionnaire_versions,
  survey.questions,
  survey.question_options,
  survey.consent_requirements,
  survey.capabilities,
  survey.responses,
  survey.answers,
  survey.consent_decisions
FROM app_runtime, worker_runtime, public_runtime, privacy_officer;

REVOKE USAGE ON SCHEMA survey FROM app_runtime, worker_runtime;

REVOKE EXECUTE ON FUNCTION survey.erase_expired_free_text(timestamptz)
FROM worker_runtime;

-- Consolidated from 000009_create_survey_responses.down.sql.
ALTER TABLE platform.rate_limit_buckets
  DROP CONSTRAINT rate_limit_scope_valid;

ALTER TABLE platform.rate_limit_buckets
  ADD CONSTRAINT rate_limit_scope_valid
    CHECK (scope IN ('invite_context', 'invite_submit'));

DROP FUNCTION IF EXISTS survey.erase_expired_free_text(timestamptz);

DROP TRIGGER IF EXISTS response_semantics_guard
  ON survey.responses;
DROP FUNCTION IF EXISTS survey.validate_response_semantics();

DROP TRIGGER IF EXISTS consent_decisions_append_only
  ON survey.consent_decisions;
DROP TRIGGER IF EXISTS answers_append_only
  ON survey.answers;
DROP TRIGGER IF EXISTS responses_append_only
  ON survey.responses;
DROP FUNCTION IF EXISTS survey.reject_historical_mutation();
DROP FUNCTION IF EXISTS survey.guard_answer_mutation();

DROP TABLE IF EXISTS survey.consent_decisions;
DROP TABLE IF EXISTS survey.answers;
DROP TABLE IF EXISTS survey.responses;
DROP TABLE IF EXISTS survey.capabilities;
DROP TYPE IF EXISTS survey.participation;

-- Consolidated from 000008_create_questionnaire_catalog.down.sql.
DROP TRIGGER IF EXISTS consent_requirements_draft_guard
  ON survey.consent_requirements;
DROP TRIGGER IF EXISTS question_options_draft_guard
  ON survey.question_options;
DROP TRIGGER IF EXISTS questions_draft_guard
  ON survey.questions;
DROP FUNCTION IF EXISTS survey.guard_draft_content();

DROP TRIGGER IF EXISTS questionnaire_version_guard
  ON survey.questionnaire_versions;
DROP FUNCTION IF EXISTS survey.guard_questionnaire_version();

DROP TABLE IF EXISTS survey.consent_requirements;
DROP TABLE IF EXISTS survey.question_options;
DROP TABLE IF EXISTS survey.questions;
DROP TABLE IF EXISTS survey.questionnaire_versions;
DROP TABLE IF EXISTS survey.questionnaires;

DROP TYPE IF EXISTS survey.data_classification;
DROP TYPE IF EXISTS survey.answer_type;
DROP TYPE IF EXISTS survey.version_status;

-- Consolidated from 000007_apply_phase2_privileges.down.sql.
REVOKE ALL ON TABLE
  core.stays,
  core.group_submissions,
  core.visitors,
  core.invites,
  platform.idempotency_records,
  platform.outbox_events,
  platform.rate_limit_buckets
FROM app_runtime, worker_runtime, public_runtime, privacy_officer;

REVOKE INSERT (
  id,
  accommodation_id,
  oidc_issuer,
  oidc_subject,
  role
) ON TABLE core.memberships
FROM app_runtime;

REVOKE UPDATE (
  role,
  active,
  updated_at,
  version
) ON TABLE core.memberships
FROM app_runtime;

REVOKE INSERT (
  id,
  aggregate_type,
  aggregate_id,
  aggregate_version,
  event_type
) ON TABLE platform.outbox_events
FROM app_runtime;

REVOKE UPDATE (
  name,
  category,
  cadastur_id,
  capacity,
  public_area_code,
  updated_at,
  version
) ON TABLE core.accommodations
FROM app_runtime;

REVOKE UPDATE (
  available_at,
  lease_owner,
  lease_until,
  attempts,
  processed_at,
  last_error_code
) ON TABLE platform.outbox_events
FROM worker_runtime;

REVOKE SELECT (expires_at) ON TABLE platform.idempotency_records
FROM worker_runtime;

REVOKE SELECT (expires_at) ON TABLE platform.rate_limit_buckets
FROM worker_runtime;

-- Consolidated from 000006_create_idempotency_outbox_and_rate_limits.down.sql.
ALTER TABLE platform.audit_events
  DROP CONSTRAINT IF EXISTS audit_actor_key_version_valid,
  DROP COLUMN IF EXISTS actor_hmac_key_version;

DROP TABLE IF EXISTS platform.rate_limit_buckets;
DROP TABLE IF EXISTS platform.outbox_events;
DROP TABLE IF EXISTS platform.idempotency_records;

-- Consolidated from 000005_create_stay_domain.down.sql.
DROP TABLE IF EXISTS core.invites;
DROP TABLE IF EXISTS core.visitors;
DROP TABLE IF EXISTS core.group_submissions;
DROP TABLE IF EXISTS core.stays;

ALTER TABLE core.memberships
  DROP CONSTRAINT IF EXISTS memberships_version_positive,
  DROP CONSTRAINT IF EXISTS memberships_role_valid,
  DROP COLUMN IF EXISTS version,
  DROP COLUMN IF EXISTS updated_at;

DROP TYPE IF EXISTS core.visitor_role;
DROP TYPE IF EXISTS core.stay_status;

-- Consolidated from 000004_apply_privileges.down.sql.
BEGIN;

ALTER DEFAULT PRIVILEGES IN SCHEMA public_data
  REVOKE SELECT ON TABLES FROM public_runtime;

REVOKE USAGE ON SCHEMA identity FROM privacy_officer;
REVOKE USAGE ON SCHEMA public_data FROM public_runtime;

REVOKE INSERT ON platform.audit_events FROM worker_runtime;
REVOKE SELECT ON
  core.organizations,
  core.accommodations,
  core.memberships
  FROM worker_runtime;
REVOKE USAGE ON SCHEMA core, platform FROM worker_runtime;

REVOKE INSERT ON platform.audit_events FROM app_runtime;
REVOKE SELECT ON
  core.organizations,
  core.accommodations,
  core.memberships
  FROM app_runtime;
REVOKE USAGE ON SCHEMA core, platform FROM app_runtime;

COMMIT;

-- Consolidated from 000003_create_audit.down.sql.
BEGIN;

DROP INDEX platform.audit_events_time_idx;
DROP TABLE platform.audit_events;

COMMIT;

-- Consolidated from 000002_create_tenancy.down.sql.
BEGIN;

DROP INDEX core.memberships_principal_idx;
DROP TABLE core.memberships;
DROP INDEX core.accommodations_organization_idx;
DROP TABLE core.accommodations;
DROP TABLE core.organizations;

COMMIT;

-- Consolidated from 000001_create_schemas.down.sql.
BEGIN;

DROP SCHEMA platform;
DROP SCHEMA public_data;
DROP SCHEMA analytics;
DROP SCHEMA survey;
DROP SCHEMA core;
DROP SCHEMA identity;

COMMIT;
