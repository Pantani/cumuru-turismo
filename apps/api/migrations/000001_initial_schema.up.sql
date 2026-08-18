-- Consolidated from 000001_initial_schema.up.sql through 000005_audit_outbox_returning_grants.up.sql (third pre-launch wave). See ADR-032.
BEGIN;

CREATE SCHEMA identity;
CREATE SCHEMA core;
CREATE SCHEMA survey;
CREATE SCHEMA analytics;
CREATE SCHEMA public_data;
CREATE SCHEMA platform;

COMMENT ON SCHEMA identity IS
  'Acesso mínimo; contém identidade cifrada e credenciais externas em fases futuras.';
COMMENT ON SCHEMA public_data IS
  'Única fonte permitida para o papel da API pública.';

COMMIT;

-- Consolidated from 000002_create_tenancy.up.sql.
BEGIN;

CREATE TABLE core.organizations (
  id uuid PRIMARY KEY,
  name text NOT NULL,
  legal_name text,
  document_hmac bytea,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  CONSTRAINT organizations_name_not_blank CHECK (btrim(name) <> '')
);

CREATE TABLE core.accommodations (
  id uuid PRIMARY KEY,
  organization_id uuid NOT NULL REFERENCES core.organizations(id),
  name text NOT NULL,
  category text NOT NULL,
  status text NOT NULL DEFAULT 'pending_review',
  cadastur_id text,
  capacity integer,
  public_area_code text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  CONSTRAINT accommodations_name_not_blank CHECK (btrim(name) <> ''),
  CONSTRAINT accommodations_category_not_blank CHECK (btrim(category) <> ''),
  CONSTRAINT accommodations_status_valid
    CHECK (status IN ('pending_review', 'active', 'suspended', 'closed')),
  CONSTRAINT accommodations_capacity_positive
    CHECK (capacity IS NULL OR capacity > 0)
);

CREATE INDEX accommodations_organization_idx
  ON core.accommodations (organization_id, status);

CREATE TABLE core.memberships (
  id uuid PRIMARY KEY,
  accommodation_id uuid NOT NULL REFERENCES core.accommodations(id),
  oidc_issuer text NOT NULL,
  oidc_subject text NOT NULL,
  role text NOT NULL,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT memberships_oidc_issuer_not_blank CHECK (btrim(oidc_issuer) <> ''),
  CONSTRAINT memberships_oidc_subject_not_blank CHECK (btrim(oidc_subject) <> ''),
  CONSTRAINT memberships_role_not_blank CHECK (btrim(role) <> ''),
  UNIQUE (accommodation_id, oidc_issuer, oidc_subject)
);

CREATE INDEX memberships_principal_idx
  ON core.memberships (oidc_issuer, oidc_subject, active);

COMMIT;

-- Consolidated from 000003_create_audit.up.sql.
BEGIN;

CREATE TABLE platform.audit_events (
  id uuid PRIMARY KEY,
  occurred_at timestamptz NOT NULL DEFAULT now(),
  actor_subject_hmac bytea,
  actor_type text NOT NULL,
  organization_id uuid,
  action text NOT NULL,
  entity_type text NOT NULL,
  entity_id uuid,
  purpose_code text,
  request_id text,
  changed_fields text[],
  outcome text NOT NULL,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  CONSTRAINT audit_actor_type_not_blank CHECK (btrim(actor_type) <> ''),
  CONSTRAINT audit_action_not_blank CHECK (btrim(action) <> ''),
  CONSTRAINT audit_entity_type_not_blank CHECK (btrim(entity_type) <> ''),
  CONSTRAINT audit_outcome_not_blank CHECK (btrim(outcome) <> '')
);

CREATE INDEX audit_events_time_idx
  ON platform.audit_events (occurred_at DESC);

COMMENT ON TABLE platform.audit_events IS
  'Append-only para os papéis de runtime; nunca contém valores pessoais alterados.';

COMMIT;

-- Consolidated from 000004_apply_privileges.up.sql.
BEGIN;

DO $$
DECLARE
  required_role text;
BEGIN
  FOREACH required_role IN ARRAY ARRAY[
    'app_runtime',
    'worker_runtime',
    'public_runtime',
    'privacy_officer'
  ]
  LOOP
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = required_role) THEN
      RAISE EXCEPTION 'required database role is missing: %', required_role;
    END IF;
  END LOOP;
END
$$;

REVOKE ALL ON SCHEMA identity, core, survey, analytics, public_data, platform
  FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA
  identity, core, survey, analytics, public_data, platform
  FROM PUBLIC;

GRANT USAGE ON SCHEMA core, platform TO app_runtime;
GRANT SELECT ON
  core.organizations,
  core.accommodations,
  core.memberships
  TO app_runtime;
GRANT INSERT ON platform.audit_events TO app_runtime;
REVOKE UPDATE, DELETE ON platform.audit_events FROM app_runtime;

GRANT USAGE ON SCHEMA core, platform TO worker_runtime;
GRANT SELECT ON
  core.organizations,
  core.accommodations,
  core.memberships
  TO worker_runtime;
GRANT INSERT ON platform.audit_events TO worker_runtime;
REVOKE UPDATE, DELETE ON platform.audit_events FROM worker_runtime;

GRANT USAGE ON SCHEMA public_data TO public_runtime;
GRANT USAGE ON SCHEMA identity TO privacy_officer;

ALTER DEFAULT PRIVILEGES IN SCHEMA identity, core, survey, analytics, platform
  REVOKE ALL ON TABLES FROM PUBLIC;
ALTER DEFAULT PRIVILEGES IN SCHEMA public_data
  REVOKE ALL ON TABLES FROM PUBLIC;
ALTER DEFAULT PRIVILEGES IN SCHEMA public_data
  GRANT SELECT ON TABLES TO public_runtime;

COMMIT;

-- Consolidated from 000005_create_stay_domain.up.sql.
CREATE TYPE core.stay_status AS ENUM (
  'draft',
  'invited',
  'pre_registered',
  'checked_in',
  'checked_out',
  'cancelled',
  'no_show'
);

CREATE TYPE core.visitor_role AS ENUM (
  'responsible',
  'companion',
  'minor'
);

ALTER TABLE core.memberships
  ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now(),
  ADD COLUMN version bigint NOT NULL DEFAULT 1,
  ADD CONSTRAINT memberships_role_valid
    CHECK (role IN ('operator', 'manager')),
  ADD CONSTRAINT memberships_version_positive
    CHECK (version > 0);

CREATE TABLE core.stays (
  id uuid PRIMARY KEY,
  accommodation_id uuid NOT NULL REFERENCES core.accommodations(id),
  created_by_membership_id uuid NOT NULL REFERENCES core.memberships(id),
  status core.stay_status NOT NULL DEFAULT 'draft',
  client_submission_id uuid NOT NULL,
  planned_arrival_on date NOT NULL,
  planned_departure_on date NOT NULL,
  expected_guest_count integer NOT NULL,
  checked_in_at timestamptz,
  checked_out_at timestamptz,
  cancelled_at timestamptz,
  no_show_at timestamptz,
  cancellation_reason_code text,
  no_show_reason_code text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  CONSTRAINT stays_dates_valid
    CHECK (planned_departure_on > planned_arrival_on),
  CONSTRAINT stays_guest_count_valid
    CHECK (expected_guest_count BETWEEN 1 AND 100),
  CONSTRAINT stays_version_positive
    CHECK (version > 0),
  CONSTRAINT stays_cancel_fields_valid
    CHECK (
      (status = 'cancelled' AND cancelled_at IS NOT NULL AND cancellation_reason_code IS NOT NULL)
      OR
      (status <> 'cancelled' AND cancelled_at IS NULL AND cancellation_reason_code IS NULL)
    ),
  CONSTRAINT stays_no_show_fields_valid
    CHECK (
      (status = 'no_show' AND no_show_at IS NOT NULL)
      OR
      (status <> 'no_show' AND no_show_at IS NULL AND no_show_reason_code IS NULL)
    ),
  UNIQUE (accommodation_id, client_submission_id)
);

CREATE INDEX stays_accommodation_created_idx
  ON core.stays (accommodation_id, created_at DESC, id DESC);

CREATE INDEX stays_presence_lookup_idx
  ON core.stays (planned_arrival_on, planned_departure_on, status);

CREATE TABLE core.group_submissions (
  id uuid PRIMARY KEY,
  stay_id uuid NOT NULL UNIQUE REFERENCES core.stays(id) ON DELETE RESTRICT,
  client_submission_id uuid NOT NULL,
  request_hash bytea NOT NULL,
  privacy_notice_version text NOT NULL,
  collection_channel text NOT NULL,
  submitted_by_membership_id uuid REFERENCES core.memberships(id),
  submitted_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT group_submissions_notice_not_blank
    CHECK (btrim(privacy_notice_version) <> ''),
  CONSTRAINT group_submissions_channel_valid
    CHECK (collection_channel IN ('assisted', 'invite')),
  CONSTRAINT group_submissions_assisted_actor_valid
    CHECK (
      (collection_channel = 'assisted' AND submitted_by_membership_id IS NOT NULL)
      OR
      (collection_channel = 'invite' AND submitted_by_membership_id IS NULL)
    )
);

CREATE UNIQUE INDEX group_submissions_client_idx
  ON core.group_submissions (stay_id, client_submission_id);

CREATE TABLE core.visitors (
  id uuid PRIMARY KEY,
  stay_id uuid NOT NULL REFERENCES core.stays(id) ON DELETE RESTRICT,
  client_id uuid NOT NULL,
  role core.visitor_role NOT NULL,
  age_band text NOT NULL,
  residence_country char(2) NOT NULL,
  residence_state char(2),
  residence_city_code text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  CONSTRAINT visitors_age_band_valid
    CHECK (
      age_band IN (
        '0_5',
        '6_11',
        '12_17',
        '18_24',
        '25_34',
        '35_44',
        '45_59',
        '60_plus'
      )
    ),
  CONSTRAINT visitors_country_uppercase
    CHECK (residence_country = upper(residence_country)),
  CONSTRAINT visitors_state_uppercase
    CHECK (residence_state IS NULL OR residence_state = upper(residence_state)),
  CONSTRAINT visitors_version_positive
    CHECK (version > 0),
  UNIQUE (stay_id, client_id)
);

CREATE INDEX visitors_stay_idx
  ON core.visitors (stay_id);

CREATE UNIQUE INDEX visitors_one_responsible_per_stay_idx
  ON core.visitors (stay_id)
  WHERE role = 'responsible';

CREATE TABLE core.invites (
  id uuid PRIMARY KEY,
  stay_id uuid NOT NULL REFERENCES core.stays(id) ON DELETE RESTRICT,
  token_hmac bytea NOT NULL UNIQUE,
  token_key_version text NOT NULL,
  purpose text NOT NULL,
  privacy_notice_version text NOT NULL,
  expires_at timestamptz NOT NULL,
  max_uses integer NOT NULL DEFAULT 1,
  use_count integer NOT NULL DEFAULT 0,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT invites_key_version_not_blank
    CHECK (btrim(token_key_version) <> ''),
  CONSTRAINT invites_purpose_valid
    CHECK (purpose = 'stay_group_submission'),
  CONSTRAINT invites_notice_not_blank
    CHECK (btrim(privacy_notice_version) <> ''),
  CONSTRAINT invites_usage_valid
    CHECK (max_uses > 0 AND use_count >= 0 AND use_count <= max_uses)
);

CREATE INDEX invites_stay_active_idx
  ON core.invites (stay_id, expires_at)
  WHERE revoked_at IS NULL;

-- Consolidated from 000006_create_idempotency_outbox_and_rate_limits.up.sql.
CREATE TABLE platform.idempotency_records (
  actor_key_hmac bytea NOT NULL,
  actor_key_version text NOT NULL,
  method text NOT NULL,
  operation_key text NOT NULL,
  resource_id uuid NOT NULL,
  idempotency_key_hmac bytea NOT NULL,
  idempotency_key_version text NOT NULL,
  request_hash bytea NOT NULL,
  state text NOT NULL,
  response_status integer,
  response_headers jsonb,
  response_body bytea,
  response_resource_id uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz,
  expires_at timestamptz NOT NULL,
  PRIMARY KEY (
    actor_key_hmac,
    method,
    operation_key,
    resource_id,
    idempotency_key_hmac
  ),
  CONSTRAINT idempotency_actor_key_version_not_blank
    CHECK (btrim(actor_key_version) <> ''),
  CONSTRAINT idempotency_key_version_not_blank
    CHECK (btrim(idempotency_key_version) <> ''),
  CONSTRAINT idempotency_method_valid
    CHECK (method = 'POST'),
  CONSTRAINT idempotency_state_valid
    CHECK (state IN ('processing', 'completed')),
  CONSTRAINT idempotency_completion_valid
    CHECK (
      (
        state = 'processing'
        AND response_status IS NULL
        AND response_headers IS NULL
        AND response_body IS NULL
        AND completed_at IS NULL
      )
      OR
      (
        state = 'completed'
        AND response_status BETWEEN 200 AND 299
        AND response_headers IS NOT NULL
        AND completed_at IS NOT NULL
      )
    ),
  CONSTRAINT idempotency_expiry_valid
    CHECK (expires_at > created_at)
);

CREATE INDEX idempotency_expiry_idx
  ON platform.idempotency_records (expires_at);

CREATE TABLE platform.outbox_events (
  id uuid PRIMARY KEY,
  aggregate_type text NOT NULL,
  aggregate_id uuid NOT NULL,
  aggregate_version bigint NOT NULL,
  event_type text NOT NULL,
  occurred_at timestamptz NOT NULL DEFAULT now(),
  available_at timestamptz NOT NULL DEFAULT now(),
  lease_owner text,
  lease_until timestamptz,
  attempts integer NOT NULL DEFAULT 0,
  processed_at timestamptz,
  last_error_code text,
  CONSTRAINT outbox_aggregate_version_positive
    CHECK (aggregate_version > 0),
  CONSTRAINT outbox_attempts_nonnegative
    CHECK (attempts >= 0)
);

CREATE UNIQUE INDEX outbox_event_identity_idx
  ON platform.outbox_events (
    aggregate_type,
    aggregate_id,
    aggregate_version,
    event_type
  );

CREATE INDEX outbox_pending_idx
  ON platform.outbox_events (available_at, occurred_at)
  WHERE processed_at IS NULL;

CREATE TABLE platform.rate_limit_buckets (
  scope text NOT NULL,
  subject_hmac bytea NOT NULL,
  subject_key_version text NOT NULL,
  window_started_at timestamptz NOT NULL,
  request_count integer NOT NULL DEFAULT 1,
  expires_at timestamptz NOT NULL,
  PRIMARY KEY (scope, subject_hmac, window_started_at),
  CONSTRAINT rate_limit_scope_valid
    CHECK (scope IN ('invite_context', 'invite_submit')),
  CONSTRAINT rate_limit_key_version_not_blank
    CHECK (btrim(subject_key_version) <> ''),
  CONSTRAINT rate_limit_count_positive
    CHECK (request_count > 0),
  CONSTRAINT rate_limit_expiry_valid
    CHECK (expires_at > window_started_at)
);

CREATE INDEX rate_limit_expiry_idx
  ON platform.rate_limit_buckets (expires_at);

ALTER TABLE platform.audit_events
  ADD COLUMN actor_hmac_key_version text;

UPDATE platform.audit_events
SET actor_hmac_key_version = 'legacy-v1'
WHERE actor_subject_hmac IS NOT NULL;

ALTER TABLE platform.audit_events
  ADD CONSTRAINT audit_actor_key_version_valid
    CHECK (
      (actor_subject_hmac IS NULL AND actor_hmac_key_version IS NULL)
      OR
      (actor_subject_hmac IS NOT NULL AND btrim(actor_hmac_key_version) <> '')
    );

-- Consolidated from 000007_apply_phase2_privileges.up.sql.
REVOKE ALL ON TABLE
  core.stays,
  core.group_submissions,
  core.visitors,
  core.invites,
  platform.idempotency_records,
  platform.outbox_events,
  platform.rate_limit_buckets
FROM PUBLIC;

REVOKE ALL ON TABLE
  core.stays,
  core.group_submissions,
  core.visitors,
  core.invites,
  platform.idempotency_records,
  platform.outbox_events,
  platform.rate_limit_buckets
FROM app_runtime, worker_runtime, public_runtime, privacy_officer;

GRANT SELECT, INSERT, UPDATE ON TABLE
  core.stays,
  core.group_submissions,
  core.visitors,
  core.invites,
  platform.idempotency_records,
  platform.rate_limit_buckets
TO app_runtime;

GRANT INSERT (
  id,
  accommodation_id,
  oidc_issuer,
  oidc_subject,
  role
) ON TABLE core.memberships
TO app_runtime;

GRANT UPDATE (
  role,
  active,
  updated_at,
  version
) ON TABLE core.memberships
TO app_runtime;

GRANT INSERT (
  id,
  aggregate_type,
  aggregate_id,
  aggregate_version,
  event_type
) ON TABLE platform.outbox_events
TO app_runtime;

GRANT UPDATE (
  name,
  category,
  cadastur_id,
  capacity,
  public_area_code,
  updated_at,
  version
) ON TABLE core.accommodations
TO app_runtime;

GRANT DELETE ON TABLE
  platform.idempotency_records,
  platform.rate_limit_buckets
TO worker_runtime;

GRANT SELECT (expires_at) ON TABLE platform.idempotency_records
TO worker_runtime;

GRANT SELECT (expires_at) ON TABLE platform.rate_limit_buckets
TO worker_runtime;

GRANT SELECT, DELETE ON TABLE platform.outbox_events
TO worker_runtime;

GRANT UPDATE (
  available_at,
  lease_owner,
  lease_until,
  attempts,
  processed_at,
  last_error_code
) ON TABLE platform.outbox_events
TO worker_runtime;

REVOKE DELETE ON TABLE
  core.memberships,
  core.stays,
  core.group_submissions,
  core.visitors,
  core.invites
FROM app_runtime, worker_runtime;

REVOKE ALL ON TABLE
  core.stays,
  core.group_submissions,
  core.visitors,
  core.invites,
  platform.idempotency_records,
  platform.outbox_events,
  platform.rate_limit_buckets
FROM public_runtime, privacy_officer;

REVOKE UPDATE, DELETE ON TABLE platform.audit_events
FROM app_runtime, worker_runtime, public_runtime, privacy_officer;

-- Consolidated from 000008_create_questionnaire_catalog.up.sql.
CREATE TYPE survey.version_status AS ENUM (
  'draft',
  'privacy_review',
  'approved',
  'published',
  'retired'
);

CREATE TYPE survey.answer_type AS ENUM (
  'short_text',
  'long_text',
  'single_choice',
  'multiple_choice',
  'boolean',
  'integer_range',
  'rating',
  'date',
  'state_city'
);

CREATE TYPE survey.data_classification AS ENUM (
  'public',
  'operational',
  'personal',
  'sensitive',
  'secret'
);

CREATE TABLE survey.questionnaires (
  id uuid PRIMARY KEY,
  stable_key text NOT NULL UNIQUE,
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT questionnaires_stable_key_valid
    CHECK (stable_key ~ '^[a-z][a-z0-9_]{2,63}$'),
  CONSTRAINT questionnaires_name_not_blank
    CHECK (btrim(name) <> '')
);

CREATE TABLE survey.questionnaire_versions (
  id uuid PRIMARY KEY,
  questionnaire_id uuid NOT NULL REFERENCES survey.questionnaires(id),
  version_number integer NOT NULL,
  revision bigint NOT NULL DEFAULT 1,
  status survey.version_status NOT NULL DEFAULT 'draft',
  title text NOT NULL,
  introduction text,
  privacy_notice_version text NOT NULL,
  last_editor_hmac bytea NOT NULL,
  last_editor_key_version text NOT NULL,
  submitted_by_hmac bytea,
  submitted_by_key_version text,
  reviewed_by_hmac bytea,
  reviewed_by_key_version text,
  change_reason_code text,
  submitted_for_review_at timestamptz,
  privacy_reviewed_at timestamptz,
  published_at timestamptz,
  retired_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (questionnaire_id, version_number),
  CONSTRAINT questionnaire_version_number_positive
    CHECK (version_number > 0),
  CONSTRAINT questionnaire_revision_positive
    CHECK (revision > 0),
  CONSTRAINT questionnaire_title_not_blank
    CHECK (btrim(title) <> ''),
  CONSTRAINT questionnaire_notice_not_blank
    CHECK (btrim(privacy_notice_version) <> ''),
  CONSTRAINT questionnaire_editor_key_version_not_blank
    CHECK (btrim(last_editor_key_version) <> ''),
  CONSTRAINT questionnaire_submission_actor_complete
    CHECK (
      (submitted_by_hmac IS NULL AND submitted_by_key_version IS NULL)
      OR
      (
        submitted_by_hmac IS NOT NULL
        AND btrim(submitted_by_key_version) <> ''
      )
    ),
  CONSTRAINT questionnaire_reviewer_complete
    CHECK (
      (reviewed_by_hmac IS NULL AND reviewed_by_key_version IS NULL)
      OR
      (
        reviewed_by_hmac IS NOT NULL
        AND btrim(reviewed_by_key_version) <> ''
      )
    ),
  CONSTRAINT questionnaire_self_approval_forbidden
    CHECK (
      reviewed_by_hmac IS NULL
      OR submitted_by_hmac IS NULL
      OR reviewed_by_hmac <> submitted_by_hmac
    ),
  CONSTRAINT questionnaire_change_reason_valid
    CHECK (
      change_reason_code IS NULL
      OR change_reason_code IN (
        'privacy_metadata_incomplete',
        'excessive_collection',
        'unsafe_condition',
        'consent_mismatch'
      )
    )
);

CREATE UNIQUE INDEX one_published_questionnaire_version_idx
  ON survey.questionnaire_versions (questionnaire_id)
  WHERE status = 'published';

CREATE INDEX questionnaire_versions_catalog_idx
  ON survey.questionnaire_versions (
    questionnaire_id,
    version_number DESC,
    id DESC
  );

CREATE TABLE survey.questions (
  id uuid PRIMARY KEY,
  questionnaire_version_id uuid NOT NULL
    REFERENCES survey.questionnaire_versions(id) ON DELETE CASCADE,
  stable_key text NOT NULL,
  prompt text NOT NULL,
  help_text text,
  answer_type survey.answer_type NOT NULL,
  required boolean NOT NULL DEFAULT false,
  data_classification survey.data_classification NOT NULL,
  purpose_code text NOT NULL,
  retention_policy_code text NOT NULL,
  analytics_key text,
  public_aggregation_allowed boolean NOT NULL DEFAULT false,
  minimum_public_cell integer,
  validation jsonb NOT NULL DEFAULT '{}'::jsonb,
  visibility_rule jsonb,
  display_order integer NOT NULL,
  UNIQUE (questionnaire_version_id, stable_key),
  UNIQUE (questionnaire_version_id, display_order),
  UNIQUE (id, questionnaire_version_id),
  CONSTRAINT questions_stable_key_valid
    CHECK (stable_key ~ '^[a-z][a-z0-9_]{0,63}$'),
  CONSTRAINT questions_prompt_not_blank
    CHECK (btrim(prompt) <> ''),
  CONSTRAINT questions_purpose_not_blank
    CHECK (btrim(purpose_code) <> ''),
  CONSTRAINT questions_retention_not_blank
    CHECK (btrim(retention_policy_code) <> ''),
  CONSTRAINT questions_display_order_valid
    CHECK (display_order BETWEEN 1 AND 100),
  CONSTRAINT questions_minimum_public_cell_valid
    CHECK (
      minimum_public_cell IS NULL
      OR minimum_public_cell >= 10
    ),
  CONSTRAINT questions_sensitive_not_public
    CHECK (
      data_classification NOT IN ('sensitive', 'secret')
      OR public_aggregation_allowed = false
    ),
  CONSTRAINT questions_free_text_private
    CHECK (
      answer_type NOT IN ('short_text', 'long_text')
      OR (
        required = false
        AND data_classification = 'personal'
        AND analytics_key IS NULL
        AND public_aggregation_allowed = false
      )
    )
);

CREATE TABLE survey.question_options (
  id uuid PRIMARY KEY,
  question_id uuid NOT NULL REFERENCES survey.questions(id) ON DELETE CASCADE,
  value text NOT NULL,
  label text NOT NULL,
  display_order integer NOT NULL,
  UNIQUE (question_id, value),
  UNIQUE (question_id, display_order),
  CONSTRAINT question_options_value_not_blank CHECK (btrim(value) <> ''),
  CONSTRAINT question_options_label_not_blank CHECK (btrim(label) <> ''),
  CONSTRAINT question_options_display_order_valid
    CHECK (display_order BETWEEN 1 AND 100)
);

CREATE TABLE survey.consent_requirements (
  questionnaire_version_id uuid NOT NULL
    REFERENCES survey.questionnaire_versions(id) ON DELETE CASCADE,
  purpose_code text NOT NULL,
  notice_version text NOT NULL,
  prompt text NOT NULL,
  required_for_answers boolean NOT NULL DEFAULT false,
  display_order integer NOT NULL,
  PRIMARY KEY (
    questionnaire_version_id,
    purpose_code,
    notice_version
  ),
  UNIQUE (questionnaire_version_id, display_order),
  CONSTRAINT consent_requirements_purpose_not_blank
    CHECK (btrim(purpose_code) <> ''),
  CONSTRAINT consent_requirements_notice_not_blank
    CHECK (btrim(notice_version) <> ''),
  CONSTRAINT consent_requirements_prompt_not_blank
    CHECK (btrim(prompt) <> ''),
  CONSTRAINT consent_requirements_display_order_valid
    CHECK (display_order BETWEEN 1 AND 100)
);

CREATE FUNCTION survey.guard_questionnaire_version()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    IF OLD.status <> 'draft' THEN
      RAISE EXCEPTION 'questionnaire version is immutable';
    END IF;
    RETURN OLD;
  END IF;

  IF OLD.status <> 'draft' AND (
    NEW.questionnaire_id,
    NEW.version_number,
    NEW.title,
    NEW.introduction,
    NEW.privacy_notice_version
  ) IS DISTINCT FROM (
    OLD.questionnaire_id,
    OLD.version_number,
    OLD.title,
    OLD.introduction,
    OLD.privacy_notice_version
  ) THEN
    RAISE EXCEPTION 'questionnaire version content is immutable';
  END IF;

  IF NEW.status <> OLD.status AND NOT (
    (OLD.status = 'draft' AND NEW.status = 'privacy_review')
    OR (OLD.status = 'privacy_review' AND NEW.status IN ('draft', 'approved'))
    OR (OLD.status = 'approved' AND NEW.status = 'published')
    OR (OLD.status = 'published' AND NEW.status = 'retired')
  ) THEN
    RAISE EXCEPTION 'invalid questionnaire version transition';
  END IF;

  IF NEW.status = 'published' AND EXISTS (
    SELECT 1
    FROM survey.questions AS q
    WHERE q.questionnaire_version_id = OLD.id
      AND q.data_classification IN ('sensitive', 'secret')
  ) THEN
    RAISE EXCEPTION 'sensitive questionnaire cannot be published';
  END IF;

  RETURN NEW;
END;
$$;

CREATE TRIGGER questionnaire_version_guard
BEFORE UPDATE OR DELETE ON survey.questionnaire_versions
FOR EACH ROW EXECUTE FUNCTION survey.guard_questionnaire_version();

CREATE FUNCTION survey.guard_draft_content()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  target_version_id uuid;
  target_status survey.version_status;
BEGIN
  IF TG_TABLE_NAME = 'question_options' THEN
    SELECT q.questionnaire_version_id
    INTO target_version_id
    FROM survey.questions AS q
    WHERE q.id = COALESCE(NEW.question_id, OLD.question_id);
  ELSE
    target_version_id := COALESCE(
      NEW.questionnaire_version_id,
      OLD.questionnaire_version_id
    );
  END IF;

  SELECT qv.status
  INTO target_status
  FROM survey.questionnaire_versions AS qv
  WHERE qv.id = target_version_id
  FOR UPDATE;

  IF target_status IS DISTINCT FROM 'draft'::survey.version_status THEN
    RAISE EXCEPTION 'questionnaire content is immutable';
  END IF;

  RETURN COALESCE(NEW, OLD);
END;
$$;

CREATE TRIGGER questions_draft_guard
BEFORE INSERT OR UPDATE OR DELETE ON survey.questions
FOR EACH ROW EXECUTE FUNCTION survey.guard_draft_content();

CREATE TRIGGER question_options_draft_guard
BEFORE INSERT OR UPDATE OR DELETE ON survey.question_options
FOR EACH ROW EXECUTE FUNCTION survey.guard_draft_content();

CREATE TRIGGER consent_requirements_draft_guard
BEFORE INSERT OR UPDATE OR DELETE ON survey.consent_requirements
FOR EACH ROW EXECUTE FUNCTION survey.guard_draft_content();

-- Consolidated from 000009_create_survey_responses.up.sql.
CREATE TYPE survey.participation AS ENUM (
  'submitted',
  'declined'
);

CREATE TABLE survey.capabilities (
  id uuid PRIMARY KEY,
  token_hmac bytea NOT NULL UNIQUE,
  token_key_version text NOT NULL,
  purpose text NOT NULL,
  stay_id uuid NOT NULL REFERENCES core.stays(id) ON DELETE RESTRICT,
  questionnaire_version_id uuid NOT NULL
    REFERENCES survey.questionnaire_versions(id) ON DELETE RESTRICT,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  consumed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (stay_id, questionnaire_version_id),
  CONSTRAINT survey_capability_key_version_not_blank
    CHECK (btrim(token_key_version) <> ''),
  CONSTRAINT survey_capability_purpose_valid
    CHECK (purpose = 'survey_response'),
  CONSTRAINT survey_capability_expiry_valid
    CHECK (expires_at > created_at)
);

CREATE INDEX survey_capabilities_stay_idx
  ON survey.capabilities (stay_id, expires_at)
  WHERE revoked_at IS NULL AND consumed_at IS NULL;

CREATE TABLE survey.responses (
  id uuid PRIMARY KEY,
  stay_id uuid NOT NULL REFERENCES core.stays(id) ON DELETE RESTRICT,
  questionnaire_version_id uuid NOT NULL
    REFERENCES survey.questionnaire_versions(id) ON DELETE RESTRICT,
  capability_id uuid NOT NULL UNIQUE
    REFERENCES survey.capabilities(id) ON DELETE RESTRICT,
  client_submission_id uuid NOT NULL,
  participation survey.participation NOT NULL,
  submitted_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (stay_id, questionnaire_version_id, client_submission_id),
  UNIQUE (stay_id, questionnaire_version_id),
  UNIQUE (id, questionnaire_version_id)
);

CREATE TABLE survey.answers (
  id uuid PRIMARY KEY,
  response_id uuid NOT NULL,
  questionnaire_version_id uuid NOT NULL,
  question_id uuid NOT NULL,
  structured_value jsonb,
  encrypted_free_text bytea,
  free_text_nonce bytea,
  encryption_key_version text,
  erase_after timestamptz,
  erased_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (response_id, question_id),
  FOREIGN KEY (response_id, questionnaire_version_id)
    REFERENCES survey.responses(id, questionnaire_version_id)
    ON DELETE RESTRICT,
  FOREIGN KEY (question_id, questionnaire_version_id)
    REFERENCES survey.questions(id, questionnaire_version_id)
    ON DELETE RESTRICT,
  CONSTRAINT answers_one_value
    CHECK (
      (
        structured_value IS NOT NULL
        AND encrypted_free_text IS NULL
        AND free_text_nonce IS NULL
        AND encryption_key_version IS NULL
        AND erase_after IS NULL
        AND erased_at IS NULL
      )
      OR
      (
        structured_value IS NULL
        AND encrypted_free_text IS NOT NULL
        AND octet_length(encrypted_free_text) > 0
        AND octet_length(free_text_nonce) = 12
        AND btrim(encryption_key_version) <> ''
        AND erase_after > created_at
        AND erase_after <= created_at + interval '24 hours'
        AND erased_at IS NULL
      )
      OR
      (
        structured_value IS NULL
        AND encrypted_free_text IS NULL
        AND free_text_nonce IS NULL
        AND encryption_key_version IS NULL
        AND erase_after IS NOT NULL
        AND erase_after <= created_at + interval '24 hours'
        AND erased_at IS NOT NULL
      )
    )
);

CREATE TABLE survey.consent_decisions (
  id uuid PRIMARY KEY,
  response_id uuid NOT NULL,
  questionnaire_version_id uuid NOT NULL,
  purpose_code text NOT NULL,
  notice_version text NOT NULL,
  granted boolean NOT NULL,
  collection_channel text NOT NULL,
  recorded_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (response_id, purpose_code, notice_version),
  FOREIGN KEY (response_id, questionnaire_version_id)
    REFERENCES survey.responses(id, questionnaire_version_id)
    ON DELETE RESTRICT,
  FOREIGN KEY (
    questionnaire_version_id,
    purpose_code,
    notice_version
  ) REFERENCES survey.consent_requirements (
    questionnaire_version_id,
    purpose_code,
    notice_version
  ) ON DELETE RESTRICT,
  CONSTRAINT consent_decisions_channel_valid
    CHECK (collection_channel = 'survey_capability')
);

CREATE FUNCTION survey.reject_historical_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'survey history is append-only';
END;
$$;

CREATE FUNCTION survey.guard_answer_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'UPDATE'
    AND OLD.encrypted_free_text IS NOT NULL
    AND OLD.erase_after <= now()
    AND NEW.structured_value IS NULL
    AND NEW.encrypted_free_text IS NULL
    AND NEW.free_text_nonce IS NULL
    AND NEW.encryption_key_version IS NULL
    AND NEW.erase_after = OLD.erase_after
    AND NEW.erased_at IS NOT NULL
    AND (
      NEW.id,
      NEW.response_id,
      NEW.questionnaire_version_id,
      NEW.question_id,
      NEW.created_at
    ) = (
      OLD.id,
      OLD.response_id,
      OLD.questionnaire_version_id,
      OLD.question_id,
      OLD.created_at
    )
  THEN
    RETURN NEW;
  END IF;

  RAISE EXCEPTION 'survey answer history is append-only';
END;
$$;

CREATE TRIGGER responses_append_only
BEFORE UPDATE OR DELETE ON survey.responses
FOR EACH ROW EXECUTE FUNCTION survey.reject_historical_mutation();

CREATE TRIGGER answers_append_only
BEFORE UPDATE OR DELETE ON survey.answers
FOR EACH ROW EXECUTE FUNCTION survey.guard_answer_mutation();

CREATE TRIGGER consent_decisions_append_only
BEFORE UPDATE OR DELETE ON survey.consent_decisions
FOR EACH ROW EXECUTE FUNCTION survey.reject_historical_mutation();

CREATE FUNCTION survey.validate_response_semantics()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, survey
AS $$
DECLARE
  requirement_count integer;
  decision_count integer;
  answer_count integer;
BEGIN
  SELECT count(*) INTO requirement_count
  FROM survey.consent_requirements AS requirement
  WHERE requirement.questionnaire_version_id = NEW.questionnaire_version_id;

  SELECT count(*) INTO decision_count
  FROM survey.consent_decisions AS decision
  WHERE decision.response_id = NEW.id;

  SELECT count(*) INTO answer_count
  FROM survey.answers AS answer
  WHERE answer.response_id = NEW.id;

  IF NEW.participation = 'declined'
    AND (decision_count <> 0 OR answer_count <> 0)
  THEN
    RAISE EXCEPTION 'declined survey must not contain answers or decisions';
  END IF;

  IF NEW.participation = 'submitted'
    AND (
      decision_count <> requirement_count
      OR EXISTS (
        SELECT 1
        FROM survey.answers AS answer
        JOIN survey.questions AS question
          ON question.id = answer.question_id
        LEFT JOIN survey.consent_decisions AS decision
          ON decision.response_id = NEW.id
          AND decision.purpose_code = question.purpose_code
          AND decision.granted = true
        WHERE answer.response_id = NEW.id
          AND decision.id IS NULL
      )
    )
  THEN
    RAISE EXCEPTION 'survey decisions do not match the version snapshot';
  END IF;

  RETURN NEW;
END;
$$;

REVOKE ALL ON FUNCTION survey.validate_response_semantics()
FROM PUBLIC;

CREATE CONSTRAINT TRIGGER response_semantics_guard
AFTER INSERT ON survey.responses
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION survey.validate_response_semantics();

CREATE FUNCTION survey.erase_expired_free_text(cutoff timestamptz)
RETURNS integer
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, survey
AS $$
DECLARE
  erased_count integer;
BEGIN
  UPDATE survey.answers
  SET
    encrypted_free_text = NULL,
    free_text_nonce = NULL,
    encryption_key_version = NULL,
    erased_at = cutoff
  WHERE encrypted_free_text IS NOT NULL
    AND erase_after <= cutoff;

  GET DIAGNOSTICS erased_count = ROW_COUNT;
  RETURN erased_count;
END;
$$;

REVOKE ALL ON FUNCTION survey.erase_expired_free_text(timestamptz)
FROM PUBLIC;

ALTER TABLE platform.rate_limit_buckets
  DROP CONSTRAINT rate_limit_scope_valid;

ALTER TABLE platform.rate_limit_buckets
  ADD CONSTRAINT rate_limit_scope_valid
    CHECK (scope IN ('invite_context', 'invite_submit', 'survey_submit'));

-- Consolidated from 000010_apply_phase3_privileges.up.sql.
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
FROM PUBLIC, app_runtime, worker_runtime, public_runtime, privacy_officer;

GRANT USAGE ON SCHEMA survey TO app_runtime, worker_runtime;

GRANT SELECT ON TABLE
  survey.questionnaires,
  survey.questionnaire_versions,
  survey.questions,
  survey.question_options,
  survey.consent_requirements,
  survey.capabilities
TO app_runtime;

GRANT INSERT ON TABLE
  survey.questionnaires,
  survey.questionnaire_versions,
  survey.questions,
  survey.question_options,
  survey.consent_requirements,
  survey.capabilities,
  survey.responses,
  survey.answers,
  survey.consent_decisions
TO app_runtime;

GRANT UPDATE ON TABLE
  survey.questionnaire_versions,
  survey.questions,
  survey.question_options,
  survey.consent_requirements,
  survey.capabilities
TO app_runtime;

GRANT DELETE ON TABLE
  survey.questions,
  survey.question_options,
  survey.consent_requirements
TO app_runtime;

REVOKE SELECT, UPDATE, DELETE ON TABLE
  survey.responses,
  survey.answers,
  survey.consent_decisions
FROM app_runtime;

REVOKE ALL ON SCHEMA survey
FROM public_runtime, privacy_officer;

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
FROM worker_runtime, public_runtime, privacy_officer;

GRANT EXECUTE ON FUNCTION survey.erase_expired_free_text(timestamptz)
TO worker_runtime;

-- Consolidated from 000011_create_analytics_domain.up.sql.
ALTER DEFAULT PRIVILEGES IN SCHEMA public_data
  REVOKE SELECT ON TABLES FROM public_runtime;

CREATE TABLE analytics.presence_days (
  stay_id uuid NOT NULL REFERENCES core.stays(id) ON DELETE CASCADE,
  visitor_id uuid NOT NULL REFERENCES core.visitors(id) ON DELETE CASCADE,
  presence_on date NOT NULL,
  kind text NOT NULL,
  weight numeric(8, 6) NOT NULL,
  source_stay_version bigint NOT NULL,
  as_of_on date NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (stay_id, visitor_id, presence_on),
  CONSTRAINT presence_days_kind_valid
    CHECK (kind IN ('observed', 'forecast')),
  CONSTRAINT presence_days_weight_valid
    CHECK (weight > 0 AND weight <= 1),
  CONSTRAINT presence_days_source_version_positive
    CHECK (source_stay_version > 0)
);

CREATE INDEX presence_days_date_kind_idx
  ON analytics.presence_days (presence_on, kind);

CREATE INDEX presence_days_stay_version_idx
  ON analytics.presence_days (stay_id, source_stay_version);

CREATE TABLE analytics.reconciliation_runs (
  id uuid PRIMARY KEY,
  run_kind text NOT NULL,
  as_of_on date NOT NULL,
  source_fingerprint text NOT NULL UNIQUE,
  status text NOT NULL,
  started_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz,
  error_code text,
  CONSTRAINT reconciliation_runs_kind_valid
    CHECK (run_kind IN ('incremental', 'full')),
  CONSTRAINT reconciliation_runs_status_valid
    CHECK (status IN ('pending', 'running', 'succeeded', 'failed')),
  CONSTRAINT reconciliation_runs_fingerprint_valid
    CHECK (source_fingerprint ~ '^[0-9a-f]{64}$'),
  CONSTRAINT reconciliation_runs_completion_valid
    CHECK (
      (status IN ('pending', 'running') AND completed_at IS NULL)
      OR
      (status IN ('succeeded', 'failed') AND completed_at IS NOT NULL)
    ),
  CONSTRAINT reconciliation_runs_error_valid
    CHECK (
      (status = 'failed' AND error_code ~ '^[a-z0-9-]+$')
      OR
      (status <> 'failed' AND error_code IS NULL)
    )
);

CREATE TABLE analytics.metric_catalog (
  privacy_policy_version text NOT NULL,
  metric_code text NOT NULL,
  unit text NOT NULL,
  period_selector text NOT NULL,
  dimension_code text NOT NULL,
  minimum_public_cell integer NOT NULL DEFAULT 10,
  minimum_reporting_accommodations integer NOT NULL DEFAULT 3,
  active boolean NOT NULL DEFAULT true,
  PRIMARY KEY (
    privacy_policy_version,
    metric_code,
    period_selector,
    dimension_code
  ),
  CONSTRAINT metric_catalog_policy_valid
    CHECK (privacy_policy_version = 'prototype-v1'),
  CONSTRAINT metric_catalog_shape_valid
    CHECK (
      (
        metric_code = 'presence'
        AND unit = 'person_day'
        AND period_selector IN ('recent_30_days', 'next_30_days')
        AND dimension_code = 'none'
      )
      OR
      (
        metric_code = 'first_visit_share'
        AND unit = 'survey_response'
        AND period_selector = 'last_complete_month'
        AND dimension_code = 'visit_profile'
      )
    ),
  CONSTRAINT metric_catalog_threshold_valid
    CHECK (minimum_public_cell >= 10),
  CONSTRAINT metric_catalog_accommodations_valid
    CHECK (minimum_reporting_accommodations >= 3)
);

CREATE TABLE analytics.metric_mappings (
  privacy_policy_version text NOT NULL,
  metric_code text NOT NULL,
  questionnaire_version_id uuid NOT NULL,
  question_id uuid NOT NULL,
  source_value text NOT NULL,
  category_code text NOT NULL,
  PRIMARY KEY (
    privacy_policy_version,
    metric_code,
    questionnaire_version_id,
    question_id,
    source_value
  ),
  FOREIGN KEY (question_id, questionnaire_version_id)
    REFERENCES survey.questions(id, questionnaire_version_id)
    ON DELETE RESTRICT,
  CONSTRAINT metric_mappings_policy_valid
    CHECK (privacy_policy_version = 'prototype-v1'),
  CONSTRAINT metric_mappings_metric_valid
    CHECK (metric_code = 'first_visit_share'),
  CONSTRAINT metric_mappings_source_value_valid
    CHECK (source_value ~ '^[a-z][a-z0-9_]*$'),
  CONSTRAINT metric_mappings_category_valid
    CHECK (category_code IN ('first_visit', 'returning'))
);

CREATE TABLE analytics.publication_runs (
  id uuid PRIMARY KEY,
  build_fingerprint text NOT NULL UNIQUE,
  as_of_on date NOT NULL,
  privacy_policy_version text NOT NULL,
  methodology_version text NOT NULL,
  status text NOT NULL,
  started_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz,
  error_code text,
  CONSTRAINT publication_runs_fingerprint_valid
    CHECK (build_fingerprint ~ '^[0-9a-f]{64}$'),
  CONSTRAINT publication_runs_policy_valid
    CHECK (privacy_policy_version = 'prototype-v1'),
  CONSTRAINT publication_runs_methodology_valid
    CHECK (methodology_version = 'explainable-baseline-v1'),
  CONSTRAINT publication_runs_status_valid
    CHECK (status IN ('building', 'published', 'failed')),
  CONSTRAINT publication_runs_completion_valid
    CHECK (
      (status = 'building' AND completed_at IS NULL)
      OR
      (status IN ('published', 'failed') AND completed_at IS NOT NULL)
    ),
  CONSTRAINT publication_runs_error_valid
    CHECK (
      (status = 'failed' AND error_code ~ '^[a-z0-9-]+$')
      OR
      (status <> 'failed' AND error_code IS NULL)
    )
);

CREATE TABLE analytics.staged_metric_cells (
  publication_run_id uuid NOT NULL
    REFERENCES analytics.publication_runs(id) ON DELETE CASCADE,
  cell_key text NOT NULL,
  metric_code text NOT NULL,
  period_selector text NOT NULL,
  period_start date NOT NULL,
  period_end date NOT NULL,
  dimension_code text NOT NULL,
  category_code text NOT NULL,
  kind text NOT NULL,
  exact_value numeric,
  exact_lower numeric,
  exact_central numeric,
  exact_upper numeric,
  sample_size integer NOT NULL,
  accommodation_count integer NOT NULL,
  protection_status text NOT NULL,
  PRIMARY KEY (publication_run_id, cell_key),
  CONSTRAINT staged_cells_key_valid
    CHECK (cell_key ~ '^[0-9a-f]{64}$'),
  CONSTRAINT staged_cells_period_valid
    CHECK (period_end > period_start),
  CONSTRAINT staged_cells_kind_valid
    CHECK (kind IN ('observed', 'forecast', 'preference')),
  CONSTRAINT staged_cells_counts_valid
    CHECK (sample_size >= 0 AND accommodation_count >= 0),
  CONSTRAINT staged_cells_status_valid
    CHECK (protection_status IN ('published', 'protected', 'unavailable')),
  CONSTRAINT staged_cells_values_valid
    CHECK (
      (
        protection_status <> 'published'
        AND exact_value IS NULL
        AND exact_lower IS NULL
        AND exact_central IS NULL
        AND exact_upper IS NULL
      )
      OR
      (
        protection_status = 'published'
        AND kind IN ('observed', 'preference')
        AND exact_value IS NOT NULL
        AND exact_value >= 0
        AND exact_lower IS NULL
        AND exact_central IS NULL
        AND exact_upper IS NULL
      )
      OR
      (
        protection_status = 'published'
        AND kind = 'forecast'
        AND exact_value IS NULL
        AND exact_lower IS NOT NULL
        AND exact_central IS NOT NULL
        AND exact_upper IS NOT NULL
        AND exact_lower >= 0
        AND exact_lower <= exact_central
        AND exact_central <= exact_upper
      )
    )
);

CREATE INDEX staged_metric_cells_run_metric_idx
  ON analytics.staged_metric_cells (
    publication_run_id,
    metric_code,
    period_selector
  );

INSERT INTO analytics.metric_catalog (
  privacy_policy_version,
  metric_code,
  unit,
  period_selector,
  dimension_code
)
VALUES
  (
    'prototype-v1',
    'presence',
    'person_day',
    'recent_30_days',
    'none'
  ),
  (
    'prototype-v1',
    'presence',
    'person_day',
    'next_30_days',
    'none'
  ),
  (
    'prototype-v1',
    'first_visit_share',
    'survey_response',
    'last_complete_month',
    'visit_profile'
  );

COMMENT ON TABLE analytics.presence_days IS
  'Fato pseudonimizado interno; nunca conceder ao public_runtime.';
COMMENT ON TABLE analytics.staged_metric_cells IS
  'Staging interno já sem valores para células protegidas; não é superfície pública.';

-- Consolidated from 000012_create_public_snapshots.up.sql.
CREATE TABLE public_data.publications (
  publication_version bigint PRIMARY KEY,
  build_fingerprint text NOT NULL UNIQUE,
  as_of_on date NOT NULL,
  data_mode text NOT NULL,
  privacy_policy_version text NOT NULL,
  methodology_version text NOT NULL,
  coverage_status text NOT NULL,
  coverage_ratio_percent integer,
  published_at timestamptz NOT NULL,
  CONSTRAINT publications_version_positive
    CHECK (publication_version > 0),
  CONSTRAINT publications_fingerprint_valid
    CHECK (build_fingerprint ~ '^[0-9a-f]{64}$'),
  CONSTRAINT publications_data_mode_valid
    CHECK (data_mode = 'prototype_fixtures'),
  CONSTRAINT publications_policy_valid
    CHECK (privacy_policy_version = 'prototype-v1'),
  CONSTRAINT publications_methodology_valid
    CHECK (methodology_version = 'explainable-baseline-v1'),
  CONSTRAINT publications_coverage_valid
    CHECK (
      (
        coverage_status = 'published'
        AND coverage_ratio_percent BETWEEN 0 AND 100
        AND coverage_ratio_percent % 5 = 0
      )
      OR
      (
        coverage_status IN ('protected', 'unavailable')
        AND coverage_ratio_percent IS NULL
      )
    )
);

CREATE TABLE public_data.metric_cells (
  publication_version bigint NOT NULL
    REFERENCES public_data.publications(publication_version) ON DELETE RESTRICT,
  cell_key text NOT NULL,
  metric_code text NOT NULL,
  period_selector text NOT NULL,
  period_start date NOT NULL,
  period_end date NOT NULL,
  unit text NOT NULL,
  dimension_code text NOT NULL,
  category_code text NOT NULL,
  kind text NOT NULL,
  status text NOT NULL,
  published_value integer,
  published_lower integer,
  published_central integer,
  published_upper integer,
  PRIMARY KEY (publication_version, cell_key),
  CONSTRAINT metric_cells_key_valid
    CHECK (cell_key ~ '^[0-9a-f]{64}$'),
  CONSTRAINT metric_cells_period_valid
    CHECK (period_end > period_start),
  CONSTRAINT metric_cells_catalog_valid
    CHECK (
      (
        metric_code = 'presence'
        AND period_selector IN ('recent_30_days', 'next_30_days')
        AND unit = 'person_day'
        AND dimension_code = 'none'
        AND category_code = 'none'
        AND kind IN ('observed', 'forecast')
        AND period_end = period_start + 1
      )
      OR
      (
        metric_code = 'first_visit_share'
        AND period_selector = 'last_complete_month'
        AND unit = 'survey_response'
        AND dimension_code = 'visit_profile'
        AND category_code IN ('first_visit', 'returning')
        AND kind = 'preference'
      )
    ),
  CONSTRAINT metric_cells_status_valid
    CHECK (status IN ('published', 'protected', 'unavailable')),
  CONSTRAINT metric_cells_values_valid
    CHECK (
      (
        status <> 'published'
        AND published_value IS NULL
        AND published_lower IS NULL
        AND published_central IS NULL
        AND published_upper IS NULL
      )
      OR
      (
        status = 'published'
        AND kind = 'observed'
        AND published_value >= 0
        AND published_value % 10 = 0
        AND published_lower IS NULL
        AND published_central IS NULL
        AND published_upper IS NULL
      )
      OR
      (
        status = 'published'
        AND kind = 'preference'
        AND published_value BETWEEN 0 AND 100
        AND published_value % 5 = 0
        AND published_lower IS NULL
        AND published_central IS NULL
        AND published_upper IS NULL
      )
      OR
      (
        status = 'published'
        AND kind = 'forecast'
        AND published_value IS NULL
        AND published_lower >= 0
        AND published_lower % 10 = 0
        AND published_central % 10 = 0
        AND published_upper % 10 = 0
        AND published_lower <= published_central
        AND published_central <= published_upper
      )
    )
);

CREATE INDEX metric_cells_current_lookup_idx
  ON public_data.metric_cells (
    publication_version,
    metric_code,
    period_selector,
    period_start,
    category_code
  );

CREATE TABLE public_data.current_publication (
  singleton boolean PRIMARY KEY DEFAULT true,
  publication_version bigint NOT NULL UNIQUE
    REFERENCES public_data.publications(publication_version) ON DELETE RESTRICT,
  CONSTRAINT current_publication_singleton CHECK (singleton)
);

CREATE TABLE analytics.quality_snapshots (
  id uuid PRIMARY KEY,
  window_code text NOT NULL,
  updated_at timestamptz NOT NULL,
  incomplete_stays integer NOT NULL,
  overdue_planned_departures integer NOT NULL,
  silent_accommodations integer NOT NULL,
  aggregation_failures integer NOT NULL,
  suspected_duplicates integer,
  suspected_duplicates_reason text NOT NULL,
  fnrh_failures integer,
  fnrh_failures_reason text NOT NULL,
  CONSTRAINT quality_snapshots_window_valid
    CHECK (window_code = 'last_30_days'),
  CONSTRAINT quality_snapshots_counts_valid
    CHECK (
      incomplete_stays >= 0
      AND overdue_planned_departures >= 0
      AND silent_accommodations >= 0
      AND aggregation_failures >= 0
    ),
  CONSTRAINT quality_snapshots_duplicates_valid
    CHECK (
      suspected_duplicates IS NULL
      AND suspected_duplicates_reason = 'pseudonym_not_approved'
    ),
  CONSTRAINT quality_snapshots_fnrh_valid
    CHECK (
      fnrh_failures IS NULL
      AND fnrh_failures_reason = 'phase_not_implemented'
    )
);

CREATE TABLE analytics.quality_coverage (
  quality_snapshot_id uuid NOT NULL
    REFERENCES analytics.quality_snapshots(id) ON DELETE CASCADE,
  category_code text NOT NULL,
  status text NOT NULL,
  ratio numeric(8, 6),
  PRIMARY KEY (quality_snapshot_id, category_code),
  CONSTRAINT quality_coverage_category_valid
    CHECK (category_code ~ '^[a-z][a-z0-9_]*$'),
  CONSTRAINT quality_coverage_status_valid
    CHECK (status IN ('available', 'not_available')),
  CONSTRAINT quality_coverage_ratio_valid
    CHECK (
      (status = 'available' AND ratio BETWEEN 0 AND 1)
      OR
      (status = 'not_available' AND ratio IS NULL)
    )
);

CREATE VIEW public_data.current_presence
WITH (security_barrier = true)
AS
SELECT
  cell.period_selector,
  cell.period_start,
  cell.period_end,
  cell.unit,
  cell.kind,
  cell.status,
  cell.published_value,
  cell.published_lower,
  cell.published_central,
  cell.published_upper,
  publication.as_of_on,
  publication.data_mode,
  publication.privacy_policy_version,
  publication.methodology_version,
  publication.coverage_status,
  publication.coverage_ratio_percent,
  publication.published_at
FROM public_data.current_publication AS current
JOIN public_data.publications AS publication
  ON publication.publication_version = current.publication_version
JOIN public_data.metric_cells AS cell
  ON cell.publication_version = publication.publication_version
WHERE current.singleton
  AND cell.metric_code = 'presence';

CREATE VIEW public_data.current_summary
WITH (security_barrier = true)
AS
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
FROM public_data.current_presence;

CREATE VIEW public_data.current_preferences
WITH (security_barrier = true)
AS
SELECT
  cell.period_selector,
  cell.period_start,
  cell.period_end,
  cell.unit,
  cell.dimension_code,
  cell.category_code,
  cell.status,
  cell.published_value AS share_percent,
  publication.as_of_on,
  publication.data_mode,
  publication.privacy_policy_version,
  publication.methodology_version,
  publication.coverage_status,
  publication.coverage_ratio_percent,
  publication.published_at
FROM public_data.current_publication AS current
JOIN public_data.publications AS publication
  ON publication.publication_version = current.publication_version
JOIN public_data.metric_cells AS cell
  ON cell.publication_version = publication.publication_version
WHERE current.singleton
  AND cell.metric_code = 'first_visit_share';

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

CREATE VIEW analytics.current_quality
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

COMMENT ON TABLE public_data.metric_cells IS
  'Base imutável protegida; nenhum runtime público recebe SELECT direto.';
COMMENT ON VIEW public_data.current_summary IS
  'Reutiliza exatamente as células correntes de presença, sem nova equação.';
COMMENT ON VIEW analytics.current_quality IS
  'Projeção agregada interna, sem IDs de hospedagem, estadia ou pessoa.';

-- Consolidated from 000013_apply_phase4_privileges.up.sql.
REVOKE ALL ON SCHEMA analytics, public_data
FROM PUBLIC, app_runtime, worker_runtime, public_runtime, privacy_officer;

REVOKE ALL ON TABLE
  analytics.presence_days,
  analytics.reconciliation_runs,
  analytics.metric_catalog,
  analytics.metric_mappings,
  analytics.publication_runs,
  analytics.staged_metric_cells,
  analytics.quality_snapshots,
  analytics.quality_coverage,
  analytics.current_quality,
  public_data.publications,
  public_data.metric_cells,
  public_data.current_publication,
  public_data.current_summary,
  public_data.current_presence,
  public_data.current_preferences,
  public_data.current_methodology
FROM PUBLIC, app_runtime, worker_runtime, public_runtime, privacy_officer;

REVOKE ALL ON ALL FUNCTIONS IN SCHEMA analytics, public_data
FROM PUBLIC, app_runtime, worker_runtime, public_runtime, privacy_officer;

REVOKE SELECT ON TABLE
  survey.responses,
  survey.answers,
  survey.consent_decisions
FROM worker_runtime;

GRANT USAGE ON SCHEMA analytics TO app_runtime, worker_runtime;
GRANT USAGE ON SCHEMA public_data TO worker_runtime, public_runtime;

GRANT SELECT ON TABLE analytics.current_quality TO app_runtime;

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE
  analytics.presence_days,
  analytics.reconciliation_runs,
  analytics.metric_catalog,
  analytics.metric_mappings,
  analytics.publication_runs,
  analytics.staged_metric_cells,
  analytics.quality_snapshots,
  analytics.quality_coverage
TO worker_runtime;

GRANT SELECT (
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
TO worker_runtime;

GRANT SELECT (
  id,
  stay_id
) ON TABLE core.visitors
TO worker_runtime;

GRANT SELECT (
  id,
  status,
  category,
  capacity,
  updated_at,
  version
) ON TABLE core.accommodations
TO worker_runtime;

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

GRANT SELECT, INSERT ON TABLE
  public_data.publications,
  public_data.metric_cells,
  public_data.current_publication
TO worker_runtime;

GRANT UPDATE (publication_version)
ON TABLE public_data.current_publication
TO worker_runtime;

REVOKE UPDATE, DELETE ON TABLE
  public_data.publications,
  public_data.metric_cells
FROM worker_runtime;

REVOKE DELETE ON TABLE public_data.current_publication
FROM worker_runtime;

GRANT SELECT ON TABLE
  public_data.current_summary,
  public_data.current_presence,
  public_data.current_preferences,
  public_data.current_methodology
TO public_runtime;

REVOKE ALL ON TABLE
  public_data.publications,
  public_data.metric_cells,
  public_data.current_publication
FROM public_runtime;

REVOKE ALL ON SCHEMA identity, core, survey, analytics, platform
FROM public_runtime;

REVOKE ALL ON SCHEMA analytics, public_data
FROM privacy_officer;

REVOKE ALL ON TABLE
  analytics.presence_days,
  analytics.reconciliation_runs,
  analytics.metric_catalog,
  analytics.metric_mappings,
  analytics.publication_runs,
  analytics.staged_metric_cells,
  analytics.quality_snapshots,
  analytics.quality_coverage,
  analytics.current_quality,
  public_data.publications,
  public_data.metric_cells,
  public_data.current_publication,
  public_data.current_summary,
  public_data.current_presence,
  public_data.current_preferences,
  public_data.current_methodology
FROM privacy_officer;

REVOKE CREATE ON SCHEMA analytics, public_data
FROM app_runtime, worker_runtime, public_runtime, privacy_officer;

-- Consolidated from 000014_harden_public_runtime_session.up.sql.
BEGIN;

DO $$
DECLARE
  database_name text := pg_catalog.current_database();
  public_login record;
BEGIN
  EXECUTE pg_catalog.format(
    'REVOKE CREATE, TEMPORARY ON DATABASE %I FROM PUBLIC, public_runtime',
    database_name
  );

  FOR public_login IN
    WITH RECURSIVE public_members(member_oid) AS (
      SELECT membership.member
      FROM pg_catalog.pg_auth_members AS membership
      WHERE membership.roleid = 'public_runtime'::regrole
      UNION
      SELECT membership.member
      FROM pg_catalog.pg_auth_members AS membership
      JOIN public_members AS parent
        ON parent.member_oid = membership.roleid
    )
    SELECT role.rolname
    FROM pg_catalog.pg_roles AS role
    JOIN public_members AS member ON member.member_oid = role.oid
    WHERE role.rolcanlogin
  LOOP
    EXECUTE pg_catalog.format(
      'REVOKE CREATE, TEMPORARY ON DATABASE %I FROM %I',
      database_name,
      public_login.rolname
    );
    EXECUTE pg_catalog.format(
      'REVOKE ALL ON SCHEMA identity, core, survey, analytics, public_data, platform FROM %I',
      public_login.rolname
    );
    EXECUTE pg_catalog.format(
      'REVOKE ALL ON ALL TABLES IN SCHEMA identity, core, survey, analytics, public_data, platform FROM %I',
      public_login.rolname
    );
    EXECUTE pg_catalog.format(
      'REVOKE ALL ON ALL SEQUENCES IN SCHEMA identity, core, survey, analytics, public_data, platform FROM %I',
      public_login.rolname
    );
    EXECUTE pg_catalog.format(
      'REVOKE ALL ON ALL FUNCTIONS IN SCHEMA identity, core, survey, analytics, public_data, platform FROM %I',
      public_login.rolname
    );
  END LOOP;

  IF pg_catalog.has_database_privilege(
      'public_runtime',
      database_name,
      'CREATE'
    )
    OR pg_catalog.has_database_privilege(
      'public_runtime',
      database_name,
      'TEMPORARY'
    )
  THEN
    RAISE EXCEPTION
      'public runtime database CREATE/TEMPORARY provisioning is unsafe';
  END IF;

  IF EXISTS (
    WITH RECURSIVE public_members(member_oid) AS (
      SELECT membership.member
      FROM pg_catalog.pg_auth_members AS membership
      WHERE membership.roleid = 'public_runtime'::regrole
      UNION
      SELECT membership.member
      FROM pg_catalog.pg_auth_members AS membership
      JOIN public_members AS parent
        ON parent.member_oid = membership.roleid
    )
    SELECT 1
    FROM pg_catalog.pg_roles AS role
    JOIN public_members AS member ON member.member_oid = role.oid
    WHERE role.rolcanlogin
      AND (
        pg_catalog.has_database_privilege(
          role.rolname,
          database_name,
          'CREATE'
        )
        OR pg_catalog.has_database_privilege(
          role.rolname,
          database_name,
          'TEMPORARY'
        )
      )
  ) THEN
    RAISE EXCEPTION
      'public login database CREATE/TEMPORARY provisioning is unsafe';
  END IF;
END
$$;

REVOKE ALL ON SCHEMA identity, core, survey, analytics, public_data, platform
FROM PUBLIC, public_runtime;

REVOKE ALL ON ALL TABLES IN SCHEMA
  identity, core, survey, analytics, public_data, platform
FROM PUBLIC, public_runtime;

REVOKE ALL ON ALL SEQUENCES IN SCHEMA
  identity, core, survey, analytics, public_data, platform
FROM PUBLIC, public_runtime;

REVOKE ALL ON ALL FUNCTIONS IN SCHEMA
  identity, core, survey, analytics, public_data, platform
FROM PUBLIC, public_runtime;

GRANT USAGE ON SCHEMA public_data TO public_runtime;

GRANT SELECT ON TABLE
  public_data.current_summary,
  public_data.current_presence,
  public_data.current_preferences,
  public_data.current_methodology
TO public_runtime;

ALTER DEFAULT PRIVILEGES IN SCHEMA
  identity, core, survey, analytics, public_data, platform
REVOKE ALL ON TABLES FROM PUBLIC, public_runtime;

ALTER DEFAULT PRIVILEGES IN SCHEMA
  identity, core, survey, analytics, public_data, platform
REVOKE ALL ON SEQUENCES FROM PUBLIC, public_runtime;

ALTER DEFAULT PRIVILEGES IN SCHEMA
  identity, core, survey, analytics, public_data, platform
REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC, public_runtime;

COMMIT;

-- Consolidated from 000015_secure_preference_aggregation.up.sql.
BEGIN;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_catalog.pg_roles AS role
    WHERE role.rolname = 'migration_admin'
      AND NOT role.rolcanlogin
  ) THEN
    RAISE EXCEPTION 'migration_admin must exist with NOLOGIN';
  END IF;
END
$$;

GRANT USAGE, CREATE ON SCHEMA analytics TO migration_admin;
GRANT USAGE ON SCHEMA survey, core TO migration_admin;

GRANT SELECT (
  privacy_policy_version,
  metric_code,
  questionnaire_version_id,
  question_id,
  source_value,
  category_code
) ON TABLE analytics.metric_mappings
TO migration_admin;

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
TO migration_admin;

GRANT SELECT (
  response_id,
  questionnaire_version_id,
  question_id,
  structured_value
) ON TABLE survey.answers
TO migration_admin;

GRANT SELECT (
  id,
  stay_id,
  questionnaire_version_id,
  participation,
  submitted_at
) ON TABLE survey.responses
TO migration_admin;

GRANT SELECT (
  response_id,
  questionnaire_version_id,
  purpose_code,
  granted
) ON TABLE survey.consent_decisions
TO migration_admin;

GRANT SELECT (
  id,
  accommodation_id
) ON TABLE core.stays
TO migration_admin;

CREATE FUNCTION analytics.aggregate_eligible_preferences(
  requested_policy_version text,
  period_start timestamptz,
  period_end timestamptz
)
RETURNS SETOF record
LANGUAGE plpgsql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $$
DECLARE
  local_month_start timestamp without time zone;
BEGIN
  IF requested_policy_version IS NULL
    OR pg_catalog.btrim(requested_policy_version) = ''
    OR period_start IS NULL
    OR period_end IS NULL
    OR period_start >= period_end
  THEN
    RAISE EXCEPTION 'invalid preference aggregation window'
      USING ERRCODE = '22023';
  END IF;

  local_month_start := pg_catalog.date_trunc(
    'month',
    period_start AT TIME ZONE 'America/Bahia'
  );
  IF period_start IS DISTINCT FROM (
      local_month_start AT TIME ZONE 'America/Bahia'
    )
    OR period_end IS DISTINCT FROM (
      (local_month_start + interval '1 month')
      AT TIME ZONE 'America/Bahia'
    )
  THEN
    RAISE EXCEPTION 'preference aggregation requires a complete civil month'
      USING ERRCODE = '22023';
  END IF;

  RETURN QUERY
  SELECT
    mapping.privacy_policy_version,
    mapping.metric_code,
    mapping.category_code,
    count(DISTINCT response.id) FILTER (
      WHERE consent.response_id IS NOT NULL
        AND stay.id IS NOT NULL
    )::bigint AS sample_size,
    count(DISTINCT stay.accommodation_id) FILTER (
      WHERE consent.response_id IS NOT NULL
    )::bigint AS accommodation_count,
    max(question.minimum_public_cell)::integer AS minimum_public_cell
  FROM analytics.metric_mappings AS mapping
  JOIN survey.questions AS question
    ON question.id = mapping.question_id
    AND question.questionnaire_version_id = mapping.questionnaire_version_id
  LEFT JOIN survey.answers AS answer
    ON answer.question_id = question.id
    AND answer.questionnaire_version_id = question.questionnaire_version_id
    AND answer.structured_value IS NOT NULL
    AND answer.structured_value #>> '{}' = mapping.source_value
  LEFT JOIN survey.responses AS response
    ON response.id = answer.response_id
    AND response.questionnaire_version_id = answer.questionnaire_version_id
    AND response.participation = 'submitted'
    AND response.submitted_at >= period_start
    AND response.submitted_at < period_end
  LEFT JOIN survey.consent_decisions AS consent
    ON consent.response_id = response.id
    AND consent.questionnaire_version_id = response.questionnaire_version_id
    AND consent.purpose_code = question.purpose_code
    AND consent.granted
  LEFT JOIN core.stays AS stay ON stay.id = response.stay_id
  WHERE mapping.privacy_policy_version = requested_policy_version
    AND question.answer_type NOT IN ('short_text', 'long_text')
    AND question.data_classification NOT IN ('sensitive', 'secret')
    AND question.public_aggregation_allowed
    AND question.analytics_key IS NOT NULL
    AND question.minimum_public_cell IS NOT NULL
  GROUP BY
    mapping.privacy_policy_version,
    mapping.metric_code,
    mapping.category_code
  ORDER BY mapping.metric_code, mapping.category_code;
END
$$;

ALTER FUNCTION analytics.aggregate_eligible_preferences(
  text,
  timestamptz,
  timestamptz
) OWNER TO migration_admin;

REVOKE ALL ON FUNCTION analytics.aggregate_eligible_preferences(
  text,
  timestamptz,
  timestamptz
) FROM PUBLIC, app_runtime, worker_runtime, public_runtime, privacy_officer;

GRANT EXECUTE ON FUNCTION analytics.aggregate_eligible_preferences(
  text,
  timestamptz,
  timestamptz
) TO worker_runtime;

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

REVOKE INSERT, UPDATE, DELETE ON TABLE
  analytics.metric_catalog,
  analytics.metric_mappings
FROM worker_runtime;

COMMIT;

-- Consolidated from 000016_expose_forecast_fallback_bounds.up.sql.
BEGIN;

CREATE OR REPLACE VIEW public_data.current_methodology
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
    AS allowed_preference_periods,
  70::integer AS forecast_fallback_lower_percent,
  130::integer AS forecast_fallback_upper_percent
FROM public_data.current_publication AS current
JOIN public_data.publications AS publication
  ON publication.publication_version = current.publication_version
WHERE current.singleton;

COMMIT;

-- Consolidated from 000017_enforce_presence_selector_kind.up.sql.
BEGIN;

ALTER TABLE public_data.metric_cells
  ADD CONSTRAINT metric_cells_presence_selector_kind_valid
  CHECK (
    metric_code <> 'presence'
    OR (
      period_selector = 'recent_30_days'
      AND kind = 'observed'
    )
    OR (
      period_selector = 'next_30_days'
      AND kind = 'forecast'
    )
  );

COMMIT;

-- Consolidated from 000018_add_bounded_operational_cleanup.up.sql.
BEGIN;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_catalog.pg_roles AS role
    WHERE role.rolname = 'migration_admin'
      AND NOT role.rolcanlogin
  ) THEN
    RAISE EXCEPTION 'migration_admin must exist with NOLOGIN';
  END IF;
END
$$;

GRANT USAGE, CREATE ON SCHEMA platform TO migration_admin;

GRANT SELECT (
  actor_key_hmac,
  method,
  operation_key,
  resource_id,
  idempotency_key_hmac,
  state,
  expires_at
) ON TABLE platform.idempotency_records
TO migration_admin;

GRANT DELETE ON TABLE platform.idempotency_records
TO migration_admin;

GRANT UPDATE (expires_at) ON TABLE platform.idempotency_records
TO migration_admin;

GRANT SELECT (
  scope,
  subject_hmac,
  window_started_at,
  expires_at
) ON TABLE platform.rate_limit_buckets
TO migration_admin;

GRANT DELETE ON TABLE platform.rate_limit_buckets
TO migration_admin;

GRANT UPDATE (expires_at) ON TABLE platform.rate_limit_buckets
TO migration_admin;

CREATE FUNCTION platform.cleanup_expired_operational_records(
  cleanup_cutoff timestamptz,
  cleanup_batch_size integer
)
RETURNS TABLE (
  idempotency_records bigint,
  rate_limit_buckets bigint
)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $$
BEGIN
  IF cleanup_cutoff IS NULL THEN
    RAISE EXCEPTION 'cleanup cutoff is required'
      USING ERRCODE = '22023';
  END IF;

  IF cleanup_batch_size IS NULL
    OR cleanup_batch_size < 1
    OR cleanup_batch_size > 1000
  THEN
    RAISE EXCEPTION 'cleanup batch size must be between 1 and 1000'
      USING ERRCODE = '22023';
  END IF;

  WITH candidates AS (
    SELECT
      record.actor_key_hmac,
      record.method,
      record.operation_key,
      record.resource_id,
      record.idempotency_key_hmac
    FROM platform.idempotency_records AS record
    WHERE record.state = 'completed'
      AND record.expires_at < cleanup_cutoff
    ORDER BY
      record.expires_at,
      record.operation_key,
      record.resource_id,
      record.actor_key_hmac,
      record.idempotency_key_hmac
    LIMIT cleanup_batch_size
    FOR UPDATE SKIP LOCKED
  )
  DELETE FROM platform.idempotency_records AS expired
  USING candidates
  WHERE expired.actor_key_hmac = candidates.actor_key_hmac
    AND expired.method = candidates.method
    AND expired.operation_key = candidates.operation_key
    AND expired.resource_id = candidates.resource_id
    AND expired.idempotency_key_hmac = candidates.idempotency_key_hmac;

  GET DIAGNOSTICS idempotency_records = ROW_COUNT;

  WITH candidates AS (
    SELECT
      bucket.scope,
      bucket.subject_hmac,
      bucket.window_started_at
    FROM platform.rate_limit_buckets AS bucket
    WHERE bucket.expires_at < cleanup_cutoff
    ORDER BY
      bucket.expires_at,
      bucket.scope,
      bucket.window_started_at,
      bucket.subject_hmac
    LIMIT cleanup_batch_size
    FOR UPDATE SKIP LOCKED
  )
  DELETE FROM platform.rate_limit_buckets AS expired
  USING candidates
  WHERE expired.scope = candidates.scope
    AND expired.subject_hmac = candidates.subject_hmac
    AND expired.window_started_at = candidates.window_started_at;

  GET DIAGNOSTICS rate_limit_buckets = ROW_COUNT;
  RETURN NEXT;
END
$$;

ALTER FUNCTION platform.cleanup_expired_operational_records(
  timestamptz,
  integer
) OWNER TO migration_admin;

REVOKE CREATE ON SCHEMA platform FROM migration_admin;

REVOKE ALL ON FUNCTION platform.cleanup_expired_operational_records(
  timestamptz,
  integer
) FROM PUBLIC, app_runtime, worker_runtime, public_runtime, privacy_officer;

GRANT EXECUTE ON FUNCTION platform.cleanup_expired_operational_records(
  timestamptz,
  integer
) TO worker_runtime;

REVOKE DELETE ON TABLE
  platform.idempotency_records,
  platform.rate_limit_buckets
FROM worker_runtime;

REVOKE SELECT (expires_at) ON TABLE platform.idempotency_records
FROM worker_runtime;

REVOKE SELECT (expires_at) ON TABLE platform.rate_limit_buckets
FROM worker_runtime;

COMMIT;

-- Consolidated from 000019_select_single_current_quality_snapshot.up.sql.
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
WHERE snapshot.id = (
  SELECT candidate.id
  FROM analytics.quality_snapshots AS candidate
  WHERE candidate.window_code = snapshot.window_code
  ORDER BY candidate.updated_at DESC, candidate.id DESC
  LIMIT 1
);

COMMENT ON VIEW analytics.current_quality IS
  'Projeção agregada interna do único snapshot mais recente por janela, sem IDs de hospedagem, estadia ou pessoa.';

COMMIT;

-- Consolidated from 000002_accommodation_onboarding.up.sql (second pre-launch wave).
BEGIN;

ALTER TABLE core.accommodations
  ADD COLUMN onboarding_submission_id uuid;

ALTER TABLE core.accommodations
  ADD CONSTRAINT accommodations_category_valid
  CHECK (
    category IN (
      'formal_lodging',
      'seasonal_rental',
      'family_hosting',
      'camping',
      'regularizing',
      'other',
      'unclassified'
    )
  );

CREATE UNIQUE INDEX accommodations_onboarding_submission_idx
  ON core.accommodations (organization_id, onboarding_submission_id)
  WHERE onboarding_submission_id IS NOT NULL;

COMMENT ON COLUMN core.accommodations.onboarding_submission_id IS
  'UUIDv7 de repetição do onboarding local; não é documento, tenant externo ou identificador FNRH.';
COMMENT ON COLUMN core.accommodations.cadastur_id IS
  'Metadado opcional provisionado por fonte confiável; nunca prova ou habilita integração FNRH.';

REVOKE SELECT ON TABLE core.organizations
FROM app_runtime, worker_runtime;

GRANT SELECT (
  id,
  name,
  legal_name,
  created_at,
  updated_at,
  version
) ON TABLE core.organizations
TO app_runtime, worker_runtime;

GRANT INSERT (
  id,
  name
) ON TABLE core.organizations
TO app_runtime;

GRANT INSERT (
  id,
  organization_id,
  name,
  category,
  status,
  capacity,
  onboarding_submission_id
) ON TABLE core.accommodations
TO app_runtime;

REVOKE UPDATE (cadastur_id) ON TABLE core.accommodations
FROM app_runtime;

COMMIT;

-- Consolidated from 000003_local_password_auth.up.sql (second pre-launch wave).
BEGIN;

CREATE SCHEMA auth;

REVOKE ALL ON SCHEMA auth FROM PUBLIC;

COMMENT ON SCHEMA auth IS
  'Credencial local de operador e sessão opaca. Não guarda documento fiscal, '
  'dado de visitante ou identificador FNRH.';

CREATE TABLE auth.accounts (
  id uuid PRIMARY KEY,
  email text NOT NULL,
  display_name text NOT NULL,
  password_hash text NOT NULL,
  scopes text[] NOT NULL,
  status text NOT NULL DEFAULT 'active',
  failed_attempts integer NOT NULL DEFAULT 0,
  locked_until timestamptz,
  password_changed_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT accounts_email_normalized CHECK (email = btrim(lower(email))),
  CONSTRAINT accounts_email_shaped CHECK (email ~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$'),
  CONSTRAINT accounts_display_name_not_blank CHECK (btrim(display_name) <> ''),
  CONSTRAINT accounts_password_hash_algorithm CHECK (password_hash LIKE '$argon2id$%'),
  CONSTRAINT accounts_scopes_bounded CHECK (
    cardinality(scopes) BETWEEN 1 AND 32
  ),
  CONSTRAINT accounts_status_valid CHECK (status IN ('active', 'disabled')),
  CONSTRAINT accounts_failed_attempts_bounded CHECK (
    failed_attempts BETWEEN 0 AND 1000000
  )
);

CREATE UNIQUE INDEX accounts_email_idx ON auth.accounts (email);

COMMENT ON TABLE auth.accounts IS
  'Conta local de operador autenticada por e-mail e senha; substitui a exigência '
  'de CNPJ ou chave federal para participar do observatório local.';
COMMENT ON COLUMN auth.accounts.password_hash IS
  'Codificação Argon2id completa, com sal por conta. Nunca é lida por '
  'worker_runtime, public_runtime ou privacy_officer.';
COMMENT ON COLUMN auth.accounts.scopes IS
  'Escopos concedidos ao principal derivado desta conta; espelham os escopos OIDC.';

CREATE TABLE auth.sessions (
  id uuid PRIMARY KEY,
  account_id uuid NOT NULL REFERENCES auth.accounts(id),
  token_hash bytea NOT NULL,
  issued_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  idle_expires_at timestamptz NOT NULL,
  absolute_expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  CONSTRAINT sessions_token_hash_sha256 CHECK (octet_length(token_hash) = 32),
  CONSTRAINT sessions_expiry_ordered CHECK (idle_expires_at <= absolute_expires_at),
  CONSTRAINT sessions_absolute_after_issue CHECK (absolute_expires_at > issued_at)
);

CREATE UNIQUE INDEX sessions_token_hash_idx ON auth.sessions (token_hash);
CREATE INDEX sessions_account_idx ON auth.sessions (account_id)
  WHERE revoked_at IS NULL;
CREATE INDEX sessions_absolute_expires_idx ON auth.sessions (absolute_expires_at);

COMMENT ON TABLE auth.sessions IS
  'Sessão opaca por SHA-256 do token; o token cru nunca é persistido.';
COMMENT ON COLUMN auth.sessions.token_hash IS
  'SHA-256 do token de sessão emitido ao cliente. Comparação por igualdade '
  'sobre o digest, nunca sobre o segredo.';

GRANT USAGE ON SCHEMA auth TO app_runtime;

GRANT SELECT (
  id,
  email,
  display_name,
  password_hash,
  scopes,
  status,
  failed_attempts,
  locked_until,
  password_changed_at
) ON TABLE auth.accounts TO app_runtime;

GRANT UPDATE (
  failed_attempts,
  locked_until,
  updated_at
) ON TABLE auth.accounts TO app_runtime;

GRANT SELECT (
  id,
  account_id,
  token_hash,
  issued_at,
  last_seen_at,
  idle_expires_at,
  absolute_expires_at,
  revoked_at
) ON TABLE auth.sessions TO app_runtime;

GRANT INSERT (
  id,
  account_id,
  token_hash,
  idle_expires_at,
  absolute_expires_at
) ON TABLE auth.sessions TO app_runtime;

GRANT UPDATE (
  last_seen_at,
  idle_expires_at,
  revoked_at
) ON TABLE auth.sessions TO app_runtime;

GRANT DELETE ON TABLE auth.sessions TO app_runtime;

REVOKE ALL ON SCHEMA auth FROM worker_runtime, public_runtime, privacy_officer;
REVOKE ALL ON ALL TABLES IN SCHEMA auth
  FROM worker_runtime, public_runtime, privacy_officer;

ALTER DEFAULT PRIVILEGES IN SCHEMA auth REVOKE ALL ON TABLES FROM PUBLIC;

COMMIT;

-- Consolidated from 000004_admin_seed_password_rotation.up.sql (second pre-launch wave).
BEGIN;

ALTER TABLE auth.accounts
  ADD COLUMN password_must_change boolean NOT NULL DEFAULT false;

COMMENT ON COLUMN auth.accounts.password_must_change IS
  'Marca a credencial semeada como provisória. Enquanto for verdadeira a sessão '
  'só alcança troca de senha, logout e leitura da própria sessão, de modo que a '
  'senha de bootstrap não sobrevive ao primeiro acesso.';

-- A rotação é executada pelo processo da API em nome do titular da sessão, e é
-- o único caminho que escreve a credencial fora do provisionamento.
GRANT SELECT (password_must_change) ON TABLE auth.accounts TO app_runtime;

GRANT UPDATE (
  password_hash,
  password_changed_at,
  password_must_change
) ON TABLE auth.accounts TO app_runtime;

REVOKE ALL ON TABLE auth.accounts
  FROM worker_runtime, public_runtime, privacy_officer;

COMMIT;
BEGIN;

-- ADR-038: the responsible party is identified by exactly one CPF or CNPJ,
-- persisted only as a keyed HMAC. The plaintext is never stored, so this column
-- can answer "already registered" and nothing else.

ALTER TABLE core.organizations
  ADD CONSTRAINT organizations_document_hmac_length
  CHECK (document_hmac IS NULL OR octet_length(document_hmac) = 32);

-- Partial: organizations provisioned before ADR-038, and the fictitious seed
-- tenants, carry no document and must not collide with each other.
CREATE UNIQUE INDEX organizations_document_hmac_idx
  ON core.organizations (document_hmac)
  WHERE document_hmac IS NOT NULL;

COMMENT ON COLUMN core.organizations.document_hmac IS
  'HMAC-SHA256 do CPF ou CNPJ do responsável, com chave rotacionável e separada '
  'da chave de dados pessoais. O valor em claro nunca é persistido nem '
  'recuperável: serve apenas para recusar documento já cadastrado. Ver ADR-038.';

COMMIT;
-- Fase 7 — autoatendimento e aprovação.
--
-- ADR-039: convite reutilizável por acomodação, purpose no MAC, token no
--          fragmento da URL.
-- ADR-040: autocadastro generalizado, proveniência de estadia e aprovação do
--          estabelecimento. O canal aberto NÃO coleta identidade; a tabela
--          identity.visitor_identities permanece inexistente por decisão da
--          ADR-020 e não é criada aqui.
-- ADR-041: ativação de conta de acomodação por capability de uso único.
--
-- A baseline 000001 está congelada pela ADR-032 e não é reaberta. Esta é a
-- terceira migração da cadeia: 000002 pertence à onda da ADR-038.
--
-- Cada bloco lógico tem sua própria transação, no estilo da baseline. Todo par
-- DROP CONSTRAINT / ADD CONSTRAINT fica na mesma transação, de modo que não
-- existe janela sem invariante.

-- 1. core.invites — segundo purpose, alvo por acomodação e uso ilimitado.
BEGIN;

ALTER TABLE core.invites
  ALTER COLUMN stay_id DROP NOT NULL,
  ALTER COLUMN max_uses DROP NOT NULL,
  ADD COLUMN accommodation_id uuid
    REFERENCES core.accommodations(id) ON DELETE RESTRICT;

-- O DEFAULT 1 é preservado deliberadamente: CreateStayInvite não lista a
-- coluna max_uses e depende dele. Removê-lo quebraria silenciosamente todo o
-- convite de estadia da Fase 2.

ALTER TABLE core.invites DROP CONSTRAINT invites_purpose_valid;
ALTER TABLE core.invites
  ADD CONSTRAINT invites_purpose_valid
    CHECK (purpose IN ('stay_group_submission', 'accommodation_self_registration'));

-- Escrito com o teste de nulidade ANTES da comparação: assim o termo
-- use_count <= max_uses nunca é avaliado com NULL e o CHECK jamais resulta em
-- UNKNOWN (que passaria silenciosamente). O invariante segue estrito sempre
-- que max_uses estiver presente.
ALTER TABLE core.invites DROP CONSTRAINT invites_usage_valid;
ALTER TABLE core.invites
  ADD CONSTRAINT invites_usage_valid
    CHECK (
      use_count >= 0
      AND (max_uses IS NULL OR (max_uses > 0 AND use_count <= max_uses))
    );

-- Exclusivo, não permissivo: um convite de autocadastro com stay_id preenchido
-- seria um caminho para injetar visitantes em estadia de outra acomodação.
-- Reimpõe max_uses NOT NULL no purpose de estadia, de modo que "ilimitado"
-- exista somente no purpose novo.
ALTER TABLE core.invites
  ADD CONSTRAINT invites_target_valid
    CHECK (
      (
        purpose = 'stay_group_submission'
        AND stay_id IS NOT NULL
        AND accommodation_id IS NULL
        AND max_uses IS NOT NULL
      )
      OR
      (
        purpose = 'accommodation_self_registration'
        AND stay_id IS NULL
        AND accommodation_id IS NOT NULL
      )
    );

CREATE INDEX invites_accommodation_active_idx
  ON core.invites (accommodation_id, expires_at)
  WHERE revoked_at IS NULL AND accommodation_id IS NOT NULL;

-- No máximo um cartaz ativo por acomodação: a rotação revoga o anterior na
-- mesma transação e este índice torna a corrida impossível.
CREATE UNIQUE INDEX invites_accommodation_single_active_idx
  ON core.invites (accommodation_id)
  WHERE revoked_at IS NULL
    AND purpose = 'accommodation_self_registration';

COMMENT ON COLUMN core.invites.accommodation_id IS
  'Alvo do convite reutilizável por acomodação (ADR-039). Nulo no convite de '
  'estadia da Fase 2; obrigatório no purpose accommodation_self_registration.';
COMMENT ON COLUMN core.invites.max_uses IS
  'Nulo significa uso ilimitado, permitido somente no convite por acomodação. '
  'O DEFAULT 1 é preservado porque CreateStayInvite não lista a coluna.';

COMMIT;

-- 2. core.stays — proveniência e aprovação sem novo valor em core.stay_status.
BEGIN;

ALTER TABLE core.stays
  ALTER COLUMN created_by_membership_id DROP NOT NULL,
  ADD COLUMN provenance text NOT NULL DEFAULT 'assisted',
  ADD COLUMN approval_state text,
  ADD COLUMN approved_at timestamptz,
  ADD COLUMN approval_decided_by_membership_id uuid
    REFERENCES core.memberships(id),
  ADD COLUMN approval_reason_code text,
  ADD COLUMN approval_expires_at timestamptz;

ALTER TABLE core.stays
  ADD CONSTRAINT stays_provenance_valid
    CHECK (provenance IN ('assisted', 'self_service'));

-- created_by_membership_id deixa de ser NOT NULL na coluna, mas a obrigação
-- volta condicionada a provenance = 'assisted'. Como provenance tem
-- DEFAULT 'assisted', toda linha já existente e toda escrita da Fase 2 que não
-- menciona a coluna continuam proibidas de ter autora nula. O caso assistido
-- não afrouxa. Não existe membership sintética de sistema: fabricar um ator
-- destruiria a distinção entre "um operador criou" e "ninguém criou".
ALTER TABLE core.stays
  ADD CONSTRAINT stays_provenance_author_valid
    CHECK (
      (
        provenance = 'assisted'
        AND created_by_membership_id IS NOT NULL
        AND approval_state IS NULL
        AND approved_at IS NULL
        AND approval_decided_by_membership_id IS NULL
        AND approval_reason_code IS NULL
        AND approval_expires_at IS NULL
      )
      OR
      (
        provenance = 'self_service'
        AND created_by_membership_id IS NULL
        AND approval_state IS NOT NULL
      )
    );

ALTER TABLE core.stays
  ADD CONSTRAINT stays_approval_state_valid
    CHECK (
      approval_state IS NULL
      OR approval_state IN ('pending', 'approved', 'rejected', 'expired')
    );

ALTER TABLE core.stays
  ADD CONSTRAINT stays_approval_fields_valid
    CHECK (
      approval_state IS NULL
      OR (
        approval_state = 'pending'
        AND approved_at IS NULL
        AND approval_decided_by_membership_id IS NULL
        AND approval_reason_code IS NULL
        AND approval_expires_at IS NOT NULL
      )
      OR (
        approval_state = 'approved'
        AND approved_at IS NOT NULL
        AND approval_decided_by_membership_id IS NOT NULL
        AND approval_reason_code IS NULL
        AND approval_expires_at IS NULL
      )
      OR (
        approval_state = 'rejected'
        AND approved_at IS NULL
        AND approval_decided_by_membership_id IS NOT NULL
        AND approval_reason_code IS NOT NULL
        AND approval_expires_at IS NULL
      )
      OR (
        -- A expiração é do sistema: não há membership decisora.
        approval_state = 'expired'
        AND approved_at IS NULL
        AND approval_decided_by_membership_id IS NULL
        AND approval_reason_code IS NULL
        AND approval_expires_at IS NULL
      )
    );

-- Lista fechada, no precedente de questionnaire_change_reason_valid. Texto
-- livre em platform.audit_events, que é append-only sem UPDATE nem DELETE,
-- viraria dado pessoal permanente exatamente no caminho desenhado para
-- eliminar dado pessoal.
ALTER TABLE core.stays
  ADD CONSTRAINT stays_approval_reason_valid
    CHECK (
      approval_reason_code IS NULL
      OR approval_reason_code IN (
        'identity_not_verified', 'not_a_guest', 'duplicate',
        'data_incorrect', 'other'
      )
    );

CREATE INDEX stays_pending_approval_idx
  ON core.stays (accommodation_id, approval_expires_at)
  WHERE approval_state = 'pending';

COMMENT ON COLUMN core.stays.provenance IS
  'assisted: criada por operador identificado (Fase 2). self_service: criada '
  'pelo canal aberto do cartaz, sem membership autora (ADR-040).';
COMMENT ON COLUMN core.stays.approval_state IS
  'Nulo para estadia assistida. A espera de aprovação é proveniência mais '
  'carimbo, nunca um valor novo em core.stay_status.';
COMMENT ON COLUMN core.stays.approval_expires_at IS
  'Prazo da pendência. A varredura de expiração elimina os dados generalizados '
  'do autocadastro, para que a retenção não sobreviva por inação.';

COMMIT;

-- 3. core.group_submissions — terceiro canal de coleta.
BEGIN;

ALTER TABLE core.group_submissions
  DROP CONSTRAINT group_submissions_channel_valid;
ALTER TABLE core.group_submissions
  ADD CONSTRAINT group_submissions_channel_valid
    CHECK (collection_channel IN ('assisted', 'invite', 'self_service'));

-- O nome group_submissions_assisted_actor_valid é preservado de propósito:
-- renomear obrigaria a caçar asserções em deploy/scripts/test-migrations.sh e
-- em phase2_postgres_test.go, e o ganho semântico não paga o risco de
-- regressão silenciosa. Exatamente um ramo casa por valor de
-- collection_channel, e nenhum canal sem operador aceita ator preenchido.
ALTER TABLE core.group_submissions
  DROP CONSTRAINT group_submissions_assisted_actor_valid;
ALTER TABLE core.group_submissions
  ADD CONSTRAINT group_submissions_assisted_actor_valid
    CHECK (
      (collection_channel = 'assisted'     AND submitted_by_membership_id IS NOT NULL)
      OR
      (collection_channel = 'invite'       AND submitted_by_membership_id IS NULL)
      OR
      (collection_channel = 'self_service' AND submitted_by_membership_id IS NULL)
    );

COMMIT;

-- 4. auth.accounts — conta pendente sem violar o CHECK de argon2id.
BEGIN;

ALTER TABLE auth.accounts ALTER COLUMN password_hash DROP NOT NULL;

ALTER TABLE auth.accounts DROP CONSTRAINT accounts_status_valid;
ALTER TABLE auth.accounts
  ADD CONSTRAINT accounts_status_valid
    CHECK (status IN ('active', 'disabled', 'pending_activation'));

-- Condicional a password_hash IS NOT NULL, nunca ao status, para que o
-- invariante do algoritmo continue valendo mesmo se um status novo aparecer.
ALTER TABLE auth.accounts DROP CONSTRAINT accounts_password_hash_algorithm;
ALTER TABLE auth.accounts
  ADD CONSTRAINT accounts_password_hash_algorithm
    CHECK (password_hash IS NULL OR password_hash LIKE '$argon2id$%');

-- A segunda metade, sem a qual relaxar a primeira abriria conta ativa sem
-- credencial: a ausência de hash fica amarrada exclusivamente a
-- pending_activation.
ALTER TABLE auth.accounts
  ADD CONSTRAINT accounts_credential_state_valid
    CHECK (
      (
        status = 'pending_activation'
        AND password_hash IS NULL
        AND password_must_change = false
        AND failed_attempts = 0
        AND locked_until IS NULL
      )
      OR
      (
        status IN ('active', 'disabled')
        AND password_hash IS NOT NULL
      )
    );

GRANT INSERT (id, email, display_name, scopes, status)
  ON TABLE auth.accounts TO app_runtime;
GRANT UPDATE (status) ON TABLE auth.accounts TO app_runtime;

REVOKE ALL ON TABLE auth.accounts
  FROM worker_runtime, public_runtime, privacy_officer;

COMMENT ON COLUMN auth.accounts.status IS
  'pending_activation: conta criada pela emissão de capability de ativação, '
  'ainda sem credencial. Não autentica (ADR-041).';

COMMIT;

-- 5. auth.activation_capabilities — espelha survey.capabilities de propósito.
BEGIN;

CREATE TABLE auth.activation_capabilities (
  id uuid PRIMARY KEY,
  account_id uuid NOT NULL REFERENCES auth.accounts(id) ON DELETE RESTRICT,
  accommodation_id uuid NOT NULL
    REFERENCES core.accommodations(id) ON DELETE RESTRICT,
  token_hmac bytea NOT NULL UNIQUE,
  token_key_version text NOT NULL,
  purpose text NOT NULL,
  expires_at timestamptz NOT NULL,
  consumed_at timestamptz,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT activation_key_version_not_blank
    CHECK (btrim(token_key_version) <> ''),
  CONSTRAINT activation_purpose_valid
    CHECK (purpose = 'accommodation_activation'),
  CONSTRAINT activation_expiry_valid
    CHECK (expires_at > created_at),
  CONSTRAINT activation_token_hmac_sha256
    CHECK (octet_length(token_hmac) = 32),
  CONSTRAINT activation_terminal_state_valid
    CHECK (consumed_at IS NULL OR revoked_at IS NULL)
);

-- No máximo uma capability aberta por conta: a reemissão revoga a anterior na
-- mesma transação e este índice torna a corrida impossível.
CREATE UNIQUE INDEX activation_capabilities_open_idx
  ON auth.activation_capabilities (account_id)
  WHERE consumed_at IS NULL AND revoked_at IS NULL;

GRANT SELECT, INSERT ON TABLE auth.activation_capabilities TO app_runtime;
GRANT UPDATE (consumed_at, revoked_at)
  ON TABLE auth.activation_capabilities TO app_runtime;
REVOKE ALL ON TABLE auth.activation_capabilities
  FROM worker_runtime, public_runtime, privacy_officer;

COMMENT ON TABLE auth.activation_capabilities IS
  'Capability de uso único que transfere o acesso da acomodação a quem a opera '
  '(ADR-041). Armazenada somente por HMAC e nunca reconstruível sem o keyring.';

COMMIT;

-- 6. platform.rate_limit_buckets — escopos próprios do canal aberto.
BEGIN;

-- Escopos separados são necessários: um cartaz público tem perfil de tráfego
-- completamente diferente de um convite de estadia, e reusar invite_submit
-- faria o autocadastro consumir o orçamento do fluxo nominal.
ALTER TABLE platform.rate_limit_buckets DROP CONSTRAINT rate_limit_scope_valid;
ALTER TABLE platform.rate_limit_buckets
  ADD CONSTRAINT rate_limit_scope_valid
    CHECK (scope IN (
      'invite_context', 'invite_submit', 'survey_submit',
      'accommodation_invite_context', 'accommodation_invite_submit',
      'activation_context', 'activation_submit'
    ));

COMMIT;

-- 7. platform.proof_of_work_spends — livro de nonces do proof-of-work.
BEGIN;

-- Sem livro de nonces a mesma solução é reenviada dentro do TTL e o controle
-- vale zero. A tabela guarda apenas o HMAC do desafio e o prazo: sem sujeito,
-- sem IP, sem token e sem qualquer referência ao titular.
CREATE TABLE platform.proof_of_work_spends (
  challenge_hmac bytea PRIMARY KEY,
  key_version text NOT NULL,
  expires_at timestamptz NOT NULL,
  CONSTRAINT proof_of_work_challenge_hmac_sha256
    CHECK (octet_length(challenge_hmac) = 32),
  CONSTRAINT proof_of_work_key_version_not_blank
    CHECK (btrim(key_version) <> '')
);

CREATE INDEX proof_of_work_spends_expiry_idx
  ON platform.proof_of_work_spends (expires_at);

GRANT SELECT, INSERT ON TABLE platform.proof_of_work_spends TO app_runtime;
REVOKE ALL ON TABLE platform.proof_of_work_spends
  FROM worker_runtime, public_runtime, privacy_officer;

GRANT SELECT (challenge_hmac, expires_at)
  ON TABLE platform.proof_of_work_spends TO migration_admin;
GRANT DELETE ON TABLE platform.proof_of_work_spends TO migration_admin;
-- SELECT ... FOR UPDATE exige privilégio de UPDATE, ainda que a função só
-- delete. Sem este grant a varredura aborta no terceiro bloco e leva junto a
-- purga de idempotency_records e rate_limit_buckets, que rodam antes na mesma
-- função: uma tabela nova desta fase desligaria a retenção operacional de todas
-- as fases. Espelha o que a baseline concede nas outras duas tabelas.
GRANT UPDATE (expires_at)
  ON TABLE platform.proof_of_work_spends TO migration_admin;

COMMENT ON TABLE platform.proof_of_work_spends IS
  'Gasto de nonce do proof-of-work. Guarda somente HMAC do desafio e prazo. '
  'Nenhuma coluna deriva de IP, token ou titular; INSERT com conflito na chave '
  'primária é replay e recebe a mesma resposta de desafio inválido.';

COMMIT;

-- 8. platform.cleanup_expired_operational_records — passa a purgar o livro de
--    nonces. A mudança do tipo de retorno exige DROP e CREATE; owner,
--    search_path, SECURITY DEFINER e ACLs são reafirmados na sequência.
BEGIN;

-- A 000001 revoga CREATE em platform de migration_admin logo após criar a
-- função. Transferir o dono de volta exige que o novo dono tenha CREATE no
-- schema, então o privilégio é reaberto pelo tempo exato do ALTER e devolvido
-- na mesma transação, como faz a baseline.
GRANT CREATE ON SCHEMA platform TO migration_admin;

DROP FUNCTION platform.cleanup_expired_operational_records(
  timestamptz,
  integer
);

CREATE FUNCTION platform.cleanup_expired_operational_records(
  cleanup_cutoff timestamptz,
  cleanup_batch_size integer
)
RETURNS TABLE (
  idempotency_records bigint,
  rate_limit_buckets bigint,
  proof_of_work_spends bigint
)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $$
BEGIN
  IF cleanup_cutoff IS NULL THEN
    RAISE EXCEPTION 'cleanup cutoff is required'
      USING ERRCODE = '22023';
  END IF;

  IF cleanup_batch_size IS NULL
    OR cleanup_batch_size < 1
    OR cleanup_batch_size > 1000
  THEN
    RAISE EXCEPTION 'cleanup batch size must be between 1 and 1000'
      USING ERRCODE = '22023';
  END IF;

  WITH candidates AS (
    SELECT
      record.actor_key_hmac,
      record.method,
      record.operation_key,
      record.resource_id,
      record.idempotency_key_hmac
    FROM platform.idempotency_records AS record
    WHERE record.state = 'completed'
      AND record.expires_at < cleanup_cutoff
    ORDER BY
      record.expires_at,
      record.operation_key,
      record.resource_id,
      record.actor_key_hmac,
      record.idempotency_key_hmac
    LIMIT cleanup_batch_size
    FOR UPDATE SKIP LOCKED
  )
  DELETE FROM platform.idempotency_records AS expired
  USING candidates
  WHERE expired.actor_key_hmac = candidates.actor_key_hmac
    AND expired.method = candidates.method
    AND expired.operation_key = candidates.operation_key
    AND expired.resource_id = candidates.resource_id
    AND expired.idempotency_key_hmac = candidates.idempotency_key_hmac;

  GET DIAGNOSTICS idempotency_records = ROW_COUNT;

  WITH candidates AS (
    SELECT
      bucket.scope,
      bucket.subject_hmac,
      bucket.window_started_at
    FROM platform.rate_limit_buckets AS bucket
    WHERE bucket.expires_at < cleanup_cutoff
    ORDER BY
      bucket.expires_at,
      bucket.scope,
      bucket.window_started_at,
      bucket.subject_hmac
    LIMIT cleanup_batch_size
    FOR UPDATE SKIP LOCKED
  )
  DELETE FROM platform.rate_limit_buckets AS expired
  USING candidates
  WHERE expired.scope = candidates.scope
    AND expired.subject_hmac = candidates.subject_hmac
    AND expired.window_started_at = candidates.window_started_at;

  GET DIAGNOSTICS rate_limit_buckets = ROW_COUNT;

  WITH candidates AS (
    SELECT spend.challenge_hmac
    FROM platform.proof_of_work_spends AS spend
    WHERE spend.expires_at < cleanup_cutoff
    ORDER BY spend.expires_at, spend.challenge_hmac
    LIMIT cleanup_batch_size
    FOR UPDATE SKIP LOCKED
  )
  DELETE FROM platform.proof_of_work_spends AS expired
  USING candidates
  WHERE expired.challenge_hmac = candidates.challenge_hmac;

  GET DIAGNOSTICS proof_of_work_spends = ROW_COUNT;
  RETURN NEXT;
END
$$;

ALTER FUNCTION platform.cleanup_expired_operational_records(
  timestamptz,
  integer
) OWNER TO migration_admin;

REVOKE CREATE ON SCHEMA platform FROM migration_admin;

REVOKE ALL ON FUNCTION platform.cleanup_expired_operational_records(
  timestamptz,
  integer
) FROM PUBLIC, app_runtime, worker_runtime, public_runtime, privacy_officer;

GRANT EXECUTE ON FUNCTION platform.cleanup_expired_operational_records(
  timestamptz,
  integer
) TO worker_runtime;

COMMIT;

-- 9. worker_runtime — varredura de expiração e filtro de presença.
BEGIN;

-- O ponto 2 do filtro de presença (presenceEligible) precisa do estado de
-- aprovação projetado pelas queries de reconciliação, que rodam como worker.
GRANT SELECT (
  provenance,
  approval_state,
  approval_expires_at,
  cancelled_at,
  cancellation_reason_code,
  created_at
) ON TABLE core.stays
TO worker_runtime;

-- A varredura de expiração carimba a estadia e elimina os visitantes
-- generalizados: eliminar somente na rejeição permitiria retenção indefinida
-- por inação (ADR-040).
GRANT UPDATE (
  status,
  approval_state,
  approval_expires_at,
  cancelled_at,
  cancellation_reason_code,
  updated_at,
  version
) ON TABLE core.stays
TO worker_runtime;

GRANT DELETE ON TABLE core.visitors TO worker_runtime;

-- A rejeição executa a mesma eliminação, pelo caminho da aplicação.
GRANT DELETE ON TABLE core.visitors TO app_runtime;

COMMIT;

-- 10. analytics.aggregate_eligible_preferences — predicado de aprovação.
BEGIN;

-- Sem esta coluna a função SECURITY DEFINER não enxerga o estado de aprovação.
GRANT SELECT (approval_state) ON TABLE core.stays TO migration_admin;

-- Corpo idêntico ao da baseline, com um único acréscimo no WHERE final. O
-- predicado precisa ir no WHERE e não na condição do LEFT JOIN: no join a
-- linha voltaria com stay.id NULL e ainda assim seria contada pelo
-- FILTER (WHERE ... stay.id IS NOT NULL) do accommodation_count.
CREATE OR REPLACE FUNCTION analytics.aggregate_eligible_preferences(
  requested_policy_version text,
  period_start timestamptz,
  period_end timestamptz
)
RETURNS SETOF record
LANGUAGE plpgsql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $$
DECLARE
  local_month_start timestamp without time zone;
BEGIN
  IF requested_policy_version IS NULL
    OR pg_catalog.btrim(requested_policy_version) = ''
    OR period_start IS NULL
    OR period_end IS NULL
    OR period_start >= period_end
  THEN
    RAISE EXCEPTION 'invalid preference aggregation window'
      USING ERRCODE = '22023';
  END IF;

  local_month_start := pg_catalog.date_trunc(
    'month',
    period_start AT TIME ZONE 'America/Bahia'
  );
  IF period_start IS DISTINCT FROM (
      local_month_start AT TIME ZONE 'America/Bahia'
    )
    OR period_end IS DISTINCT FROM (
      (local_month_start + interval '1 month')
      AT TIME ZONE 'America/Bahia'
    )
  THEN
    RAISE EXCEPTION 'preference aggregation requires a complete civil month'
      USING ERRCODE = '22023';
  END IF;

  RETURN QUERY
  SELECT
    mapping.privacy_policy_version,
    mapping.metric_code,
    mapping.category_code,
    count(DISTINCT response.id) FILTER (
      WHERE consent.response_id IS NOT NULL
        AND stay.id IS NOT NULL
    )::bigint AS sample_size,
    count(DISTINCT stay.accommodation_id) FILTER (
      WHERE consent.response_id IS NOT NULL
    )::bigint AS accommodation_count,
    max(question.minimum_public_cell)::integer AS minimum_public_cell
  FROM analytics.metric_mappings AS mapping
  JOIN survey.questions AS question
    ON question.id = mapping.question_id
    AND question.questionnaire_version_id = mapping.questionnaire_version_id
  LEFT JOIN survey.answers AS answer
    ON answer.question_id = question.id
    AND answer.questionnaire_version_id = question.questionnaire_version_id
    AND answer.structured_value IS NOT NULL
    AND answer.structured_value #>> '{}' = mapping.source_value
  LEFT JOIN survey.responses AS response
    ON response.id = answer.response_id
    AND response.questionnaire_version_id = answer.questionnaire_version_id
    AND response.participation = 'submitted'
    AND response.submitted_at >= period_start
    AND response.submitted_at < period_end
  LEFT JOIN survey.consent_decisions AS consent
    ON consent.response_id = response.id
    AND consent.questionnaire_version_id = response.questionnaire_version_id
    AND consent.purpose_code = question.purpose_code
    AND consent.granted
  LEFT JOIN core.stays AS stay ON stay.id = response.stay_id
  WHERE mapping.privacy_policy_version = requested_policy_version
    AND question.answer_type NOT IN ('short_text', 'long_text')
    AND question.data_classification NOT IN ('sensitive', 'secret')
    AND question.public_aggregation_allowed
    AND question.analytics_key IS NOT NULL
    AND question.minimum_public_cell IS NOT NULL
    AND (stay.approval_state IS NULL OR stay.approval_state = 'approved')
  GROUP BY
    mapping.privacy_policy_version,
    mapping.metric_code,
    mapping.category_code
  ORDER BY mapping.metric_code, mapping.category_code;
END
$$;

-- CREATE OR REPLACE preserva owner e ACL, mas a reafirmação é exigida pela
-- ADR-032 e conferida pelo teste de migrations: sem ela o dump pode divergir
-- do esperado sem que nada falhe em tempo de execução.
ALTER FUNCTION analytics.aggregate_eligible_preferences(
  text,
  timestamptz,
  timestamptz
) OWNER TO migration_admin;

REVOKE ALL ON FUNCTION analytics.aggregate_eligible_preferences(
  text,
  timestamptz,
  timestamptz
) FROM PUBLIC, app_runtime, worker_runtime, public_runtime, privacy_officer;

GRANT EXECUTE ON FUNCTION analytics.aggregate_eligible_preferences(
  text,
  timestamptz,
  timestamptz
) TO worker_runtime;

COMMIT;
BEGIN;

-- O código passou a ser nomeado por funcionalidade, não pela fase que a
-- entregou. O valor 'phase_not_implemented' era o último resquício do
-- vocabulário de fase numa resposta pública da API: quem lê
-- UnavailableQualityCount.reason_code não tem como saber que fase é essa.
--
-- A constraint fixa um único valor, então a ordem é obrigatória: soltar a
-- constraint, reescrever as linhas existentes e só então refazê-la. Recriar a
-- constraint antes do UPDATE recusaria as próprias linhas que já estão na
-- tabela.

ALTER TABLE analytics.quality_snapshots
  DROP CONSTRAINT IF EXISTS quality_snapshots_fnrh_valid;

UPDATE analytics.quality_snapshots
  SET fnrh_failures_reason = 'not_implemented'
  WHERE fnrh_failures_reason = 'phase_not_implemented';

ALTER TABLE analytics.quality_snapshots
  ADD CONSTRAINT quality_snapshots_fnrh_valid
  CHECK (
    fnrh_failures IS NULL
    AND fnrh_failures_reason = 'not_implemented'
  );

COMMIT;
BEGIN;

-- Fecha a classe de incidente aberta em CreateActivationAccount
-- (queries/auth.sql:140-146): app_runtime tinha INSERT sem nenhum SELECT em
-- platform.audit_events e platform.outbox_events, então a PRIMEIRA query com
-- RETURNING sobre qualquer uma delas derruba a transação inteira com
-- "permission denied for table", no próprio INSERT. O incidente já mordeu
-- auth.accounts; aqui é prevenção, não reação — nenhuma query nova passa a
-- usar RETURNING nesta migração.
--
-- O escopo é o menor que evita a classe: só a coluna `id`. É a única coluna
-- que uma cláusula RETURNING precisaria projetar quando o valor já é gerado
-- pelo cliente (como acontece hoje: id vem de sqlc.arg(id) nas duas tabelas,
-- não de DEFAULT do servidor) — o padrão real já observado em
-- CreateActivationAccount, onde o Go só consumia row.ID. Nenhuma outra
-- coluna destas tabelas é concedida: actor_subject_hmac, metadata e o
-- payload do outbox continuam sem SELECT nenhum para app_runtime.

GRANT SELECT (id) ON TABLE platform.audit_events TO app_runtime;
GRANT SELECT (id) ON TABLE platform.outbox_events TO app_runtime;

COMMIT;
