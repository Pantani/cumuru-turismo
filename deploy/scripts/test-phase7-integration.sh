#!/usr/bin/env bash
set -euo pipefail

# Fase 7 — autoatendimento e aprovação, integração em PostgreSQL real.
#
# O teste central desta fase é a reprodução da falha silenciosa de max_uses
# nulo. Ela só é reproduzível DEPOIS do DDL da 000003, porque na baseline a
# coluna é NOT NULL: por isso a asserção compara, sobre o mesmo schema novo, o
# predicado antigo (`use_count < max_uses`) contra o corrigido
# (`max_uses IS NULL OR use_count < max_uses`). O antigo afeta zero linhas e é
# essa a falha; qualquer suíte que exercite apenas convite limitado fica verde.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROJECT_NAME="cumuru-phase7-integration-${PPID}-$$"
. "${ROOT_DIR}/deploy/scripts/lib/compose-subnet.sh"
cumuru_acquire_subnet phase7-integration
COMPOSE=(
  "${ROOT_DIR}/deploy/scripts/with-build-metadata.sh"
  docker compose
  --file "${ROOT_DIR}/compose.yaml"
  --file "${ROOT_DIR}/deploy/compose.phase7-integration.yaml"
  --project-name "${PROJECT_NAME}"
)
MIGRATION_URL="postgres://cumuru_migration:cumuru-local-migration-only@postgres:5432/cumuru?sslmode=disable"
GATE_LOCK_DIR="${TMPDIR:-/tmp}/cumuru-docker-subnet-172-30-7.lock"
GATE_LOCK_HELD=false

release_gate_lock() {
  if test "${GATE_LOCK_HELD}" != "true"; then
    return
  fi
  rm -f "${GATE_LOCK_DIR}/pid"
  rmdir "${GATE_LOCK_DIR}" 2>/dev/null || true
  GATE_LOCK_HELD=false
}

acquire_gate_lock() {
  local deadline=$((SECONDS + 900))
  local owner_pid=""
  while ! mkdir "${GATE_LOCK_DIR}" 2>/dev/null; do
    if test -f "${GATE_LOCK_DIR}/pid"; then
      owner_pid="$(tr -cd '0-9' <"${GATE_LOCK_DIR}/pid")"
      if test -n "${owner_pid}" && ! kill -0 "${owner_pid}" 2>/dev/null; then
        rm -f "${GATE_LOCK_DIR}/pid"
        rmdir "${GATE_LOCK_DIR}" 2>/dev/null || true
        continue
      fi
    fi
    if test "${SECONDS}" -ge "${deadline}"; then
      echo "phase 7 integration timed out waiting for Docker subnet lock" >&2
      return 1
    fi
    sleep 1
  done
  printf '%s\n' "$$" >"${GATE_LOCK_DIR}/pid"
  GATE_LOCK_HELD=true
}

cleanup() {
  local primary_status=$?
  trap - EXIT
  set +e
  "${COMPOSE[@]}" down --volumes --remove-orphans >/dev/null 2>&1
  local cleanup_status=$?
  release_gate_lock
  set -e
  # O lock só cai depois do teardown: é isso que impede a execução seguinte de
  # pedir um endereço que ainda está sendo liberado.
  cumuru_release_subnet
  if test "${primary_status}" -ne 0; then
    exit "${primary_status}"
  fi
  exit "${cleanup_status}"
}
trap cleanup EXIT

acquire_gate_lock

psql_as() {
  local user="$1"
  local password="$2"
  shift 2
  "${COMPOSE[@]}" exec -T \
    -e "PGPASSWORD=${password}" \
    postgres psql --no-psqlrc --set=ON_ERROR_STOP=1 \
    --username="${user}" --dbname=cumuru "$@"
}

migration_query() {
  psql_as cumuru_migration cumuru-local-migration-only \
    --tuples-only --no-align --command="$1"
}

# Falhar não basta. Uma fixture escrita contra o schema imaginado morre por FK
# ausente ou coluna obrigatória vazia, e o teste fica verde sem provar invariante
# algum. Esta asserção exige o nome da constraint na mensagem do PostgreSQL, de
# modo que um erro de fixture apareça como erro de fixture.
expect_constraint_violation() {
  local description="$1"
  local constraint="$2"
  local statement="$3"
  local output=""
  if output="$(
    psql_as cumuru_migration cumuru-local-migration-only \
      --command="${statement}" 2>&1
  )"; then
    echo "${description}: the statement unexpectedly succeeded" >&2
    exit 1
  fi
  case "${output}" in
    *"${constraint}"*) return 0 ;;
  esac
  echo "${description}: failed on something other than ${constraint}" >&2
  echo "${output}" >&2
  exit 1
}

assert_equals() {
  local description="$1"
  local expected="$2"
  local actual="$3"
  if test "${actual}" != "${expected}"; then
    echo "${description}: expected ${expected}, got ${actual}" >&2
    exit 1
  fi
}

"${COMPOSE[@]}" up --detach --wait postgres
"${COMPOSE[@]}" run --rm --no-deps migrate \
  -path=/migrations -database="${MIGRATION_URL}" up

# Fixture fictícia e mínima: uma organização, uma acomodação ativa, um manager.
psql_as cumuru_migration cumuru-local-migration-only <<'SQL'
INSERT INTO core.organizations (id, name, legal_name) VALUES (
  '00000000-0000-7000-8000-0000000007a1',
  'Organização Fictícia Fase 7',
  'Organização Fictícia Fase 7 LTDA'
);

INSERT INTO core.accommodations (id, organization_id, name, status, category, capacity)
VALUES (
  '00000000-0000-7000-8000-0000000007b1',
  '00000000-0000-7000-8000-0000000007a1',
  'Pousada Fictícia Fase 7',
  'active',
  'formal_lodging',
  10
);

INSERT INTO core.memberships (id, accommodation_id, oidc_issuer, oidc_subject, role)
VALUES (
  '00000000-0000-7000-8000-0000000007c1',
  '00000000-0000-7000-8000-0000000007b1',
  'https://identity.example/',
  'phase7-manager',
  'manager'
);

-- Estadia assistida válida. Existe para que o teste de invites_target_valid
-- tenha um stay_id que realmente resolve: apontar para uma estadia inexistente
-- faria o INSERT morrer por chave estrangeira e o teste passaria sem nunca
-- exercitar o CHECK que pretende provar.
INSERT INTO core.stays (
  id, accommodation_id, created_by_membership_id, client_submission_id,
  planned_arrival_on, planned_departure_on, expected_guest_count
) VALUES (
  '00000000-0000-7000-8000-0000000007e1',
  '00000000-0000-7000-8000-0000000007b1',
  '00000000-0000-7000-8000-0000000007c1',
  '00000000-0000-7000-8000-0000000007e2',
  '2026-02-01', '2026-02-05', 2
);

-- Cartaz ilimitado: max_uses nulo, stay_id nulo, alvo por acomodação.
INSERT INTO core.invites (
  id, accommodation_id, token_hmac, token_key_version, purpose,
  privacy_notice_version, expires_at, max_uses
) VALUES (
  '00000000-0000-7000-8000-0000000007d1',
  '00000000-0000-7000-8000-0000000007b1',
  decode(repeat('a1', 32), 'hex'),
  'v1',
  'accommodation_self_registration',
  '2026-01-01',
  now() + interval '365 days',
  NULL
);
SQL

# --- A falha silenciosa de max_uses nulo -------------------------------------
#
# Predicado ANTIGO sobre o schema novo: `use_count < max_uses` avalia UNKNOWN
# com max_uses nulo, o UPDATE afeta zero linhas e o cartaz ilimitado passa a se
# comportar como convite já consumido.
legacy_consumed="$(
  migration_query "
    WITH consumed AS (
      UPDATE core.invites AS i
      SET use_count = i.use_count + 1
      WHERE i.id = '00000000-0000-7000-8000-0000000007d1'
        AND i.revoked_at IS NULL
        AND i.expires_at > now()
        AND i.use_count < i.max_uses
      RETURNING i.id
    )
    SELECT count(*) FROM consumed
  "
)"
assert_equals \
  "the legacy predicate unexpectedly consumed an unlimited invite; the silent failure no longer reproduces and this test lost its meaning" \
  "0" "${legacy_consumed}"

# Predicado CORRIGIDO, idêntico ao de ConsumeInvite: o teste de nulidade precede
# a comparação, então o cartaz ilimitado consome.
fixed_consumed="$(
  migration_query "
    WITH consumed AS (
      UPDATE core.invites AS i
      SET use_count = i.use_count + 1
      WHERE i.id = '00000000-0000-7000-8000-0000000007d1'
        AND i.revoked_at IS NULL
        AND i.expires_at > now()
        AND (i.max_uses IS NULL OR i.use_count < i.max_uses)
      RETURNING i.id
    )
    SELECT count(*) FROM consumed
  "
)"
assert_equals \
  "the corrected predicate failed to consume an unlimited invite" \
  "1" "${fixed_consumed}"

# O cartaz ilimitado aceita N consumos sem violar invites_usage_valid (N-04).
migration_query "
  UPDATE core.invites SET use_count = use_count + 1
  WHERE id = '00000000-0000-7000-8000-0000000007d1'
    AND (max_uses IS NULL OR use_count < max_uses)
" >/dev/null
unlimited_use_count="$(
  migration_query "
    SELECT use_count FROM core.invites
    WHERE id = '00000000-0000-7000-8000-0000000007d1'
  "
)"
assert_equals "unlimited invite lost a consumption" "2" "${unlimited_use_count}"

# --- Invariantes do convite ---------------------------------------------------

# invites_target_valid: autocadastro COM stay_id abriria caminho para injetar
# visitantes numa estadia de outra acomodação (T-07).
expect_constraint_violation \
  "invites_target_valid accepted a self-registration invite bound to a stay" \
  "invites_target_valid" \
  "INSERT INTO core.invites (
     id, stay_id, accommodation_id, token_hmac, token_key_version, purpose,
     privacy_notice_version, expires_at
   ) VALUES (
     '00000000-0000-7000-8000-0000000007d2',
     '00000000-0000-7000-8000-0000000007e1',
     '00000000-0000-7000-8000-0000000007b1',
     decode(repeat('a2', 32), 'hex'), 'v1',
     'accommodation_self_registration', '2026-01-01', now() + interval '1 day'
   )"

# invites_target_valid: "ilimitado" só existe no purpose novo.
expect_constraint_violation \
  "invites_target_valid accepted an unlimited stay invite" \
  "invites_target_valid" \
  "INSERT INTO core.invites (
     id, accommodation_id, token_hmac, token_key_version, purpose,
     privacy_notice_version, expires_at, max_uses
   ) VALUES (
     '00000000-0000-7000-8000-0000000007d3',
     '00000000-0000-7000-8000-0000000007b1',
     decode(repeat('a3', 32), 'hex'), 'v1',
     'stay_group_submission', '2026-01-01', now() + interval '1 day', NULL
   )"

# O DEFAULT 1 sobrevive: CreateStayInvite não lista a coluna e depende dele.
default_max_uses="$(
  migration_query "
    SELECT column_default FROM information_schema.columns
    WHERE table_schema = 'core' AND table_name = 'invites'
      AND column_name = 'max_uses'
  "
)"
assert_equals "the DEFAULT 1 of core.invites.max_uses was lost" "1" "${default_max_uses}"

# --- Proveniência e aprovação da estadia -------------------------------------

# O caso assistido não afrouxa: autora nula continua proibida via
# stays_provenance_author_valid, mesmo com a coluna agora nulável.
expect_constraint_violation \
  "stays_provenance_author_valid accepted an assisted stay without an author" \
  "stays_provenance_author_valid" \
  "INSERT INTO core.stays (
     id, accommodation_id, client_submission_id,
     planned_arrival_on, planned_departure_on, expected_guest_count
   ) VALUES (
     '00000000-0000-7000-8000-0000000007f1',
     '00000000-0000-7000-8000-0000000007b1',
     '00000000-0000-7000-8000-0000000007f9',
     '2026-03-01', '2026-03-05', 2
   )"

# O autocadastro nasce sem autora, pendente e com prazo.
psql_as cumuru_migration cumuru-local-migration-only <<'SQL'
INSERT INTO core.stays (
  id, accommodation_id, status, client_submission_id,
  planned_arrival_on, planned_departure_on, expected_guest_count,
  provenance, approval_state, approval_expires_at
) VALUES (
  '00000000-0000-7000-8000-0000000007f2',
  '00000000-0000-7000-8000-0000000007b1',
  'pre_registered',
  '00000000-0000-7000-8000-0000000007fa',
  '2026-03-01', '2026-03-05', 2,
  'self_service', 'pending', now() + interval '72 hours'
);
SQL

# Estadia self_service COM autora é contradição e precisa falhar.
expect_constraint_violation \
  "stays_provenance_author_valid accepted a self-service stay with an author" \
  "stays_provenance_author_valid" \
  "INSERT INTO core.stays (
     id, accommodation_id, created_by_membership_id, client_submission_id,
     planned_arrival_on, planned_departure_on, expected_guest_count,
     provenance, approval_state, approval_expires_at
   ) VALUES (
     '00000000-0000-7000-8000-0000000007f3',
     '00000000-0000-7000-8000-0000000007b1',
     '00000000-0000-7000-8000-0000000007c1',
     '00000000-0000-7000-8000-0000000007fb',
     '2026-03-01', '2026-03-05', 2,
     'self_service', 'pending', now() + interval '72 hours'
   )"

# stays_approval_fields_valid: aprovada exige decisora e proíbe prazo residual.
expect_constraint_violation \
  "stays_approval_fields_valid accepted an approved stay without a decider" \
  "stays_approval_fields_valid" \
  "UPDATE core.stays
     SET approval_state = 'approved', approved_at = now(), approval_expires_at = NULL
   WHERE id = '00000000-0000-7000-8000-0000000007f2'"

# stays_approval_reason_valid: motivo fora da lista fechada é recusado (E-03).
expect_constraint_violation \
  "stays_approval_reason_valid accepted a free-text rejection reason" \
  "stays_approval_reason_valid" \
  "UPDATE core.stays
     SET approval_state = 'rejected',
         approval_reason_code = 'CPF falso, não é hóspede',
         approval_decided_by_membership_id = '00000000-0000-7000-8000-0000000007c1',
         approval_expires_at = NULL,
         status = 'cancelled', cancelled_at = now(),
         cancellation_reason_code = 'accommodation_request'
   WHERE id = '00000000-0000-7000-8000-0000000007f2'"

# --- Canal de coleta ----------------------------------------------------------

# group_submissions_assisted_actor_valid preserva o nome e os três ramos.
expect_constraint_violation \
  "group_submissions_assisted_actor_valid accepted a self-service submission with an actor" \
  "group_submissions_assisted_actor_valid" \
  "INSERT INTO core.group_submissions (
     id, stay_id, client_submission_id, request_hash, privacy_notice_version,
     collection_channel, submitted_by_membership_id
   ) VALUES (
     '00000000-0000-7000-8000-000000000801',
     '00000000-0000-7000-8000-0000000007f2',
     '00000000-0000-7000-8000-000000000802',
     decode(repeat('b1', 32), 'hex'), '2026-01-01',
     'self_service', '00000000-0000-7000-8000-0000000007c1'
   )"

actor_check_branches="$(
  migration_query "
    SELECT count(*) FROM pg_constraint
    WHERE conrelid = 'core.group_submissions'::regclass
      AND conname = 'group_submissions_assisted_actor_valid'
      AND pg_get_constraintdef(oid) LIKE '%self_service%'
  "
)"
assert_equals \
  "the constraint name group_submissions_assisted_actor_valid was renamed or lost its third branch" \
  "1" "${actor_check_branches}"

# --- Conta pendente -----------------------------------------------------------

# Conta pendente nasce sem hash; conta ativa sem hash continua impossível.
psql_as cumuru_migration cumuru-local-migration-only <<'SQL'
INSERT INTO auth.accounts (id, email, display_name, scopes, status)
VALUES (
  '00000000-0000-7000-8000-000000000901',
  'pendente@example.test',
  'Operadora Fictícia',
  ARRAY['stays:read:own'],
  'pending_activation'
);
SQL

expect_constraint_violation \
  "accounts_credential_state_valid allowed an active account without a credential" \
  "accounts_credential_state_valid" \
  "INSERT INTO auth.accounts (id, email, display_name, scopes, status)
   VALUES (
     '00000000-0000-7000-8000-000000000902',
     'ativa-sem-hash@example.test',
     'Operadora Fictícia',
     ARRAY['stays:read:own'],
     'active'
   )"

# O CHECK do algoritmo continua valendo quando há hash, agora condicionado a
# password_hash IS NOT NULL e não ao status.
expect_constraint_violation \
  "accounts_password_hash_algorithm accepted a non-argon2id hash" \
  "accounts_password_hash_algorithm" \
  "INSERT INTO auth.accounts (id, email, display_name, password_hash, scopes, status)
   VALUES (
     '00000000-0000-7000-8000-000000000903',
     'hash-errado@example.test',
     'Operadora Fictícia',
     'plaintext',
     ARRAY['stays:read:own'],
     'active'
   )"

# --- Proof-of-work ------------------------------------------------------------

# N-21: o livro de nonces não pode ter nenhuma coluna derivada de IP, token ou
# titular. A varredura é por information_schema, não por inspeção visual.
spend_columns="$(
  migration_query "
    SELECT string_agg(column_name, ',' ORDER BY column_name)
    FROM information_schema.columns
    WHERE table_schema = 'platform' AND table_name = 'proof_of_work_spends'
  "
)"
assert_equals \
  "platform.proof_of_work_spends gained a column outside the nonce/expiry contract" \
  "challenge_hmac,expires_at,key_version" "${spend_columns}"

# O gasto é único: o segundo INSERT do mesmo desafio não afeta linha alguma, e é
# exatamente isso que impede o replay da mesma solução dentro do TTL (N-17).
psql_as cumuru_migration cumuru-local-migration-only <<'SQL'
INSERT INTO platform.proof_of_work_spends (challenge_hmac, key_version, expires_at)
VALUES (decode(repeat('c1', 32), 'hex'), 'v1', now() + interval '300 seconds');
SQL

replayed_spend="$(
  migration_query "
    WITH spent AS (
      INSERT INTO platform.proof_of_work_spends (challenge_hmac, key_version, expires_at)
      VALUES (decode(repeat('c1', 32), 'hex'), 'v1', now() + interval '300 seconds')
      ON CONFLICT (challenge_hmac) DO NOTHING
      RETURNING challenge_hmac
    )
    SELECT count(*) FROM spent
  "
)"
assert_equals \
  "the proof-of-work nonce ledger accepted a replayed challenge" \
  "0" "${replayed_spend}"

# --- Filtro de presença -------------------------------------------------------

# A projeção de approval_state existe para o ponto 2 do filtro decidir no Go. O
# predicado NÃO pode estar no WHERE da reconciliação, senão os fatos de uma
# estadia rejeitada ficam órfãos.
approval_projection="$(
  migration_query "
    SELECT count(*) FROM information_schema.columns
    WHERE table_schema = 'core' AND table_name = 'stays'
      AND column_name IN ('provenance', 'approval_state', 'approval_expires_at')
  "
)"
assert_equals "core.stays lost a Phase 7 approval column" "3" "${approval_projection}"

# Ponto 3: a função SECURITY DEFINER precisa do predicado, senão o gate vale
# para presença e é falso para first_visit_share.
#
# A asserção é sensível à POSIÇÃO, não à mera presença da string. Um
# `prosrc LIKE '%approval_state%'` não distingue predicado no WHERE de predicado
# na condição do LEFT JOIN — e é exatamente esse o erro que o desenho diz ter
# evitado: no join a linha volta com stay.id NULL e continua sendo contada pelo
# FILTER (WHERE ... stay.id IS NOT NULL) do accommodation_count.
#
# Campo 1: o predicado aparece depois do WHERE final.
# Campo 2: o predicado NÃO aparece no trecho entre o LEFT JOIN e esse WHERE.
preference_filter="$(
  migration_query "
    SELECT
      (position('approval_state' in substring(
        prosrc from position('WHERE mapping.privacy_policy_version' in prosrc)
      )) > 0)::text
      || ':' ||
      (position('approval_state' in substring(
        prosrc
        from position('LEFT JOIN core.stays' in prosrc)
        for  position('WHERE mapping.privacy_policy_version' in prosrc)
             - position('LEFT JOIN core.stays' in prosrc)
      )) = 0)::text
    FROM pg_proc
    WHERE oid = 'analytics.aggregate_eligible_preferences(text, timestamptz, timestamptz)'::regprocedure
  "
)"
assert_equals \
  "the approval predicate is missing from the WHERE or leaked into the LEFT JOIN of analytics.aggregate_eligible_preferences" \
  "true:true" "${preference_filter}"

# O owner, o search_path e o SECURITY DEFINER precisam sobreviver ao
# CREATE OR REPLACE, senão o dump diverge sem que nada falhe em execução.
preference_definer="$(
  migration_query "
    SELECT pg_get_userbyid(proowner)
      || ':' || prosecdef::text
      || ':' || array_to_string(proconfig, ',')
    FROM pg_proc
    WHERE oid = 'analytics.aggregate_eligible_preferences(text, timestamptz, timestamptz)'::regprocedure
  "
)"
assert_equals \
  "analytics.aggregate_eligible_preferences lost its owner, SECURITY DEFINER or search_path" \
  "migration_admin:true:search_path=pg_catalog" "${preference_definer}"

# N-40: public_runtime continua sem alcance sobre core depois da fase.
if psql_as cumuru_public cumuru-local-public-only \
  --command="SELECT count(*) FROM core.stays" >/dev/null 2>&1; then
  echo "public_runtime unexpectedly read core.stays after the Phase 7 migration" >&2
  exit 1
fi

# As asserções acima são de schema. A prova de eliminação da ADR-040 (N-29 e
# N-30, varredura de information_schema após rejeição e após expiração) vive na
# suíte Go de integração, e sem esta invocação o build da fase nunca a exercita.
published_address="$("${COMPOSE[@]}" port postgres 5432)"
database_port="${published_address##*:}"
case "${database_port}" in
  "" | *[!0-9]*)
    echo "could not resolve the ephemeral PostgreSQL port" >&2
    exit 1
    ;;
esac

admin_dsn="postgres://cumuru_migration:cumuru-local-migration-only@127.0.0.1:${database_port}/cumuru?sslmode=disable"
runtime_dsn="postgres://cumuru_app:cumuru-local-app-only@127.0.0.1:${database_port}/cumuru?sslmode=disable"

CUMURU_TEST_ADMIN_DATABASE_URL="${admin_dsn}" \
  CUMURU_TEST_DATABASE_URL="${runtime_dsn}" \
  go -C "${ROOT_DIR}/apps/api" test \
  -tags=integration -race -count=1 ./internal/platform/store

# --- N-38: o ponto 3 do filtro, provado por EXECUÇÃO ---------------------------
#
# A asserção posicional acima inspeciona o texto da função e nunca a chama. Ela
# detecta o predicado no lugar errado, mas não prova comportamento: se a cadeia
# metric_mappings → questions → answers → responses → consent_decisions → stays
# nunca alcançasse a função, o texto continuaria correto e o filtro continuaria
# valendo nada. Aqui a função é chamada de verdade, duas vezes.
#
# A chamada é feita como cumuru_worker porque EXECUTE é concedido somente a
# worker_runtime; a função é SECURITY DEFINER e lê as tabelas como migration_admin.
psql_as cumuru_migration cumuru-local-migration-only <<'SQL'
-- Transação única: survey.responses tem trigger de constraint diferida que
-- compara decisões com requisitos de consentimento. Em autocommit a resposta
-- seria validada antes de as decisões existirem e o INSERT falharia com
-- "survey decisions do not match the version snapshot".
BEGIN;

INSERT INTO core.accommodations (id, organization_id, name, status, category, capacity)
VALUES (
  '00000000-0000-7000-8000-000000000b02',
  '00000000-0000-7000-8000-0000000007a1',
  'Pousada Fictícia N38',
  'active', 'formal_lodging', 8
);

-- Estadia assistida: approval_state nulo, portanto sempre elegível.
INSERT INTO core.stays (
  id, accommodation_id, created_by_membership_id, client_submission_id,
  planned_arrival_on, planned_departure_on, expected_guest_count
) VALUES (
  '00000000-0000-7000-8000-000000000c01',
  '00000000-0000-7000-8000-0000000007b1',
  '00000000-0000-7000-8000-0000000007c1',
  '00000000-0000-7000-8000-000000000c02',
  '2026-05-02', '2026-05-06', 2
);

-- Estadia autocadastrada pendente: é ela que o predicado tem de excluir.
INSERT INTO core.stays (
  id, accommodation_id, status, client_submission_id,
  planned_arrival_on, planned_departure_on, expected_guest_count,
  provenance, approval_state, approval_expires_at
) VALUES (
  '00000000-0000-7000-8000-000000000c03',
  '00000000-0000-7000-8000-000000000b02',
  'pre_registered',
  '00000000-0000-7000-8000-000000000c04',
  '2026-05-03', '2026-05-07', 2,
  'self_service', 'pending', now() + interval '72 hours'
);

INSERT INTO survey.questionnaires (id, stable_key, name)
VALUES ('00000000-0000-7000-8000-000000000d01', 'perfil_n38', 'Perfil Fictício N38');

INSERT INTO survey.questionnaire_versions (
  id, questionnaire_id, version_number, status, title,
  privacy_notice_version, last_editor_hmac, last_editor_key_version, published_at
) VALUES (
  '00000000-0000-7000-8000-000000000d02',
  '00000000-0000-7000-8000-000000000d01',
  1, 'draft', 'Perfil Fictício N38', '2026-01-01',
  decode(repeat('e1', 32), 'hex'), 'v1', NULL
);

-- public_aggregation_allowed, analytics_key e minimum_public_cell são exigidos
-- pelo WHERE da função; answer_type e data_classification precisam ficar fora
-- das listas que ela exclui.
INSERT INTO survey.questions (
  id, questionnaire_version_id, stable_key, prompt, answer_type, required,
  data_classification, purpose_code, retention_policy_code, analytics_key,
  public_aggregation_allowed, minimum_public_cell, display_order
) VALUES (
  '00000000-0000-7000-8000-000000000d03',
  '00000000-0000-7000-8000-000000000d02',
  'primeira_visita', 'Primeira visita?', 'single_choice', true,
  'operational', 'estatistica_publica', 'padrao', 'visit_profile',
  true, 10, 1
);

INSERT INTO survey.consent_requirements (
  questionnaire_version_id, purpose_code, notice_version, prompt,
  required_for_answers, display_order
) VALUES (
  '00000000-0000-7000-8000-000000000d02',
  'estatistica_publica', '2026-01-01', 'Aceita uso agregado?', false, 1
);

-- A publicação respeita a máquina de estados da Fase 3:
-- draft -> privacy_review -> approved -> published. Pular etapa dispara
-- "invalid questionnaire version transition".
UPDATE survey.questionnaire_versions SET status = 'privacy_review'
 WHERE id = '00000000-0000-7000-8000-000000000d02';
UPDATE survey.questionnaire_versions SET status = 'approved'
 WHERE id = '00000000-0000-7000-8000-000000000d02';
UPDATE survey.questionnaire_versions
   SET status = 'published', published_at = '2026-04-01'
 WHERE id = '00000000-0000-7000-8000-000000000d02';

INSERT INTO analytics.metric_mappings (
  privacy_policy_version, metric_code, questionnaire_version_id, question_id,
  source_value, category_code
) VALUES (
  'prototype-v1', 'first_visit_share',
  '00000000-0000-7000-8000-000000000d02',
  '00000000-0000-7000-8000-000000000d03',
  'first_visit', 'first_visit'
);

-- Uma resposta por estadia, ambas dentro do mês civil de maio/2026, ambas com
-- consentimento concedido. A única diferença entre as duas é a aprovação.
INSERT INTO survey.capabilities (
  id, token_hmac, token_key_version, purpose, stay_id,
  questionnaire_version_id, expires_at
) VALUES
  ('00000000-0000-7000-8000-000000000e01', decode(repeat('f1', 32), 'hex'), 'v1',
   'survey_response', '00000000-0000-7000-8000-000000000c01',
   '00000000-0000-7000-8000-000000000d02', now() + interval '30 days'),
  ('00000000-0000-7000-8000-000000000e02', decode(repeat('f2', 32), 'hex'), 'v1',
   'survey_response', '00000000-0000-7000-8000-000000000c03',
   '00000000-0000-7000-8000-000000000d02', now() + interval '30 days');

INSERT INTO survey.responses (
  id, stay_id, questionnaire_version_id, capability_id, client_submission_id,
  participation, submitted_at
) VALUES
  ('00000000-0000-7000-8000-000000000e03',
   '00000000-0000-7000-8000-000000000c01',
   '00000000-0000-7000-8000-000000000d02',
   '00000000-0000-7000-8000-000000000e01',
   '00000000-0000-7000-8000-000000000e05', 'submitted', '2026-05-10 12:00:00-03'),
  ('00000000-0000-7000-8000-000000000e04',
   '00000000-0000-7000-8000-000000000c03',
   '00000000-0000-7000-8000-000000000d02',
   '00000000-0000-7000-8000-000000000e02',
   '00000000-0000-7000-8000-000000000e06', 'submitted', '2026-05-11 12:00:00-03');

INSERT INTO survey.answers (
  id, response_id, questionnaire_version_id, question_id, structured_value
) VALUES
  ('00000000-0000-7000-8000-000000000e07',
   '00000000-0000-7000-8000-000000000e03',
   '00000000-0000-7000-8000-000000000d02',
   '00000000-0000-7000-8000-000000000d03', '"first_visit"'::jsonb),
  ('00000000-0000-7000-8000-000000000e08',
   '00000000-0000-7000-8000-000000000e04',
   '00000000-0000-7000-8000-000000000d02',
   '00000000-0000-7000-8000-000000000d03', '"first_visit"'::jsonb);

INSERT INTO survey.consent_decisions (
  id, response_id, questionnaire_version_id, purpose_code, notice_version,
  granted, collection_channel
) VALUES
  ('00000000-0000-7000-8000-000000000e09',
   '00000000-0000-7000-8000-000000000e03',
   '00000000-0000-7000-8000-000000000d02',
   'estatistica_publica', '2026-01-01', true, 'survey_capability'),
  ('00000000-0000-7000-8000-000000000e0a',
   '00000000-0000-7000-8000-000000000e04',
   '00000000-0000-7000-8000-000000000d02',
   'estatistica_publica', '2026-01-01', true, 'survey_capability');

COMMIT;
SQL

aggregate_preferences() {
  psql_as cumuru_worker cumuru-local-worker-only --tuples-only --no-align --command="
    SELECT category_code || ':' || sample_size || ':' || accommodation_count
    FROM analytics.aggregate_eligible_preferences(
      'prototype-v1',
      timestamptz '2026-05-01 00:00:00-03',
      timestamptz '2026-06-01 00:00:00-03'
    ) AS agregado(
      privacy_policy_version text,
      metric_code text,
      category_code text,
      sample_size bigint,
      accommodation_count bigint,
      minimum_public_cell integer
    )
  " | tr -d '[:space:]'
}

# Com a pendente ainda pendente: só a assistida conta.
pending_excluded="$(aggregate_preferences)"
assert_equals \
  "analytics.aggregate_eligible_preferences counted a stay awaiting approval" \
  "first_visit:1:1" "${pending_excluded}"

# Aprovar é a ÚNICA mudança entre as duas chamadas. Sem esta segunda metade,
# "não contou" passaria também se a fixture estivesse vazia ou se a cadeia nunca
# alcançasse a função — que é o modo de falha que esta fase encontrou oito vezes.
psql_as cumuru_migration cumuru-local-migration-only --command="
  UPDATE core.stays
     SET approval_state = 'approved',
         approved_at = now(),
         approval_decided_by_membership_id = '00000000-0000-7000-8000-0000000007c1',
         approval_expires_at = NULL
   WHERE id = '00000000-0000-7000-8000-000000000c03'
" >/dev/null

approved_included="$(aggregate_preferences)"
assert_equals \
  "analytics.aggregate_eligible_preferences ignored a stay that was just approved" \
  "first_visit:2:2" "${approved_included}"

echo "phase 7 PostgreSQL integration passed: unlimited-invite silent failure reproduced and fixed, invite targeting, stay provenance and approval invariants, pending account credential state, proof-of-work nonce ledger and the approval filter in the SECURITY DEFINER aggregate proved by calling it: pending excluded, approved counted"
