import type { components, operations } from "../../generated/schema";

type Schemas = components["schemas"];

export const phase2OperationNames = [
  "listAccommodations",
  "createAccommodation",
  "getAccommodation",
  "updateAccommodation",
  "listAccommodationMemberships",
  "createAccommodationMembership",
  "updateAccommodationMembership",
  "listStays",
  "createStay",
  "getStay",
  "updateStay",
  "getStayGroup",
  "submitAssistedStayGroup",
  "createStayInvite",
  "checkInStay",
  "checkOutStay",
  "cancelStay",
  "markStayNoShow",
  "getInvite",
  "submitInviteGroup",
] as const satisfies readonly (keyof operations)[];

type Phase2Operation = (typeof phase2OperationNames)[number];
type OperationStatus<Operation extends Phase2Operation> =
  (typeof successContracts)[Operation]["status"];
type OperationResponse<Operation extends Phase2Operation> =
  operations[Operation]["responses"][
    OperationStatus<Operation> & keyof operations[Operation]["responses"]
  ] extends {
    content: { "application/json": infer Data };
  }
    ? Data
    : never;
type RequestContent<Operation extends Phase2Operation> =
  operations[Operation] extends {
    requestBody: { content: infer Content };
  }
    ? Content
    : never;
type JsonRequest<Operation extends Phase2Operation> =
  RequestContent<Operation> extends { "application/json": infer Data }
    ? Data
    : never;
type MergePatchRequest<Operation extends Phase2Operation> =
  RequestContent<Operation> extends {
    "application/merge-patch+json": infer Data;
  }
    ? Data
    : never;

interface SuccessContract {
  etag?: true;
  location?: true;
  replay?: true;
  status: 200 | 201;
}

const successContracts = {
  listAccommodations: { status: 200 },
  createAccommodation: {
    status: 201,
    etag: true,
    location: true,
    replay: true,
  },
  getAccommodation: { status: 200, etag: true },
  updateAccommodation: { status: 200, etag: true },
  listAccommodationMemberships: { status: 200 },
  createAccommodationMembership: {
    status: 201,
    etag: true,
    location: true,
    replay: true,
  },
  updateAccommodationMembership: { status: 200, etag: true },
  listStays: { status: 200 },
  createStay: {
    status: 201,
    etag: true,
    location: true,
    replay: true,
  },
  getStay: { status: 200, etag: true },
  updateStay: { status: 200, etag: true },
  getStayGroup: { status: 200, etag: true },
  submitAssistedStayGroup: {
    status: 200,
    etag: true,
    replay: true,
  },
  createStayInvite: {
    status: 201,
    etag: true,
    location: true,
    replay: true,
  },
  checkInStay: { status: 200, etag: true, replay: true },
  checkOutStay: { status: 200, etag: true, replay: true },
  cancelStay: { status: 200, etag: true, replay: true },
  markStayNoShow: { status: 200, etag: true, replay: true },
  getInvite: { status: 200 },
  submitInviteGroup: {
    status: 200,
    etag: true,
    replay: true,
  },
} as const satisfies Record<Phase2Operation, SuccessContract>;

export const strongEtagPattern = /^"[1-9][0-9]*"$/;
const relativeLocationPattern = /^\/api\/v1\/\S+$/;

/**
 * Phase 7 adds the approval queue as two filters on the very same listing, so
 * there is no second listing endpoint and cursor, limit, ordering and
 * membership isolation stay exactly the ones of Phase 2.
 */
export interface StayListFilters {
  accommodationId?: string;
  approvalState?: Schemas["StayApprovalState"];
  cursor?: string;
  limit?: number;
  provenance?: Schemas["StayProvenance"];
  status?: Schemas["StayStatus"];
}

export interface ApiResult<T> {
  data: T;
  etag: string | null;
  location: string | null;
  requestId: string;
  replayed: boolean | null;
  surveyCapability: string | null;
}

export class Phase2ApiError extends Error {
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
    this.name = "Phase2ApiError";
    this.status = status;
    this.problem = problem;
    this.retryAfterSeconds = retryAfterSeconds;
    this.requestId = requestId;
  }
}

interface ClientOptions {
  baseUrl?: string;
  fetcher?: typeof fetch;
  getAccessToken: () => string | null;
}

interface RequestOptions {
  authenticated?: boolean;
  body?: unknown;
  contentType?: string;
  headers?: Record<string, string>;
  method: string;
  path: string;
}

function parseRetryAfter(value: string | null) {
  if (value === null || !/^\d+$/.test(value)) {
    return null;
  }
  return Number(value);
}

function resultHeaders<T>(
  response: Response,
  data: T,
  requestId: string,
): ApiResult<T> {
  const replayed = response.headers.get("Idempotency-Replayed");
  return {
    data,
    etag: response.headers.get("ETag"),
    location: response.headers.get("Location"),
    requestId,
    replayed: replayed === null ? null : replayed === "true",
    surveyCapability: response.headers.get("Survey-Capability"),
  };
}

function invalidResponse(detail: string, requestId: string | null = null) {
  return new Phase2ApiError(
    502,
    {
      type: "urn:cumuru:problem:invalid-api-response",
      title: "O serviço respondeu fora do contrato.",
      status: 502,
      detail,
    },
    null,
    requestId,
  );
}

function requireRequestId(response: Response) {
  const requestId = response.headers.get("X-Request-ID");
  if (requestId === null || requestId.trim().length === 0) {
    throw invalidResponse("X-Request-ID obrigatório ausente ou vazio.");
  }
  return requestId.trim();
}

function requireNoStore(response: Response, requestId: string) {
  if (response.headers.get("Cache-Control") !== "no-store") {
    throw invalidResponse(
      "Cache-Control deve ser exatamente no-store.",
      requestId,
    );
  }
}

function responseRequestId(response: Response) {
  const requestId = requireRequestId(response);
  requireNoStore(response, requestId);
  return requestId;
}

function requireEtag(
  response: Response,
  required: boolean,
  requestId: string,
) {
  if (
    required &&
    !strongEtagPattern.test(response.headers.get("ETag") ?? "")
  ) {
    throw invalidResponse(
      "ETag forte obrigatório ausente ou inválido.",
      requestId,
    );
  }
}

function requireLocation(
  response: Response,
  required: boolean,
  requestId: string,
) {
  if (
    required &&
    !relativeLocationPattern.test(response.headers.get("Location") ?? "")
  ) {
    throw invalidResponse(
      "Location relativa obrigatória ausente ou inválida.",
      requestId,
    );
  }
}

function requireReplay(
  response: Response,
  required: boolean,
  requestId: string,
) {
  const replay = response.headers.get("Idempotency-Replayed");
  if (required && replay !== "true" && replay !== "false") {
    throw invalidResponse(
      "Idempotency-Replayed obrigatório ausente ou inválido.",
      requestId,
    );
  }
}

function requireSurveyCapability(response: Response, requestId: string) {
  const capability = response.headers.get("Survey-Capability");
  if (
    capability !== null &&
    !/^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/.test(capability)
  ) {
    throw invalidResponse(
      "Survey-Capability presente, mas inválida.",
      requestId,
    );
  }
}

function requireResponseHeaders(
  response: Response,
  contract: SuccessContract,
  requestId: string,
) {
  requireEtag(response, contract.etag === true, requestId);
  requireLocation(response, contract.location === true, requestId);
  requireReplay(response, contract.replay === true, requestId);
  requireSurveyCapability(response, requestId);
}

async function problemFrom(response: Response): Promise<Schemas["Problem"]> {
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

function usableToken(authenticated: boolean | undefined, token: string | null) {
  if (authenticated === false || token === null) {
    return false;
  }
  return token.length > 0;
}

function requestHeaders(
  options: RequestOptions,
  accessToken: string | null,
) {
  const headers = new Headers({
    Accept: "application/json, application/problem+json",
    ...options.headers,
  });
  if (options.body !== undefined) {
    headers.set("Content-Type", options.contentType ?? "application/json");
  }
  if (usableToken(options.authenticated, accessToken)) {
    headers.set("Authorization", `Bearer ${accessToken}`);
  }
  return headers;
}

function missingSessionError() {
  return new Phase2ApiError(
    401,
    {
      type: "urn:cumuru:problem:oidc-provider-unavailable",
      title: "Sessão institucional indisponível.",
      status: 401,
    },
    null,
  );
}

function requireSession(
  authenticated: boolean,
  accessToken: string | null,
) {
  if (authenticated && (accessToken === null || accessToken.length === 0)) {
    throw missingSessionError();
  }
}

function requireIfMatch(options: RequestOptions) {
  const ifMatch = options.headers?.["If-Match"];
  if (ifMatch !== undefined && !strongEtagPattern.test(ifMatch)) {
    throw invalidResponse("If-Match deve conter um ETag forte canônico.");
  }
}

function requestBody(value: unknown) {
  return value === undefined ? undefined : JSON.stringify(value);
}

function buildRequest(
  baseUrl: string,
  options: RequestOptions,
  accessToken: string | null,
) {
  const body = requestBody(options.body);
  return new Request(new URL(options.path, baseUrl), {
    method: options.method,
    headers: requestHeaders(options, accessToken),
    cache: "no-store",
    credentials: "omit",
    ...(body === undefined ? {} : { body }),
  });
}

async function responseData<Operation extends Phase2Operation>(
  operation: Operation,
  response: Response,
  requestId: string,
) {
  if (!response.ok) {
    throw new Phase2ApiError(
      response.status,
      await problemFrom(response),
      parseRetryAfter(response.headers.get("Retry-After")),
      requestId,
    );
  }
  const contract: SuccessContract = successContracts[operation];
  if (response.status !== contract.status) {
    throw invalidResponse(
      `Status ${response.status} inesperado para ${operation}.`,
      requestId,
    );
  }
  requireResponseHeaders(response, contract, requestId);
  return (await response.json()) as OperationResponse<Operation>;
}

function queryString(values: Record<string, number | string | undefined>) {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(values)) {
    if (value !== undefined) {
      query.set(key, String(value));
    }
  }
  const serialized = query.toString();
  return serialized.length > 0 ? `?${serialized}` : "";
}

export function createPhase2Client(options: ClientOptions) {
  const baseUrl =
    options.baseUrl ??
    (typeof window === "undefined" ? "http://localhost" : window.location.origin);

  async function request<Operation extends Phase2Operation>(
    operation: Operation,
    requestOptions: RequestOptions,
  ): Promise<ApiResult<OperationResponse<Operation>>> {
    const accessToken = options.getAccessToken();
    requireSession(requestOptions.authenticated !== false, accessToken);
    requireIfMatch(requestOptions);
    const request = buildRequest(baseUrl, requestOptions, accessToken);
    const response = await (options.fetcher ?? fetch)(request);
    const requestId = responseRequestId(response);
    const data = await responseData(operation, response, requestId);
    return resultHeaders(response, data, requestId);
  }

  const authenticatedGet = <Operation extends Phase2Operation>(
    operation: Operation,
    path: string,
  ) => request(operation, { method: "GET", path });
  const concurrencyHeaders = (etag: string, idempotencyKey?: string) => ({
    "If-Match": etag,
    ...(idempotencyKey === undefined
      ? {}
      : { "Idempotency-Key": idempotencyKey }),
  });

  return {
    listAccommodations: (cursor?: string, limit?: number) =>
      authenticatedGet(
        "listAccommodations",
        `/api/v1/accommodations${queryString({ cursor, limit })}`,
      ),
    createAccommodation: (
      body: JsonRequest<"createAccommodation">,
      idempotencyKey: string,
    ) =>
      request("createAccommodation", {
        method: "POST",
        path: "/api/v1/accommodations",
        body,
        headers: { "Idempotency-Key": idempotencyKey },
      }),
    getAccommodation: (id: string) =>
      authenticatedGet(
        "getAccommodation",
        `/api/v1/accommodations/${encodeURIComponent(id)}`,
      ),
    updateAccommodation: (
      id: string,
      body: MergePatchRequest<"updateAccommodation">,
      etag: string,
    ) =>
      request("updateAccommodation", {
        method: "PATCH",
        path: `/api/v1/accommodations/${encodeURIComponent(id)}`,
        body,
        contentType: "application/merge-patch+json",
        headers: concurrencyHeaders(etag),
      }),
    listAccommodationMemberships: (
      accommodationId: string,
      cursor?: string,
      limit?: number,
    ) =>
      authenticatedGet(
        "listAccommodationMemberships",
        `/api/v1/accommodations/${encodeURIComponent(accommodationId)}/memberships${queryString({ cursor, limit })}`,
      ),
    createAccommodationMembership: (
      accommodationId: string,
      body: JsonRequest<"createAccommodationMembership">,
      idempotencyKey: string,
    ) =>
      request("createAccommodationMembership", {
        method: "POST",
        path: `/api/v1/accommodations/${encodeURIComponent(accommodationId)}/memberships`,
        body,
        headers: { "Idempotency-Key": idempotencyKey },
      }),
    updateAccommodationMembership: (
      accommodationId: string,
      membershipId: string,
      body: MergePatchRequest<"updateAccommodationMembership">,
      etag: string,
    ) =>
      request("updateAccommodationMembership", {
        method: "PATCH",
        path: `/api/v1/accommodations/${encodeURIComponent(accommodationId)}/memberships/${encodeURIComponent(membershipId)}`,
        body,
        contentType: "application/merge-patch+json",
        headers: concurrencyHeaders(etag),
      }),
    listStays: (filters: StayListFilters = {}) =>
      authenticatedGet(
        "listStays",
        `/api/v1/stays${queryString({
          accommodation_id: filters.accommodationId,
          status: filters.status,
          approval_state: filters.approvalState,
          provenance: filters.provenance,
          cursor: filters.cursor,
          limit: filters.limit,
        })}`,
      ),
    createStay: (
      body: JsonRequest<"createStay">,
      idempotencyKey: string,
    ) =>
      request("createStay", {
        method: "POST",
        path: "/api/v1/stays",
        body,
        headers: { "Idempotency-Key": idempotencyKey },
      }),
    getStay: (id: string) =>
      authenticatedGet(
        "getStay",
        `/api/v1/stays/${encodeURIComponent(id)}`,
      ),
    updateStay: (
      id: string,
      body: MergePatchRequest<"updateStay">,
      etag: string,
    ) =>
      request("updateStay", {
        method: "PATCH",
        path: `/api/v1/stays/${encodeURIComponent(id)}`,
        body,
        contentType: "application/merge-patch+json",
        headers: concurrencyHeaders(etag),
      }),
    getStayGroup: (stayId: string) =>
      authenticatedGet(
        "getStayGroup",
        `/api/v1/stays/${encodeURIComponent(stayId)}/group`,
      ),
    submitAssistedStayGroup: (
      stayId: string,
      body: JsonRequest<"submitAssistedStayGroup">,
      etag: string,
      idempotencyKey: string,
    ) =>
      request("submitAssistedStayGroup", {
        method: "POST",
        path: `/api/v1/stays/${encodeURIComponent(stayId)}/group`,
        body,
        headers: concurrencyHeaders(etag, idempotencyKey),
      }),
    createStayInvite: (
      stayId: string,
      body: JsonRequest<"createStayInvite">,
      etag: string,
      idempotencyKey: string,
    ) =>
      request("createStayInvite", {
        method: "POST",
        path: `/api/v1/stays/${encodeURIComponent(stayId)}/invite`,
        body,
        headers: concurrencyHeaders(etag, idempotencyKey),
      }),
    checkInStay: (
      stayId: string,
      body: JsonRequest<"checkInStay">,
      etag: string,
      idempotencyKey: string,
    ) =>
      request("checkInStay", {
        method: "POST",
        path: `/api/v1/stays/${encodeURIComponent(stayId)}/check-in`,
        body,
        headers: concurrencyHeaders(etag, idempotencyKey),
      }),
    checkOutStay: (
      stayId: string,
      body: JsonRequest<"checkOutStay">,
      etag: string,
      idempotencyKey: string,
    ) =>
      request("checkOutStay", {
        method: "POST",
        path: `/api/v1/stays/${encodeURIComponent(stayId)}/check-out`,
        body,
        headers: concurrencyHeaders(etag, idempotencyKey),
      }),
    cancelStay: (
      stayId: string,
      body: JsonRequest<"cancelStay">,
      etag: string,
      idempotencyKey: string,
    ) =>
      request("cancelStay", {
        method: "POST",
        path: `/api/v1/stays/${encodeURIComponent(stayId)}/cancel`,
        body,
        headers: concurrencyHeaders(etag, idempotencyKey),
      }),
    markStayNoShow: (
      stayId: string,
      body: JsonRequest<"markStayNoShow">,
      etag: string,
      idempotencyKey: string,
    ) =>
      request("markStayNoShow", {
        method: "POST",
        path: `/api/v1/stays/${encodeURIComponent(stayId)}/no-show`,
        body,
        headers: concurrencyHeaders(etag, idempotencyKey),
      }),
    getInvite: (capability: string) =>
      request("getInvite", {
        method: "GET",
        path: `/api/v1/invites/${encodeURIComponent(capability)}`,
        authenticated: false,
      }),
    submitInviteGroup: (
      capability: string,
      body: JsonRequest<"submitInviteGroup">,
      idempotencyKey: string,
    ) =>
      request("submitInviteGroup", {
        method: "POST",
        path: `/api/v1/invites/${encodeURIComponent(capability)}/submit`,
        authenticated: false,
        body,
        headers: { "Idempotency-Key": idempotencyKey },
      }),
  };
}

export type Phase2Client = ReturnType<typeof createPhase2Client>;
