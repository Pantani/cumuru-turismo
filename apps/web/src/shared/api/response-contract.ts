import type { components } from "../../generated/schema";

type Schemas = components["schemas"];

/**
 * Checks every phase client applies to a response before trusting it.
 *
 * Each client used to carry its own copy of these, and the copies had already
 * drifted: one trimmed the request id before testing it and another did not.
 * A single definition means a tightened rule reaches every surface at once.
 */

const INVALID_RESPONSE_TYPE = "urn:cumuru:problem:invalid-api-response";

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
 * Every API response must be exactly no-store. Anything weaker would let a
 * shared cache retain a tenant-scoped body.
 */
export function isNoStore(response: Response) {
  return response.headers.get("Cache-Control") === "no-store";
}

export const MISSING_REQUEST_ID_DETAIL =
  "X-Request-ID obrigatório ausente ou vazio.";

export const NON_NO_STORE_DETAIL = "Cache-Control deve ser exatamente no-store.";
