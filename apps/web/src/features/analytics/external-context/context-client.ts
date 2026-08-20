/**
 * Cliente da camada de contexto externo.
 *
 * Cliente próprio, e não um método a mais no cliente de analytics, porque a
 * ADR-045 §2 separa superfície e identidade de cache: payload compartilhado
 * faria a atualização externa invalidar a ETag do snapshot protegido, e a
 * cadência de invalidação viraria oráculo do horário de publicação da release.
 * A separação de módulo é o que impede que uma resposta desta rota entre no
 * mesmo `queryKey`, no mesmo validador e no mesmo tratamento de erro da série
 * medida.
 *
 * Nenhuma chamada parte deste módulo para fora da origem: o host é `self`, e a
 * CSP (`connect-src 'self'`) recusaria qualquer outro.
 */

import type { components } from "../../../generated/schema";
import {
  ApiError,
  type HttpClientOptions,
  invalidResponse,
  resolveBaseUrl,
} from "../../../shared/api/http-client";
import {
  MISSING_PUBLIC_ETAG_DETAIL,
  MISSING_REQUEST_ID_DETAIL,
  NON_PUBLIC_CACHE_DETAIL,
  problemFrom,
  PUBLIC_CACHE_CONTROL,
  publicEtagPattern,
  responseRequestId,
  retryAfterSeconds,
} from "../../../shared/api/response-contract";
import { isPublicContext } from "./context-payload";

type Schemas = components["schemas"];

export const PUBLIC_CONTEXT_PATH = "/api/v1/public/context";

/** Documento único, sem seletor: a rota não aceita parâmetro de janela. */
export interface ContextResult {
  data: Schemas["PublicContext"];
  etag: string | null;
  requestId: string;
}

function requestIdOf(response: Response) {
  const requestId = responseRequestId(response);
  if (requestId === null) {
    throw invalidResponse(MISSING_REQUEST_ID_DETAIL);
  }
  return requestId;
}

async function requireSuccess(response: Response, requestId: string) {
  if (response.ok && response.status === 200) {
    return;
  }
  if (!response.ok) {
    throw new ApiError(
      response.status,
      await problemFrom(response),
      retryAfterSeconds(response),
      requestId,
    );
  }
  throw invalidResponse(`Status ${response.status} inesperado.`, requestId);
}

function requirePublicHeaders(response: Response, requestId: string) {
  if (response.headers.get("Cache-Control") !== PUBLIC_CACHE_CONTROL) {
    throw invalidResponse(NON_PUBLIC_CACHE_DETAIL, requestId);
  }
  if (!publicEtagPattern.test(response.headers.get("ETag") ?? "")) {
    throw invalidResponse(MISSING_PUBLIC_ETAG_DETAIL, requestId);
  }
}

function contextRequest(baseUrl: string) {
  return new Request(new URL(PUBLIC_CONTEXT_PATH, baseUrl), {
    method: "GET",
    headers: new Headers({
      Accept: "application/json, application/problem+json",
    }),
    cache: "default",
    credentials: "omit",
  });
}

export function createContextClient(options: HttpClientOptions) {
  const baseUrl = resolveBaseUrl(options.baseUrl);

  async function getContext(): Promise<ContextResult> {
    // Resolvido por chamada: o módulo constrói o cliente no import, muito
    // antes de um teste poder instalar um duplo de `fetch`.
    const send = options.fetcher ?? fetch;
    const response = await send(contextRequest(baseUrl));
    const requestId = requestIdOf(response);
    await requireSuccess(response, requestId);
    requirePublicHeaders(response, requestId);
    const value = (await response.json()) as unknown;
    if (!isPublicContext(value)) {
      throw invalidResponse(
        "O documento de contexto externo contém shape ou propriedade fora da allowlist.",
        requestId,
      );
    }
    return {
      data: value as Schemas["PublicContext"],
      etag: response.headers.get("ETag"),
      requestId,
    };
  }

  return { getContext };
}

export type ContextClient = ReturnType<typeof createContextClient>;

export const publicContextClient = createContextClient({
  getAccessToken: () => null,
});
