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
-- O denominador do comparativo, e nada além dele. Os dois números decidem se a
-- vila pode aparecer ao lado do dado próprio e morrem no processo: a resposta
-- HTTP carrega apenas o veredito, porque "somos sete com 210 leitos" já é
-- informação sobre terceiros.
SELECT
  count(*) FILTER (WHERE reporting.reported)::integer AS accommodations,
  coalesce(
    sum(reporting.capacity) FILTER (WHERE reporting.reported), 0
  )::bigint AS capacity,
  coalesce(sum(reporting.person_days), 0)::bigint AS person_days
FROM (
  SELECT
    accommodation.capacity,
    (
      SELECT coalesce(round(sum(fact.weight)), 0)
      FROM analytics.presence_days AS fact
      JOIN core.stays AS stay
        ON stay.id = fact.stay_id
      WHERE stay.accommodation_id = accommodation.id
        AND fact.kind = 'observed'
        AND fact.presence_on >= sqlc.arg(start_on)
        AND fact.presence_on < sqlc.arg(end_on)
    ) AS person_days,
    EXISTS (
      SELECT 1
      FROM analytics.presence_days AS fact
      JOIN core.stays AS stay
        ON stay.id = fact.stay_id
      WHERE stay.accommodation_id = accommodation.id
        AND fact.kind = 'observed'
        AND fact.presence_on >= sqlc.arg(start_on)
        AND fact.presence_on < sqlc.arg(end_on)
    ) AS reported
  FROM core.accommodations AS accommodation
  WHERE accommodation.status = 'active'
    AND accommodation.capacity IS NOT NULL
) AS reporting;
