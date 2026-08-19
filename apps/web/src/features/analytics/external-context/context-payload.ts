/**
 * Allowlist de runtime do documento `GET /public/context`.
 *
 * A camada externa tem allowlist própria, e não a de `analytics-payload.ts`,
 * pela mesma razão que ADR-045 §2 lhe dá endpoint e ETag próprias: um
 * validador compartilhado seria o primeiro lugar onde a fronteira entre
 * medido e copiado voltaria a se dissolver. Aqui não há amostra, não há
 * `sample_size` e não há arredondamento em base 10 — copiar aquele validador
 * afirmaria que a tubulação de supressão rodou sobre este número.
 *
 * Nada neste módulo fala com a rede.
 */

import type { components } from "../../../generated/schema";

type Schemas = components["schemas"];

type Validator = (value: unknown) => boolean;
type ValidatorMap = Readonly<Record<string, Validator>>;

const dateTimePattern =
  /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/u;
/** Só `https`: ADR-045 §6 recusa esquema em claro em toda a camada externa. */
const httpsPattern = /^https:\/\/\S+$/u;

function record(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function hasOwn(object: Record<string, unknown>, key: string) {
  return Object.prototype.hasOwnProperty.call(object, key);
}

function requiredValid(object: Record<string, unknown>, required: ValidatorMap) {
  return Object.entries(required).every(
    ([key, validate]) => hasOwn(object, key) && validate(object[key]),
  );
}

function optionalValid(object: Record<string, unknown>, optional: ValidatorMap) {
  return Object.entries(optional).every(
    ([key, validate]) => !hasOwn(object, key) || validate(object[key]),
  );
}

function objectValidator(
  required: ValidatorMap,
  optional: ValidatorMap = {},
): Validator {
  const allowed = new Set([...Object.keys(required), ...Object.keys(optional)]);
  return (value) => {
    const object = record(value);
    return (
      object !== null &&
      Object.keys(object).every((key) => allowed.has(key)) &&
      requiredValid(object, required) &&
      optionalValid(object, optional)
    );
  };
}

function arrayValidator(item: Validator, minimum: number, maximum: number) {
  return (value: unknown) =>
    Array.isArray(value) &&
    value.length >= minimum &&
    value.length <= maximum &&
    value.every(item);
}

function unionValidator(...validators: readonly Validator[]): Validator {
  return (value) => validators.some((validate) => validate(value));
}

function literalValidator(...allowed: readonly unknown[]): Validator {
  const values = new Set(allowed);
  return (value) => values.has(value);
}

function textValidator(maximum: number): Validator {
  return (value) =>
    typeof value === "string" && value.length >= 1 && value.length <= maximum;
}

/**
 * Instante que existe no calendário, e não só na gramática.
 *
 * `Date.parse` rola data impossível em vez de recusá-la: `2026-02-31` vira 3 de
 * março, e `2027-02-29` vira 1º de março. Hora fora de faixa já cai em `NaN`,
 * então só o dia civil precisa de checagem própria, feita por ida e volta —
 * o dia é aceito quando a data reconstruída devolve o mesmo texto.
 *
 * Não é preciosismo de formato. Este documento é anônimo e cacheável por cache
 * compartilhado, e a proveniência que ele carrega é obrigação de licença: um
 * `observed_at` rolado mostraria ao leitor um instante de origem que a fonte
 * nunca publicou, e um `retrieved_at` rolado deslocaria a classificação de
 * frescor em dias, fazendo coleta velha se anunciar recente. O módulo já
 * recusa licença servida fora de `https` pela mesma razão: a allowlist não
 * pergunta se o nosso servidor erraria.
 */
function isRealInstant(text: string): boolean {
  const day = text.slice(0, 10);
  const reconstructed = new Date(`${day}T00:00:00Z`);
  return (
    Number.isFinite(Date.parse(text)) &&
    Number.isFinite(reconstructed.getTime()) &&
    reconstructed.toISOString().startsWith(day)
  );
}

const isDateTime: Validator = (value) =>
  typeof value === "string" &&
  dateTimePattern.test(value) &&
  isRealInstant(value);

const isHttpsUrl: Validator = (value) =>
  typeof value === "string" && httpsPattern.test(value);

const isBoolean: Validator = (value) => typeof value === "boolean";

const isCountedInteger: Validator = (value) =>
  typeof value === "number" && Number.isInteger(value) && value >= 0;

/**
 * Sem `multipleOf`, por assimetria deliberada contra `/public/presence`: lá o
 * arredondamento em base 10 é controle de divulgação, e aqui não há nada a
 * suprimir.
 */
const isObservationValue: Validator = (value) =>
  typeof value === "number" && Number.isFinite(value);

export const EXTERNAL_SOURCE_CODES = [
  "open_meteo_forecast",
  "open_meteo_archive",
  "open_meteo_marine",
  "wikimedia_pageviews",
  "ibge_aggregates",
  "brasilapi_holidays",
  "cadastur",
  "chm_harmonics",
] as const satisfies readonly Schemas["ExternalSourceCode"][];

export const EXTERNAL_UNIT_CODES = [
  "celsius",
  "millimetre",
  "metre",
  "metre_per_second",
  "pageview",
  "person",
  "brl",
  "count",
  "degree",
] as const satisfies readonly Schemas["ExternalUnitCode"][];

export const EXTERNAL_CARD_CODES = [
  "weather_daily",
  "tide",
] as const satisfies readonly Schemas["ExternalCardCode"][];

export const EXTERNAL_DATA_MODES = [
  "real_source",
  "prototype_fixtures",
] as const satisfies readonly Schemas["ExternalCardDataMode"][];

/**
 * Lista fechada, sem texto livre, e sem `protected`: na série protegida essa
 * palavra significa "reprovado pelo limiar k-anônimo", e reusá-la aqui
 * afirmaria que a supressão rodou sobre dado que não passou por ela.
 */
export const EXTERNAL_REASON_CODES = [
  "source_unavailable",
  "source_rate_limited",
  "source_not_licensed",
  "source_data_missing",
  "constants_not_imported",
  "stale_beyond_declared_lag",
] as const satisfies readonly Schemas["UnavailableContextCard"]["reason_code"][];

const creditedFields: ValidatorMap = {
  source_code: literalValidator(...EXTERNAL_SOURCE_CODES),
  publisher: textValidator(200),
  license_code: textValidator(100),
  license_url: isHttpsUrl,
  attribution_text: textValidator(500),
  terms_url: isHttpsUrl,
};

const isCoveredPeriod = objectValidator({
  start: isDateTime,
  end: isDateTime,
  end_exclusive: literalValidator(true),
  time_zone: literalValidator("America/Bahia"),
});

const isCreditedSource = objectValidator(creditedFields);

/**
 * Obrigatória também no ramo indisponível: fonte, licença e atribuição existem
 * porque a fonte existe, não porque a requisição deu certo (ADR-045 §7).
 */
const isProvenance = objectValidator(
  {
    ...creditedFields,
    retrieved_at: isDateTime,
    covered_period: isCoveredPeriod,
    declared_lag_seconds: isCountedInteger,
    revision: isCountedInteger,
    derived: isBoolean,
  },
  {
    observed_at: isDateTime,
    derivation_code: literalValidator("tide_harmonic_prediction"),
    source_revision_label: textValidator(100),
  },
);

const isSeriesPoint = objectValidator({
  period_start: isDateTime,
  period_end: isDateTime,
  value: isObservationValue,
});

const cardFields: ValidatorMap = {
  card_code: literalValidator(...EXTERNAL_CARD_CODES),
  data_mode: literalValidator(...EXTERNAL_DATA_MODES),
  provenance: isProvenance,
};

const isPublishedCard = objectValidator({
  ...cardFields,
  status: literalValidator("published"),
  unit_code: literalValidator(...EXTERNAL_UNIT_CODES),
  series: arrayValidator(isSeriesPoint, 1, 800),
});

const isUnavailableCard = objectValidator({
  ...cardFields,
  status: literalValidator("unavailable"),
  reason_code: literalValidator(...EXTERNAL_REASON_CODES),
});

export const isPublicContext = objectValidator({
  generated_at: isDateTime,
  layer: literalValidator("external_context"),
  disclaimer_code: literalValidator(
    "external_context_not_platform_measurement",
  ),
  cards: arrayValidator(unionValidator(isPublishedCard, isUnavailableCard), 1, 12),
  sources: arrayValidator(isCreditedSource, 1, 12),
});
