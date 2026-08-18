#!/usr/bin/env bash
set -euo pipefail

# autoatendimento — autoatendimento e aprovação, stack completa.
#
# Este teste é escrito ANTES dos handlers, na disciplina de teste-primeiro do
# plano da fase: ele fica vermelho até a Wave B implementar o domínio, e o
# motivo da falha é explícito em vez de opaco. Um stub que sai com exit 2 diz
# apenas "não implementado"; este diz qual operação falta e em que ponto do
# fluxo.
#
# Cobertura pretendida, na ordem do fluxo real:
#   1. emissão da capability de ativação e ativação da conta sem e-mail;
#   2. emissão e rotação do cartaz reutilizável, com o token no fragmento;
#   3. autocadastro generalizado pelo cartaz, com proof-of-work;
#   4. recusa de role='minor' e de qualquer campo de identidade;
#   5. fila de aprovação, aprovação e rejeição com motivo de lista fechada;
#   6. a estadia pendente não produz presença nem alcança public_data;
#   7. pedido de convite da hospedagem: contexto aberto, criação sem eco do
#      contato, conflito do pendente repetido, escopo na fila, aprovação que
#      cria a acomodação e recusa que elimina o contato (ADR-042).

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROJECT_NAME="cumuru-self-service-full-stack-${PPID}-$$"
. "${ROOT_DIR}/deploy/scripts/lib/compose-subnet.sh"
cumuru_acquire_subnet self-service-full-stack

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

# Compose interpola `compose.local.yaml` inteiro, inclusive o serviço de seed
# sob profile, então as credenciais fictícias do demo são exigidas mesmo aqui.
# Em clone limpo e na CI não há `.env`; o exemplo versionado entra como origem.
LOCAL_ENV_FILE="${ROOT_DIR}/.env"
test -f "${LOCAL_ENV_FILE}" || LOCAL_ENV_FILE="${ROOT_DIR}/.env.example"

COMPOSE=(
  docker compose
  --env-file "${LOCAL_ENV_FILE}"
  --file "${ROOT_DIR}/compose.yaml"
  --file "${ROOT_DIR}/compose.local.yaml"
  --project-name "${PROJECT_NAME}"
)

cleanup() {
  local primary_status=$?
  trap - EXIT
  set +e
  "${COMPOSE[@]}" down --volumes --remove-orphans >/dev/null 2>&1
  local cleanup_status=$?
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

fail() {
  echo "SELF_SERVICE_FULL_STACK=FAIL $1" >&2
  exit 1
}

# O contrato é a fonte: as operações exercitadas aqui são exatamente as que o
# OpenAPI declara como autoatendimento, então acrescentar operação sem teste quebra esta
# asserção antes de quebrar o gate.
declared_operations="$(
  grep -c 'x-cumuru-feature: self-service' "${ROOT_DIR}/contracts/openapi.yaml" || true
)"
if test "${declared_operations}" -ne 15; then
  fail "the contract declares ${declared_operations} self-service operations, expected 15"
fi

for operation in \
  createAccommodationActivation \
  getAccommodationActivation \
  activateAccommodationAccount \
  createAccommodationInvite \
  getAccommodationInvite \
  revokeAccommodationInvite \
  getAccommodationInviteContext \
  submitAccommodationSelfRegistration \
  approveStay \
  rejectStay \
  getAccommodationAccessRequestContext \
  createAccommodationAccessRequest \
  listAccommodationAccessRequests \
  approveAccommodationAccessRequest \
  rejectAccommodationAccessRequest; do
  if ! grep -q "operationId: ${operation}$" "${ROOT_DIR}/contracts/openapi.yaml"; then
    fail "the contract lost operationId ${operation}"
  fi
  if ! grep -q "${operation}" "${ROOT_DIR}/apps/web/src/generated/schema.ts"; then
    fail "the generated web client does not expose ${operation}; run make generate"
  fi
done

"${COMPOSE[@]}" up --detach --wait postgres
"${COMPOSE[@]}" run --rm --no-deps migrate
# local-demo está fora do perfil padrão (perfil test); a aprovação abaixo
# entra como o operador fictício, então as fixtures dele precisam existir
# antes da API subir. `run --rm` alcança o serviço sem precisar de --profile;
# --build pelo mesmo motivo do api/web logo abaixo.
"${COMPOSE[@]}" run --build --rm --no-deps local-demo
# --build é obrigatório. Sem ele o gate roda contra a imagem que estiver na
# máquina, que pode ser anterior às rotas da fase: o teste ficaria verde, ou
# vermelho, por causa de um binário velho e não do código da árvore.
"${COMPOSE[@]}" up --detach --wait --build api web

# A API não é publicada no host: as requisições saem de dentro do container web,
# que é quem alcança o serviço, no mesmo padrão do gate de analytics.
#
# Sem eval, de propósito. A versão anterior montava a linha de comando numa
# string e a passava a `eval`, e duas chamadas morriam com "unterminated quoted
# string" sem derrubar o teste: o corpo vinha vazio e a asserção seguinte passava
# no vácuo. Aqui cada valor viaja como variável de ambiente e o shell interno
# nunca reinterpreta aspas.
ORIGIN="http://127.0.0.1:4173"

web_curl() {
  local method="$1"
  local path="$2"
  local header="$3"
  local header2="$4"
  local flags="$5"
  "${COMPOSE[@]}" exec -T \
    -e "CURL_METHOD=${method}" \
    -e "CURL_PATH=${path}" \
    -e "CURL_HEADER=${header}" \
    -e "CURL_HEADER2=${header2}" \
    -e "CURL_FLAGS=${flags}" \
    web sh -eu -c '
      set -- --silent --request "${CURL_METHOD}"
      if test -n "${CURL_HEADER}"; then
        set -- "$@" --header "${CURL_HEADER}"
      fi
      if test -n "${CURL_HEADER2}"; then
        set -- "$@" --header "${CURL_HEADER2}"
      fi
      # shellcheck disable=SC2086
      curl "$@" ${CURL_FLAGS} "http://127.0.0.1:8080${CURL_PATH}"
    '
}

# Toda leitura passa por aqui: resposta vazia significa que o curl não rodou, e
# isso é falha do teste, não sucesso silencioso.
web_read() {
  local description="$1"
  shift
  local output=""
  output="$(web_curl "$@")"
  if test -z "${output}"; then
    fail "${description}: the request produced no output; the probe did not run"
  fi
  printf '%s' "${output}"
}

request_status() {
  web_read "$1" "$2" "$3" "$4" "" "--output /dev/null --write-out %{http_code}"
}

request_body() {
  web_read "$1" "$2" "$3" "$4" "" ""
}

request_headers() {
  web_read "$1" "$2" "$3" "$4" "" "--output /dev/null --dump-header -"
}

expect_status() {
  local description="$1"
  local expected="$2"
  local actual="$3"
  if test "${actual}" != "${expected}"; then
    fail "${description}: expected ${expected}, got ${actual}"
  fi
}

# ---------------------------------------------------------------------------
# GUARDA FAIL-CLOSED. Precede toda asserção.
#
# Sem SELF_SERVICE_ENABLED as rotas do canal aberto não são registradas, e TODA
# asserção de 404 abaixo passaria por ausência de rota em vez de por
# comportamento do handler — verde com o handler arbitrariamente quebrado.
#
# O discriminador é o preflight: as rotas da fase vivem sob inviteCORS, que
# responde 204 ao OPTIONS e devolve o allowlist de cabeçalhos. Rota ausente
# devolve 404 ao mesmo OPTIONS. É prova de rota registrada sem tocar o banco e
# sem depender de fixture alguma.
feature_probe="$(request_status \
  "the self-service preflight" OPTIONS /api/v1/accommodation-invite \
  "Origin: ${ORIGIN}" "Access-Control-Request-Method: GET")"
if test "${feature_probe}" = "404"; then
  fail "SELF-SERVICE IS DISABLED in this stack: the open channel routes are not registered, so every assertion below would pass by absence of route. Set SELF_SERVICE_ENABLED and the proof-of-work keyring in the Compose files."
fi
expect_status "the self-service preflight" "204" "${feature_probe}"

# O transporte por cabeçalho precisa estar no allowlist do preflight, senão o
# navegador recusa a requisição real e o canal aberto fica inalcançável.
preflight_headers="$(request_headers \
  "the self-service preflight allowlist" OPTIONS /api/v1/accommodation-invite \
  "Origin: ${ORIGIN}" "Access-Control-Request-Method: GET")"
case "${preflight_headers}" in
  *X-Cumuru-Invite-Token*) ;;
  *) fail "the preflight allowlist does not carry X-Cumuru-Invite-Token" ;;
esac

# ---------------------------------------------------------------------------
# Com a fase ligada, os 404 abaixo vêm do handler e não da ausência de rota.

expect_status \
  "GET /accommodation-invite with a well-formed unknown token" "404" \
  "$(request_status "unknown poster token" GET /api/v1/accommodation-invite \
    'X-Cumuru-Invite-Token: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' '')"

expect_status \
  "GET /accommodation-invite with no token header" "404" \
  "$(request_status "absent poster token" GET /api/v1/accommodation-invite '' '')"

# Ausente, incorreto, expirado, consumido e revogado precisam ser o mesmo 404.
# request_id é correlação por requisição e difere entre duas chamadas do MESMO
# caso, então sai da comparação; o que tem de coincidir é type, title e status.
strip_request_id() {
  sed 's/,"request_id":"[^"]*"//'
}
unknown_body="$(request_body "unknown poster token body" GET \
  /api/v1/accommodation-invite 'X-Cumuru-Invite-Token: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' '' | strip_request_id)"
absent_body="$(request_body "absent poster token body" GET \
  /api/v1/accommodation-invite '' '' | strip_request_id)"
if test -z "${unknown_body}"; then
  fail "the 404 body is empty; the comparison below would pass vacuously"
fi
if test "${unknown_body}" != "${absent_body}"; then
  fail "the 404 body distinguishes an unknown token from an absent one; a probe could tell them apart"
fi

# O token no caminho não pode existir: se responder algo que não seja 404, o
# transporte regrediu para a linha de requisição e o token do cartaz passa a ser
# gravado em access log.
expect_status \
  "a path-based accommodation invite route" "404" \
  "$(request_status "path-based poster route" GET \
    /api/v1/accommodation-invites/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa '' '')"

expect_status \
  "GET /activation with an unknown capability" "404" \
  "$(request_status "unknown activation capability" GET /api/v1/activation \
    'X-Cumuru-Activation-Token: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' '')"

# A fila de aprovação não é endpoint novo: é filtro de listStays, e sem sessão
# precisa ser recusada por autenticação, não por rota inexistente.
expect_status \
  "GET /stays?approval_state=pending without a session" "401" \
  "$(request_status "approval queue without a session" GET \
    '/api/v1/stays?approval_state=pending' '' '')"

# N-20 (parcial): nenhuma resposta do fluxo aberto pode emitir cookie. O convite
# do núcleo não usa cookie de propósito — é o que faz Idempotency-Key e JSON
# estrito forçarem preflight — e um cookie aqui traria de volta a superfície CSRF
# e criaria identificador persistente no aparelho do hóspede.
poster_headers="$(request_headers "poster response headers" GET \
  /api/v1/accommodation-invite 'X-Cumuru-Invite-Token: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' '')"
case "$(printf '%s' "${poster_headers}" | tr 'A-Z' 'a-z')" in
  *set-cookie:*) fail "/accommodation-invite emitted a cookie; the open flow must stay cookie-free" ;;
esac

activation_headers="$(request_headers "activation response headers" GET \
  /api/v1/activation 'X-Cumuru-Activation-Token: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' '')"
case "$(printf '%s' "${activation_headers}" | tr 'A-Z' 'a-z')" in
  *set-cookie:*) fail "/activation emitted a cookie; the open flow must stay cookie-free" ;;
esac

# ---------------------------------------------------------------------------
# Par da prova de trabalho, pelo fio real.
#
# Exercita a cadeia inteira: emissor Go cunha o desafio em GET
# /accommodation-invite, o desafio viaja no corpo, a solução volta em POST
# /accommodation-invite/submit, o verificador Go a aceita e o livro de nonces
# registra o gasto. O harness cruzado que o QA montou provava só a concordância
# de encoding entre duas funções; isto prova a cadeia.

web_request() {
  local method="$1"
  local path="$2"
  local body="$3"
  local flags="$4"
  "${COMPOSE[@]}" exec -T \
    -e "M=${method}" -e "P=${path}" -e "B=${body}" -e "F=${flags}" \
    -e "H1=${5:-}" -e "H2=${6:-}" -e "H3=${7:-}" \
    web sh -eu -c '
      set -- --silent --request "$M"
      for header in "$H1" "$H2" "$H3"; do
        if test -n "${header}"; then
          set -- "$@" --header "${header}"
        fi
      done
      if test -n "$B"; then
        set -- "$@" --header "Content-Type: application/json" --data "$B"
      fi
      # shellcheck disable=SC2086
      curl "$@" ${F} "http://127.0.0.1:8080${P}"
    '
}

json_field() {
  JSON_PAYLOAD="$1" JSON_FIELD="$2" node -e '
    const payload = JSON.parse(process.env.JSON_PAYLOAD);
    const value = process.env.JSON_FIELD
      .split(".")
      .reduce((node, key) => (node == null ? node : node[key]), payload);
    if (value === undefined || value === null) {
      process.exit(3);
    }
    process.stdout.write(String(value));
  '
}

DEMO_EMAIL="operador@cumuru.local"
DEMO_PASSWORD="${LOCAL_DEMO_ACCOUNT_PASSWORD:-demonstracao-local-2026}"

login_payload="$(web_request POST /api/v1/auth/login \
  "{\"email\":\"${DEMO_EMAIL}\",\"password\":\"${DEMO_PASSWORD}\"}" "")"
session_token="$(json_field "${login_payload}" token 2>/dev/null || true)"
if test -z "${session_token}"; then
  fail "could not open a session for ${DEMO_EMAIL}; the proof-of-work pair cannot be exercised"
fi
authorization="Authorization: Bearer ${session_token}"

# Duas acomodações distintas, uma por cartaz. O controle de desafio alheio
# precisa de dois convites vivos ao mesmo tempo, e a rotação sobre a mesma
# acomodação não serve para isso — além de estar quebrada hoje, ver o relatório.
read_accommodations() {
  "${COMPOSE[@]}" exec -T -e PGPASSWORD=cumuru-local-migration-only postgres \
    psql -U cumuru_migration -d cumuru -tA -c \
    "SELECT id FROM core.accommodations WHERE status = 'active' ORDER BY id LIMIT 2"
}

accommodation_ids="$(read_accommodations | tr -d '\r' | tr '\n' ' ')"
primary_accommodation="$(printf '%s' "${accommodation_ids}" | awk '{print $1}')"
secondary_accommodation="$(printf '%s' "${accommodation_ids}" | awk '{print $2}')"
if test -z "${primary_accommodation}" || test -z "${secondary_accommodation}"; then
  fail "the demo fixture needs two active accommodations for the foreign-challenge control"
fi

issue_poster() {
  local target="$1"
  local suffix="$2"
  local etag=""
  etag="$(web_request GET "/api/v1/accommodations/${target}" "" \
    "--output /dev/null --dump-header -" "${authorization}" |
    tr -d '\r' | sed -n 's/^[Ee][Tt][Aa][Gg]: //p')"
  if test -z "${etag}"; then
    fail "could not read the accommodation ETag; If-Match is required to issue a poster"
  fi
  web_request POST "/api/v1/accommodations/${target}/invite" \
    '{"privacy_notice_version":"2026-01-01"}' "" \
    "${authorization}" \
    "If-Match: ${etag}" \
    "Idempotency-Key: self-service-full-stack-poster-${suffix}"
}

poster_token_for() {
  local payload="$1"
  local url=""
  url="$(json_field "${payload}" url 2>/dev/null || true)"
  if test -z "${url}"; then
    fail "the poster response carried no url: ${payload}"
  fi
  case "${url}" in
    *"#"*) ;;
    *) fail "the poster url does not carry the token in the fragment: ${url}" ;;
  esac
  printf '%s' "${url##*#}"
}

live_token="$(poster_token_for "$(issue_poster "${primary_accommodation}" primary)")"
foreign_token="$(poster_token_for "$(issue_poster "${secondary_accommodation}" secondary)")"
if test "${live_token}" = "${foreign_token}"; then
  fail "two accommodations produced the same poster token; the foreign-challenge control is void"
fi

fetch_challenge() {
  local token="$1"
  web_request GET /api/v1/accommodation-invite "" "" \
    "X-Cumuru-Invite-Token: ${token}"
}

live_context="$(fetch_challenge "${live_token}")"
live_challenge="$(json_field "${live_context}" proof_of_work.challenge 2>/dev/null || true)"
live_difficulty="$(json_field "${live_context}" proof_of_work.difficulty_bits 2>/dev/null || true)"
if test -z "${live_challenge}" || test -z "${live_difficulty}"; then
  fail "the invite context carried no proof-of-work challenge: ${live_context}"
fi

solution="$(node "${ROOT_DIR}/deploy/scripts/self-service-solve-pow.mjs" \
  "${live_challenge}" "${live_difficulty}")"
if test -z "${solution}"; then
  fail "the solver produced no solution for difficulty ${live_difficulty}"
fi

submit_body() {
  local challenge="$1"
  local answer="$2"
  local client_id="$3"
  cat <<JSON
{"client_submission_id":"019fae20-0000-7000-8000-0000000000${client_id}",
"privacy_notice_version":"2026-01-01",
"planned_arrival_on":"2026-09-01","planned_departure_on":"2026-09-04",
"visitors":[{"client_id":"019fae21-0000-7000-8000-0000000000${client_id}",
"role":"responsible","age_band":"25_34","residence_country":"BR",
"residence_state":"BA","residence_city_code":"2925303"}],
"proof_of_work":{"challenge":"${challenge}","solution":"${answer}"}}
JSON
}

submit_status() {
  local body="$1"
  local key="$2"
  web_request POST /api/v1/accommodation-invite/submit "$(printf '%s' "${body}" | tr -d '\n')" \
    "--output /dev/null --write-out %{http_code}" \
    "X-Cumuru-Invite-Token: ${live_token}" \
    "Idempotency-Key: self-service-full-stack-submit-${key}"
}

# Controle negativo 1: solução adulterada. Sem ele, um caminho feliz que sempre
# passa não distingue verificação de ausência de verificação.
tampered_status="$(submit_status "$(submit_body "${live_challenge}" "${solution}A" 01)" tampered)"
if test "${tampered_status}" = "200"; then
  fail "a tampered proof-of-work solution was accepted; the verifier is not verifying"
fi

# Controle negativo 2: desafio de outro cartaz, com solução válida para ele.
foreign_context="$(fetch_challenge "${foreign_token}")"
foreign_challenge="$(json_field "${foreign_context}" proof_of_work.challenge)"
if test "${foreign_challenge}" = "${live_challenge}"; then
  fail "two challenge issues collided; the replay control would be void"
fi
foreign_solution="$(node "${ROOT_DIR}/deploy/scripts/self-service-solve-pow.mjs" \
  "${foreign_challenge}" "${live_difficulty}")"
mismatched_status="$(submit_status "$(submit_body "${live_challenge}" "${foreign_solution}" 02)" mismatched)"
if test "${mismatched_status}" = "200"; then
  fail "a solution computed for a different challenge was accepted"
fi

# Caminho feliz: só agora, e depois dos negativos, para que um verde aqui
# signifique verificação e não ausência dela.
accepted_status="$(submit_status "$(submit_body "${live_challenge}" "${solution}" 03)" accepted)"
expect_status "the real proof-of-work pair over HTTP" "200" "${accepted_status}"

# Prova de carga: exatamente UMA estadia self_service pode existir. Se os
# controles negativos tivessem sido recusados por outro motivo que não a prova de
# trabalho — corpo malformado, por exemplo — eles não teriam criado estadia e o
# "!= 200" passaria sem discriminar nada. Se a verificação estivesse ausente,
# haveria três. O número é o que separa verificação de ausência de verificação.
self_service_stays="$(
  "${COMPOSE[@]}" exec -T -e PGPASSWORD=cumuru-local-migration-only postgres \
    psql -U cumuru_migration -d cumuru -tA -c \
    "SELECT count(*) FROM core.stays WHERE provenance = 'self_service'" |
    tr -d '[:space:]'
)"
if test "${self_service_stays}" != "1"; then
  fail "expected exactly one self-service stay after one accepted and two refused submissions, found ${self_service_stays}"
fi

# E ela nasce pendente, sem autora, com prazo: o contrato da ADR-040.
pending_shape="$(
  "${COMPOSE[@]}" exec -T -e PGPASSWORD=cumuru-local-migration-only postgres \
    psql -U cumuru_migration -d cumuru -tA -c \
    "SELECT approval_state
       || ':' || (created_by_membership_id IS NULL)::text
       || ':' || (approval_expires_at IS NOT NULL)::text
       || ':' || status
     FROM core.stays WHERE provenance = 'self_service'" |
    tr -d '[:space:]'
)"
if test "${pending_shape}" != "pending:true:true:pre_registered"; then
  fail "the accepted self-registration has the wrong shape: ${pending_shape}"
fi

# Replay do mesmo desafio resolvido (N-17): o livro de nonces já gastou o nonce,
# e a segunda submissão precisa ser recusada.
replay_status="$(submit_status "$(submit_body "${live_challenge}" "${solution}" 04)" replay)"
if test "${replay_status}" = "200"; then
  fail "a spent proof-of-work challenge was accepted again; the nonce ledger is not spending"
fi

# ---------------------------------------------------------------------------
# Rotação do cartaz (N-03, N-14). Promessa explícita da ADR-039: "a rotação
# invalida o cartaz anterior". Roda na acomodação secundária, cujo cartaz já
# cumpriu seu papel no controle de desafio alheio, para não derrubar o token que
# o par da prova de trabalho usou.
#
# Esta asserção existe porque o gate a encontrou quebrada: a segunda emissão
# devolvia 503 e a transação abortava inteira, deixando o cartaz anterior vivo.
# A causa era a tupla repetida em platform.outbox_events, não o índice parcial.
poster_status() {
  web_request GET /api/v1/accommodation-invite "" \
    "--output /dev/null --write-out %{http_code}" \
    "X-Cumuru-Invite-Token: $1"
}

expect_status "the secondary poster before rotation" "200" \
  "$(poster_status "${foreign_token}")"

rotated_token="$(poster_token_for \
  "$(issue_poster "${secondary_accommodation}" rotated)")"
if test "${rotated_token}" = "${foreign_token}"; then
  fail "the rotation reissued the same token; the previous poster would stay valid"
fi

# O cartaz anterior tem de morrer imediatamente: é o cartaz físico na parede que
# deixa de funcionar, e o 404 é o mesmo de token inexistente.
expect_status "the poster replaced by rotation" "404" \
  "$(poster_status "${foreign_token}")"

# E o novo tem de funcionar, senão a rotação teria apenas destruído o acesso.
expect_status "the poster issued by rotation" "200" \
  "$(poster_status "${rotated_token}")"

# Exatamente um cartaz ativo por acomodação, que é o invariante do índice
# parcial invites_accommodation_single_active_idx.
active_posters="$(
  "${COMPOSE[@]}" exec -T -e PGPASSWORD=cumuru-local-migration-only postgres \
    psql -U cumuru_migration -d cumuru -tA -c \
    "SELECT count(*) FROM core.invites
      WHERE purpose = 'accommodation_self_registration'
        AND revoked_at IS NULL" |
    tr -d '[:space:]'
)"
if test "${active_posters}" != "2"; then
  fail "expected one active poster per accommodation after rotation, found ${active_posters}"
fi

# ---------------------------------------------------------------------------
# Pedido de convite da hospedagem (ADR-042), pelo fio real.
#
# Este é o único canal aberto que coleta identidade, e a ADR-042 só abriu a
# exceção à ADR-040 porque assumiu uma contrapartida: a recusa elimina o
# contato na mesma transação. Declarar as cinco operações no contrato não prova
# nenhuma delas, e a contrapartida menos ainda — por isso o fluxo inteiro é
# exercitado aqui, na mesma disciplina do autocadastro do hóspede acima:
# desafio emitido pelo servidor, resolvido pelo solucionador, gasto no livro de
# nonces, e cada asserção dizendo qual operação falhou e em que ponto.

# json_value distingue três respostas que json_field colapsa numa só: o valor
# presente, o nulo e o campo ausente. A eliminação do contato tem nulo como
# resultado ESPERADO, e um campo que sumisse do JSON passaria por eliminado sem
# nunca ter sido gravado.
json_value() {
  JSON_PAYLOAD="$1" JSON_FIELD="$2" node -e '
    const payload = JSON.parse(process.env.JSON_PAYLOAD);
    const field = process.env.JSON_FIELD;
    if (Object.prototype.hasOwnProperty.call(payload, field)) {
      const value = payload[field];
      process.stdout.write(value === null ? "null" : String(value));
    } else {
      process.stdout.write("absent");
    }
  '
}

json_key_list() {
  JSON_PAYLOAD="$1" node -e '
    const keys = Object.keys(JSON.parse(process.env.JSON_PAYLOAD));
    process.stdout.write(keys.sort().join(","));
  '
}

# O item sai da listagem administrativa, e não do banco: consultar a tabela
# provaria a linha sem provar a rota que a tela usa para decidir.
queue_item() {
  JSON_PAYLOAD="$1" JSON_ID="$2" node -e '
    const page = JSON.parse(process.env.JSON_PAYLOAD);
    const items = page.items || [];
    const item = items.find((entry) => entry.id === process.env.JSON_ID);
    if (item === undefined) {
      process.exit(3);
    }
    process.stdout.write(JSON.stringify(item));
  '
}

access_request_context() {
  web_request GET /api/v1/accommodation-access-requests/context "" ""
}

# Sem Authorization de propósito. Quem chega aqui ainda não tem conta, e um
# contexto que exigisse sessão tornaria o canal inalcançável para exatamente
# quem ele existe para atender.
open_context="$(access_request_context)"
open_notice_version="$(json_field "${open_context}" privacy_notice_version 2>/dev/null || true)"
open_challenge="$(json_field "${open_context}" proof_of_work.challenge 2>/dev/null || true)"
open_difficulty="$(json_field "${open_context}" proof_of_work.difficulty_bits 2>/dev/null || true)"
if test -z "${open_challenge}" || test -z "${open_difficulty}"; then
  fail "getAccommodationAccessRequestContext served no proof-of-work challenge without a session: ${open_context}"
fi
if test -z "${open_notice_version}"; then
  fail "getAccommodationAccessRequestContext served no privacy notice version, so the form has nothing to display before collecting the contact: ${open_context}"
fi

# A versão do aviso vem do contexto e volta na submissão: é assim que o
# servidor sabe que o aviso exibido era o aviso vigente. Fixar um literal aqui
# faria o teste passar sobre um aviso que a tela nunca mostrou.
access_request_body() {
  local challenge="$1"
  local answer="$2"
  local suffix="$3"
  local email="$4"
  cat <<JSON
{"client_submission_id":"019fae30-0000-7000-8000-0000000000${suffix}",
"accommodation_name":"Pousada Pedido Fictícia ${suffix}",
"category":"formal_lodging","capacity":12,
"contact_name":"Responsável Fictícia","contact_email":"${email}",
"contact_phone":"+55 73 90000-0000",
"city_label":"Cumuruxatiba","state_code":"BA",
"privacy_notice_version":"${open_notice_version}",
"proof_of_work":{"challenge":"${challenge}","solution":"${answer}"}}
JSON
}

# Cada submissão pede o desafio próprio. O livro de nonces gasta o anterior, e
# reaproveitá-lo faria a recusa vir da prova de trabalho em vez de vir do que
# está sendo testado — um verde, ou um vermelho, pelo motivo errado.
submit_access_request() {
  local suffix="$1"
  local email="$2"
  local flags="$3"
  local context=""
  local challenge=""
  local difficulty=""
  local answer=""
  context="$(access_request_context)"
  challenge="$(json_field "${context}" proof_of_work.challenge 2>/dev/null || true)"
  difficulty="$(json_field "${context}" proof_of_work.difficulty_bits 2>/dev/null || true)"
  if test -z "${challenge}" || test -z "${difficulty}"; then
    fail "createAccommodationAccessRequest could not be exercised: the context carried no challenge for submission ${suffix}: ${context}"
  fi
  answer="$(node "${ROOT_DIR}/deploy/scripts/self-service-solve-pow.mjs" \
    "${challenge}" "${difficulty}")"
  if test -z "${answer}"; then
    fail "the solver produced no solution for the access request context at difficulty ${difficulty}"
  fi
  web_request POST /api/v1/accommodation-access-requests \
    "$(access_request_body "${challenge}" "${answer}" "${suffix}" "${email}" | tr -d '\n')" \
    "${flags}" \
    "Idempotency-Key: self-service-full-stack-access-request-${suffix}"
}

APPROVED_EMAIL="pousada-aprovada@exemplo.invalid"
REJECTED_EMAIL="pousada-recusada@exemplo.invalid"

# Corpo e código na mesma chamada: pedir os dois em requisições separadas faria
# a segunda cair no replay idempotente, e a asserção falaria da réplica em vez
# da criação.
created_response="$(submit_access_request 01 "${APPROVED_EMAIL}" '--write-out \n%{http_code}')"
created_status="$(printf '%s\n' "${created_response}" | tail -n 1)"
created_body="$(printf '%s\n' "${created_response}" | head -n 1)"
expect_status "createAccommodationAccessRequest with a solved challenge" "201" "${created_status}"

# O recibo é mínimo por decisão de segurança, não por economia: a rota é
# aberta, e ecoar o que foi gravado transformaria a criação em consulta de dado
# de contato alheio.
created_keys="$(json_key_list "${created_body}" 2>/dev/null || true)"
if test "${created_keys}" != "created_at,id"; then
  fail "the creation receipt carried the fields [${created_keys}]; ADR-042 allows only id and created_at on this open route"
fi
case "${created_body}" in
  *"${APPROVED_EMAIL}"*)
    fail "createAccommodationAccessRequest echoed the submitted e-mail back to an unauthenticated caller"
    ;;
esac
approved_request_id="$(json_field "${created_body}" id 2>/dev/null || true)"
if test -z "${approved_request_id}"; then
  fail "the creation receipt carried no id, so the approval below cannot be addressed: ${created_body}"
fi

# Um pendente por endereço, pelo índice único parcial. Quem reenviou porque não
# teve resposta recebe conflito, e o conflito NÃO carrega Retry-After: esperar
# não resolve, só uma decisão tira o pedido da fila.
duplicate_headers="$(submit_access_request 02 "${APPROVED_EMAIL}" \
  '--output /dev/null --dump-header -')"
case "${duplicate_headers}" in
  *" 409 "*) ;;
  *)
    fail "a second access request for the same pending e-mail did not conflict; the partial unique index is not holding: ${duplicate_headers}"
    ;;
esac
case "$(printf '%s' "${duplicate_headers}" | tr 'A-Z' 'a-z')" in
  *retry-after:*)
    fail "the duplicate-pending conflict carried Retry-After; re-sending does not resolve it, only a decision does"
    ;;
esac

# A fila e a decisão custam o mesmo escopo que criar a acomodação à mão, porque
# produzem o mesmo efeito. Sem sessão é 401; com sessão e sem o escopo é 403, e
# são duas recusas diferentes: só a segunda prova que o escopo é o que barra.
expect_status \
  "listAccommodationAccessRequests without a session" "401" \
  "$(request_status "access request queue without a session" GET \
    /api/v1/accommodation-access-requests '' '')"

onboard_scope_sql() {
  "${COMPOSE[@]}" exec -T -e PGPASSWORD=cumuru-local-migration-only postgres \
    psql -U cumuru_migration -d cumuru -tA -c "$1" >/dev/null
}

onboard_scope_sql "UPDATE auth.accounts
   SET scopes = array_remove(scopes, 'accommodations:onboard')
   WHERE email = '${DEMO_EMAIL}'"
expect_status \
  "listAccommodationAccessRequests for a session without accommodations:onboard" "403" \
  "$(web_request GET /api/v1/accommodation-access-requests "" \
    '--output /dev/null --write-out %{http_code}' "${authorization}")"
onboard_scope_sql "UPDATE auth.accounts
   SET scopes = array_append(scopes, 'accommodations:onboard')
   WHERE email = '${DEMO_EMAIL}'
     AND NOT ('accommodations:onboard' = ANY (scopes))"

queue_page="$(web_request GET \
  '/api/v1/accommodation-access-requests?approval_state=pending' "" "" \
  "${authorization}")"
approved_item="$(queue_item "${queue_page}" "${approved_request_id}" 2>/dev/null || true)"
if test -z "${approved_item}"; then
  fail "the pending queue does not list the request just created; listAccommodationAccessRequests is not reading the channel: ${queue_page}"
fi
approved_version="$(json_value "${approved_item}" version 2>/dev/null || true)"
if test -z "${approved_version}"; then
  fail "the queue item carried no version, so If-Match cannot be composed: ${approved_item}"
fi

# Aprovar cria a acomodação na mesma transação, e é por isso que aprovado sem
# accommodation_id não pode existir.
approval="$(web_request POST \
  "/api/v1/accommodation-access-requests/${approved_request_id}/approve" '{}' "" \
  "${authorization}" \
  "If-Match: \"${approved_version}\"" \
  "Idempotency-Key: self-service-full-stack-access-approve")"
approved_state="$(json_value "${approval}" approval_state 2>/dev/null || true)"
if test "${approved_state}" != "approved"; then
  fail "approveAccommodationAccessRequest left the request in state [${approved_state}]: ${approval}"
fi
created_accommodation="$(json_value "${approval}" accommodation_id 2>/dev/null || true)"
case "${created_accommodation}" in
  "" | null | absent)
    fail "approveAccommodationAccessRequest returned accommodation_id=[${created_accommodation}]; the approval did not create the record it promises: ${approval}"
    ;;
esac

# "Existe e permite emitir ativação" não se prova lendo o status: prova-se
# emitindo. A emissão é gated em status='active' na própria SQL, então um 201
# aqui é a afirmação inteira — e é o passo seguinte real da ADR-041, que a
# aprovação deliberadamente NÃO faz sozinha.
approved_accommodation_etag="$(web_request GET \
  "/api/v1/accommodations/${created_accommodation}" "" \
  "--output /dev/null --dump-header -" "${authorization}" |
  tr -d '\r' | sed -n 's/^[Ee][Tt][Aa][Gg]: //p')"
if test -z "${approved_accommodation_etag}"; then
  fail "the accommodation created by the approval is not readable at /accommodations/${created_accommodation}; the approved request points at nothing"
fi
expect_status \
  "issuing the activation on the accommodation the approval created" "201" \
  "$(web_request POST "/api/v1/accommodations/${created_accommodation}/activation" \
    '{"email":"ativacao-pedida@exemplo.invalid","display_name":"Responsável Fictícia"}' \
    '--output /dev/null --write-out %{http_code}' \
    "${authorization}" \
    "If-Match: ${approved_accommodation_etag}" \
    "Idempotency-Key: self-service-full-stack-access-activation")"

# A asserção mais importante do conjunto: a recusa elimina o contato. É a
# contrapartida que a ADR-042 assumiu para justificar coletar identidade em
# canal aberto, e sem prova de ponta a ponta a promessa não existe.
rejected_response="$(submit_access_request 03 "${REJECTED_EMAIL}" '--write-out \n%{http_code}')"
expect_status "createAccommodationAccessRequest for the request to be refused" "201" \
  "$(printf '%s\n' "${rejected_response}" | tail -n 1)"
rejected_request_id="$(json_field "$(printf '%s\n' "${rejected_response}" | head -n 1)" id 2>/dev/null || true)"
if test -z "${rejected_request_id}"; then
  fail "the second creation receipt carried no id, so the rejection below cannot be addressed: ${rejected_response}"
fi

rejected_queue="$(web_request GET \
  '/api/v1/accommodation-access-requests?approval_state=pending' "" "" \
  "${authorization}")"
rejected_item="$(queue_item "${rejected_queue}" "${rejected_request_id}" 2>/dev/null || true)"
if test -z "${rejected_item}"; then
  fail "the pending queue does not list the request about to be refused: ${rejected_queue}"
fi

# Prova de carga da eliminação, ANTES de afirmá-la: se o contato nunca tivesse
# sido gravado, o nulo depois da recusa não significaria eliminação, apenas
# ausência. A fila mostra o contato porque é ele que a administração usa para
# decidir.
stored_email="$(json_value "${rejected_item}" contact_email 2>/dev/null || true)"
if test "${stored_email}" != "${REJECTED_EMAIL}"; then
  fail "the pending queue carries contact_email=[${stored_email}]; the erasure assertion below would pass by absence instead of by erasure"
fi
rejected_version="$(json_value "${rejected_item}" version 2>/dev/null || true)"
if test -z "${rejected_version}"; then
  fail "the queue item to be refused carried no version, so If-Match cannot be composed: ${rejected_item}"
fi

rejection="$(web_request POST \
  "/api/v1/accommodation-access-requests/${rejected_request_id}/reject" \
  '{"reason_code":"not_a_lodging"}' "" \
  "${authorization}" \
  "If-Match: \"${rejected_version}\"" \
  "Idempotency-Key: self-service-full-stack-access-reject")"
for field in contact_name contact_email contact_phone; do
  erased="$(json_value "${rejection}" "${field}" 2>/dev/null || true)"
  if test "${erased}" != "null"; then
    fail "rejectAccommodationAccessRequest answered ${field}=[${erased}]; ADR-042 requires the contact erased in the same transaction as the decision"
  fi
done
rejection_reason="$(json_value "${rejection}" rejection_reason_code 2>/dev/null || true)"
if test "${rejection_reason}" != "not_a_lodging"; then
  fail "the refusal did not preserve the closed-list reason, answering [${rejection_reason}]: ${rejection}"
fi

# E no banco, que é onde a retenção acontece de fato: a linha permanece, o
# motivo e o instante da decisão permanecem, os três campos de contato não.
rejected_shape="$(
  "${COMPOSE[@]}" exec -T -e PGPASSWORD=cumuru-local-migration-only postgres \
    psql -U cumuru_migration -d cumuru -tA -c \
    "SELECT approval_state
       || ':' || (contact_name IS NULL)::text
       || ':' || (contact_email IS NULL)::text
       || ':' || (contact_phone IS NULL)::text
       || ':' || (decided_at IS NOT NULL)::text
       || ':' || coalesce(rejection_reason_code, 'missing')
     FROM core.accommodation_access_requests
     WHERE id = '${rejected_request_id}'" |
    tr -d '[:space:]'
)"
if test "${rejected_shape}" != "rejected:true:true:true:true:not_a_lodging"; then
  fail "the refused access request has the wrong shape in the database: [${rejected_shape}], expected rejected:true:true:true:true:not_a_lodging"
fi

# A varredura é da tabela inteira, não da linha: o endereço recusado não pode
# sobreviver em nenhum pedido, inclusive num que outra escrita tivesse deixado
# para trás.
lingering_contact="$(
  "${COMPOSE[@]}" exec -T -e PGPASSWORD=cumuru-local-migration-only postgres \
    psql -U cumuru_migration -d cumuru -tA -c \
    "SELECT count(*) FROM core.accommodation_access_requests
      WHERE contact_email = '${REJECTED_EMAIL}'" |
    tr -d '[:space:]'
)"
if test "${lingering_contact}" != "0"; then
  fail "the refused e-mail still exists in core.accommodation_access_requests in ${lingering_contact} row(s); the erasure promised by ADR-042 did not happen"
fi

# ---------------------------------------------------------------------------
# Content-Security-Policy (N-20, metade não afirmada).
#
# A promessa mais forte do canal aberto — o formulário público não fala com
# terceiro — repousa inteiramente nesta política. Até aqui ela existia no nginx e
# em nenhum teste: podia sumir numa edição de configuração sem quebrar nada.
#
# A asserção é sobre a política SERVIDA, não sobre o arquivo de configuração:
# ler default.conf provaria a intenção, não a entrega.
csp_header="$(request_headers "the served security policy" GET / '' '' |
  tr -d '\r' | sed -n 's/^[Cc]ontent-[Ss]ecurity-[Pp]olicy: //p')"
if test -z "${csp_header}"; then
  fail "the web surface served no Content-Security-Policy header"
fi

# Diretivas exigidas, com o valor exato. 'self' em connect-src é o que impede a
# página de abrir conexão para qualquer host; data: em img-src é o que permite o
# QR desenhado no navegador sem abrir espaço para imagem remota.
for directive in \
  "default-src 'self'" \
  "connect-src 'self'" \
  "script-src 'self'" \
  "img-src 'self' data:" \
  "style-src 'self'" \
  "base-uri 'none'" \
  "frame-ancestors 'none'" \
  "form-action 'self'"; do
  case "${csp_header}" in
    *"${directive}"*) ;;
    *) fail "the Content-Security-Policy lost the directive [${directive}]: ${csp_header}" ;;
  esac
done

# Nenhuma diretiva de busca pode admitir origem externa. A checagem acima casa
# prefixo e passaria se alguém ACRESCENTASSE um host — "connect-src 'self'
# https://terceiro" contém "connect-src 'self'". Esta segunda varredura fecha
# essa porta olhando o valor inteiro de cada diretiva.
for directive in connect-src script-src img-src default-src style-src; do
  value="$(printf '%s' "${csp_header}" |
    tr ';' '\n' |
    sed -n "s/^ *${directive} //p")"
  if test -z "${value}"; then
    continue
  fi
  for token in ${value}; do
    case "${token}" in
      "'self'" | "'none'" | "data:") ;;
      *) fail "the Content-Security-Policy admits an external source in ${directive}: [${token}]" ;;
    esac
  done
done

# ---------------------------------------------------------------------------
# N-07, N-08, N-09, N-44: nenhum segredo real do fluxo aparece em stdout, em
# platform.audit_events ou em platform.outbox_events.
#
# Reaproveita os segredos que o fluxo real acima já produziu — nenhuma
# chamada nova, nenhum fixture sintético. session_token é o Bearer da sessão
# do operador; live_token, foreign_token e rotated_token são as capabilities
# do cartaz (só existem no fragmento da URL, nunca no caminho); DEMO_PASSWORD
# é a senha literal usada no login. São exatamente os quatro tipos que N-07
# pede para varrer: token, HMAC (coberto indiretamente — request_hash e
# actor_subject_hmac são derivados destes mesmos tokens/segredos, então a
# ausência do preimage nos destroi também a hipótese de reversão trivial) e
# senha.
secrets_probe=(
  "${session_token}"
  "${live_token}"
  "${foreign_token}"
  "${rotated_token}"
  "${DEMO_PASSWORD}"
)
for secret in "${secrets_probe[@]}"; do
  if test -z "${secret}"; then
    fail "one of the real secrets collected from the flow is empty; the negative scans below would pass vacuously"
  fi
done

# N-07: docker compose logs é o stdout/stderr real dos quatro serviços do
# fluxo inteiro, não uma amostra.
full_stack_logs="$("${COMPOSE[@]}" logs --no-color --timestamps 2>&1 || true)"

# Prova de carga da varredura em si, ANTES de usá-la para provar ausência: se
# ela não encontrasse nem o que sabemos que está nos logs, "nenhum segredo
# apareceu" significaria "a varredura não lê nada", não "não vazou". O
# caminho literal da submissão real por HTTP é um valor que sabemos ter sido
# processado pelo serviço `api` e por isso tem de estar em algum lugar do
# stdout combinado.
case "${full_stack_logs}" in
  *"/api/v1/accommodation-invite/submit"*) ;;
  *)
    fail "the stdout scan cannot find a request path we know was served; docker compose logs read nothing useful"
    ;;
esac

for secret in "${secrets_probe[@]}"; do
  if grep --fixed-strings --quiet -- "${secret}" <<<"${full_stack_logs}"; then
    fail "a real secret from the self-service flow appeared verbatim in docker compose logs (N-07): ${secret}"
  fi
done

# N-08/N-09/N-44: platform.audit_events e platform.outbox_events, no molde de
# N-29/N-30 (varredura de banco real, não inspeção de código). Os quatro
# segredos viajam como parâmetro de bind (--set / :'name'), nunca interpolados
# na string SQL, e cada um é comparado por LIKE contra as colunas de texto e
# jsonb das duas tabelas — inclusive changed_fields, purpose_code e
# request_id, que são os únicos lugares em audit_events onde texto livre
# poderia esconder um segredo.
# psql only expands `:'name'` in script mode (stdin/-f), not in `-c`
# command-string mode — `-c` sends the text verbatim and Postgres itself trips
# on the bare `:`. Piped stdin here is what makes the bind-style substitution
# actually bind instead of parsing as literal SQL.
channel_leak="$(
  "${COMPOSE[@]}" exec -T -e PGPASSWORD=cumuru-local-migration-only postgres \
    psql -U cumuru_migration -d cumuru -tA \
    --set=s1="${session_token}" \
    --set=s2="${live_token}" \
    --set=s3="${foreign_token}" \
    --set=s4="${rotated_token}" \
    --set=s5="${DEMO_PASSWORD}" <<'SQL' | tr -d '[:space:]'
      WITH secrets(value) AS (
        VALUES (:'s1'), (:'s2'), (:'s3'), (:'s4'), (:'s5')
      )
      SELECT
        (
          SELECT count(*) FROM platform.audit_events, secrets
          WHERE metadata::text LIKE '%' || secrets.value || '%'
             OR coalesce(request_id, '') LIKE '%' || secrets.value || '%'
             OR coalesce(purpose_code, '') LIKE '%' || secrets.value || '%'
             OR coalesce(array_to_string(changed_fields, ','), '')
                  LIKE '%' || secrets.value || '%'
        )
        || ':' ||
        (
          SELECT count(*) FROM platform.outbox_events, secrets
          WHERE aggregate_type LIKE '%' || secrets.value || '%'
             OR event_type LIKE '%' || secrets.value || '%'
             OR coalesce(lease_owner, '') LIKE '%' || secrets.value || '%'
             OR coalesce(last_error_code, '') LIKE '%' || secrets.value || '%'
        )
SQL
)"
if test -z "${channel_leak}"; then
  fail "the audit_events/outbox_events scan produced no output; the negative assertion would pass vacuously"
fi

# Prova de carga da varredura de canal: os dois canais têm de conter LINHAS
# reais do fluxo que acabou de rodar, senão "zero vazamentos" pode significar
# "zero linhas", não "linhas sem segredo".
# PostgreSQL 17 (pinned in compose.yaml) renders an explicit boolean::text
# cast as "true"/"false", not the "t"/"f" a bare boolean column gets over the
# wire — confirmed against the pinned image, not assumed. The comparison
# below matches the cast this query actually performs.
channel_populated="$(
  "${COMPOSE[@]}" exec -T -e PGPASSWORD=cumuru-local-migration-only postgres \
    psql -U cumuru_migration -d cumuru -tA -c "
      SELECT (count(*) > 0)::text FROM platform.audit_events
        WHERE occurred_at >= now() - interval '5 minutes'
    "
)"
if test "${channel_populated}" != "true"; then
  fail "platform.audit_events has no recent row; the self-service flow above wrote nothing to scan against, so the negative assertion is vacuous"
fi

if test "${channel_leak}" != "0:0"; then
  fail "a real secret from the self-service flow appeared inside platform.audit_events or platform.outbox_events (N-08/N-09/N-44): ${channel_leak}"
fi

echo "self-service full stack passed: contract surface, generated client, uniform 404 for absent capabilities, header-only capability transport, the approval queue as a listStays filter, the real proof-of-work pair over HTTP with tampered, mismatched and replayed controls, poster rotation invalidating the previous token, the lodging access request end to end — open context, receipt with no echo of the contact, 409 without Retry-After on the repeated pending e-mail, accommodations:onboard guarding the queue, approval creating an accommodation that accepts an activation, and refusal erasing the contact while preserving the reason and the instant — a Content-Security-Policy that admits no external source, and no session token, capability or password leaking into stdout, audit_events or outbox_events"
