/**
 * Documento de contexto externo usado pelos testes desta aba.
 *
 * Fixture, e não resposta de rede: nenhum gate desta onda depende de internet
 * pública. O card de maré nasce indisponível com `constants_not_imported`
 * porque é assim que ele nasce em produção — o gate é de direitos, e nenhuma
 * linha de código o destrava (ADR-045 §8).
 */

import type { components } from "../../../generated/schema";

type Schemas = components["schemas"];

export const OPEN_METEO_ATTRIBUTION =
  "Dados meteorológicos por Open-Meteo.com, licenciados sob CC BY 4.0; modelos a montante do ERA5/Copernicus.";
export const CADASTUR_ATTRIBUTION =
  "Cadastur — Ministério do Turismo, registro público de prestadores de serviços turísticos.";

const coveredPeriod: Schemas["ExternalCoveredPeriod"] = {
  start: "2026-08-15T03:00:00Z",
  end: "2026-08-18T03:00:00Z",
  end_exclusive: true,
  time_zone: "America/Bahia",
};

export const WEATHER_RETRIEVED_AT = "2026-08-18T09:00:00Z";
export const WEATHER_OBSERVED_AT = "2026-08-17T15:00:00Z";
/** Uma hora depois da coleta: dentro da defasagem declarada e da folga. */
export const FRESH_NOW = Date.parse("2026-08-18T10:00:00Z");
/** Nove dias depois: muito além de qualquer defasagem declarada. */
export const STALE_NOW = Date.parse("2026-08-27T10:00:00Z");

const weatherCard: Schemas["PublishedContextCard"] = {
  card_code: "weather_daily",
  status: "published",
  data_mode: "real_source",
  unit_code: "celsius",
  provenance: {
    source_code: "open_meteo_forecast",
    publisher: "Open-Meteo",
    license_code: "CC-BY-4.0",
    license_url: "https://creativecommons.org/licenses/by/4.0/",
    attribution_text: OPEN_METEO_ATTRIBUTION,
    terms_url: "https://open-meteo.com/en/terms",
    retrieved_at: WEATHER_RETRIEVED_AT,
    observed_at: WEATHER_OBSERVED_AT,
    covered_period: coveredPeriod,
    declared_lag_seconds: 10800,
    revision: 0,
    derived: false,
  },
  series: [
    {
      period_start: "2026-08-15T03:00:00Z",
      period_end: "2026-08-16T03:00:00Z",
      value: 24.7,
    },
    {
      period_start: "2026-08-16T03:00:00Z",
      period_end: "2026-08-17T03:00:00Z",
      value: 25.2,
    },
    {
      period_start: "2026-08-17T03:00:00Z",
      period_end: "2026-08-18T03:00:00Z",
      value: 26.1,
    },
  ],
};

const tideCard: Schemas["UnavailableContextCard"] = {
  card_code: "tide",
  status: "unavailable",
  data_mode: "real_source",
  reason_code: "constants_not_imported",
  provenance: {
    source_code: "chm_harmonics",
    publisher: "Centro de Hidrografia da Marinha",
    license_code: "CHM-BNDO-termo-de-compromisso",
    license_url: "https://www.marinha.mil.br/chm/dados-do-bndo",
    attribution_text:
      "Constantes harmônicas do Banco Nacional de Dados Oceanográficos, Centro de Hidrografia da Marinha.",
    terms_url: "https://www.marinha.mil.br/chm/dados-do-bndo/termo",
    retrieved_at: WEATHER_RETRIEVED_AT,
    covered_period: coveredPeriod,
    declared_lag_seconds: 0,
    revision: 0,
    derived: true,
    derivation_code: "tide_harmonic_prediction",
  },
};

const cadasturSource: Schemas["ExternalCreditedSource"] = {
  source_code: "cadastur",
  publisher: "Ministério do Turismo",
  license_code: "CC-BY-4.0",
  license_url: "https://creativecommons.org/licenses/by/4.0/",
  attribution_text: CADASTUR_ATTRIBUTION,
  terms_url: "https://dados.gov.br/dados/conjuntos-dados/cadastur",
};

export const contextDocument: Schemas["PublicContext"] = {
  generated_at: "2026-08-18T09:05:00Z",
  layer: "external_context",
  disclaimer_code: "external_context_not_platform_measurement",
  cards: [weatherCard, tideCard],
  sources: [
    {
      source_code: "open_meteo_forecast",
      publisher: "Open-Meteo",
      license_code: "CC-BY-4.0",
      license_url: "https://creativecommons.org/licenses/by/4.0/",
      attribution_text: OPEN_METEO_ATTRIBUTION,
      terms_url: "https://open-meteo.com/en/terms",
    },
    cadasturSource,
  ],
};
