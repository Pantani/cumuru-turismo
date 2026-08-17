-- name: CheckReadiness :one
SELECT 1::integer AS ready;

-- name: ListActiveTenantMemberships :many
SELECT
  m.id AS membership_id,
  m.role,
  a.id AS accommodation_id,
  a.organization_id
FROM core.memberships AS m
JOIN core.accommodations AS a ON a.id = m.accommodation_id
WHERE m.oidc_issuer = sqlc.arg(oidc_issuer)
  AND m.oidc_subject = sqlc.arg(oidc_subject)
  AND m.active = true
  AND a.status = 'active'
ORDER BY a.organization_id, a.id;

-- name: SpendProofOfWorkChallenge :execrows
-- Livro de nonces do proof-of-work. O INSERT com conflito na chave primária
-- afeta zero linhas e é exatamente o replay: sem este gasto, a mesma solução
-- seria reenviada durante todo o TTL e o controle valeria zero. O chamador
-- devolve a mesma resposta indistinguível dada a um desafio inválido, para o
-- endpoint não virar oráculo.
INSERT INTO platform.proof_of_work_spends (
  challenge_hmac,
  key_version,
  expires_at
)
VALUES (
  sqlc.arg(challenge_hmac),
  sqlc.arg(key_version),
  sqlc.arg(expires_at)
)
ON CONFLICT (challenge_hmac) DO NOTHING;
