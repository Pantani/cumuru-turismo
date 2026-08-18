-- Volta ao recorte de trinta dias. A série longa não cabe no vocabulário
-- anterior: as células fora da janela são apagadas antes do rename, porque
-- `recent_30_days` carregando dois anos de dias seria um seletor mentindo
-- sobre o próprio período.
BEGIN;

ALTER TABLE analytics.metric_catalog
  DROP CONSTRAINT metric_catalog_shape_valid;

ALTER TABLE public_data.metric_cells
  DROP CONSTRAINT metric_cells_catalog_valid;

ALTER TABLE public_data.metric_cells
  DROP CONSTRAINT metric_cells_presence_selector_kind_valid;

DELETE FROM public_data.metric_cells AS cell
USING public_data.publications AS publication
WHERE publication.publication_version = cell.publication_version
  AND cell.metric_code = 'presence'
  AND cell.period_selector = 'observed_daily'
  AND cell.period_start < publication.as_of_on - 29;

UPDATE public_data.metric_cells
SET period_selector = 'recent_30_days'
WHERE metric_code = 'presence'
  AND period_selector = 'observed_daily';

UPDATE analytics.metric_catalog
SET period_selector = 'recent_30_days'
WHERE metric_code = 'presence'
  AND period_selector = 'observed_daily';

ALTER TABLE analytics.metric_catalog
  ADD CONSTRAINT metric_catalog_shape_valid
  CHECK (
    (
      metric_code = 'presence'
      AND unit = 'person_day'
      AND period_selector IN ('recent_30_days', 'next_30_days')
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
      AND period_selector IN ('recent_30_days', 'next_30_days')
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
      period_selector = 'recent_30_days'
      AND kind = 'observed'
    )
    OR (
      period_selector = 'next_30_days'
      AND kind = 'forecast'
    )
  );

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
FROM public_data.current_presence;

-- Remover coluna exige recriar a view; `CREATE OR REPLACE` só acrescenta ao
-- fim da lista. O único privilégio que a view carrega é o SELECT do papel
-- público, reposto logo abaixo.
DROP VIEW public_data.current_methodology;

CREATE VIEW public_data.current_methodology
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
  ARRAY['recent_30_days', 'next_30_days']::text[]
    AS allowed_presence_windows,
  ARRAY['last_complete_month']::text[]
    AS allowed_preference_periods,
  70::integer AS forecast_fallback_lower_percent,
  130::integer AS forecast_fallback_upper_percent
FROM public_data.current_publication AS current
JOIN public_data.publications AS publication
  ON publication.publication_version = current.publication_version
WHERE current.singleton;

GRANT SELECT ON TABLE public_data.current_methodology TO public_runtime;

COMMIT;
