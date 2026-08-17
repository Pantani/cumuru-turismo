-- Rollback da Fase 7 — autoatendimento e aprovação.
--
-- Ordem inversa exata do forward. Três blocos destroem dados por necessidade,
-- porque restaurar NOT NULL é impossível enquanto existir linha que só o novo
-- desenho permite:
--
--   * estadias self_service têm created_by_membership_id nulo;
--   * contas pending_activation têm password_hash nulo;
--   * convites por acomodação têm stay_id e max_uses nulos.
--
-- Isso é aceitável enquanto o projeto é PROTOTYPE_ONLY com fixtures fictícias,
-- e está registrado na ADR-040. Depois do piloto o down deixa de ser opção real
-- e a política volta a ser forward-fix.

-- 10. analytics.aggregate_eligible_preferences — restaura o corpo anterior por
--     completo, não apenas remove o predicado.
BEGIN;

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

REVOKE SELECT (approval_state) ON TABLE core.stays FROM migration_admin;

COMMIT;

-- 9. worker_runtime e app_runtime — devolve os grants da varredura.
BEGIN;

REVOKE SELECT (
  provenance,
  approval_state,
  approval_expires_at,
  cancelled_at,
  cancellation_reason_code,
  created_at
) ON TABLE core.stays
FROM worker_runtime;

REVOKE UPDATE (
  status,
  approval_state,
  approval_expires_at,
  cancelled_at,
  cancellation_reason_code,
  updated_at,
  version
) ON TABLE core.stays
FROM worker_runtime;

REVOKE DELETE ON TABLE core.visitors FROM worker_runtime, app_runtime;

COMMIT;

-- 8. platform.cleanup_expired_operational_records — volta às duas colunas.
BEGIN;

-- Mesmo motivo do `up`: migration_admin não tem CREATE em platform depois da
-- 000001, e sem ele o ALTER ... OWNER falha. O privilégio é reaberto e
-- devolvido dentro da mesma transação.
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

COMMIT;

-- 7. platform.proof_of_work_spends — a tabela inteira desaparece.
BEGIN;

DROP TABLE platform.proof_of_work_spends;

COMMIT;

-- 6. platform.rate_limit_buckets — os escopos novos precisam sumir antes do
--    CHECK antigo voltar.
BEGIN;

DELETE FROM platform.rate_limit_buckets
WHERE scope IN (
  'accommodation_invite_context', 'accommodation_invite_submit',
  'activation_context', 'activation_submit'
);

ALTER TABLE platform.rate_limit_buckets DROP CONSTRAINT rate_limit_scope_valid;
ALTER TABLE platform.rate_limit_buckets
  ADD CONSTRAINT rate_limit_scope_valid
    CHECK (scope IN ('invite_context', 'invite_submit', 'survey_submit'));

COMMIT;

-- 5. auth.activation_capabilities — precede a limpeza de contas por causa do
--    FK RESTRICT sobre auth.accounts.
BEGIN;

DROP TABLE auth.activation_capabilities;

COMMIT;

-- 4. auth.accounts — o NOT NULL só volta depois que nenhuma conta pendente
--    existir.
BEGIN;

DELETE FROM auth.accounts WHERE status = 'pending_activation';

ALTER TABLE auth.accounts DROP CONSTRAINT accounts_credential_state_valid;

ALTER TABLE auth.accounts DROP CONSTRAINT accounts_password_hash_algorithm;
ALTER TABLE auth.accounts
  ADD CONSTRAINT accounts_password_hash_algorithm
    CHECK (password_hash LIKE '$argon2id$%');

ALTER TABLE auth.accounts DROP CONSTRAINT accounts_status_valid;
ALTER TABLE auth.accounts
  ADD CONSTRAINT accounts_status_valid CHECK (status IN ('active', 'disabled'));

ALTER TABLE auth.accounts ALTER COLUMN password_hash SET NOT NULL;

REVOKE INSERT (id, email, display_name, scopes, status)
  ON TABLE auth.accounts FROM app_runtime;
REVOKE UPDATE (status) ON TABLE auth.accounts FROM app_runtime;

COMMENT ON COLUMN auth.accounts.status IS NULL;

COMMIT;

-- 3. core.group_submissions — o canal self_service some junto com suas linhas.
BEGIN;

DELETE FROM core.group_submissions WHERE collection_channel = 'self_service';

ALTER TABLE core.group_submissions
  DROP CONSTRAINT group_submissions_assisted_actor_valid;
ALTER TABLE core.group_submissions
  ADD CONSTRAINT group_submissions_assisted_actor_valid
    CHECK (
      (collection_channel = 'assisted' AND submitted_by_membership_id IS NOT NULL)
      OR
      (collection_channel = 'invite' AND submitted_by_membership_id IS NULL)
    );

ALTER TABLE core.group_submissions
  DROP CONSTRAINT group_submissions_channel_valid;
ALTER TABLE core.group_submissions
  ADD CONSTRAINT group_submissions_channel_valid
    CHECK (collection_channel IN ('assisted', 'invite'));

COMMIT;

-- 2. core.stays — as estadias sem autora precisam sumir com todas as suas
--    dependências antes de created_by_membership_id voltar a NOT NULL. A ordem
--    de deleção é ditada pelos ON DELETE RESTRICT.
BEGIN;

DELETE FROM analytics.presence_days
WHERE stay_id IN (SELECT id FROM core.stays WHERE provenance = 'self_service');

DELETE FROM core.visitors
WHERE stay_id IN (SELECT id FROM core.stays WHERE provenance = 'self_service');

DELETE FROM core.group_submissions
WHERE stay_id IN (SELECT id FROM core.stays WHERE provenance = 'self_service');

DELETE FROM core.stays WHERE provenance = 'self_service';

DROP INDEX core.stays_pending_approval_idx;

ALTER TABLE core.stays DROP CONSTRAINT stays_approval_reason_valid;
ALTER TABLE core.stays DROP CONSTRAINT stays_approval_fields_valid;
ALTER TABLE core.stays DROP CONSTRAINT stays_approval_state_valid;
ALTER TABLE core.stays DROP CONSTRAINT stays_provenance_author_valid;
ALTER TABLE core.stays DROP CONSTRAINT stays_provenance_valid;

COMMENT ON COLUMN core.stays.provenance IS NULL;
COMMENT ON COLUMN core.stays.approval_state IS NULL;
COMMENT ON COLUMN core.stays.approval_expires_at IS NULL;

ALTER TABLE core.stays
  DROP COLUMN approval_expires_at,
  DROP COLUMN approval_reason_code,
  DROP COLUMN approval_decided_by_membership_id,
  DROP COLUMN approved_at,
  DROP COLUMN approval_state,
  DROP COLUMN provenance;

ALTER TABLE core.stays ALTER COLUMN created_by_membership_id SET NOT NULL;

COMMIT;

-- 1. core.invites — os convites por acomodação somem antes de stay_id e
--    max_uses voltarem a NOT NULL.
BEGIN;

DELETE FROM core.invites WHERE purpose = 'accommodation_self_registration';

DROP INDEX core.invites_accommodation_single_active_idx;
DROP INDEX core.invites_accommodation_active_idx;

ALTER TABLE core.invites DROP CONSTRAINT invites_target_valid;

ALTER TABLE core.invites DROP CONSTRAINT invites_usage_valid;
ALTER TABLE core.invites
  ADD CONSTRAINT invites_usage_valid
    CHECK (max_uses > 0 AND use_count >= 0 AND use_count <= max_uses);

ALTER TABLE core.invites DROP CONSTRAINT invites_purpose_valid;
ALTER TABLE core.invites
  ADD CONSTRAINT invites_purpose_valid
    CHECK (purpose = 'stay_group_submission');

COMMENT ON COLUMN core.invites.accommodation_id IS NULL;
COMMENT ON COLUMN core.invites.max_uses IS NULL;

ALTER TABLE core.invites
  DROP COLUMN accommodation_id,
  ALTER COLUMN max_uses SET NOT NULL,
  ALTER COLUMN stay_id SET NOT NULL;

COMMIT;
