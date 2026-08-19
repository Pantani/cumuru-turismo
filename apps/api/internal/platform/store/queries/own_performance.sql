-- name: ListAccommodationObservedPresence :many
-- O isolamento por membership fica no SQL, como em stay.sql e accommodation.sql:
-- a consulta não devolve linha alguma para quem não é da hospedagem, em vez de
-- devolver e filtrar em Go.
SELECT
  fact.presence_on,
  round(sum(fact.weight))::integer AS person_days
FROM analytics.presence_days AS fact
JOIN core.stays AS stay
  ON stay.id = fact.stay_id
JOIN core.memberships AS m
  ON m.accommodation_id = stay.accommodation_id
WHERE m.oidc_issuer = sqlc.arg(oidc_issuer)
  AND m.oidc_subject = sqlc.arg(oidc_subject)
  AND m.active = true
  AND stay.accommodation_id = sqlc.arg(accommodation_id)
  AND fact.kind = 'observed'
  AND fact.presence_on >= sqlc.arg(start_on)
  AND fact.presence_on < sqlc.arg(end_on)
GROUP BY fact.presence_on
ORDER BY fact.presence_on;

-- name: SummarizeVillageReporting :one
-- O denominador do comparativo, e nada além dele. Os números decidem se a vila
-- pode aparecer ao lado do dado próprio e morrem no processo: a resposta HTTP
-- carrega apenas o veredito, porque "somos sete com 210 leitos" já é informação
-- sobre terceiros.
--
-- Uma passagem por acomodação, não duas: `reported` sai de count(*) na mesma
-- varredura que soma as pessoas-dia. Vem de count(*), e não de person_days > 0,
-- porque uma hospedagem que reportou pesos pequenos arredondaria para zero e
-- deixaria de contar como reportante.
SELECT
  count(*) FILTER (WHERE observed.reported)::integer AS accommodations,
  coalesce(
    sum(accommodation.capacity) FILTER (WHERE observed.reported), 0
  )::bigint AS capacity,
  coalesce(sum(observed.person_days), 0)::bigint AS person_days
FROM core.accommodations AS accommodation
CROSS JOIN LATERAL (
  SELECT
    coalesce(round(sum(fact.weight)), 0) AS person_days,
    count(*) > 0 AS reported
  FROM analytics.presence_days AS fact
  JOIN core.stays AS stay
    ON stay.id = fact.stay_id
  WHERE stay.accommodation_id = accommodation.id
    AND fact.kind = 'observed'
    AND fact.presence_on >= sqlc.arg(start_on)
    AND fact.presence_on < sqlc.arg(end_on)
) AS observed
WHERE accommodation.status = 'active'
  AND accommodation.capacity IS NOT NULL;
