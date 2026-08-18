-- Esquema lógico inicial. Converta em migrações pequenas antes da implementação.
-- IDs devem ser UUIDv7 gerados pela aplicação.

CREATE SCHEMA IF NOT EXISTS auth;
CREATE SCHEMA IF NOT EXISTS identity;
CREATE SCHEMA IF NOT EXISTS core;
CREATE SCHEMA IF NOT EXISTS survey;
CREATE SCHEMA IF NOT EXISTS analytics;
CREATE SCHEMA IF NOT EXISTS public_data;
CREATE SCHEMA IF NOT EXISTS platform;

CREATE TYPE core.accommodation_status AS ENUM (
  'pending_review', 'active', 'suspended', 'closed'
);

CREATE TYPE core.stay_status AS ENUM (
  'draft', 'invited', 'pre_registered', 'checked_in', 'checked_out',
  'cancelled', 'no_show'
);

CREATE TYPE core.visitor_role AS ENUM (
  'responsible', 'companion', 'minor'
);

CREATE TYPE survey.version_status AS ENUM (
  'draft', 'privacy_review', 'approved', 'published', 'retired'
);

CREATE TYPE survey.answer_type AS ENUM (
  'short_text', 'long_text', 'single_choice', 'multiple_choice', 'boolean',
  'integer_range', 'rating', 'date', 'state_city'
);

CREATE TYPE survey.data_classification AS ENUM (
  'public', 'operational', 'personal', 'sensitive', 'secret'
);

CREATE TYPE platform.job_status AS ENUM (
  'pending', 'leased', 'succeeded', 'dead'
);

CREATE TYPE identity.privacy_request_status AS ENUM (
  'pending_verification', 'open', 'in_review', 'completed', 'rejected', 'cancelled'
);

CREATE TABLE auth.accounts (
  id uuid PRIMARY KEY,
  email text NOT NULL,
  display_name text NOT NULL,
  -- Nulo somente no estado pending_activation; ver
  -- accounts_credential_state_valid (ADR-041).
  password_hash text,
  scopes text[] NOT NULL,
  status text NOT NULL DEFAULT 'active',
  failed_attempts integer NOT NULL DEFAULT 0,
  locked_until timestamptz,
  password_changed_at timestamptz NOT NULL DEFAULT now(),
  password_must_change boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT accounts_email_normalized CHECK (email = btrim(lower(email))),
  CONSTRAINT accounts_display_name_not_blank CHECK (btrim(display_name) <> ''),
  CONSTRAINT accounts_password_hash_algorithm CHECK (password_hash LIKE '$argon2id$%'),
  CONSTRAINT accounts_scopes_bounded CHECK (cardinality(scopes) BETWEEN 1 AND 32),
  CONSTRAINT accounts_status_valid CHECK (status IN ('active', 'disabled'))
);

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
  CONSTRAINT sessions_expiry_ordered CHECK (idle_expires_at <= absolute_expires_at)
);

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
  status core.accommodation_status NOT NULL DEFAULT 'pending_review',
  cadastur_id text,
  capacity integer,
  public_area_code text,
  onboarding_submission_id uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  CONSTRAINT accommodations_capacity_positive
    CHECK (capacity IS NULL OR capacity > 0),
  CONSTRAINT accommodations_category_valid
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
    )
);

CREATE INDEX accommodations_organization_idx
  ON core.accommodations (organization_id, status);

CREATE UNIQUE INDEX accommodations_onboarding_submission_idx
  ON core.accommodations (organization_id, onboarding_submission_id)
  WHERE onboarding_submission_id IS NOT NULL;

-- Capability de uso único que transfere o acesso da acomodação a quem a opera
-- (ADR-041). Armazenada somente por HMAC e nunca reconstruível sem o keyring.
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

CREATE TABLE core.memberships (
  id uuid PRIMARY KEY,
  accommodation_id uuid NOT NULL REFERENCES core.accommodations(id),
  oidc_issuer text NOT NULL,
  oidc_subject text NOT NULL,
  role text NOT NULL,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  UNIQUE (accommodation_id, oidc_issuer, oidc_subject),
  CONSTRAINT memberships_oidc_issuer_not_blank CHECK (btrim(oidc_issuer) <> ''),
  CONSTRAINT memberships_oidc_subject_not_blank CHECK (btrim(oidc_subject) <> ''),
  CONSTRAINT memberships_role_valid CHECK (role IN ('operator', 'manager')),
  CONSTRAINT memberships_version_positive CHECK (version > 0)
);

CREATE INDEX memberships_principal_idx
  ON core.memberships (oidc_issuer, oidc_subject, active);

CREATE TABLE core.stays (
  id uuid PRIMARY KEY,
  accommodation_id uuid NOT NULL REFERENCES core.accommodations(id),
  -- Nulo somente quando provenance = 'self_service'; ver
  -- stays_provenance_author_valid (ADR-040).
  created_by_membership_id uuid REFERENCES core.memberships(id),
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
  provenance text NOT NULL DEFAULT 'assisted',
  approval_state text,
  approved_at timestamptz,
  approval_decided_by_membership_id uuid REFERENCES core.memberships(id),
  approval_reason_code text,
  approval_expires_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  CONSTRAINT stays_dates_valid
    CHECK (planned_departure_on > planned_arrival_on),
  CONSTRAINT stays_provenance_valid
    CHECK (provenance IN ('assisted', 'self_service')),
  CONSTRAINT stays_provenance_author_valid
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
    ),
  CONSTRAINT stays_approval_state_valid
    CHECK (
      approval_state IS NULL
      OR approval_state IN ('pending', 'approved', 'rejected', 'expired')
    ),
  CONSTRAINT stays_approval_fields_valid
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
        approval_state = 'expired'
        AND approved_at IS NULL
        AND approval_decided_by_membership_id IS NULL
        AND approval_reason_code IS NULL
        AND approval_expires_at IS NULL
      )
    ),
  CONSTRAINT stays_approval_reason_valid
    CHECK (
      approval_reason_code IS NULL
      OR approval_reason_code IN (
        'identity_not_verified', 'not_a_guest', 'duplicate',
        'data_incorrect', 'other'
      )
    ),
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
        '0_5', '6_11', '12_17', '18_24',
        '25_34', '35_44', '45_59', '60_plus'
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

CREATE INDEX visitors_stay_idx ON core.visitors (stay_id);

CREATE UNIQUE INDEX visitors_one_responsible_per_stay_idx
  ON core.visitors (stay_id)
  WHERE role = 'responsible';

CREATE TABLE identity.visitor_identities (
  visitor_id uuid PRIMARY KEY REFERENCES core.visitors(id) ON DELETE RESTRICT,
  encrypted_name bytea,
  encrypted_document bytea,
  encrypted_contact bytea,
  document_hmac bytea,
  contact_hmac bytea,
  encryption_key_version text NOT NULL,
  retention_policy_code text NOT NULL,
  erase_after timestamptz,
  legal_hold boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX visitor_identity_document_hmac_idx
  ON identity.visitor_identities (document_hmac)
  WHERE document_hmac IS NOT NULL;

CREATE TABLE identity.external_credentials (
  id uuid PRIMARY KEY,
  accommodation_id uuid NOT NULL REFERENCES core.accommodations(id),
  provider text NOT NULL,
  encrypted_secret bytea NOT NULL,
  encryption_key_version text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  rotated_at timestamptz,
  UNIQUE (accommodation_id, provider)
);

CREATE TABLE core.invites (
  id uuid PRIMARY KEY,
  stay_id uuid REFERENCES core.stays(id),
  accommodation_id uuid REFERENCES core.accommodations(id) ON DELETE RESTRICT,
  token_hmac bytea NOT NULL UNIQUE,
  token_key_version text NOT NULL,
  purpose text NOT NULL,
  privacy_notice_version text NOT NULL,
  expires_at timestamptz NOT NULL,
  -- Nulo é uso ilimitado, permitido somente no convite por acomodação. O
  -- DEFAULT 1 é preservado porque CreateStayInvite não lista a coluna.
  max_uses integer DEFAULT 1,
  use_count integer NOT NULL DEFAULT 0,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT invites_key_version_not_blank
    CHECK (btrim(token_key_version) <> ''),
  CONSTRAINT invites_purpose_valid
    CHECK (purpose IN ('stay_group_submission', 'accommodation_self_registration')),
  CONSTRAINT invites_notice_not_blank
    CHECK (btrim(privacy_notice_version) <> ''),
  -- O teste de nulidade precede a comparação, de modo que o CHECK nunca avalia
  -- UNKNOWN (que passaria) quando max_uses é nulo.
  CONSTRAINT invites_usage_valid
    CHECK (
      use_count >= 0
      AND (max_uses IS NULL OR (max_uses > 0 AND use_count <= max_uses))
    ),
  CONSTRAINT invites_target_valid
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
    )
);

CREATE INDEX invites_stay_active_idx
  ON core.invites (stay_id, expires_at)
  WHERE revoked_at IS NULL;

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
    CHECK (collection_channel IN ('assisted', 'invite', 'self_service')),
  -- O nome é preservado de propósito: renomear obrigaria a caçar asserções em
  -- test-migrations.sh e core_postgres_test.go sem ganho proporcional.
  CONSTRAINT group_submissions_assisted_actor_valid
    CHECK (
      (collection_channel = 'assisted'     AND submitted_by_membership_id IS NOT NULL)
      OR
      (collection_channel = 'invite'       AND submitted_by_membership_id IS NULL)
      OR
      (collection_channel = 'self_service' AND submitted_by_membership_id IS NULL)
    ),
  UNIQUE (stay_id, client_submission_id)
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
  CONSTRAINT questions_minimum_cell_valid
    CHECK (minimum_public_cell IS NULL OR minimum_public_cell >= 10),
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

CREATE TYPE survey.participation AS ENUM (
  'submitted', 'declined'
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

CREATE FUNCTION survey.erase_expired_free_text(cutoff timestamptz)
RETURNS integer
LANGUAGE sql
AS $$
  WITH erased AS (
    UPDATE survey.answers
    SET
      encrypted_free_text = NULL,
      free_text_nonce = NULL,
      encryption_key_version = NULL,
      erased_at = cutoff
    WHERE encrypted_free_text IS NOT NULL
      AND erase_after <= cutoff
    RETURNING 1
  )
  SELECT count(*)::integer FROM erased;
$$;

CREATE TABLE identity.privacy_requests (
  id uuid PRIMARY KEY,
  request_type text NOT NULL,
  status identity.privacy_request_status NOT NULL DEFAULT 'pending_verification',
  encrypted_contact bytea NOT NULL,
  contact_hmac bytea NOT NULL,
  verification_token_hmac bytea,
  encryption_key_version text NOT NULL,
  stay_code_hmac bytea,
  verified_at timestamptz,
  due_at timestamptz,
  assigned_subject text,
  outcome_code text,
  completed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT privacy_request_type_valid
    CHECK (
      request_type IN ('access', 'correction', 'deletion', 'information', 'objection')
    )
);

CREATE INDEX privacy_requests_contact_idx
  ON identity.privacy_requests (contact_hmac, created_at DESC);

CREATE TABLE platform.retention_policies (
  code text NOT NULL,
  version integer NOT NULL,
  data_class text NOT NULL,
  action text NOT NULL,
  retention_interval interval,
  legal_basis_reference text,
  approved_by text,
  approved_at timestamptz,
  active boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (code, version),
  CONSTRAINT retention_action_valid
    CHECK (action IN ('delete', 'anonymize', 'review')),
  CONSTRAINT retention_version_positive CHECK (version > 0)
);

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
  CONSTRAINT presence_days_weight_valid CHECK (weight > 0 AND weight <= 1),
  CONSTRAINT presence_days_kind_valid CHECK (kind IN ('observed', 'forecast')),
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
        AND period_selector IN ('observed_daily', 'next_30_days')
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
    )::bigint,
    count(DISTINCT stay.accommodation_id) FILTER (
      WHERE consent.response_id IS NOT NULL
    )::bigint,
    max(question.minimum_public_cell)::integer
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
        AND period_selector IN ('observed_daily', 'next_30_days')
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
  CONSTRAINT metric_cells_presence_selector_kind_valid
    CHECK (
      metric_code <> 'presence'
      OR (
        period_selector = 'observed_daily'
        AND kind = 'observed'
      )
      OR (
        period_selector = 'next_30_days'
        AND kind = 'forecast'
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
      AND fnrh_failures_reason = 'not_implemented'
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

-- O resumo é a capa: um dia observado e o horizonte previsto. Sem o recorte a
-- view devolveria os 730 dias de histórico para responder "quantos hoje".
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
FROM public_data.current_presence
WHERE kind = 'forecast'
  OR period_start = as_of_on;

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
  ARRAY[
    'recent_30_days',
    'recent_90_days',
    'recent_365_days',
    'recent_730_days',
    'next_30_days',
    'month'
  ]::text[]
    AS allowed_presence_windows,
  ARRAY['last_complete_month']::text[]
    AS allowed_preference_periods,
  70::integer AS forecast_fallback_lower_percent,
  130::integer AS forecast_fallback_upper_percent,
  -- Coluna nova entra no fim da lista: `CREATE OR REPLACE VIEW` recusa
  -- inserção no meio, e recriar a view custaria os grants do papel público.
  730::integer AS presence_history_days
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
WHERE snapshot.id = (
  SELECT candidate.id
  FROM analytics.quality_snapshots AS candidate
  WHERE candidate.window_code = snapshot.window_code
  ORDER BY candidate.updated_at DESC, candidate.id DESC
  LIMIT 1
);

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
    CHECK (scope IN (
      'invite_context', 'invite_submit', 'survey_submit',
      'accommodation_invite_context', 'accommodation_invite_submit',
      'activation_context', 'activation_submit'
    )),
  CONSTRAINT rate_limit_key_version_not_blank
    CHECK (btrim(subject_key_version) <> ''),
  CONSTRAINT rate_limit_count_positive
    CHECK (request_count > 0),
  CONSTRAINT rate_limit_expiry_valid
    CHECK (expires_at > window_started_at)
);

-- Gasto de nonce do proof-of-work. Guarda somente HMAC do desafio e prazo:
-- nenhuma coluna deriva de IP, token ou titular.
CREATE TABLE platform.proof_of_work_spends (
  challenge_hmac bytea PRIMARY KEY,
  key_version text NOT NULL,
  expires_at timestamptz NOT NULL,
  CONSTRAINT proof_of_work_challenge_hmac_sha256
    CHECK (octet_length(challenge_hmac) = 32),
  CONSTRAINT proof_of_work_key_version_not_blank
    CHECK (btrim(key_version) <> '')
);


CREATE INDEX rate_limit_expiry_idx
  ON platform.rate_limit_buckets (expires_at);

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

CREATE TABLE platform.jobs (
  id uuid PRIMARY KEY,
  job_type text NOT NULL,
  deduplication_key text,
  payload jsonb NOT NULL,
  status platform.job_status NOT NULL DEFAULT 'pending',
  available_at timestamptz NOT NULL DEFAULT now(),
  lease_owner text,
  lease_until timestamptz,
  attempts integer NOT NULL DEFAULT 0,
  max_attempts integer NOT NULL,
  last_error_code text,
  created_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz,
  CONSTRAINT jobs_attempts_valid
    CHECK (attempts >= 0 AND max_attempts > 0)
);

CREATE UNIQUE INDEX jobs_active_deduplication_idx
  ON platform.jobs (job_type, deduplication_key)
  WHERE deduplication_key IS NOT NULL
    AND status IN ('pending', 'leased');

CREATE INDEX jobs_ready_idx
  ON platform.jobs (available_at, created_at)
  WHERE status = 'pending';

CREATE TABLE core.integration_deliveries (
  id uuid PRIMARY KEY,
  stay_id uuid NOT NULL REFERENCES core.stays(id),
  provider text NOT NULL,
  adapter_version text NOT NULL,
  operation text NOT NULL,
  idempotency_key text NOT NULL,
  state text NOT NULL,
  external_reference text,
  last_error_code text,
  attempt_count integer NOT NULL DEFAULT 0,
  last_attempt_at timestamptz,
  completed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, idempotency_key),
  CONSTRAINT integration_state_valid
    CHECK (state IN ('pending', 'succeeded', 'retry', 'failed_permanent'))
);

CREATE TABLE platform.audit_events (
  id uuid PRIMARY KEY,
  occurred_at timestamptz NOT NULL DEFAULT now(),
  actor_subject_hmac bytea,
  actor_hmac_key_version text,
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
  CONSTRAINT audit_actor_key_version_valid
    CHECK (
      (actor_subject_hmac IS NULL AND actor_hmac_key_version IS NULL)
      OR
      (actor_subject_hmac IS NOT NULL AND btrim(actor_hmac_key_version) <> '')
    )
);

CREATE INDEX audit_events_time_idx
  ON platform.audit_events (occurred_at DESC);

COMMENT ON SCHEMA identity IS
  'Acesso mínimo; contém identidade cifrada e credenciais externas.';
COMMENT ON SCHEMA public_data IS
  'Única fonte de dados permitida para o papel da API pública.';
COMMENT ON TABLE analytics.presence_days IS
  'Microdados pseudonimizados internos; nunca expor ao dashboard público.';
COMMENT ON TABLE platform.audit_events IS
  'Append-only para o papel da aplicação; sem valores pessoais alterados.';

REVOKE ALL ON SCHEMA identity, core, survey, analytics, public_data, platform
  FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA
  identity, core, survey, analytics, public_data, platform
  FROM PUBLIC;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA
  identity, core, survey, analytics, public_data, platform
  FROM PUBLIC, public_runtime;
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA
  identity, core, survey, analytics, public_data, platform
  FROM PUBLIC, public_runtime;

REVOKE ALL ON SCHEMA identity, core, survey, analytics, public_data, platform
  FROM public_runtime;
REVOKE ALL ON ALL TABLES IN SCHEMA
  identity, core, survey, analytics, public_data, platform
  FROM public_runtime;

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

GRANT USAGE, CREATE ON SCHEMA analytics TO migration_admin;
GRANT USAGE ON SCHEMA survey, core, platform TO migration_admin;
GRANT SELECT (
  privacy_policy_version,
  metric_code,
  questionnaire_version_id,
  question_id,
  source_value,
  category_code
) ON TABLE analytics.metric_mappings TO migration_admin;
GRANT SELECT (
  id,
  questionnaire_version_id,
  answer_type,
  data_classification,
  purpose_code,
  analytics_key,
  public_aggregation_allowed,
  minimum_public_cell
) ON TABLE survey.questions TO migration_admin;
GRANT SELECT (
  response_id,
  questionnaire_version_id,
  question_id,
  structured_value
) ON TABLE survey.answers TO migration_admin;
GRANT SELECT (
  id,
  stay_id,
  questionnaire_version_id,
  participation,
  submitted_at
) ON TABLE survey.responses TO migration_admin;
GRANT SELECT (
  response_id,
  questionnaire_version_id,
  purpose_code,
  granted
) ON TABLE survey.consent_decisions TO migration_admin;
GRANT SELECT (id, accommodation_id) ON TABLE core.stays TO migration_admin;

GRANT SELECT (
  actor_key_hmac,
  method,
  operation_key,
  resource_id,
  idempotency_key_hmac,
  state,
  expires_at
) ON TABLE platform.idempotency_records TO migration_admin;
GRANT DELETE ON TABLE platform.idempotency_records TO migration_admin;
GRANT UPDATE (expires_at) ON TABLE platform.idempotency_records
TO migration_admin;

GRANT SELECT (
  scope,
  subject_hmac,
  window_started_at,
  expires_at
) ON TABLE platform.rate_limit_buckets TO migration_admin;
GRANT DELETE ON TABLE platform.rate_limit_buckets TO migration_admin;
GRANT UPDATE (expires_at) ON TABLE platform.rate_limit_buckets
TO migration_admin;

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
) ON TABLE survey.responses FROM worker_runtime;
REVOKE SELECT (
  response_id,
  questionnaire_version_id,
  question_id,
  structured_value
) ON TABLE survey.answers FROM worker_runtime;
REVOKE SELECT (
  id,
  questionnaire_version_id,
  answer_type,
  data_classification,
  purpose_code,
  analytics_key,
  public_aggregation_allowed,
  minimum_public_cell
) ON TABLE survey.questions FROM worker_runtime;
REVOKE SELECT (
  response_id,
  questionnaire_version_id,
  purpose_code,
  notice_version,
  granted
) ON TABLE survey.consent_decisions FROM worker_runtime;
REVOKE INSERT, UPDATE, DELETE ON TABLE
  analytics.metric_catalog,
  analytics.metric_mappings
  FROM worker_runtime;

DO $$
DECLARE
  database_name text := pg_catalog.current_database();
BEGIN
  EXECUTE pg_catalog.format(
    'REVOKE CREATE, TEMPORARY ON DATABASE %I FROM PUBLIC, public_runtime',
    database_name
  );
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
END
$$;
