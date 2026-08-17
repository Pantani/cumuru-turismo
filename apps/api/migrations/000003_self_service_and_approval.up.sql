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
