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
-- A estadia tem no máximo uma submissão (`group_submissions.stay_id` é único),
-- mas pode ter mais de um convite: um revogado e outro emitido em seguida.
-- Sem `use_count > 0` na junção, a mesma submissão seria contada uma vez por
-- convite e inflaria a amostra da mediana.
LEFT JOIN core.group_submissions AS submission
  ON submission.stay_id = invite.stay_id
  AND submission.collection_channel = 'invite'
  AND invite.use_count > 0
WHERE invite.created_at >= sqlc.arg(window_start)
  AND invite.created_at < sqlc.arg(window_end);

-- name: SummarizeSurveyFunnel :one
-- A conclusão sai de `survey.capabilities.consumed_at`, não de
-- `survey.responses`: `app_runtime` tem INSERT e não SELECT nas respostas — a
-- API grava a resposta do hóspede e não a lê de volta, e isso é controle de
-- privacidade, não lacuna a corrigir com GRANT.
--
-- O preço é real e fica declarado: sem ler `participation`, o funil não separa
-- quem respondeu de quem recusou explicitamente. As duas contam como concluídas.
-- Essa separação só pode vir do worker, que é quem enxerga a resposta.
SELECT
  count(*)::integer AS issued,
  count(*) FILTER (WHERE capability.consumed_at IS NOT NULL)::integer AS completed,
  count(*) FILTER (
    WHERE capability.consumed_at IS NULL
      AND capability.revoked_at IS NULL
      AND capability.expires_at < sqlc.arg(as_of)
  )::integer AS expired_unanswered,
  count(*) FILTER (WHERE capability.revoked_at IS NOT NULL)::integer AS revoked,
  count(capability.consumed_at)::integer AS latency_sample,
  coalesce(
    percentile_cont(0.5) WITHIN GROUP (
      ORDER BY extract(
        epoch FROM (capability.consumed_at - capability.created_at)
      ) / 3600
    ),
    0
  )::double precision AS median_hours
FROM survey.capabilities AS capability
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
