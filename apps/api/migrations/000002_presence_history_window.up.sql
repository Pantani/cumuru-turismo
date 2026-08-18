-- O histórico observado deixa de ser um recorte de trinta dias gravado como
-- seletor próprio e passa a ser uma série diária única de dois anos. A janela
-- volta a ser o que sempre foi na leitura pública — um recorte — em vez de um
-- valor de armazenamento, e por isso o mesmo dia não é mais duplicado célula a
-- célula a cada janela que a capa oferece.
BEGIN;

ALTER TABLE analytics.metric_catalog
  DROP CONSTRAINT metric_catalog_shape_valid;

ALTER TABLE public_data.metric_cells
  DROP CONSTRAINT metric_cells_catalog_valid;

ALTER TABLE public_data.metric_cells
  DROP CONSTRAINT metric_cells_presence_selector_kind_valid;

UPDATE analytics.metric_catalog
SET period_selector = 'observed_daily'
WHERE metric_code = 'presence'
  AND period_selector = 'recent_30_days';

UPDATE public_data.metric_cells
SET period_selector = 'observed_daily'
WHERE metric_code = 'presence'
  AND period_selector = 'recent_30_days';

ALTER TABLE analytics.metric_catalog
  ADD CONSTRAINT metric_catalog_shape_valid
  CHECK (
    (
      metric_code = 'presence'
      AND unit = 'person_day'
      AND period_selector IN ('observed_daily', 'next_30_days')
      AND dimension_code = 'none'
    )
    OR
    (
      metric_code = 'first_visit_share'
      AND unit = 'survey_response'
      AND period_selector = 'last_complete_month'
      AND dimension_code = 'visit_profile'
    )
  );

ALTER TABLE public_data.metric_cells
  ADD CONSTRAINT metric_cells_catalog_valid
  CHECK (
    (
      metric_code = 'presence'
      AND period_selector IN ('observed_daily', 'next_30_days')
      AND unit = 'person_day'
      AND dimension_code = 'none'
      AND category_code = 'none'
      AND kind IN ('observed', 'forecast')
      AND period_end = period_start + 1
    )
    OR
    (
      metric_code = 'first_visit_share'
      AND period_selector = 'last_complete_month'
      AND unit = 'survey_response'
      AND dimension_code = 'visit_profile'
      AND category_code IN ('first_visit', 'returning')
      AND kind = 'preference'
    )
  );

ALTER TABLE public_data.metric_cells
  ADD CONSTRAINT metric_cells_presence_selector_kind_valid
  CHECK (
    metric_code <> 'presence'
    OR (
      period_selector = 'observed_daily'
      AND kind = 'observed'
    )
    OR (
      period_selector = 'next_30_days'
      AND kind = 'forecast'
    )
  );

-- O resumo é a capa: um dia observado e o horizonte previsto. Sem o recorte a
-- view devolveria os 730 dias de histórico para responder "quantos hoje".
CREATE OR REPLACE VIEW public_data.current_summary
WITH (security_barrier = true)
AS
SELECT
  period_selector,
  period_start,
  period_end,
  unit,
  kind,
  status,
  published_value,
  published_lower,
  published_central,
  published_upper,
  as_of_on,
  data_mode,
  privacy_policy_version,
  methodology_version,
  coverage_status,
  coverage_ratio_percent,
  published_at
FROM public_data.current_presence
WHERE kind = 'forecast'
  OR period_start = as_of_on;

CREATE OR REPLACE VIEW public_data.current_methodology
WITH (security_barrier = true)
AS
SELECT
  publication.as_of_on,
  publication.data_mode,
  publication.privacy_policy_version,
  publication.methodology_version,
  publication.coverage_status,
  publication.coverage_ratio_percent,
  publication.published_at,
  '[arrival,departure)'::text AS presence_interval,
  'America/Bahia'::text AS time_zone,
  'checked_presence_through_as_of'::text AS observed_definition_code,
  'explainable-baseline-v1'::text AS forecast_definition_code,
  85::integer AS forecast_lower_percent,
  115::integer AS forecast_upper_percent,
  10::integer AS primary_threshold,
  3::integer AS minimum_reporting_accommodations,
  true AS complementary_suppression,
  10::integer AS rounding_base,
  'stable-half-up'::text AS rounding_mode,
  ARRAY[
    'recent_30_days',
    'recent_90_days',
    'recent_365_days',
    'recent_730_days',
    'next_30_days',
    'month'
  ]::text[]
    AS allowed_presence_windows,
  ARRAY['last_complete_month']::text[]
    AS allowed_preference_periods,
  70::integer AS forecast_fallback_lower_percent,
  130::integer AS forecast_fallback_upper_percent,
  -- Coluna nova entra no fim da lista: `CREATE OR REPLACE VIEW` recusa
  -- inserção no meio, e recriar a view custaria os grants do papel público.
  730::integer AS presence_history_days
FROM public_data.current_publication AS current
JOIN public_data.publications AS publication
  ON publication.publication_version = current.publication_version
WHERE current.singleton;

COMMIT;
