-- name: ListAccommodationObservedPresence :many
-- As pessoas-dia da própria hospedagem são derivadas de `core.stays` e
-- `core.visitors`, não de `analytics.presence_days`: `app_runtime` tem SELECT
-- nas duas primeiras e nenhum acesso à terceira, que é tabela do worker. A
-- fronteira é deliberada e não se corrige com GRANT — o que a API devolve aqui
-- é a mesma estadia que a hospedagem já lê em `GET /stays`, recortada por dia.
--
-- A elegibilidade repete `presenceEligible`: estado contável e aprovação
-- resolvida. Divergir dela faria a curva própria contar estadia que a
-- publicação recusou.
SELECT
  day::date AS presence_on,
  count(*)::integer AS person_days
FROM core.stays AS stay
JOIN core.memberships AS m
  ON m.accommodation_id = stay.accommodation_id
JOIN core.visitors AS visitor
  ON visitor.stay_id = stay.id
CROSS JOIN LATERAL generate_series(
  greatest(stay.planned_arrival_on, sqlc.arg(start_on)::date),
  least(stay.planned_departure_on - 1, sqlc.arg(end_on)::date - 1),
  interval '1 day'
) AS day
WHERE m.oidc_issuer = sqlc.arg(oidc_issuer)
  AND m.oidc_subject = sqlc.arg(oidc_subject)
  AND m.active = true
  AND stay.accommodation_id = sqlc.arg(accommodation_id)
  AND stay.status IN ('pre_registered', 'checked_in', 'checked_out')
  AND (stay.approval_state IS NULL OR stay.approval_state = 'approved')
GROUP BY day
ORDER BY day;

-- name: SummarizeVillageReporting :one
-- O denominador do comparativo, e nada além dele. Os números decidem se a vila
-- pode aparecer ao lado do dado próprio e morrem no processo: a resposta HTTP
-- carrega apenas o veredito, porque "somos sete com 210 leitos" já é informação
-- sobre terceiros.
--
-- Uma passagem por acomodação, e sobre as mesmas tabelas do núcleo, pela mesma
-- razão de privilégio da consulta acima. `reported` sai de count(*) para não
-- depender de arredondamento.
SELECT
  count(*) FILTER (WHERE observed.reported)::integer AS accommodations,
  coalesce(
    sum(accommodation.capacity) FILTER (WHERE observed.reported), 0
  )::bigint AS capacity,
  coalesce(sum(observed.person_days), 0)::bigint AS person_days
FROM core.accommodations AS accommodation
CROSS JOIN LATERAL (
  SELECT
    count(*) AS person_days,
    count(*) > 0 AS reported
  FROM core.stays AS stay
  JOIN core.visitors AS visitor
    ON visitor.stay_id = stay.id
  CROSS JOIN LATERAL generate_series(
    greatest(stay.planned_arrival_on, sqlc.arg(start_on)::date),
    least(stay.planned_departure_on - 1, sqlc.arg(end_on)::date - 1),
    interval '1 day'
  ) AS day
  WHERE stay.accommodation_id = accommodation.id
    AND stay.status IN ('pre_registered', 'checked_in', 'checked_out')
    AND (stay.approval_state IS NULL OR stay.approval_state = 'approved')
) AS observed
WHERE accommodation.status = 'active'
  AND accommodation.capacity IS NOT NULL;
