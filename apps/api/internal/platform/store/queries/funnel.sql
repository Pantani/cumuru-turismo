-- name: SummarizeInviteFunnel :one
-- O funil não coleta nada: ele conta o que o registro já guarda. Convite
-- emitido, convite usado, convite que expirou sem uso — três estados que já
-- existem em `core.invites`, lidos por janela.
SELECT
  count(*)::integer AS issued,
  count(*) FILTER (WHERE invite.use_count > 0)::integer AS submitted,
  count(*) FILTER (
    WHERE invite.use_count = 0
      AND invite.revoked_at IS NULL
      AND invite.expires_at < sqlc.arg(as_of)
  )::integer AS expired_unused,
  count(*) FILTER (WHERE invite.revoked_at IS NOT NULL)::integer AS revoked,
  count(submission.id)::integer AS latency_sample,
  -- O coalesce existe para o scan, não para a leitura: a mediana só é lida com
  -- amostra a partir do piso (analytics.FunnelLatencyMinimum), e com amostra
  -- nesse tamanho percentile_cont nunca é nulo. O ramo do zero é inalcançável
  -- em toda resposta que publica a mediana.
  coalesce(
    percentile_cont(0.5) WITHIN GROUP (
      ORDER BY extract(
        epoch FROM (submission.submitted_at - invite.created_at)
      ) / 3600
    ),
    0
  )::double precision AS median_hours
FROM core.invites AS invite
LEFT JOIN core.group_submissions AS submission
  ON submission.stay_id = invite.stay_id
  AND submission.collection_channel = 'invite'
WHERE invite.created_at >= sqlc.arg(window_start)
  AND invite.created_at < sqlc.arg(window_end);

-- name: SummarizeSurveyFunnel :one
-- `participation` separa quem respondeu de quem recusou explicitamente. A
-- recusa é resposta, não abandono, e misturar as duas esconderia justamente o
-- que o questionário precisa saber sobre si mesmo.
SELECT
  count(*)::integer AS issued,
  count(*) FILTER (WHERE response.participation = 'submitted')::integer AS answered,
  count(*) FILTER (WHERE response.participation = 'declined')::integer AS declined,
  count(*) FILTER (
    WHERE capability.consumed_at IS NULL
      AND capability.revoked_at IS NULL
      AND capability.expires_at < sqlc.arg(as_of)
  )::integer AS expired_unanswered,
  count(response.id)::integer AS latency_sample,
  coalesce(
    percentile_cont(0.5) WITHIN GROUP (
      ORDER BY extract(
        epoch FROM (response.submitted_at - capability.created_at)
      ) / 3600
    ),
    0
  )::double precision AS median_hours
FROM survey.capabilities AS capability
LEFT JOIN survey.responses AS response
  ON response.capability_id = capability.id
WHERE capability.created_at >= sqlc.arg(window_start)
  AND capability.created_at < sqlc.arg(window_end);

-- name: SummarizeSelfRegistrationFunnel :one
SELECT
  count(*)::integer AS started,
  count(*) FILTER (WHERE stay.approval_state = 'pending')::integer AS pending,
  count(*) FILTER (WHERE stay.approval_state = 'approved')::integer AS approved,
  count(*) FILTER (WHERE stay.approval_state = 'rejected')::integer AS rejected,
  count(*) FILTER (WHERE stay.approval_state = 'expired')::integer AS expired
FROM core.stays AS stay
WHERE stay.provenance = 'self_service'
  AND stay.created_at >= sqlc.arg(window_start)
  AND stay.created_at < sqlc.arg(window_end);
