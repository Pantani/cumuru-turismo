import type { components, operations } from "../../generated/schema";
import {
  isMethodology,
  isPreferences,
  isPresence,
  isFunnel,
  isQuality,
  isSummary,
} from "./analytics-payload";
import {
  ApiError,
  type HttpClientOptions,
  invalidResponse,
  resolveBaseUrl,
} from "./http-client";
import {
  isNoStore,
  MISSING_PUBLIC_ETAG_DETAIL,
  MISSING_REQUEST_ID_DETAIL,
  NON_NO_STORE_DETAIL,
  NON_PUBLIC_CACHE_DETAIL,
  problemFrom,
  PUBLIC_CACHE_CONTROL,
  publicEtagPattern,
  responseRequestId,
  retryAfterSeconds,
} from "./response-contract";

type Schemas = components["schemas"];
type PresenceWindow = components["parameters"]["PresenceWindow"];
type PresenceMonth = components["parameters"]["PresenceMonth"];
type PreferencePeriod = components["parameters"]["PreferencePeriod"];
type QualityWindow = components["parameters"]["QualityWindow"];

/**
 * The published metrics surface. It does not share the mutation transport: all
 * five operations are GETs, four of them are anonymous and cached by shared
 * caches, and every body is checked against a runtime allowlist before it
 * reaches a chart.
 */
export const analyticsOperationNames = [
  "getPublicSummary",
  "getPublicPresence",
  "getPublicPreferences",
  "getPublicMethodology",
  "getAnalyticsQuality",
  "getAnalyticsFunnel",
] as const satisfies readonly (keyof operations)[];

/**
 * `month` só acompanha `window=month`: enviado em qualquer outra janela o
 * servidor recusa a requisição, e omiti-lo dentro dela não nomeia documento.
 */
function presenceQuery(window: PresenceWindow, month?: PresenceMonth) {
  const query = new URLSearchParams({ window });
  if (window === "month" && month !== undefined) {
    query.set("month", month);
  }
  return query.toString();
}

export interface AnalyticsResult<T> {
  data: T;
  etag: string | null;
  requestId: string;
}

interface AnalyticsRequest {
  authenticated: boolean;
  isValid: (value: unknown) => boolean;
  path: string;
}

function requireNoStore(response: Response, requestId: string) {
  if (!isNoStore(response)) {
    throw invalidResponse(NON_NO_STORE_DETAIL, requestId);
  }
}

function requirePublicHeaders(response: Response, requestId: string) {
  if (response.headers.get("Cache-Control") !== PUBLIC_CACHE_CONTROL) {
    throw invalidResponse(NON_PUBLIC_CACHE_DETAIL, requestId);
  }
  if (!publicEtagPattern.test(response.headers.get("ETag") ?? "")) {
    throw invalidResponse(MISSING_PUBLIC_ETAG_DETAIL, requestId);
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

/**
 * A public body is cacheable, so the no-store rule the shared reader enforces
 * does not apply here; only the correlation id is mandatory at this point.
 */
function analyticsRequestId(response: Response) {
  const requestId = responseRequestId(response);
  if (requestId === null) {
    throw invalidResponse(MISSING_REQUEST_ID_DETAIL);
  }
  return requestId;
}

async function requireSuccess(response: Response, requestId: string) {
  if (!response.ok) {
    requireNoStore(response, requestId);
    throw new ApiError(
      response.status,
      await problemFrom(response),
      retryAfterSeconds(response),
      requestId,
    );
  }
  if (response.status !== 200) {
    throw invalidResponse(`Status ${response.status} inesperado.`, requestId);
  }
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
    throw new ApiError(
      401,
      {
        type: "urn:cumuru:problem:oidc-provider-unavailable",
        title: "Sessão institucional indisponível.",
        status: 401,
      },
      null,
    );
  }
  return token;
}

function buildRequest(
  baseUrl: string,
  path: string,
  token: string | null,
  authenticated: boolean,
) {
  const headers = new Headers({
    Accept: "application/json, application/problem+json",
  });
  if (token !== null) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  return new Request(new URL(path, baseUrl), {
    method: "GET",
    headers,
    cache: authenticated ? "no-store" : "default",
    credentials: "omit",
  });
}

export function createAnalyticsClient(options: HttpClientOptions) {
  const baseUrl = resolveBaseUrl(options.baseUrl);

  async function request<T>(
    spec: AnalyticsRequest,
  ): Promise<AnalyticsResult<T>> {
    const token = sessionToken(spec.authenticated, options.getAccessToken);
    // Resolved per call: this module builds a client at import time, long
    // before a caller can install a fetch double.
    const send = options.fetcher ?? fetch;
    const response = await send(
      buildRequest(baseUrl, spec.path, token, spec.authenticated),
    );
    const requestId = analyticsRequestId(response);
    await requireSuccess(response, requestId);
    requireCacheDiscipline(response, spec.authenticated, requestId);
    return {
      data: await payloadFrom<T>(response, requestId, spec.isValid),
      etag: response.headers.get("ETag"),
      requestId,
    };
  }

  const published = <T,>(path: string, isValid: (value: unknown) => boolean) =>
    request<T>({ authenticated: false, isValid, path });

  return {
    getSummary: () =>
      published<Schemas["PublicSummary"]>("/api/v1/public/summary", isSummary),
    getPresence: (window: PresenceWindow, month?: PresenceMonth) =>
      published<Schemas["PublicPresence"]>(
        `/api/v1/public/presence?${presenceQuery(window, month)}`,
        isPresence,
      ),
    getPreferences: (period: PreferencePeriod = "last_complete_month") =>
      published<Schemas["PublicPreferences"]>(
        `/api/v1/public/preferences?period=${encodeURIComponent(period)}`,
        isPreferences,
      ),
    getMethodology: () =>
      published<Schemas["PublicMethodology"]>(
        "/api/v1/public/methodology",
        isMethodology,
      ),
    getFunnel: (window: QualityWindow = "last_30_days") =>
      request<Schemas["AdoptionFunnel"]>({
        authenticated: true,
        isValid: isFunnel,
        path: `/api/v1/analytics/funnel?window=${encodeURIComponent(window)}`,
      }),
    getQuality: (window: QualityWindow = "last_30_days") =>
      request<Schemas["QualitySnapshot"]>({
        authenticated: true,
        isValid: isQuality,
        path: `/api/v1/analytics/quality?window=${encodeURIComponent(window)}`,
      }),
  };
}

export type AnalyticsClient = ReturnType<typeof createAnalyticsClient>;

export const publicAnalyticsClient = createAnalyticsClient({
  getAccessToken: () => null,
});
