import type { components } from "../../generated/schema";

type Schemas = components["schemas"];

/**
 * Checks every domain client applies to a response before trusting it.
 *
 * Each client used to carry its own copy of these, and the copies had already
 * drifted: one trimmed the request id before testing it and another did not.
 * A single definition means a tightened rule reaches every surface at once.
 */

const INVALID_RESPONSE_TYPE = "urn:cumuru:problem:invalid-api-response";

/** A strong ETag over a monotonic version, used by every tenant-scoped body. */
export const strongEtagPattern = /^"[1-9][0-9]*"$/u;

/** A strong ETag over a content digest, used by the public analytics bodies. */
export const publicEtagPattern = /^"sha256-[0-9a-f]{64}"$/u;

/** A created resource is always announced by a relative API path. */
export const relativeLocationPattern = /^\/api\/v1\/\S+$/u;

/** The survey capability is a two-part opaque token, never an identifier. */
export const surveyCapabilityPattern = /^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/u;

/** Shared-cache policy the public analytics surfaces must answer with. */
export const PUBLIC_CACHE_CONTROL = "public, max-age=300, stale-if-error=86400";

/** Body of the 502 a client raises when the service answers off-contract. */
export function invalidResponseProblem(detail: string): Schemas["Problem"] {
  return {
    type: INVALID_RESPONSE_TYPE,
    title: "O serviço respondeu fora do contrato.",
    status: 502,
    detail,
  };
}

/**
 * Reads the problem body of a failed response. A body that is absent or not
 * JSON still has to produce a usable problem, because the status alone is what
 * the caller will render.
 */
export async function problemFrom(
  response: Response,
): Promise<Schemas["Problem"]> {
  try {
    return (await response.json()) as Schemas["Problem"];
  } catch {
    return {
      type: "about:blank",
      title: "Não foi possível concluir a solicitação.",
      status: response.status,
    };
  }
}

/** The correlation id, or null when the header is missing or blank. */
export function responseRequestId(response: Response) {
  const requestId = response.headers.get("X-Request-ID")?.trim() ?? "";
  return requestId.length === 0 ? null : requestId;
}

/**
 * Every tenant-scoped response must be exactly no-store. Anything weaker would
 * let a shared cache retain a tenant-scoped body.
 */
export function isNoStore(response: Response) {
  return response.headers.get("Cache-Control") === "no-store";
}

/** Retry-After in whole seconds, or null when absent or not a count. */
export function retryAfterSeconds(response: Response) {
  const value = response.headers.get("Retry-After");
  return value !== null && /^\d+$/u.test(value) ? Number(value) : null;
}

/** Reads the replay flag as a tri-state: absent, replayed, or fresh. */
export function replayedFlag(response: Response) {
  const replayed = response.headers.get("Idempotency-Replayed");
  return replayed === null ? null : replayed === "true";
}

export const MISSING_REQUEST_ID_DETAIL =
  "X-Request-ID obrigatório ausente ou vazio.";

export const NON_NO_STORE_DETAIL = "Cache-Control deve ser exatamente no-store.";

export const MISSING_ETAG_DETAIL =
  "ETag forte obrigatório ausente ou inválido.";

export const MISSING_LOCATION_DETAIL =
  "Location relativa obrigatória ausente ou inválida.";

export const MISSING_REPLAY_DETAIL =
  "Idempotency-Replayed obrigatório ausente ou inválido.";

export const INVALID_SURVEY_CAPABILITY_DETAIL =
  "Survey-Capability presente, mas inválida.";

export const NON_PUBLIC_CACHE_DETAIL =
  "Cache-Control público obrigatório ausente ou inválido.";

export const MISSING_PUBLIC_ETAG_DETAIL =
  "ETag público forte ausente ou inválido.";
