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
#   6. a estadia pendente não produz presença nem alcança public_data.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROJECT_NAME="cumuru-self-service-full-stack-${PPID}-$$"

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
if test "${declared_operations}" -ne 10; then
  fail "the contract declares ${declared_operations} self-service operations, expected 10"
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
  rejectStay; do
  if ! grep -q "operationId: ${operation}$" "${ROOT_DIR}/contracts/openapi.yaml"; then
    fail "the contract lost operationId ${operation}"
  fi
  if ! grep -q "${operation}" "${ROOT_DIR}/apps/web/src/generated/schema.ts"; then
    fail "the generated web client does not expose ${operation}; run make generate"
  fi
done

"${COMPOSE[@]}" up --detach --wait postgres
"${COMPOSE[@]}" run --rm --no-deps migrate
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

echo "self-service full stack passed: contract surface, generated client, uniform 404 for absent capabilities, header-only capability transport, the approval queue as a listStays filter, the real proof-of-work pair over HTTP with tampered, mismatched and replayed controls, poster rotation invalidating the previous token, and a Content-Security-Policy that admits no external source"
