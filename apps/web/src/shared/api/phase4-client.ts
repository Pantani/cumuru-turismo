import type { components, operations } from "../../generated/schema";
import {
  invalidResponseProblem,
  isNoStore,
  MISSING_REQUEST_ID_DETAIL,
  NON_NO_STORE_DETAIL,
  problemFrom,
  responseRequestId as contractRequestId,
} from "./response-contract";

type Schemas = components["schemas"];
type PresenceWindow = components["parameters"]["PresenceWindow"];
type PreferencePeriod = components["parameters"]["PreferencePeriod"];
type QualityWindow = components["parameters"]["QualityWindow"];

export const phase4OperationNames = [
  "getPublicSummary",
  "getPublicPresence",
  "getPublicPreferences",
  "getPublicMethodology",
  "getAnalyticsQuality",
] as const satisfies readonly (keyof operations)[];

export interface Phase4Result<T> {
  data: T;
  etag: string | null;
  requestId: string;
}

export class Phase4ApiError extends Error {
  readonly problem: Schemas["Problem"];
  readonly requestId: string | null;
  readonly status: number;

  constructor(
    status: number,
    problem: Schemas["Problem"],
    requestId: string | null,
  ) {
    super(problem.title);
    this.name = "Phase4ApiError";
    this.status = status;
    this.problem = problem;
    this.requestId = requestId;
  }
}

interface ClientOptions {
  baseUrl?: string;
  fetcher?: typeof fetch;
  getAccessToken: () => string | null;
}

interface RequestSpec {
  authenticated: boolean;
  isValid: (value: unknown) => boolean;
  path: string;
}

const publicCache = "public, max-age=300, stale-if-error=86400";
const publicEtagPattern = /^"sha256-[0-9a-f]{64}"$/;
const datePattern = /^\d{4}-\d{2}-\d{2}$/u;
const dateTimePattern =
  /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/u;
const categoryCodePattern = /^[a-z][a-z0-9_]*$/u;

type Validator = (value: unknown) => boolean;
type ValidatorMap = Readonly<Record<string, Validator>>;

function invalidResponse(detail: string, requestId: string | null = null) {
  return new Phase4ApiError(502, invalidResponseProblem(detail), requestId);
}

function missingSessionError() {
  return new Phase4ApiError(
    401,
    {
      type: "urn:cumuru:problem:oidc-provider-unavailable",
      title: "Sessão institucional indisponível.",
      status: 401,
    },
    null,
  );
}

function record(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function hasOwn(object: Record<string, unknown>, key: string) {
  return Object.prototype.hasOwnProperty.call(object, key);
}

function objectValidator(
  required: ValidatorMap,
  optional: ValidatorMap = {},
): Validator {
  const allowed = new Set([
    ...Object.keys(required),
    ...Object.keys(optional),
  ]);
  return (value) => {
    const object = record(value);
    if (object === null) {
      return false;
    }
    if (Object.keys(object).some((key) => !allowed.has(key))) {
      return false;
    }
    const requiredValid = Object.entries(required).every(
      ([key, validate]) => hasOwn(object, key) && validate(object[key]),
    );
    if (!requiredValid) {
      return false;
    }
    return Object.entries(optional).every(
      ([key, validate]) => !hasOwn(object, key) || validate(object[key]),
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
const isPresence = unionValidator(isObservedPresence, isForecastPresence);
const isForecastPeakProtected = objectValidator({
  kind: literalValidator("forecast"),
  status: literalValidator("protected", "unavailable"),
});
const isForecastPeak = unionValidator(
  isPublishedForecastPoint,
  isForecastPeakProtected,
);
const isSummary = objectValidator({
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
const isPreferences = objectValidator({
  metadata: isPublicMetadata,
  period: literalValidator("last_complete_month"),
  metrics: arrayValidator(isPublicPreferenceMetric, 1, 1),
});
const isMethodology = objectValidator({
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
    "phase_not_implemented",
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
const isQuality = objectValidator({
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

function requestIdFrom(response: Response) {
  const requestId = contractRequestId(response);
  if (requestId === null) {
    throw invalidResponse(MISSING_REQUEST_ID_DETAIL);
  }
  return requestId;
}

function requireNoStore(response: Response, requestId: string) {
  if (!isNoStore(response)) {
    throw invalidResponse(NON_NO_STORE_DETAIL, requestId);
  }
}

function requirePublicHeaders(response: Response, requestId: string) {
  if (response.headers.get("Cache-Control") !== publicCache) {
    throw invalidResponse(
      "Cache-Control público obrigatório ausente ou inválido.",
      requestId,
    );
  }
  if (!publicEtagPattern.test(response.headers.get("ETag") ?? "")) {
    throw invalidResponse("ETag público forte ausente ou inválido.", requestId);
  }
}

function requestHeaders(token: string | null) {
  const headers = new Headers({
    Accept: "application/json, application/problem+json",
  });
  if (token !== null) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  return headers;
}

function buildRequest(
  baseUrl: string,
  path: string,
  token: string | null,
  authenticated: boolean,
) {
  return new Request(new URL(path, baseUrl), {
    method: "GET",
    headers: requestHeaders(token),
    cache: authenticated ? "no-store" : "default",
    credentials: "omit",
  });
}

async function payloadFrom<T>(
  response: Response,
  requestId: string,
  isValid: (value: unknown) => boolean,
) {
  const value = (await response.json()) as unknown;
  if (!isValid(value)) {
    throw invalidResponse(
      "O payload contém shape ou propriedade fora da allowlist.",
      requestId,
    );
  }
  return value as T;
}

function sessionToken(
  authenticated: boolean,
  getAccessToken: () => string | null,
) {
  if (!authenticated) {
    return null;
  }
  const token = getAccessToken();
  if (token === null || token.length === 0) {
    throw missingSessionError();
  }
  return token;
}

async function requireSuccess(response: Response, requestId: string) {
  if (!response.ok) {
    requireNoStore(response, requestId);
    throw new Phase4ApiError(
      response.status,
      await problemFrom(response),
      requestId,
    );
  }
  if (response.status !== 200) {
    throw invalidResponse(`Status ${response.status} inesperado.`, requestId);
  }
}

/**
 * An authenticated payload must never be cached; a public one carries the
 * shared-cache headers instead.
 */
function requireCacheDiscipline(
  response: Response,
  authenticated: boolean,
  requestId: string,
) {
  if (authenticated) {
    requireNoStore(response, requestId);
    return;
  }
  requirePublicHeaders(response, requestId);
}

export function createPhase4Client(options: ClientOptions) {
  const baseUrl =
    options.baseUrl ??
    (typeof window === "undefined"
      ? "http://localhost"
      : window.location.origin);

  async function request<T>(
    spec: RequestSpec,
  ): Promise<Phase4Result<T>> {
    const token = sessionToken(spec.authenticated, options.getAccessToken);
    const response = await (options.fetcher ?? fetch)(
      buildRequest(baseUrl, spec.path, token, spec.authenticated),
    );
    const requestId = requestIdFrom(response);
    await requireSuccess(response, requestId);
    requireCacheDiscipline(response, spec.authenticated, requestId);
    return {
      data: await payloadFrom<T>(response, requestId, spec.isValid),
      etag: response.headers.get("ETag"),
      requestId,
    };
  }

  return {
    getSummary: () =>
      request<Schemas["PublicSummary"]>({
        authenticated: false,
        isValid: isSummary,
        path: "/api/v1/public/summary",
      }),
    getPresence: (window: PresenceWindow) =>
      request<Schemas["PublicPresence"]>({
        authenticated: false,
        isValid: isPresence,
        path: `/api/v1/public/presence?window=${encodeURIComponent(window)}`,
      }),
    getPreferences: (
      period: PreferencePeriod = "last_complete_month",
    ) =>
      request<Schemas["PublicPreferences"]>({
        authenticated: false,
        isValid: isPreferences,
        path: `/api/v1/public/preferences?period=${encodeURIComponent(period)}`,
      }),
    getMethodology: () =>
      request<Schemas["PublicMethodology"]>({
        authenticated: false,
        isValid: isMethodology,
        path: "/api/v1/public/methodology",
      }),
    getQuality: (window: QualityWindow = "last_30_days") =>
      request<Schemas["QualitySnapshot"]>({
        authenticated: true,
        isValid: isQuality,
        path: `/api/v1/analytics/quality?window=${encodeURIComponent(window)}`,
      }),
  };
}

export type Phase4Client = ReturnType<typeof createPhase4Client>;

export const phase4PublicClient = createPhase4Client({
  getAccessToken: () => null,
});
