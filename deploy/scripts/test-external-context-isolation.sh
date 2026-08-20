#!/usr/bin/env bash
set -euo pipefail

# Gate de isolamento da camada de contexto externo (ADR-045, Fase 8).
#
# Prova três coisas com o upstream PARADO, e nenhuma delas depende de rede
# pública: a fase B fixa os hosts do Open-Meteo no loopback do contêiner, então
# a tentativa de egresso é recusada localmente.
#
#   1. as quatro rotas públicas de analytics respondem corpo e ETag idênticos
#      com e sem a camada externa (critério central da onda);
#   2. `/public/context` responde 200 com card `unavailable`, nunca 503 por
#      fonte de terceiro;
#   3. o card de maré e o crédito ao Cadastur existem no documento servido, que
#      é o que a matriz da Fase 8 e a decisão U-7 exigem.
#
# A fase A mede a linha de base com a camada desligada; a fase B recria api e
# worker com ela ligada e compara byte a byte.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROJECT_NAME="cumuru-external-context-${PPID}-$$"
. "${ROOT_DIR}/deploy/scripts/lib/compose-subnet.sh"
cumuru_acquire_subnet external-context

BASE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/cumuru-external-context.XXXXXX")"
WORKER_LOG="${BASE_DIR}/worker.log"
EVIDENCE_DIR="${TMPDIR:-/tmp}/cumuru-external-context-evidence"
rm -rf -- "${EVIDENCE_DIR}"

build_metadata=()
while IFS= read -r value; do
  build_metadata+=("${value}")
done < <(
  "${ROOT_DIR}/deploy/scripts/with-build-metadata.sh" \
    sh -c 'printf "%s\n%s\n%s\n" \
      "${CUMURU_BUILD_VERSION}" \
      "${CUMURU_BUILD_REVISION}" \
      "${CUMURU_BUILD_TIME}"'
)
test "${#build_metadata[@]}" = "3"
export CUMURU_BUILD_VERSION="${build_metadata[0]}"
export CUMURU_BUILD_REVISION="${build_metadata[1]}"
export CUMURU_BUILD_TIME="${build_metadata[2]}"

LOCAL_ENV_FILE="${ROOT_DIR}/.env"
test -f "${LOCAL_ENV_FILE}" || LOCAL_ENV_FILE="${ROOT_DIR}/.env.example"

COMPOSE=(
  docker compose
  --env-file "${LOCAL_ENV_FILE}"
  --file "${ROOT_DIR}/compose.yaml"
  --file "${ROOT_DIR}/compose.local.yaml"
  --file "${ROOT_DIR}/deploy/compose.external-context.yaml"
  --project-name "${PROJECT_NAME}"
)

COMPOSE_ENABLED=(
  "${COMPOSE[@]}"
  --file "${ROOT_DIR}/deploy/compose.external-context-enabled.yaml"
)

cleanup() {
  local primary_status=$?
  local cleanup_status=0
  local residual=""
  trap - EXIT
  set +e
  "${COMPOSE_ENABLED[@]}" down --volumes --remove-orphans
  residual="$(
    docker container ls --all --quiet \
      --filter "label=com.docker.compose.project=${PROJECT_NAME}"
  )"
  if test -n "${residual}"; then
    echo "external context gate left containers behind" >&2
    cleanup_status=1
  fi
  if test "${primary_status}" -ne 0; then
    # Evidência sobrevive ao teardown quando o gate reprova: a comparação sem os
    # dois corpos é uma afirmação sem prova.
    cp -R "${BASE_DIR}" "${EVIDENCE_DIR}" 2>/dev/null || true
    echo "external context gate evidence kept at ${EVIDENCE_DIR}" >&2
  fi
  rm -rf -- "${BASE_DIR}"
  set -e
  cumuru_release_subnet
  if test "${primary_status}" -ne 0; then
    exit "${primary_status}"
  fi
  if test "${cleanup_status}" -ne 0; then
    exit "${cleanup_status}"
  fi
  echo "external context gate cleanup confirmed"
}
trap cleanup EXIT

psql_migration() {
  "${COMPOSE[@]}" exec -T \
    -e PGPASSWORD=cumuru-local-migration-only \
    postgres psql --no-psqlrc --set=ON_ERROR_STOP=1 \
    --username=cumuru_migration --dbname=cumuru "$@"
}

# Devolve o status e grava headers e corpo. Não falha por status: quem chama
# decide, porque este gate mede justamente qual status a camada produz.
request_public() {
  local path="$1"
  local headers_path="$2"
  local body_path="$3"
  local status=""
  status="$(
    "${COMPOSE[@]}" exec -T \
      -e "REQUEST_PATH=${path}" \
      web sh -eu -c '
        curl \
          --silent \
          --show-error \
          --dump-header /tmp/external-context.headers \
          --output /tmp/external-context.body \
          --write-out "%{http_code}" \
          "http://127.0.0.1:8080${REQUEST_PATH}"
      '
  )"
  "${COMPOSE[@]}" exec -T web \
    sh -eu -c 'cat /tmp/external-context.headers' >"${headers_path}"
  "${COMPOSE[@]}" exec -T web \
    sh -eu -c 'cat /tmp/external-context.body' >"${body_path}"
  printf '%s\n' "${status}"
}

header_value() {
  local name="$1"
  local path="$2"
  tr -d '\r' <"${path}" |
    awk -v expected="${name}" '
      tolower($1) == tolower(expected ":") {
        sub(/^[^:]+:[[:space:]]*/, "")
        print
        exit
      }
    '
}

# As quatro rotas protegidas, com o seletor que cada uma exige: `presence` e
# `preferences` recusam requisição sem ele, e uma comparação feita sobre 400
# provaria apenas que os dois erros são iguais.
PROTECTED_ROUTES=(
  "summary:/api/v1/public/summary"
  "presence:/api/v1/public/presence?window=recent_30_days"
  "preferences:/api/v1/public/preferences?period=last_complete_month"
  "methodology:/api/v1/public/methodology"
)

# Cada captura é auto-validante. As quatro requisições levam segundos, e uma
# reconciliação disparada no meio delas deixaria os quatro corpos incoerentes
# entre si — sem que nenhuma comparação posterior conseguisse perceber. Medir a
# release antes e depois transforma isso em falha explícita, com o nome da causa,
# em vez de um diff de ETag que parece regressão da onda.
capture_protected() {
  local phase="$1"
  local entry=""
  local name=""
  local path=""
  local status=""
  local opened=""
  local closed=""
  opened="$(release_fingerprint)"
  test -n "${opened}"
  for entry in "${PROTECTED_ROUTES[@]}"; do
    name="${entry%%:*}"
    path="${entry#*:}"
    status="$(
      request_public \
        "${path}" \
        "${BASE_DIR}/${phase}-${name}.headers" \
        "${BASE_DIR}/${phase}-${name}.body"
    )"
    if test "${status}" != "200"; then
      echo "public/${name} answered ${status} in phase ${phase}" >&2
      return 1
    fi
  done
  closed="$(release_fingerprint)"
  if test "${opened}" != "${closed}"; then
    echo "the protected release changed during phase ${phase}:" >&2
    echo "  before: ${opened}" >&2
    echo "  after:  ${closed}" >&2
    echo "the analytics reconciler must be frozen before measuring" >&2
    return 1
  fi
  printf '%s\n' "${opened}" >"${BASE_DIR}/${phase}.release"
}

# A release medida em duas fases tem de ser a mesma, senão a identidade byte a
# byte não afirma nada sobre a camada externa.
assert_same_release() {
  local left="$1"
  local right="$2"
  if ! cmp --silent "${BASE_DIR}/${left}.release" "${BASE_DIR}/${right}.release"; then
    echo "the protected release changed between ${left} and ${right}:" >&2
    echo "  ${left}:  $(cat "${BASE_DIR}/${left}.release")" >&2
    echo "  ${right}: $(cat "${BASE_DIR}/${right}.release")" >&2
    return 1
  fi
}

wait_for_publication() {
  local ready=""
  local attempt=0
  while test "${attempt}" -lt 90; do
    ready="$(
      psql_migration --tuples-only --no-align \
        --command="
          SELECT COALESCE(
            (
              SELECT publication_version::text
              FROM public_data.current_publication
              WHERE singleton
            ),
            ''
          )
        "
    )"
    if test -n "${ready}"; then
      return 0
    fi
    attempt=$((attempt + 1))
    sleep 1
  done
  echo "worker did not create the initial analytics publication" >&2
  return 1
}

compare_phases() {
  local left="$1"
  local right="$2"
  local label="$3"
  local entry=""
  local route=""
  local left_etag=""
  local right_etag=""
  local identical=true
  for entry in "${PROTECTED_ROUTES[@]}"; do
    route="${entry%%:*}"
    left_etag="$(header_value ETag "${BASE_DIR}/${left}-${route}.headers")"
    right_etag="$(header_value ETag "${BASE_DIR}/${right}-${route}.headers")"
    test -n "${left_etag}"
    if test "${left_etag}" != "${right_etag}"; then
      echo "${label}: public/${route} ETag differs" >&2
      identical=false
    fi
    if ! cmp --silent \
      "${BASE_DIR}/${left}-${route}.body" "${BASE_DIR}/${right}-${route}.body"; then
      echo "${label}: public/${route} body differs" >&2
      identical=false
    fi
  done
  test "${identical}" = "true"
}

# A impressão digital da release protegida, e não só a versão: `published_at` e
# `coverage_ratio_percent` são exatamente os dois campos que a reconciliação move
# — eles aparecem no payload como `updated_at` e `coverage.ratio`. Comparar só a
# versão deixaria passar um replantio que mudasse o conteúdo, e comparar o corpo
# devolveria a diferença como se fosse regressão da camada externa.
release_fingerprint() {
  psql_migration --tuples-only --no-align \
    --command="
      SELECT concat_ws(
        '|',
        publications.publication_version::text,
        publications.build_fingerprint,
        publications.published_at::text,
        coalesce(publications.coverage_ratio_percent::text, 'null')
      )
      FROM public_data.current_publication AS current
      JOIN public_data.publications AS publications
        ON publications.publication_version = current.publication_version
      WHERE current.singleton
    "
}

# A espera é por fonte, não global: a fase C insere a observação com
# `SELECT ... FROM external.fetch_runs WHERE source_code = 'open_meteo_forecast'`,
# e uma run de outra fonte satisfaria uma espera global sem satisfazer aquele
# SELECT. O INSERT gravaria zero linha, o script seguiria, e a falha apareceria
# depois como "weather card did not publish" — nomeando a causa errada.
wait_for_fetch_run() {
  local source_code="${1:-open_meteo_forecast}"
  local runs=""
  local attempt=0
  while test "${attempt}" -lt 90; do
    runs="$(
      psql_migration --tuples-only --no-align \
        --command="SELECT count(*) FROM external.fetch_runs
          WHERE source_code = '${source_code}'"
    )"
    if test "${runs}" -gt 0; then
      return 0
    fi
    attempt=$((attempt + 1))
    sleep 1
  done
  echo "external ingestion never recorded a fetch run for ${source_code}" >&2
  return 1
}

# ---------------------------------------------------------------- fase A
"${COMPOSE[@]}" up --detach --wait postgres
"${COMPOSE[@]}" run --rm --no-deps migrate
"${COMPOSE[@]}" run --build --rm --no-deps local-demo
# api e worker carregam o código da onda e por isso são construídos; `web` é
# apenas a borda que serve o painel e origina o curl, e reusa a imagem local
# já publicada pelo build do repositório.
"${COMPOSE[@]}" up --build --detach --wait api worker
"${COMPOSE[@]}" up --detach --wait web
wait_for_publication

# O worker da fase A reconcilia a cada 500 ms. Uma estadia que termina, ou a
# simples virada do minuto, republica e move `updated_at` e `coverage.ratio`
# sozinha — foi assim que a medida anterior virou corrida de relógio. Parar o
# worker aqui, antes da primeira requisição, é o que torna a janela inteira de
# medição imóvel. Ele volta na fase B já com analytics desligado.
"${COMPOSE[@]}" stop worker
capture_protected before

# Camada desligada: a rota não é registrada, e isso é 404 e não meia-superfície.
context_status="$(
  request_public \
    "/api/v1/public/context" \
    "${BASE_DIR}/before-context.headers" \
    "${BASE_DIR}/before-context.body"
)"
test "${context_status}" = "404"
echo "EXTERNAL_CONTEXT_ROUTE_ABSENT_WHEN_DISABLED=PASS"

# ---------------------------------------------------------------- fase B
# A fase B recria api e worker com a camada ligada e recria `web` junto: o nginx
# resolve o upstream `api` na subida e guarda o endereço, então um contêiner de
# api novo atrás de um nginx velho responde 502 por DNS obsoleto, não por
# comportamento da aplicação.
"${COMPOSE_ENABLED[@]}" up --detach --wait --force-recreate api worker
"${COMPOSE_ENABLED[@]}" up --detach --wait --force-recreate web
wait_for_fetch_run

capture_protected after
assert_same_release before after

compare_phases before after "external layer enabled"
echo "EXTERNAL_CONTEXT_PROTECTED_ROUTES_IDENTICAL=PASS"

# O egresso saiu, foi recusado localmente e o log não carrega URL com query,
# corpo nem headers.
"${COMPOSE[@]}" logs --no-color worker >"${WORKER_LOG}" 2>&1
grep --fixed-strings --quiet "external fetch finished" "${WORKER_LOG}"
for forbidden in "latitude=" "timezone=" "forecast_days" "https://api.open-meteo"; do
  if grep --fixed-strings --quiet "${forbidden}" "${WORKER_LOG}"; then
    echo "egress log leaked the request URL: ${forbidden}" >&2
    exit 1
  fi
done
echo "EXTERNAL_CONTEXT_EGRESS_LOG_BOUNDED=PASS"

# A sexta view não está na lista negativa de `external_runtime` em
# test-migrations.sh (só a quinta está). A ADR-045 avisa que a asserção negativa
# é cega ao que não varre, então a varredura é feita aqui.
echo "--- external_runtime não alcança as views públicas ---"
for public_view in current_external_context current_external_sources; do
  if "${COMPOSE[@]}" exec -T \
    -e PGPASSWORD=cumuru-local-external-only \
    postgres psql --no-psqlrc --set=ON_ERROR_STOP=1 \
    --username=cumuru_external --dbname=cumuru \
    --command="SELECT count(*) FROM public_data.${public_view}" >/dev/null 2>&1; then
    echo "external_runtime unexpectedly read public_data.${public_view}" >&2
    exit 1
  fi
done
echo "EXTERNAL_CONTEXT_INGESTION_ROLE_BLIND_TO_PUBLIC_VIEWS=PASS"

# Estado da camada no banco, impresso antes da requisição: um 503 sem isto vira
# adivinhação sobre qual das duas metades do documento ficou vazia.
echo "--- estado de external no banco ---"
psql_migration --command="
  SELECT
    (SELECT count(*) FROM external.sources) AS sources,
    (SELECT count(*) FROM external.sources WHERE active) AS active_sources,
    (SELECT count(*) FROM external.series) AS series,
    (SELECT count(*) FROM external.series WHERE public_exposable) AS exposable,
    (SELECT count(*) FROM external.fetch_runs) AS fetch_runs,
    (SELECT count(*) FROM external.observations) AS observations,
    (SELECT count(*) FROM public_data.current_external_context) AS view_rows
"
psql_migration --command="
  SELECT source_code, series_code, card_code, public_exposable
  FROM external.series ORDER BY source_code, series_code
"
psql_migration --command="
  SELECT source_code, active FROM external.sources ORDER BY source_code
"

# A rota nova responde 200 com card indisponível, nunca 503 por fonte morta.
context_status="$(
  request_public \
    "/api/v1/public/context" \
    "${BASE_DIR}/after-context.headers" \
    "${BASE_DIR}/after-context.body"
)"
echo "external context status with the upstream stopped: ${context_status}"
cat "${BASE_DIR}/after-context.body"
echo
test "${context_status}" = "200"
echo "EXTERNAL_CONTEXT_TWO_HUNDRED_WITH_UPSTREAM_STOPPED=PASS"

node - "${BASE_DIR}/after-context.body" <<'NODE'
const fs = require("node:fs");
const document = JSON.parse(fs.readFileSync(process.argv[2], "utf8"));
const forbidden = ["coverage", "ratio", "sample_size", "accommodation_count"];
const serialized = JSON.stringify(document);
for (const name of forbidden) {
  if (serialized.includes(name)) {
    throw new Error(`external payload carries ${name}`);
  }
}
const cards = new Map(document.cards.map((card) => [card.card_code, card]));
const weather = cards.get("weather_daily");
if (!weather || weather.status !== "unavailable") {
  throw new Error("weather card is not unavailable with the upstream stopped");
}
if (!weather.provenance || !weather.provenance.attribution_text) {
  throw new Error("unavailable card lost its provenance");
}
const tide = cards.get("tide");
if (!tide) {
  throw new Error("tide card is absent from the served document");
}
if (tide.status !== "unavailable" ||
  tide.reason_code !== "constants_not_imported") {
  throw new Error("tide card is not unavailable/constants_not_imported");
}
const credited = document.sources.map((source) => source.source_code);
if (!credited.includes("cadastur")) {
  throw new Error("Cadastur is not credited in the served document");
}
NODE
echo "EXTERNAL_CONTEXT_DOCUMENT_SHAPE=PASS"

# --------------------------------------------------- fase C: `external` cheio
# A matriz exige que a reconciliação de analytics produza digest idêntico com
# `external` **populado** e vazio. Até aqui `observations` estava em zero, então
# o "populado" não tinha sido medido — e o ramo `published` da camada nunca
# havia sido exercido contra PostgreSQL por gate nenhum.
#
# A observação é inserida sob `cumuru_external`, o mesmo papel da ingestão e com
# os mesmos privilégios: isto substitui o que o upstream teria respondido, não a
# aplicação. O que se prova aqui é o caminho de leitura — view, handler,
# payload — e a não interferência na série protegida; o caminho de coleta
# continua provado pelos testes de `Fetcher` e pelo egresso recusado da fase B.
"${COMPOSE[@]}" exec -T \
  -e PGPASSWORD=cumuru-local-external-only \
  postgres psql --no-psqlrc --set=ON_ERROR_STOP=1 \
  --username=cumuru_external --dbname=cumuru \
  --command="
    INSERT INTO external.observations (
      source_code, series_code, period_kind, period_start, period_end,
      revision, observed_value, quality_flag, retrieved_at, payload_digest,
      fetch_run_id
    )
    SELECT
      'open_meteo_forecast', 'temperature_2m_max', 'day',
      date_trunc('day', now()), date_trunc('day', now()) + interval '1 day',
      1, 26.1, 'ok', now(),
      repeat('a', 64), id
    FROM external.fetch_runs
    WHERE source_code = 'open_meteo_forecast'
    ORDER BY started_at DESC
    LIMIT 1
  " >/dev/null

context_status="$(
  request_public \
    "/api/v1/public/context" \
    "${BASE_DIR}/full-context.headers" \
    "${BASE_DIR}/full-context.body"
)"
test "${context_status}" = "200"
cat "${BASE_DIR}/full-context.body"
echo

node - "${BASE_DIR}/full-context.body" <<'NODE'
const fs = require("node:fs");
const document = JSON.parse(fs.readFileSync(process.argv[2], "utf8"));
const serialized = JSON.stringify(document);
for (const name of ["coverage", "ratio", "sample_size", "accommodation_count"]) {
  if (serialized.includes(name)) {
    throw new Error(`populated payload carries ${name}`);
  }
}
const weather = document.cards.find((card) => card.card_code === "weather_daily");
if (!weather || weather.status !== "published") {
  throw new Error("weather card did not publish with an observation present");
}
if (!Array.isArray(weather.series) || weather.series.length === 0) {
  throw new Error("published card carries no series point");
}
if (!weather.provenance.retrieved_at || !weather.provenance.attribution_text) {
  throw new Error("published card lost its provenance");
}
const tide = document.cards.find((card) => card.card_code === "tide");
if (!tide || tide.status !== "unavailable" ||
  tide.reason_code !== "constants_not_imported") {
  throw new Error("tide card changed when the weather series filled");
}
NODE
echo "EXTERNAL_CONTEXT_PUBLISHED_BRANCH=PASS"

# E o ponto que a matriz cobra: a série protegida não se move quando `external`
# deixa de estar vazio.
capture_protected full
assert_same_release before full
compare_phases before full "external populated"
echo "EXTERNAL_CONTEXT_PROTECTED_ROUTES_IDENTICAL_WHEN_POPULATED=PASS"

echo "EXTERNAL_CONTEXT_ISOLATION=PASS"
