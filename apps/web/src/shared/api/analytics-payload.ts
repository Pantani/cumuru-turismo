/**
 * Runtime allowlist for every public analytics payload.
 *
 * The public surfaces are read by anonymous callers and cached by shared
 * caches, so the client refuses a body that carries a shape or a property the
 * contract does not declare, rather than rendering whatever arrived. The
 * combinators below build that allowlist declaratively; nothing here talks to
 * the network.
 */

type Validator = (value: unknown) => boolean;
type ValidatorMap = Readonly<Record<string, Validator>>;

const datePattern = /^\d{4}-\d{2}-\d{2}$/u;
const dateTimePattern =
  /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/u;
const categoryCodePattern = /^[a-z][a-z0-9_]*$/u;

function record(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function hasOwn(object: Record<string, unknown>, key: string) {
  return Object.prototype.hasOwnProperty.call(object, key);
}

function allowedKeys(object: Record<string, unknown>, allowed: Set<string>) {
  return Object.keys(object).every((key) => allowed.has(key));
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
      allowedKeys(object, allowed) &&
      requiredValid(object, required) &&
      optionalValid(object, optional)
    );
  };
}

function arrayValidator(
  validateItem: Validator,
  minimum = 0,
  maximum = Number.POSITIVE_INFINITY,
): Validator {
  return (value) =>
    Array.isArray(value) &&
    value.length >= minimum &&
    value.length <= maximum &&
    value.every(validateItem);
}

function unionValidator(...validators: Validator[]): Validator {
  return (value) => validators.some((validate) => validate(value));
}

function literalValidator(...allowed: readonly unknown[]): Validator {
  const values = new Set(allowed);
  return (value) => values.has(value);
}

function exactArrayValidator(...expected: readonly unknown[]): Validator {
  return (value) =>
    Array.isArray(value) &&
    value.length === expected.length &&
    expected.every((item, index) => Object.is(value[index], item));
}

function integerValidator(
  minimum: number,
  maximum = Number.POSITIVE_INFINITY,
  multiple = 1,
): Validator {
  return (value) =>
    typeof value === "number" &&
    Number.isInteger(value) &&
    value >= minimum &&
    value <= maximum &&
    value % multiple === 0;
}
const isDate: Validator = (value) =>
  typeof value === "string" && datePattern.test(value);
const isDateTime: Validator = (value) =>
  typeof value === "string" &&
  dateTimePattern.test(value) &&
  Number.isFinite(Date.parse(value));
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
const isObservedPresence = objectValidator({
  metadata: isPublicMetadata,
  window: literalValidator("recent_30_days"),
  series: arrayValidator(isObservedPresencePoint, 0, 30),
});
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
  allowed_presence_windows: arrayValidator(
    literalValidator("recent_30_days", "next_30_days"),
    2,
    2,
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
