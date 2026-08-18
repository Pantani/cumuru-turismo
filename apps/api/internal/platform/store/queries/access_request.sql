-- name: CreateAccommodationAccessRequest :one
-- A resposta devolve só id e created_at porque a rota é aberta: ecoar o que foi
-- gravado transformaria a criação em consulta de contato alheio. approval_state
-- e version ficam no DEFAULT da tabela, que é a única fonte do estado inicial.
INSERT INTO core.accommodation_access_requests (
  id,
  accommodation_name,
  category,
  capacity,
  contact_name,
  contact_email,
  contact_phone,
  city_label,
  state_code,
  expires_at,
  created_at,
  updated_at
)
VALUES (
  sqlc.arg(request_id),
  sqlc.arg(accommodation_name),
  sqlc.arg(category),
  sqlc.arg(capacity),
  sqlc.arg(contact_name),
  sqlc.arg(contact_email),
  sqlc.narg(contact_phone),
  sqlc.arg(city_label),
  sqlc.arg(state_code),
  sqlc.arg(expires_at),
  sqlc.arg(now),
  sqlc.arg(now)
)
RETURNING id, created_at;

-- name: ListAccommodationAccessRequests :many
-- O cursor é (created_at, id), o mesmo par da listagem de acomodações, e o
-- filtro ausente devolve todos os estados.
SELECT
  request.id,
  request.accommodation_name,
  request.category,
  request.capacity,
  request.contact_name,
  request.contact_email,
  request.contact_phone,
  request.city_label,
  request.state_code,
  request.approval_state,
  request.expires_at,
  request.accommodation_id,
  request.rejection_reason_code,
  request.version,
  request.created_at,
  request.updated_at
FROM core.accommodation_access_requests AS request
WHERE (
    sqlc.narg(approval_state)::text IS NULL
    OR request.approval_state = sqlc.narg(approval_state)::text
  )
  AND (
    sqlc.narg(cursor_created_at)::timestamptz IS NULL
    OR (request.created_at, request.id) < (
      sqlc.narg(cursor_created_at)::timestamptz,
      sqlc.narg(cursor_id)::uuid
    )
  )
ORDER BY request.created_at DESC, request.id DESC
LIMIT sqlc.arg(page_limit);

-- name: LockAccommodationAccessRequestForDecision :one
-- A trava vem antes da decisão para que a criação da acomodação e a escrita do
-- estado enxerguem a mesma linha. O estado e a versão voltam para que ausente,
-- já decidido e versão errada sejam três respostas diferentes.
SELECT
  request.id,
  request.accommodation_name,
  request.category,
  request.capacity,
  request.approval_state,
  request.version
FROM core.accommodation_access_requests AS request
WHERE request.id = sqlc.arg(request_id)
FOR UPDATE OF request;

-- name: ApproveAccommodationAccessRequest :one
-- accommodation_id chega preenchido porque a acomodação foi criada na mesma
-- transação: a constraint de decisão recusa aprovado sem cadastro. Zero linhas
-- significa que a linha mudou entre a trava e a escrita, o que é conflito.
UPDATE core.accommodation_access_requests AS request
SET
  approval_state = 'approved',
  accommodation_id = sqlc.arg(accommodation_id),
  decided_at = sqlc.arg(decided_at),
  decided_by_oidc_issuer = sqlc.arg(oidc_issuer),
  decided_by_oidc_subject = sqlc.arg(oidc_subject),
  updated_at = sqlc.arg(decided_at),
  version = request.version + 1
WHERE request.id = sqlc.arg(request_id)
  AND request.version = sqlc.arg(expected_version)
  AND request.approval_state = 'pending'
RETURNING
  request.id,
  request.accommodation_name,
  request.category,
  request.capacity,
  request.contact_name,
  request.contact_email,
  request.contact_phone,
  request.city_label,
  request.state_code,
  request.approval_state,
  request.expires_at,
  request.accommodation_id,
  request.rejection_reason_code,
  request.version,
  request.created_at,
  request.updated_at;

-- name: RejectAccommodationAccessRequest :one
-- O contato é eliminado na mesma transação que carimba a recusa, e não num
-- passo posterior: a constraint de decisão recusa 'rejected' que ainda carregue
-- nome, e-mail ou telefone, então a purga não depende de a aplicação lembrar de
-- fazê-la. O fato, o motivo e o instante permanecem (ADR-042).
UPDATE core.accommodation_access_requests AS request
SET
  approval_state = 'rejected',
  rejection_reason_code = sqlc.arg(reason_code),
  contact_name = NULL,
  contact_email = NULL,
  contact_phone = NULL,
  decided_at = sqlc.arg(decided_at),
  decided_by_oidc_issuer = sqlc.arg(oidc_issuer),
  decided_by_oidc_subject = sqlc.arg(oidc_subject),
  updated_at = sqlc.arg(decided_at),
  version = request.version + 1
WHERE request.id = sqlc.arg(request_id)
  AND request.version = sqlc.arg(expected_version)
  AND request.approval_state = 'pending'
RETURNING
  request.id,
  request.accommodation_name,
  request.category,
  request.capacity,
  request.contact_name,
  request.contact_email,
  request.contact_phone,
  request.city_label,
  request.state_code,
  request.approval_state,
  request.expires_at,
  request.accommodation_id,
  request.rejection_reason_code,
  request.version,
  request.created_at,
  request.updated_at;

-- name: ExpireAccommodationAccessRequests :many
-- A varredura do worker lê e escreve exatamente o que o grant permite: enxerga
-- id, approval_state e expires_at para achar o vencido, escreve o estado, os
-- três campos de contato e updated_at, e devolve apenas o id.
--
-- version não é incrementada de propósito. O worker não tem SELECT na coluna, e
-- incrementar exige lê-la; expired é terminal e nenhuma escrita com If-Match o
-- segue, então a versão parada não abre janela para escrita perdida.
UPDATE core.accommodation_access_requests AS request
SET
  approval_state = 'expired',
  contact_name = NULL,
  contact_email = NULL,
  contact_phone = NULL,
  updated_at = sqlc.arg(expired_at)
FROM (
  SELECT candidate.id
  FROM core.accommodation_access_requests AS candidate
  WHERE candidate.approval_state = 'pending'
    AND candidate.expires_at < sqlc.arg(cutoff)
  ORDER BY candidate.expires_at
  LIMIT sqlc.arg(batch_size)
  FOR UPDATE SKIP LOCKED
) AS overdue
WHERE request.id = overdue.id
RETURNING request.id;
