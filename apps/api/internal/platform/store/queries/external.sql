-- Camada de contexto externo (ADR-045). A leitura pública sai da view em
-- `public_data`; a escrita entra em `external` sob `external_runtime`. Os dois
-- caminhos nunca se cruzam com a série protegida.

-- name: ListCurrentExternalContext :many
-- Documento único, sem seletor: `/public/context` responde a página inteira,
-- como `/public/methodology`. A ordenação é estável para que o ETag do JSON
-- canônico não dependa do plano de execução.
SELECT
  card_code,
  source_code,
  series_code,
  unit_code,
  period_kind,
  data_mode,
  derived,
  derivation_code,
  unavailable_reason_code,
  definition_version,
  declared_lag_seconds,
  publisher,
  license_code,
  license_url,
  attribution_text,
  terms_url,
  period_start,
  period_end,
  observed_value,
  quality_flag,
  retrieved_at,
  revision,
  source_revision_label,
  last_fetch_outcome,
  last_fetch_finished_at
FROM public_data.current_external_context
ORDER BY card_code, source_code, series_code, period_start;

-- name: ListCreditedExternalSources :many
-- As fontes creditadas do documento público. O Cadastur entra por aqui e só
-- por aqui (U-7): atribuição e link, sem contagem calculada pela plataforma,
-- sem card com valor e sem série de universo publicada.
SELECT
  source_code,
  publisher,
  license_code,
  license_url,
  attribution_text,
  terms_url
-- Lê a view, não `external.sources`: esta consulta roda no pool público, e
-- `public_runtime` não tem USAGE em `external`. Ler a tabela base aqui fazia a
-- rota responder 503 sempre, com o documento montável.
FROM public_data.current_external_sources
ORDER BY source_code;

-- name: UpsertExternalSource :exec
INSERT INTO external.sources (
  source_code,
  publisher,
  license_code,
  license_url,
  attribution_text,
  terms_url,
  commercial_use_allowed,
  active
) VALUES (
  sqlc.arg(source_code),
  sqlc.arg(publisher),
  sqlc.arg(license_code),
  sqlc.arg(license_url),
  sqlc.arg(attribution_text),
  sqlc.arg(terms_url),
  sqlc.arg(commercial_use_allowed),
  sqlc.arg(active)
)
ON CONFLICT (source_code) DO UPDATE SET
  publisher = EXCLUDED.publisher,
  license_code = EXCLUDED.license_code,
  license_url = EXCLUDED.license_url,
  attribution_text = EXCLUDED.attribution_text,
  terms_url = EXCLUDED.terms_url,
  commercial_use_allowed = EXCLUDED.commercial_use_allowed,
  active = EXCLUDED.active,
  updated_at = now();

-- name: UpsertExternalSeries :exec
-- `unit_code` e `period_kind` não são atualizados aqui de propósito: mudar a
-- unidade de uma série é `series_code` novo, senão a observação antiga passa a
-- mentir sobre a própria unidade.
INSERT INTO external.series (
  source_code,
  series_code,
  card_code,
  unit_code,
  period_kind,
  value_kind,
  declared_lag,
  retention_days,
  public_exposable,
  geo_scope,
  definition_version,
  data_mode,
  derived,
  derivation_code,
  unavailable_reason_code
) VALUES (
  sqlc.arg(source_code),
  sqlc.arg(series_code),
  sqlc.narg(card_code),
  sqlc.arg(unit_code),
  sqlc.arg(period_kind),
  sqlc.arg(value_kind),
  sqlc.arg(declared_lag),
  sqlc.arg(retention_days),
  sqlc.arg(public_exposable),
  sqlc.arg(geo_scope),
  sqlc.arg(definition_version),
  sqlc.arg(data_mode),
  sqlc.arg(derived),
  sqlc.narg(derivation_code),
  sqlc.narg(unavailable_reason_code)
)
ON CONFLICT (source_code, series_code) DO UPDATE SET
  card_code = EXCLUDED.card_code,
  value_kind = EXCLUDED.value_kind,
  declared_lag = EXCLUDED.declared_lag,
  retention_days = EXCLUDED.retention_days,
  public_exposable = EXCLUDED.public_exposable,
  geo_scope = EXCLUDED.geo_scope,
  definition_version = EXCLUDED.definition_version,
  data_mode = EXCLUDED.data_mode,
  derived = EXCLUDED.derived,
  derivation_code = EXCLUDED.derivation_code,
  unavailable_reason_code = EXCLUDED.unavailable_reason_code,
  updated_at = now();

-- name: StartExternalFetchRun :exec
INSERT INTO external.fetch_runs (
  id,
  source_code,
  started_at,
  outcome
) VALUES (
  sqlc.arg(id),
  sqlc.arg(source_code),
  sqlc.arg(started_at),
  sqlc.arg(outcome)
);

-- name: FinishExternalFetchRun :exec
UPDATE external.fetch_runs
SET
  finished_at = sqlc.arg(finished_at),
  outcome = sqlc.arg(outcome),
  http_status = sqlc.narg(http_status),
  observations_written = sqlc.arg(observations_written),
  batch_budget_exhausted = sqlc.arg(batch_budget_exhausted)
WHERE id = sqlc.arg(id);

-- name: GetLatestExternalFetchRun :one
SELECT
  id,
  source_code,
  started_at,
  finished_at,
  outcome,
  http_status,
  observations_written,
  batch_budget_exhausted
FROM external.fetch_runs
WHERE source_code = sqlc.arg(source_code)
ORDER BY started_at DESC
LIMIT 1;

-- name: NextExternalObservationRevision :one
-- Digest igual é no-op no INSERT; digest diferente entra como revisão nova.
-- A revisão é fato gravado porque ERA5 e Wikimedia backfillam dado já
-- publicado, e trocar o valor in-place apagaria o rastro.
SELECT coalesce(max(revision), 0) + 1 AS next_revision
FROM external.observations
WHERE source_code = sqlc.arg(source_code)
  AND series_code = sqlc.arg(series_code)
  AND period_start = sqlc.arg(period_start);

-- name: InsertExternalObservation :execrows
-- Devolve a contagem de linhas afetadas. Zero significa que o índice único
-- (source, series, period_start, payload_digest) recusou a gravação porque o
-- fato já existia — é resultado esperado, não erro. Um significa revisão nova.
-- É essa contagem que responde "a linha apareceu?", no lugar de reler a
-- revisão depois do INSERT.
INSERT INTO external.observations (
  source_code,
  series_code,
  period_kind,
  period_start,
  period_end,
  revision,
  observed_value,
  quality_flag,
  retrieved_at,
  source_revision_label,
  payload_digest,
  fetch_run_id
) VALUES (
  sqlc.arg(source_code),
  sqlc.arg(series_code),
  sqlc.arg(period_kind),
  sqlc.arg(period_start),
  sqlc.arg(period_end),
  sqlc.arg(revision),
  sqlc.narg(observed_value),
  sqlc.arg(quality_flag),
  sqlc.arg(retrieved_at),
  sqlc.narg(source_revision_label),
  sqlc.arg(payload_digest),
  sqlc.arg(fetch_run_id)
)
ON CONFLICT (source_code, series_code, period_start, payload_digest)
DO NOTHING;

-- name: DeleteExpiredExternalObservations :execrows
-- Retenção por série, em lote limitado: resposta bruta de terceiro é cache com
-- prazo, não acervo. O teto existe porque o ciclo de ingestão tem orçamento
-- próprio e não pode ficar pendurado no relógio de requisição.
DELETE FROM external.observations AS observation
WHERE (observation.source_code, observation.series_code, observation.period_start, observation.revision) IN (
  SELECT
    candidate.source_code,
    candidate.series_code,
    candidate.period_start,
    candidate.revision
  FROM external.observations AS candidate
  JOIN external.series AS series
    ON series.source_code = candidate.source_code
    AND series.series_code = candidate.series_code
  WHERE candidate.period_start
    < sqlc.arg(reference_at)::timestamptz
      - make_interval(days => series.retention_days)
  -- Sem ordem estável, o LIMIT escolhe um conjunto arbitrário a cada ciclo e
  -- a varredura pode nunca alcançar a observação mais antiga: o lote seguinte
  -- reencontra as mesmas linhas. A ordem faz a retenção progredir.
  ORDER BY candidate.period_start, candidate.revision
  LIMIT sqlc.arg(batch_limit)
);

-- name: CountExternalTideHarmonics :one
-- Sem constantes importadas o card de maré é `unavailable` declarado com
-- `constants_not_imported` (ADR-045 §8). Nenhuma constante fictícia, nenhum
-- fallback para altura de superfície do mar modelada.
SELECT count(*) AS harmonics
FROM external.tide_harmonics
WHERE station_code = sqlc.arg(station_code);
