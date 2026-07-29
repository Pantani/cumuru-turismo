#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 3 ]]; then
  echo "usage: bootstrap-runtime-secret.sh SECRET_ARN REGION OUTPUT_FILE" >&2
  exit 2
fi

secret_arn="$1"
region="$2"
output_file="$3"
temporary_file="${output_file}.new"

umask 077
mkdir -p "$(dirname "${output_file}")"

if secret_string="$(
  aws secretsmanager get-secret-value \
    --secret-id "${secret_arn}" \
    --region "${region}" \
    --query SecretString \
    --output text 2>/dev/null
)"; then
  printf '%s' "${secret_string}" >"${temporary_file}"
else
  password() {
    openssl rand -hex 32
  }

  key() {
    openssl rand -base64 32 | tr -d '\n'
  }

  jq -n \
    --arg migration_password "$(password)" \
    --arg app_password "$(password)" \
    --arg worker_password "$(password)" \
    --arg public_password "$(password)" \
    --arg invite_key "$(key)" \
    --arg actor_key "$(key)" \
    --arg idempotency_key "$(key)" \
    --arg rate_limit_key "$(key)" \
    --arg cursor_key "$(key)" \
    '{
      database_migration_password: $migration_password,
      database_app_password: $app_password,
      database_worker_password: $worker_password,
      database_public_password: $public_password,
      invite_hmac_key: $invite_key,
      actor_hmac_key: $actor_key,
      idempotency_hmac_key: $idempotency_key,
      rate_limit_hmac_key: $rate_limit_key,
      cursor_hmac_key: $cursor_key
    }' >"${temporary_file}"

  aws secretsmanager put-secret-value \
    --secret-id "${secret_arn}" \
    --region "${region}" \
    --secret-string "file://${temporary_file}" >/dev/null
fi

jq -e '
  [
    .database_migration_password,
    .database_app_password,
    .database_worker_password,
    .database_public_password,
    .invite_hmac_key,
    .actor_hmac_key,
    .idempotency_hmac_key,
    .rate_limit_hmac_key,
    .cursor_hmac_key
  ]
  | all(type == "string" and length >= 32)
' "${temporary_file}" >/dev/null

mv -f "${temporary_file}" "${output_file}"
chmod 0600 "${output_file}"
