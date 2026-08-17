import type { components } from "../../generated/schema";
import {
  INVALID_SURVEY_CAPABILITY_DETAIL,
  invalidResponseProblem,
  isNoStore,
  MISSING_ETAG_DETAIL,
  MISSING_LOCATION_DETAIL,
  MISSING_REPLAY_DETAIL,
  MISSING_REQUEST_ID_DETAIL,
  NON_NO_STORE_DETAIL,
  problemFrom,
  relativeLocationPattern,
  replayedFlag,
  responseRequestId,
  retryAfterSeconds,
  strongEtagPattern,
  surveyCapabilityPattern,
} from "./response-contract";

type Schemas = components["schemas"];

/**
 * The single transport every domain client is built on.
 *
 * Each client used to own a private copy of the request builder, the session
 * check, the header contract and its own error class. Four copies meant four
 * places to tighten a rule and, for the caller, four unrelated error types to
 * catch for one kind of failure — `use-operation` really did have to test
 * `instanceof` twice to render a single message. One transport and one
 * `ApiError` remove both problems.
 */

export interface ApiResult<T> {
  data: T;
  etag: string | null;
  location: string | null;
  replayed: boolean | null;
  requestId: string;
  surveyCapability: string | null;
}

export class ApiError extends Error {
  readonly problem: Schemas["Problem"];
  readonly requestId: string | null;
  readonly retryAfterSeconds: number | null;
  readonly status: number;

  constructor(
    status: number,
    problem: Schemas["Problem"],
    retryAfterSeconds: number | null,
    requestId: string | null = null,
  ) {
    super(problem.title);
    this.name = "ApiError";
    this.status = status;
    this.problem = problem;
    this.retryAfterSeconds = retryAfterSeconds;
    this.requestId = requestId;
  }
}

/**
 * A capability token travels only in its own custom header — never in the path,
 * never in a query string, never in a fragment sent to the server — and each
 * purpose owns a distinct header, because separate purposes are separate
 * cryptographic domains (ADR-039, ADR-041).
 */
export interface CapabilityHeader {
  header: string;
  value: string;
}

export interface RequestSpec {
  authenticated?: boolean;
  body?: unknown;
  capability?: CapabilityHeader;
  contentType?: string;
  etag?: boolean;
  headers?: Record<string, string>;
  location?: boolean;
  method: string;
  noContent?: boolean;
  path: string;
  replay?: boolean;
  status?: number;
}

export interface HttpClientOptions {
  baseUrl?: string;
  fetcher?: typeof fetch;
  getAccessToken: () => string | null;
}

const capabilityPattern = /^[A-Za-z0-9_-]{64,128}$/u;

export function clientError(status: number, title: string) {
  return new ApiError(
    status,
    { type: "urn:cumuru:problem:invalid-api-response", title, status },
    null,
  );
}

export function invalidResponse(detail: string, requestId: string | null = null) {
  return new ApiError(502, invalidResponseProblem(detail), null, requestId);
}

export function resolveBaseUrl(configured: string | undefined) {
  if (configured !== undefined) {
    return configured;
  }
  return typeof window === "undefined"
    ? "http://localhost"
    : window.location.origin;
}

/**
 * Reads the correlation id and proves the body may not be cached. Both checks
 * belong together: a response without an id cannot report its own violation.
 */
export function requireRequestId(response: Response) {
  const requestId = responseRequestId(response);
  if (requestId === null) {
    throw invalidResponse(MISSING_REQUEST_ID_DETAIL);
  }
  if (!isNoStore(response)) {
    throw invalidResponse(NON_NO_STORE_DETAIL, requestId);
  }
  return requestId;
}

function headerMatches(response: Response, name: string, pattern: RegExp) {
  return pattern.test(response.headers.get(name) ?? "");
}

function requireEtag(response: Response, spec: RequestSpec, requestId: string) {
  if (spec.etag === true && !headerMatches(response, "ETag", strongEtagPattern)) {
    throw invalidResponse(MISSING_ETAG_DETAIL, requestId);
  }
}

function requireLocation(
  response: Response,
  spec: RequestSpec,
  requestId: string,
) {
  if (
    spec.location === true &&
    !headerMatches(response, "Location", relativeLocationPattern)
  ) {
    throw invalidResponse(MISSING_LOCATION_DETAIL, requestId);
  }
}

/**
 * The header must be a literal "true" or "false". Testing only for absence
 * would read any other value as "not replayed", which is precisely the silent
 * fallback the idempotency contract exists to prevent.
 */
function requireReplay(response: Response, spec: RequestSpec, requestId: string) {
  const replayed = response.headers.get("Idempotency-Replayed");
  if (spec.replay === true && replayed !== "true" && replayed !== "false") {
    throw invalidResponse(MISSING_REPLAY_DETAIL, requestId);
  }
}

/**
 * The header is optional, but a malformed one is a contract violation: it would
 * otherwise be handed to a survey surface as if it were a usable capability.
 */
function requireSurveyCapability(response: Response, requestId: string) {
  const capability = response.headers.get("Survey-Capability");
  if (capability !== null && !surveyCapabilityPattern.test(capability)) {
    throw invalidResponse(INVALID_SURVEY_CAPABILITY_DETAIL, requestId);
  }
}

function requireMetadata(
  response: Response,
  spec: RequestSpec,
  requestId: string,
) {
  requireEtag(response, spec, requestId);
  requireLocation(response, spec, requestId);
  requireReplay(response, spec, requestId);
  requireSurveyCapability(response, requestId);
}

function requireSession(spec: RequestSpec, token: string | null) {
  const needed = spec.authenticated !== false;
  if (needed && (token === null || token.length === 0)) {
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
}

/** A conditional request carries a canonical strong ETag or it is not sent. */
function requireIfMatch(spec: RequestSpec) {
  const ifMatch = spec.headers?.["If-Match"];
  if (ifMatch !== undefined && !strongEtagPattern.test(ifMatch)) {
    throw clientError(400, "If-Match deve conter um ETag forte canônico.");
  }
}

/**
 * Folded into the request path so a malformed capability is a rejected promise
 * like every other refusal, and so the token can only ever become a header.
 */
function applyCapability(spec: RequestSpec): RequestSpec {
  const capability = spec.capability;
  if (capability === undefined) {
    return spec;
  }
  if (!capabilityPattern.test(capability.value)) {
    throw clientError(400, "Capability ausente ou fora do formato esperado.");
  }
  return {
    ...spec,
    headers: { ...spec.headers, [capability.header]: capability.value },
  };
}

function usableToken(spec: RequestSpec, token: string | null) {
  if (spec.authenticated === false || token === null) {
    return false;
  }
  return token.length > 0;
}

function requestHeaders(spec: RequestSpec, token: string | null) {
  const headers = new Headers({
    Accept: "application/json, application/problem+json",
    ...spec.headers,
  });
  if (spec.body !== undefined) {
    headers.set("Content-Type", spec.contentType ?? "application/json");
  }
  if (usableToken(spec, token)) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  return headers;
}

function buildRequest(baseUrl: string, spec: RequestSpec, token: string | null) {
  const body = spec.body === undefined ? undefined : JSON.stringify(spec.body);
  return new Request(new URL(spec.path, baseUrl), {
    method: spec.method,
    headers: requestHeaders(spec, token),
    cache: "no-store",
    credentials: "omit",
    ...(body === undefined ? {} : { body }),
  });
}

function resultFrom<T>(
  response: Response,
  data: T,
  requestId: string,
): ApiResult<T> {
  return {
    data,
    etag: response.headers.get("ETag"),
    location: response.headers.get("Location"),
    replayed: replayedFlag(response),
    requestId,
    surveyCapability: response.headers.get("Survey-Capability"),
  };
}

async function decodeBody<T>(response: Response, spec: RequestSpec) {
  if (spec.noContent === true) {
    return null as T;
  }
  return (await response.json()) as T;
}

async function requireSuccess(
  response: Response,
  spec: RequestSpec,
  requestId: string,
) {
  if (!response.ok) {
    throw new ApiError(
      response.status,
      await problemFrom(response),
      retryAfterSeconds(response),
      requestId,
    );
  }
  if (response.status !== (spec.status ?? 200)) {
    throw invalidResponse(`Status ${response.status} inesperado.`, requestId);
  }
}

export type HttpRequest = <T>(spec: RequestSpec) => Promise<ApiResult<T>>;

export function createHttpClient(options: HttpClientOptions): HttpRequest {
  const baseUrl = resolveBaseUrl(options.baseUrl);

  return async function request<T>(raw: RequestSpec): Promise<ApiResult<T>> {
    const spec = applyCapability(raw);
    const token = options.getAccessToken();
    requireSession(spec, token);
    requireIfMatch(spec);
    // Resolved per call: a caller that installs a fetch double after building
    // the client must still be intercepted.
    const send = options.fetcher ?? fetch;
    const response = await send(buildRequest(baseUrl, spec, token));
    const requestId = requireRequestId(response);
    await requireSuccess(response, spec, requestId);
    requireMetadata(response, spec, requestId);
    return resultFrom(response, await decodeBody<T>(response, spec), requestId);
  };
}

/** Serializes optional query values, dropping the ones the caller omitted. */
export function queryString(
  values: Record<string, number | string | undefined>,
) {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(values)) {
    if (value !== undefined) {
      query.set(key, String(value));
    }
  }
  const serialized = query.toString();
  return serialized.length > 0 ? `?${serialized}` : "";
}

/** Page-cursor query shared by every paginated listing. */
export function pageQuery(cursor?: string, limit?: number) {
  return queryString({ cursor, limit });
}

export function segment(value: string) {
  return encodeURIComponent(value);
}
