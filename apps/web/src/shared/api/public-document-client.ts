/**
 * Transporte comum dos documentos servidos por GET e validados por allowlist:
 * os painéis públicos de analytics e a lista pública de hospedagens.
 *
 * Está separado dos dois porque as regras que ele guarda não são de nenhuma
 * feature — correlação obrigatória, disciplina de cache conforme o documento
 * seja público ou autenticado, e recusa de corpo fora da allowlist. Duas cópias
 * dessas regras divergiriam, e a que divergisse em silêncio seria justamente a
 * do documento aberto.
 */

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

export interface DocumentResult<T> {
  data: T;
  etag: string | null;
  requestId: string;
}

interface DocumentRequest {
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
function documentRequestId(response: Response) {
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


/**
 * Um leitor por cliente: a base é resolvida uma vez, e o fetch é resolvido por
 * chamada porque o módulo constrói o cliente na importação, antes de um teste
 * poder instalar o dobro.
 */
export function createDocumentReader(options: HttpClientOptions) {
  const baseUrl = resolveBaseUrl(options.baseUrl);

  async function request<T>(spec: DocumentRequest): Promise<DocumentResult<T>> {
    const token = sessionToken(spec.authenticated, options.getAccessToken);
    const send = options.fetcher ?? fetch;
    const response = await send(
      buildRequest(baseUrl, spec.path, token, spec.authenticated),
    );
    const requestId = documentRequestId(response);
    await requireSuccess(response, requestId);
    requireCacheDiscipline(response, spec.authenticated, requestId);
    return {
      data: await payloadFrom<T>(response, requestId, spec.isValid),
      etag: response.headers.get("ETag"),
      requestId,
    };
  }

  return {
    published: <T,>(path: string, isValid: (value: unknown) => boolean) =>
      request<T>({ authenticated: false, isValid, path }),
    authenticated: <T,>(path: string, isValid: (value: unknown) => boolean) =>
      request<T>({ authenticated: true, isValid, path }),
  };
}
