/**
 * Runtime allowlist for every public analytics payload.
 *
 * The public surfaces are read by anonymous callers and cached by shared
 * caches, so the client refuses a body that carries a shape or a property the
 * contract does not declare, rather than rendering whatever arrived. The
 * combinators below build that allowlist declaratively; nothing here talks to
 * the network.
 */

import {
  arrayValidator,
  exactArrayValidator,
  integerValidator,
  isDate,
  isDateTime,
  literalValidator,
  objectValidator,
  record,
  unionValidator,
  type Validator,
} from "./payload-validators";

/**
 * Dias civis de histórico observado publicados pela release corrente. O número
 * é do contrato, não de uma escolha da interface: a metodologia o declara e
 * este allowlist recusa qualquer outro.
 */
export const PRESENCE_HISTORY_DAYS = 730;

const categoryCodePattern = /^[a-z][a-z0-9_]*$/u;
const civilMonthPattern = /^\d{4}-(?:0[1-9]|1[0-2])$/u;

const isPublicPercentage = integerValidator(0, 100, 5);
const isPublishedValue = integerValidator(0, Number.POSITIVE_INFINITY, 10);
const isQualityCountValue = integerValidator(0);
const isQualityRatio: Validator = (value) =>
  typeof value === "number" &&
  Number.isFinite(value) &&
  value >= 0 &&
  value <= 1;

const isPublicPeriod = objectValidator({
  start: isDate,
  end: isDate,
  end_exclusive: literalValidator(true),
  time_zone: literalValidator("America/Bahia"),
});
const isPublishedCoverage = objectValidator({
  status: literalValidator("published"),
  ratio: isPublicPercentage,
});
const isProtectedCoverage = objectValidator({
  status: literalValidator("protected", "unavailable"),
});
const isPublicCoverage = unionValidator(
  isPublishedCoverage,
  isProtectedCoverage,
);
const isPublicMetadata = objectValidator({
  period: isPublicPeriod,
  unit: literalValidator("person_day", "survey_response"),
  data_mode: literalValidator("prototype_fixtures"),
  updated_at: isDateTime,
  privacy_policy_version: literalValidator("prototype-v1"),
  methodology_version: literalValidator("explainable-baseline-v1"),
  coverage: isPublicCoverage,
});
const isPublishedObservedPoint = objectValidator({
  date: isDate,
  kind: literalValidator("observed"),
  status: literalValidator("published"),
  value: isPublishedValue,
});
const isPublishedForecastShape = objectValidator({
  date: isDate,
  kind: literalValidator("forecast"),
  status: literalValidator("published"),
  lower: isPublishedValue,
  central: isPublishedValue,
  upper: isPublishedValue,
});

/** A published forecast must arrive ordered; an inverted band is a contract break. */
function orderedBand(object: Record<string, unknown> | null) {
  const lower = Number(object?.lower);
  const central = Number(object?.central);
  return lower <= central && central <= Number(object?.upper);
}

function isPublishedForecastPoint(value: unknown) {
  return isPublishedForecastShape(value) && orderedBand(record(value));
}

const isProtectedObservedPoint = objectValidator({
  date: isDate,
  kind: literalValidator("observed"),
  status: literalValidator("protected", "unavailable"),
});
const isObservedPresencePoint = unionValidator(
  isPublishedObservedPoint,
  isProtectedObservedPoint,
);
const isProtectedForecastPoint = objectValidator({
  date: isDate,
  kind: literalValidator("forecast"),
  status: literalValidator("protected", "unavailable"),
});
const isForecastPresencePoint = unionValidator(
  isPublishedForecastPoint,
  isProtectedForecastPoint,
);
const isCivilMonth: Validator = (value) =>
  typeof value === "string" && civilMonthPattern.test(value);

/** Toda janela observada recorta a mesma série diária publicada. */
const RECENT_WINDOWS = [
  "recent_30_days",
  "recent_90_days",
  "recent_365_days",
  "recent_730_days",
] as const;
const OBSERVED_WINDOWS = [...RECENT_WINDOWS, "month"] as const;

const isObservedSeries = arrayValidator(
  isObservedPresencePoint,
  0,
  PRESENCE_HISTORY_DAYS,
);

/**
 * O par `window`/`month` é exigido dos dois lados: a data só existe dentro da
 * janela de mês, e a janela de mês sem data não nomeia documento. Aceitar
 * `month` como campo opcional de qualquer janela deixaria o cliente cego
 * justamente para a resposta inconsistente que ele deveria recusar.
 */
const isRecentWindowPresence = objectValidator({
  metadata: isPublicMetadata,
  window: literalValidator(...RECENT_WINDOWS),
  series: isObservedSeries,
});
const isMonthWindowPresence = objectValidator({
  metadata: isPublicMetadata,
  window: literalValidator("month"),
  month: isCivilMonth,
  series: isObservedSeries,
});
const isObservedPresence = unionValidator(
  isRecentWindowPresence,
  isMonthWindowPresence,
);
const isForecastPresence = objectValidator({
  metadata: isPublicMetadata,
  window: literalValidator("next_30_days"),
  series: arrayValidator(isForecastPresencePoint, 0, 30),
});
export const isPresence = unionValidator(isObservedPresence, isForecastPresence);
const isForecastPeakProtected = objectValidator({
  kind: literalValidator("forecast"),
  status: literalValidator("protected", "unavailable"),
});
const isForecastPeak = unionValidator(
  isPublishedForecastPoint,
  isForecastPeakProtected,
);
export const isSummary = objectValidator({
  metadata: isPublicMetadata,
  presence_today: isObservedPresencePoint,
  forecast_peak_next_30_days: isForecastPeak,
});
const isPublishedPreferenceCategory = objectValidator({
  category_code: literalValidator("first_visit", "returning"),
  status: literalValidator("published"),
  share_percent: isPublicPercentage,
});
const isProtectedPreferenceCategory = objectValidator({
  category_code: literalValidator("first_visit", "returning"),
  status: literalValidator("protected", "unavailable"),
});
const isPreferenceCategory = unionValidator(
  isPublishedPreferenceCategory,
  isProtectedPreferenceCategory,
);
const isPublicPreferenceMetric = objectValidator({
  metric_code: literalValidator("first_visit_share"),
  dimension_code: literalValidator("visit_profile"),
  categories: arrayValidator(isPreferenceCategory, 2, 2),
});
export const isPreferences = objectValidator({
  metadata: isPublicMetadata,
  period: literalValidator("last_complete_month"),
  metrics: arrayValidator(isPublicPreferenceMetric, 1, 1),
});
export const isMethodology = objectValidator({
  metadata: isPublicMetadata,
  presence_interval: literalValidator("[arrival,departure)"),
  time_zone: literalValidator("America/Bahia"),
  observed_definition_code: literalValidator(
    "checked_presence_through_as_of",
  ),
  forecast_definition_code: literalValidator("explainable-baseline-v1"),
  forecast_bounds_percent: exactArrayValidator(85, 115),
  forecast_fallback_bounds_percent: exactArrayValidator(70, 130),
  primary_threshold: literalValidator(10),
  minimum_reporting_accommodations: literalValidator(3),
  complementary_suppression: literalValidator(true),
  rounding_base: literalValidator(10),
  rounding_mode: literalValidator("stable-half-up"),
  presence_history_days: literalValidator(PRESENCE_HISTORY_DAYS),
  allowed_presence_windows: arrayValidator(
    literalValidator(...OBSERVED_WINDOWS, "next_30_days"),
    6,
    6,
  ),
  allowed_preference_periods: arrayValidator(
    literalValidator("last_complete_month"),
    1,
    1,
  ),
});
const isAvailableQualityCount = objectValidator({
  status: literalValidator("available"),
  value: isQualityCountValue,
});
const isUnavailableQualityCount = objectValidator({
  status: literalValidator("not_available"),
  reason_code: literalValidator(
    "not_implemented",
    "pseudonym_not_approved",
    "insufficient_source",
  ),
});
const isQualityCount = unionValidator(
  isAvailableQualityCount,
  isUnavailableQualityCount,
);
const isAvailableQualityCoverage = objectValidator({
  category_code: (value) =>
    typeof value === "string" && categoryCodePattern.test(value),
  status: literalValidator("available"),
  ratio: isQualityRatio,
});
const isUnavailableQualityCoverage = objectValidator({
  category_code: (value) =>
    typeof value === "string" && categoryCodePattern.test(value),
  status: literalValidator("not_available"),
});
const isQualityCoverage = unionValidator(
  isAvailableQualityCoverage,
  isUnavailableQualityCoverage,
);
export const isQuality = objectValidator({
  window: literalValidator("last_30_days"),
  updated_at: isDateTime,
  incomplete_stays: isQualityCount,
  overdue_planned_departures: isQualityCount,
  silent_accommodations: isQualityCount,
  aggregation_failures: isQualityCount,
  suspected_duplicates: isQualityCount,
  fnrh_failures: isQualityCount,
  coverage_by_category: arrayValidator(isQualityCoverage, 0, 50),
});
