-- Camada de contexto externo (ADR-045). Dado de terceiro é sumidouro, nunca
-- insumo da série protegida, e a direcionalidade é garantida por ACL do
-- PostgreSQL: `external_runtime` escreve aqui e não lê `core`, `survey`,
-- `analytics` nem `public_data`; `worker_runtime`, que reconcilia a série
-- protegida, não recebe nenhum privilégio neste schema.
--
-- Append-only a partir de 000003 (ADR-032, emendado pela ADR-045). A baseline
-- 000001 não é editada: o requisito do papel novo entra aqui, como bloco
-- próprio.
BEGIN;

DO $$
DECLARE
  required_role text;
BEGIN
  FOREACH required_role IN ARRAY ARRAY[
    'external_runtime'
  ]
  LOOP
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = required_role) THEN
      RAISE EXCEPTION 'required database role is missing: %', required_role;
    END IF;
  END LOOP;
END
$$;

CREATE SCHEMA external;

COMMENT ON SCHEMA external IS
  'Contexto copiado de terceiro, com licença e proveniência; nunca insumo da série protegida.';

-- O publicador, não a série. `attribution_text` é o texto exato que vai ao
-- público: texto de licença concatenado em Go é texto de licença que ninguém
-- revisa (ADR-045 §7).
CREATE TABLE external.sources (
  source_code text PRIMARY KEY,
  publisher text NOT NULL,
  license_code text NOT NULL,
  license_url text NOT NULL,
  attribution_text text NOT NULL,
  terms_url text NOT NULL,
  commercial_use_allowed boolean NOT NULL,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT sources_code_shape_valid
    CHECK (source_code ~ '^[a-z][a-z0-9_]*$'),
  CONSTRAINT sources_code_vocabulary_valid
    CHECK (
      source_code IN (
        'open_meteo_forecast',
        'open_meteo_archive',
        'open_meteo_marine',
        'wikimedia_pageviews',
        'ibge_aggregates',
        'brasilapi_holidays',
        'cadastur',
        'chm_harmonics'
      )
    ),
  CONSTRAINT sources_publisher_not_blank CHECK (btrim(publisher) <> ''),
  CONSTRAINT sources_license_code_not_blank CHECK (btrim(license_code) <> ''),
  CONSTRAINT sources_license_url_valid CHECK (license_url ~ '^https://'),
  CONSTRAINT sources_attribution_not_blank
    CHECK (btrim(attribution_text) <> ''),
  CONSTRAINT sources_terms_url_valid CHECK (terms_url ~ '^https://')
);

-- A unidade e o período moram aqui, nunca na observação: unidade em texto
-- livre por linha é o defeito clássico desse desenho. Mudar a unidade de uma
-- série é `series_code` novo, nunca ALTER in-place — senão a observação antiga
-- passa a mentir sobre a própria unidade.
CREATE TABLE external.series (
  source_code text NOT NULL
    REFERENCES external.sources (source_code),
  series_code text NOT NULL,
  card_code text,
  unit_code text NOT NULL,
  period_kind text NOT NULL,
  value_kind text NOT NULL,
  declared_lag interval NOT NULL,
  retention_days integer NOT NULL,
  public_exposable boolean NOT NULL,
  geo_scope text NOT NULL,
  definition_version integer NOT NULL DEFAULT 1,
  data_mode text NOT NULL,
  derived boolean NOT NULL DEFAULT false,
  derivation_code text,
  unavailable_reason_code text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (source_code, series_code),
  CONSTRAINT series_code_shape_valid
    CHECK (series_code ~ '^[a-z][a-z0-9_]*$'),
  CONSTRAINT series_unit_vocabulary_valid
    CHECK (
      unit_code IN (
        'celsius',
        'millimetre',
        'metre',
        'metre_per_second',
        'pageview',
        'person',
        'brl',
        'count',
        'degree'
      )
    ),
  CONSTRAINT series_period_kind_valid
    CHECK (period_kind IN ('instant', 'hour', 'day', 'month', 'year')),
  CONSTRAINT series_value_kind_valid
    CHECK (value_kind IN ('scalar', 'interval')),
  CONSTRAINT series_declared_lag_valid CHECK (declared_lag >= interval '0'),
  CONSTRAINT series_retention_days_valid CHECK (retention_days > 0),
  CONSTRAINT series_geo_scope_valid
    CHECK (geo_scope IN ('municipality', 'station', 'point')),
  CONSTRAINT series_definition_version_valid CHECK (definition_version >= 1),
  -- `data_mode` é por card (ADR-045 §7): uma página que mistura clima real com
  -- presença fictícia sob um rótulo global mente nas duas direções.
  CONSTRAINT series_data_mode_valid
    CHECK (data_mode IN ('real_source', 'prototype_fixtures')),
  -- Entrega 1 publica dois cards. Card novo é emenda da ADR-045 §10, não
  -- escolha silenciosa de uma onda.
  CONSTRAINT series_card_code_valid
    CHECK (card_code IS NULL OR card_code IN ('weather_daily', 'tide')),
  -- Série exposta ao público precisa de card; série interna não pode ter um.
  CONSTRAINT series_card_matches_exposure
    CHECK (
      (public_exposable AND card_code IS NOT NULL)
      OR
      (NOT public_exposable AND card_code IS NULL)
    ),
  CONSTRAINT series_derivation_code_valid
    CHECK (
      (derived AND btrim(coalesce(derivation_code, '')) <> '')
      OR
      (NOT derived AND derivation_code IS NULL)
    ),
  -- Motivo estrutural de indisponibilidade, quando existe: a maré nasce
  -- `constants_not_imported` por U-4 e não destrava por código. Vocabulário
  -- fechado, sem texto livre (ADR-045 §7).
  CONSTRAINT series_unavailable_reason_valid
    CHECK (
      unavailable_reason_code IS NULL
      OR unavailable_reason_code IN (
        'source_unavailable',
        'source_rate_limited',
        'source_not_licensed',
        'source_data_missing',
        'constants_not_imported',
        'stale_beyond_declared_lag'
      )
    ),
  -- Chave alternativa que sustenta a FK composta de `observations`: o
  -- `period_kind` da observação é provadamente o mesmo da série, e não uma
  -- segunda fonte de verdade.
  UNIQUE (source_code, series_code, period_kind)
);

-- Sem esta tabela, "fonte indisponível" é indistinguível de "ainda não rodou"
-- e de "a fonte não tem esse período". O card público lê o `outcome` da última
-- run, não a ausência de linhas.
CREATE TABLE external.fetch_runs (
  id uuid PRIMARY KEY,
  source_code text NOT NULL
    REFERENCES external.sources (source_code),
  started_at timestamptz NOT NULL,
  finished_at timestamptz,
  outcome text NOT NULL,
  http_status integer,
  observations_written integer NOT NULL DEFAULT 0,
  batch_budget_exhausted boolean NOT NULL DEFAULT false,
  -- `write_error` separa falha de gravação de falha de rede. Sem ele, erro de
  -- banco era registrado como `http_error` e a trilha nomeava a causa errada:
  -- quem depurasse procuraria rede onde houve persistência. A ADR-045 §7
  -- justifica `fetch_runs` por tornar "indisponível" distinguível de "não
  -- rodou"; distinguir mal a causa é a mesma falha um nível abaixo.
  CONSTRAINT fetch_runs_outcome_valid
    CHECK (
      outcome IN (
        'ok',
        'unchanged',
        'http_error',
        'parse_error',
        'write_error',
        'rate_limited',
        'skipped_budget'
      )
    ),
  CONSTRAINT fetch_runs_finished_after_started
    CHECK (finished_at IS NULL OR finished_at >= started_at),
  CONSTRAINT fetch_runs_http_status_valid
    CHECK (http_status IS NULL OR http_status BETWEEN 100 AND 599),
  CONSTRAINT fetch_runs_written_valid CHECK (observations_written >= 0)
);

CREATE INDEX fetch_runs_source_started_idx
  ON external.fetch_runs (source_code, started_at DESC);

-- O fato, com revisão. ERA5 e Wikimedia backfillam dado já publicado: a
-- revisão precisa ser fato gravado, não upsert in-place que troca o valor com
-- o mesmo `retrieved_at` e sem rastro.
CREATE TABLE external.observations (
  source_code text NOT NULL,
  series_code text NOT NULL,
  period_kind text NOT NULL,
  period_start timestamptz NOT NULL,
  period_end timestamptz NOT NULL,
  revision integer NOT NULL DEFAULT 1,
  observed_value numeric,
  quality_flag text NOT NULL DEFAULT 'ok',
  retrieved_at timestamptz NOT NULL,
  source_revision_label text,
  payload_digest text NOT NULL,
  fetch_run_id uuid NOT NULL
    REFERENCES external.fetch_runs (id),
  PRIMARY KEY (source_code, series_code, period_start, revision),
  FOREIGN KEY (source_code, series_code, period_kind)
    REFERENCES external.series (source_code, series_code, period_kind),
  CONSTRAINT observations_period_half_open
    CHECK (period_end > period_start),
  CONSTRAINT observations_revision_valid CHECK (revision >= 1),
  CONSTRAINT observations_quality_flag_valid
    CHECK (quality_flag IN ('ok', 'estimated', 'suspect')),
  CONSTRAINT observations_digest_valid
    CHECK (payload_digest ~ '^[0-9a-f]{64}$'),
  -- A duração amarrada ao `period_kind` que a FK composta prova ser o da série.
  CONSTRAINT observations_period_matches_kind
    CHECK (
      (period_kind = 'instant' AND period_end = period_start + interval '1 second')
      OR (period_kind = 'hour' AND period_end = period_start + interval '1 hour')
      OR (period_kind = 'day' AND period_end = period_start + interval '1 day')
      OR (period_kind = 'month' AND period_end = period_start + interval '1 month')
      OR (period_kind = 'year' AND period_end = period_start + interval '1 year')
    )
);

-- Idempotência de escrita: digest igual é no-op (`ON CONFLICT DO NOTHING`);
-- digest diferente insere `revision = max+1`.
CREATE UNIQUE INDEX observations_idempotency_idx
  ON external.observations (
    source_code,
    series_code,
    period_start,
    payload_digest
  );

CREATE INDEX observations_series_period_idx
  ON external.observations (source_code, series_code, period_start DESC);

-- Constantes harmônicas do CHM não são polling e não podem morar em
-- `observations`, que é a tabela alimentada pelo worker: importação é curada,
-- por ato humano, e carrega o documento de proveniência.
CREATE TABLE external.tide_stations (
  station_code text PRIMARY KEY,
  station_name text NOT NULL,
  latitude numeric(8, 5) NOT NULL,
  longitude numeric(8, 5) NOT NULL,
  datum text NOT NULL,
  provenance_document text NOT NULL,
  imported_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT tide_stations_code_valid CHECK (station_code ~ '^[0-9]{5}$'),
  CONSTRAINT tide_stations_name_not_blank CHECK (btrim(station_name) <> ''),
  CONSTRAINT tide_stations_latitude_valid
    CHECK (latitude BETWEEN -90 AND 90),
  CONSTRAINT tide_stations_longitude_valid
    CHECK (longitude BETWEEN -180 AND 180),
  CONSTRAINT tide_stations_datum_not_blank CHECK (btrim(datum) <> ''),
  CONSTRAINT tide_stations_provenance_not_blank
    CHECK (btrim(provenance_document) <> '')
);

CREATE TABLE external.tide_harmonics (
  station_code text NOT NULL
    REFERENCES external.tide_stations (station_code),
  constituent text NOT NULL,
  amplitude numeric(8, 4) NOT NULL,
  phase_degrees numeric(7, 3) NOT NULL,
  datum text NOT NULL,
  epoch date NOT NULL,
  provenance_document text NOT NULL,
  harmonics_version integer NOT NULL,
  imported_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (station_code, constituent),
  CONSTRAINT tide_harmonics_constituent_valid
    CHECK (constituent ~ '^[A-Z][A-Z0-9]{0,7}$'),
  CONSTRAINT tide_harmonics_amplitude_valid CHECK (amplitude >= 0),
  CONSTRAINT tide_harmonics_phase_valid
    CHECK (phase_degrees >= 0 AND phase_degrees < 360),
  CONSTRAINT tide_harmonics_datum_not_blank CHECK (btrim(datum) <> ''),
  CONSTRAINT tide_harmonics_provenance_not_blank
    CHECK (btrim(provenance_document) <> ''),
  CONSTRAINT tide_harmonics_version_valid CHECK (harmonics_version >= 1)
);

COMMENT ON TABLE external.sources IS
  'Publicador, licença e texto de atribuição; o texto vai ao público como está.';
COMMENT ON TABLE external.series IS
  'Unidade, período, retenção e exposição pública; nunca na observação.';
COMMENT ON TABLE external.observations IS
  'Fato externo com revisão gravada; cache com prazo, não acervo.';
COMMENT ON TABLE external.fetch_runs IS
  'Torna "indisponível" distinguível de "não rodou"; o card lê o outcome.';
COMMENT ON TABLE external.tide_harmonics IS
  'Constantes curadas do CHM; sem elas o card de maré é unavailable declarado.';

-- A quinta view mora em `public_data`, não em `external`. Ela lê as tabelas
-- base sob os privilégios do dono, então `public_runtime` fica sem qualquer
-- privilégio e sem USAGE em `external`, e `publicRuntimeSearchPath`
-- (`store/public_pool.go:14`) não muda. Isso evita, por desenho, o modo de
-- falha em que ler `external` pelo pool público derruba o startup da API.
--
-- Dirigida por `series`, com LEFT JOIN na observação: uma série sem observação
-- ainda devolve uma linha com proveniência completa, que é o que a ADR-045 §7
-- exige do ramo `unavailable`.
CREATE VIEW public_data.current_external_context
WITH (security_barrier = true)
AS
SELECT
  series.card_code,
  series.source_code,
  series.series_code,
  series.unit_code,
  series.period_kind,
  series.data_mode,
  series.derived,
  series.derivation_code,
  series.unavailable_reason_code,
  series.definition_version,
  EXTRACT(EPOCH FROM series.declared_lag)::bigint AS declared_lag_seconds,
  source.publisher,
  source.license_code,
  source.license_url,
  source.attribution_text,
  source.terms_url,
  observation.period_start,
  observation.period_end,
  observation.observed_value,
  observation.quality_flag,
  observation.retrieved_at,
  observation.revision,
  observation.source_revision_label,
  last_run.outcome AS last_fetch_outcome,
  last_run.finished_at AS last_fetch_finished_at
FROM external.series AS series
JOIN external.sources AS source
  ON source.source_code = series.source_code
LEFT JOIN LATERAL (
  SELECT DISTINCT ON (candidate.period_start)
    candidate.period_start,
    candidate.period_end,
    candidate.observed_value,
    candidate.quality_flag,
    candidate.retrieved_at,
    candidate.revision,
    candidate.source_revision_label
  FROM external.observations AS candidate
  WHERE candidate.source_code = series.source_code
    AND candidate.series_code = series.series_code
  ORDER BY candidate.period_start DESC, candidate.revision DESC
) AS observation ON true
LEFT JOIN LATERAL (
  SELECT candidate.outcome, candidate.finished_at
  FROM external.fetch_runs AS candidate
  WHERE candidate.source_code = series.source_code
  ORDER BY candidate.started_at DESC
  LIMIT 1
) AS last_run ON true
WHERE series.public_exposable
  AND source.active;

COMMENT ON VIEW public_data.current_external_context IS
  'Recorte público da camada externa; filtra public_exposable e credita a fonte.';

-- Sexta view: a lista de créditos. Ela existe separada da quinta porque, por
-- U-7, o Cadastur é creditado **sem card** — não há linha com forma de card
-- onde ele caiba, e forçá-lo na quinta exigiria inventar um card sem valor,
-- que é exatamente o que a ADR-045 §5 proíbe.
--
-- Sem ela, a rota pública teria de ler `external.sources` pelo pool público, e
-- `public_runtime` não tem USAGE em `external` — a leitura falharia com
-- `permission denied for schema external` e a rota responderia 503 sempre,
-- mesmo com o documento montável.
CREATE VIEW public_data.current_external_sources
WITH (security_barrier = true)
AS
SELECT
  source.source_code,
  source.publisher,
  source.license_code,
  source.license_url,
  source.attribution_text,
  source.terms_url
FROM external.sources AS source
WHERE source.active;

COMMENT ON VIEW public_data.current_external_sources IS
  'Créditos das fontes ativas; atribuição e link, sem contagem nem série.';

COMMIT;

BEGIN;

-- Privilégios no padrão do ADR-030: REVOKE explícito antes de cada GRANT, e
-- default privileges futuros fechados, como a 000011 original fez para
-- `public_data`.
REVOKE ALL ON SCHEMA external
FROM PUBLIC, app_runtime, worker_runtime, public_runtime, privacy_officer,
  external_runtime;
REVOKE ALL ON ALL TABLES IN SCHEMA external
FROM PUBLIC, app_runtime, worker_runtime, public_runtime, privacy_officer,
  external_runtime;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA external
FROM PUBLIC, app_runtime, worker_runtime, public_runtime, privacy_officer,
  external_runtime;
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA external
FROM PUBLIC, app_runtime, worker_runtime, public_runtime, privacy_officer,
  external_runtime;

ALTER DEFAULT PRIVILEGES IN SCHEMA external
  REVOKE ALL ON TABLES FROM PUBLIC, app_runtime, worker_runtime,
    public_runtime, privacy_officer;
ALTER DEFAULT PRIVILEGES IN SCHEMA external
  REVOKE ALL ON SEQUENCES FROM PUBLIC, app_runtime, worker_runtime,
    public_runtime, privacy_officer;
ALTER DEFAULT PRIVILEGES IN SCHEMA external
  REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC, app_runtime, worker_runtime,
    public_runtime, privacy_officer;

-- A ingestão externa, e só ela, escreve aqui.
GRANT USAGE ON SCHEMA external TO external_runtime;
GRANT SELECT, INSERT, UPDATE ON TABLE
  external.sources,
  external.series,
  external.observations,
  external.fetch_runs,
  external.tide_stations,
  external.tide_harmonics
TO external_runtime;

-- DELETE somente onde a varredura de retenção precisa apagar: observação
-- vencida e run antiga. Catálogo, estação e harmônica são curados e não são
-- varridos.
GRANT DELETE ON TABLE
  external.observations,
  external.fetch_runs
TO external_runtime;

-- Direcionalidade da ADR-045 §1: a ingestão externa não enxerga nada da série
-- protegida nem do operacional.
REVOKE ALL ON SCHEMA identity, core, survey, analytics, public_data, platform
FROM external_runtime;
REVOKE ALL ON ALL TABLES IN SCHEMA
  identity, core, survey, analytics, public_data, platform
FROM external_runtime;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA
  identity, core, survey, analytics, public_data, platform
FROM external_runtime;
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA
  identity, core, survey, analytics, public_data, platform
FROM external_runtime;
REVOKE CREATE ON SCHEMA external FROM external_runtime;

-- E o inverso, que é a prova mais importante da onda: o papel que reconcilia a
-- série protegida não alcança a camada externa.
REVOKE ALL ON SCHEMA external FROM worker_runtime, app_runtime;
REVOKE ALL ON TABLE
  external.sources,
  external.series,
  external.observations,
  external.fetch_runs,
  external.tide_stations,
  external.tide_harmonics
FROM worker_runtime, app_runtime, public_runtime, privacy_officer;

-- O papel público lê a view, e só a view.
REVOKE ALL ON TABLE
  public_data.current_external_context,
  public_data.current_external_sources
FROM PUBLIC, app_runtime, worker_runtime, public_runtime, privacy_officer,
  external_runtime;
GRANT SELECT ON TABLE
  public_data.current_external_context,
  public_data.current_external_sources
TO public_runtime;

DO $$
BEGIN
  IF pg_catalog.has_schema_privilege('public_runtime', 'external', 'USAGE')
    OR pg_catalog.has_schema_privilege('worker_runtime', 'external', 'USAGE')
    OR pg_catalog.has_schema_privilege('app_runtime', 'external', 'USAGE')
  THEN
    RAISE EXCEPTION 'external schema USAGE leaked to a protected-series role';
  END IF;
  IF pg_catalog.has_schema_privilege(
      'external_runtime', 'analytics', 'USAGE'
    )
    OR pg_catalog.has_schema_privilege(
      'external_runtime', 'public_data', 'USAGE'
    )
    OR pg_catalog.has_schema_privilege('external_runtime', 'core', 'USAGE')
    OR pg_catalog.has_schema_privilege('external_runtime', 'survey', 'USAGE')
  THEN
    RAISE EXCEPTION 'external ingestion role reached the protected series';
  END IF;
END
$$;

COMMIT;
