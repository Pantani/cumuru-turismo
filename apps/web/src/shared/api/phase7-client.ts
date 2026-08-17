import type { components, operations } from "../../generated/schema";

type Schemas = components["schemas"];

/**
 * Phase 7 client. Two rules shape it and both come from ADR-039 and ADR-041:
 * the capability travels only in a custom header — never in the path, never in
 * a query string, never in a fragment sent to the server — and each purpose has
 * its own header, because separate purposes are separate cryptographic domains.
 */
export const INVITE_TOKEN_HEADER = "X-Cumuru-Invite-Token";
export const ACTIVATION_TOKEN_HEADER = "X-Cumuru-Activation-Token";

export const phase7OperationNames = [
  "createAccommodationActivation",
  "getAccommodationActivation",
  "activateAccommodationAccount",
  "createAccommodationInvite",
  "getAccommodationInvite",
  "revokeAccommodationInvite",
  "getAccommodationInviteContext",
  "submitAccommodationSelfRegistration",
  "approveStay",
  "rejectStay",
] as const satisfies readonly (keyof operations)[];

const strongEtagPattern = /^"[1-9][0-9]*"$/u;
const relativeLocationPattern = /^\/api\/v1\/\S+$/u;
const capabilityPattern = /^[A-Za-z0-9_-]{64,128}$/u;
const surveyCapabilityPattern = /^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/u;

export interface Phase7Result<T> {
  data: T;
  etag: string | null;
  location: string | null;
  replayed: boolean | null;
  requestId: string;
  surveyCapability: string | null;
}

export class Phase7ApiError extends Error {
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
    this.name = "Phase7ApiError";
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

interface CapabilityHeader {
  header: string;
  value: string;
}

interface RequestSpec {
  authenticated?: boolean;
  body?: unknown;
  capability?: CapabilityHeader;
  etag?: boolean;
  headers?: Record<string, string>;
  location?: boolean;
  method: string;
  noContent?: boolean;
  path: string;
  replay?: boolean;
  status?: number;
}

function clientError(status: number, title: string, detail?: string) {
  return new Phase7ApiError(
    status,
    {
      type: "urn:cumuru:problem:invalid-api-response",
      title,
      status,
      ...(detail === undefined ? {} : { detail }),
    },
    null,
  );
}

function invalidResponse(detail: string, requestId: string | null = null) {
  return new Phase7ApiError(
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

function requireCapability(capability: string) {
  if (!capabilityPattern.test(capability)) {
    throw clientError(400, "Capability ausente ou fora do formato esperado.");
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
  requireCapability(capability.value);
  return {
    ...spec,
    headers: { ...spec.headers, [capability.header]: capability.value },
  };
}

function retryAfterFrom(response: Response) {
  const value = response.headers.get("Retry-After");
  return value !== null && /^\d+$/u.test(value) ? Number(value) : null;
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

function requestIdFrom(response: Response) {
  const requestId = response.headers.get("X-Request-ID");
  if (requestId === null || requestId.trim().length === 0) {
    throw invalidResponse("X-Request-ID obrigatório ausente ou vazio.");
  }
  if (response.headers.get("Cache-Control") !== "no-store") {
    throw invalidResponse(
      "Cache-Control deve ser exatamente no-store.",
      requestId.trim(),
    );
  }
  return requestId.trim();
}

function requireEtag(response: Response, spec: RequestSpec, requestId: string) {
  if (
    spec.etag === true &&
    !strongEtagPattern.test(response.headers.get("ETag") ?? "")
  ) {
    throw invalidResponse("ETag forte obrigatório ausente ou inválido.", requestId);
  }
}

function requireLocation(
  response: Response,
  spec: RequestSpec,
  requestId: string,
) {
  if (
    spec.location === true &&
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
  spec: RequestSpec,
  requestId: string,
) {
  const replay = response.headers.get("Idempotency-Replayed");
  if (spec.replay === true && replay !== "true" && replay !== "false") {
    throw invalidResponse(
      "Idempotency-Replayed obrigatório ausente ou inválido.",
      requestId,
    );
  }
}

function requireSurveyCapability(response: Response, requestId: string) {
  const capability = response.headers.get("Survey-Capability");
  if (capability !== null && !surveyCapabilityPattern.test(capability)) {
    throw invalidResponse("Survey-Capability presente, mas inválida.", requestId);
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
    throw clientError(401, "Sessão da hospedagem indisponível.");
  }
}

function requireIfMatch(spec: RequestSpec) {
  const ifMatch = spec.headers?.["If-Match"];
  if (ifMatch !== undefined && !strongEtagPattern.test(ifMatch)) {
    throw clientError(400, "If-Match deve conter um ETag forte canônico.");
  }
}

function requestHeaders(spec: RequestSpec, token: string | null) {
  const headers = new Headers({
    Accept: "application/json, application/problem+json",
    ...spec.headers,
  });
  if (spec.body !== undefined) {
    headers.set("Content-Type", "application/json");
  }
  if (spec.authenticated !== false && token !== null && token.length > 0) {
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
): Phase7Result<T> {
  const replayed = response.headers.get("Idempotency-Replayed");
  return {
    data,
    etag: response.headers.get("ETag"),
    location: response.headers.get("Location"),
    replayed: replayed === null ? null : replayed === "true",
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

export function createPhase7Client(options: ClientOptions) {
  const baseUrl =
    options.baseUrl ??
    (typeof window === "undefined" ? "http://localhost" : window.location.origin);

  async function request<T>(raw: RequestSpec): Promise<Phase7Result<T>> {
    const spec = applyCapability(raw);
    const token = options.getAccessToken();
    requireSession(spec, token);
    requireIfMatch(spec);
    const response = await (options.fetcher ?? fetch)(
      buildRequest(baseUrl, spec, token),
    );
    const requestId = requestIdFrom(response);
    if (!response.ok) {
      throw new Phase7ApiError(
        response.status,
        await problemFrom(response),
        retryAfterFrom(response),
        requestId,
      );
    }
    if (response.status !== (spec.status ?? 200)) {
      throw invalidResponse(`Status ${response.status} inesperado.`, requestId);
    }
    requireMetadata(response, spec, requestId);
    return resultFrom(response, await decodeBody<T>(response, spec), requestId);
  }

  const accommodationPath = (accommodationId: string) =>
    `/api/v1/accommodations/${encodeURIComponent(accommodationId)}`;
  const stayCommand = (
    stayId: string,
    command: string,
    etag: string,
    idempotencyKey: string,
    body: object,
  ) =>
    request<Schemas["StayMutationResult"]>({
      method: "POST",
      path: `/api/v1/stays/${encodeURIComponent(stayId)}/${command}`,
      body,
      etag: true,
      replay: true,
      headers: { "If-Match": etag, "Idempotency-Key": idempotencyKey },
    });
  const guest = <T,>(spec: RequestSpec) =>
    request<T>({ ...spec, authenticated: false });

  return {
    createAccommodationActivation: (
      accommodationId: string,
      body: Schemas["CreateAccommodationActivationRequest"],
      etag: string,
      idempotencyKey: string,
    ) =>
      request<Schemas["AccommodationActivationCreated"]>({
        method: "POST",
        path: `${accommodationPath(accommodationId)}/activation`,
        body,
        etag: true,
        location: true,
        replay: true,
        status: 201,
        headers: { "If-Match": etag, "Idempotency-Key": idempotencyKey },
      }),
    getAccommodationActivation: (capability: string) =>
      guest<Schemas["AccommodationActivationContext"]>({
        method: "GET",
        path: "/api/v1/activation",
        capability: { header: ACTIVATION_TOKEN_HEADER, value: capability },
      }),
    activateAccommodationAccount: (
      capability: string,
      body: Schemas["ActivateAccountRequest"],
    ) =>
      guest<null>({
        method: "POST",
        path: "/api/v1/activation/complete",
        body,
        noContent: true,
        status: 204,
        capability: { header: ACTIVATION_TOKEN_HEADER, value: capability },
      }),
    getAccommodationInvite: (accommodationId: string) =>
      request<Schemas["AccommodationInviteStatus"]>({
        method: "GET",
        path: `${accommodationPath(accommodationId)}/invite`,
      }),
    createAccommodationInvite: (
      accommodationId: string,
      body: Schemas["CreateAccommodationInviteRequest"],
      etag: string,
      idempotencyKey: string,
    ) =>
      request<Schemas["AccommodationInviteCreated"]>({
        method: "POST",
        path: `${accommodationPath(accommodationId)}/invite`,
        body,
        etag: true,
        location: true,
        replay: true,
        status: 201,
        headers: { "If-Match": etag, "Idempotency-Key": idempotencyKey },
      }),
    revokeAccommodationInvite: (
      accommodationId: string,
      etag: string,
      idempotencyKey: string,
    ) =>
      request<Schemas["AccommodationInviteStatus"]>({
        method: "POST",
        path: `${accommodationPath(accommodationId)}/invite/revoke`,
        body: {},
        etag: true,
        replay: true,
        headers: { "If-Match": etag, "Idempotency-Key": idempotencyKey },
      }),
    getAccommodationInviteContext: (capability: string) =>
      guest<Schemas["AccommodationInviteContext"]>({
        method: "GET",
        path: "/api/v1/accommodation-invite",
        capability: { header: INVITE_TOKEN_HEADER, value: capability },
      }),
    submitAccommodationSelfRegistration: (
      capability: string,
      body: Schemas["SelfRegistrationRequest"],
      idempotencyKey: string,
    ) =>
      guest<Schemas["SelfRegistrationAccepted"]>({
        method: "POST",
        path: "/api/v1/accommodation-invite/submit",
        body,
        etag: true,
        replay: true,
        capability: { header: INVITE_TOKEN_HEADER, value: capability },
        headers: { "Idempotency-Key": idempotencyKey },
      }),
    approveStay: (stayId: string, etag: string, idempotencyKey: string) =>
      stayCommand(stayId, "approve", etag, idempotencyKey, {}),
    rejectStay: (
      stayId: string,
      body: Schemas["RejectStayRequest"],
      etag: string,
      idempotencyKey: string,
    ) => stayCommand(stayId, "reject", etag, idempotencyKey, body),
  };
}

export type Phase7Client = ReturnType<typeof createPhase7Client>;
