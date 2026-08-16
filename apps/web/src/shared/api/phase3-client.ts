import type { components, operations } from "../../generated/schema";

type Schemas = components["schemas"];

export const phase3OperationNames = [
  "listQuestionnaires",
  "createQuestionnaire",
  "getQuestionnaire",
  "listQuestionnaireVersions",
  "cloneQuestionnaireVersion",
  "getQuestionnaireVersion",
  "updateQuestionnaireVersion",
  "submitQuestionnaireVersionReview",
  "requestQuestionnaireVersionChanges",
  "approveQuestionnaireVersion",
  "publishQuestionnaireVersion",
  "retireQuestionnaireVersion",
  "getActiveQuestionnaire",
  "submitSurveyResponse",
] as const satisfies readonly (keyof operations)[];

export interface Phase3Result<T> {
  data: T;
  etag: string | null;
  replayed: boolean | null;
  requestId: string;
}

export class Phase3ApiError extends Error {
  readonly problem: Schemas["Problem"];
  readonly requestId: string | null;
  readonly retryAfterSeconds: number | null;
  readonly status: number;

  constructor(
    status: number,
    problem: Schemas["Problem"],
    retryAfterSeconds: number | null,
    requestId: string | null,
  ) {
    super(problem.title);
    this.name = "Phase3ApiError";
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

interface RequestSpec {
  authenticated?: boolean;
  body?: unknown;
  etag?: boolean;
  headers?: Record<string, string>;
  method: string;
  path: string;
  replay?: boolean;
  status?: 200 | 201;
}

function invalidResponse(detail: string, requestId: string | null = null) {
  return new Phase3ApiError(
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

function requestIdFrom(response: Response) {
  const requestId = response.headers.get("X-Request-ID")?.trim() ?? "";
  if (requestId.length === 0) {
    throw invalidResponse("X-Request-ID obrigatório ausente ou vazio.");
  }
  if (response.headers.get("Cache-Control") !== "no-store") {
    throw invalidResponse("Cache-Control deve ser exatamente no-store.", requestId);
  }
  return requestId;
}

function retryAfterFrom(response: Response) {
  const value = response.headers.get("Retry-After");
  return value !== null && /^\d+$/.test(value) ? Number(value) : null;
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

function requireStrongEtag(response: Response, requestId: string) {
  if (!/^"[1-9][0-9]*"$/.test(response.headers.get("ETag") ?? "")) {
    throw invalidResponse("ETag forte obrigatório ausente ou inválido.", requestId);
  }
}

function requireReplayFlag(response: Response, requestId: string) {
  const replayed = response.headers.get("Idempotency-Replayed");
  if (replayed !== "true" && replayed !== "false") {
    throw invalidResponse(
      "Idempotency-Replayed obrigatório ausente ou inválido.",
      requestId,
    );
  }
}

function requireMetadata(
  response: Response,
  spec: RequestSpec,
  requestId: string,
) {
  if (spec.etag === true) {
    requireStrongEtag(response, requestId);
  }
  if (spec.replay === true) {
    requireReplayFlag(response, requestId);
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
  if (spec.authenticated !== false && token !== null) {
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

function requireSession(spec: RequestSpec, token: string | null) {
  if (spec.authenticated !== false && (token === null || token.length === 0)) {
    throw new Phase3ApiError(
      401,
      {
        type: "urn:cumuru:problem:oidc-provider-unavailable",
        title: "Sessão institucional indisponível.",
        status: 401,
      },
      null,
      null,
    );
  }
}

function resultFrom<T>(
  response: Response,
  data: T,
  requestId: string,
): Phase3Result<T> {
  const replayed = response.headers.get("Idempotency-Replayed");
  return {
    data,
    etag: response.headers.get("ETag"),
    replayed: replayed === null ? null : replayed === "true",
    requestId,
  };
}

function pageQuery(cursor?: string, limit?: number) {
  const query = new URLSearchParams();
  if (cursor !== undefined) {
    query.set("cursor", cursor);
  }
  if (limit !== undefined) {
    query.set("limit", String(limit));
  }
  const value = query.toString();
  return value.length === 0 ? "" : `?${value}`;
}

export function createPhase3Client(options: ClientOptions) {
  const baseUrl =
    options.baseUrl ??
    (typeof window === "undefined" ? "http://localhost" : window.location.origin);

  async function request<T>(spec: RequestSpec): Promise<Phase3Result<T>> {
    const token = options.getAccessToken();
    requireSession(spec, token);
    const response = await (options.fetcher ?? fetch)(
      buildRequest(baseUrl, spec, token),
    );
    const requestId = requestIdFrom(response);
    if (!response.ok) {
      throw new Phase3ApiError(
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
    return resultFrom(response, (await response.json()) as T, requestId);
  }

  const transition = (
    path: string,
    etag: string,
    idempotencyKey: string,
    body: object = {},
  ) =>
    request<Schemas["QuestionnaireVersionMutationResult"]>({
      method: "POST",
      path,
      body,
      etag: true,
      replay: true,
      headers: { "If-Match": etag, "Idempotency-Key": idempotencyKey },
    });

  return {
    listQuestionnaires: (cursor?: string, limit?: number) =>
      request<Schemas["QuestionnairePage"]>({
        method: "GET",
        path: `/api/v1/questionnaires${pageQuery(cursor, limit)}`,
      }),
    createQuestionnaire: (
      body: Schemas["CreateQuestionnaireRequest"],
      idempotencyKey: string,
    ) =>
      request<Schemas["QuestionnaireVersionMutationResult"]>({
        method: "POST",
        path: "/api/v1/questionnaires",
        body,
        etag: true,
        replay: true,
        status: 201,
        headers: { "Idempotency-Key": idempotencyKey },
      }),
    getQuestionnaire: (id: string) =>
      request<Schemas["Questionnaire"]>({
        method: "GET",
        path: `/api/v1/questionnaires/${encodeURIComponent(id)}`,
      }),
    listQuestionnaireVersions: (
      questionnaireId: string,
      cursor?: string,
      limit?: number,
    ) =>
      request<Schemas["QuestionnaireVersionPage"]>({
        method: "GET",
        path: `/api/v1/questionnaires/${encodeURIComponent(questionnaireId)}/versions${pageQuery(cursor, limit)}`,
      }),
    cloneQuestionnaireVersion: (
      questionnaireId: string,
      body: Schemas["CloneQuestionnaireVersionRequest"],
      idempotencyKey: string,
    ) =>
      request<Schemas["QuestionnaireVersionMutationResult"]>({
        method: "POST",
        path: `/api/v1/questionnaires/${encodeURIComponent(questionnaireId)}/versions`,
        body,
        etag: true,
        replay: true,
        status: 201,
        headers: { "Idempotency-Key": idempotencyKey },
      }),
    getQuestionnaireVersion: (id: string) =>
      request<Schemas["QuestionnaireVersionAdmin"]>({
        method: "GET",
        path: `/api/v1/questionnaire-versions/${encodeURIComponent(id)}`,
        etag: true,
      }),
    updateQuestionnaireVersion: (
      id: string,
      body: Schemas["UpdateQuestionnaireVersionRequest"],
      etag: string,
    ) =>
      request<Schemas["QuestionnaireVersionAdmin"]>({
        method: "PUT",
        path: `/api/v1/questionnaire-versions/${encodeURIComponent(id)}`,
        body,
        etag: true,
        headers: { "If-Match": etag },
      }),
    submitReview: (id: string, etag: string, key: string) =>
      transition(
        `/api/v1/questionnaire-versions/${encodeURIComponent(id)}/submit-review`,
        etag,
        key,
      ),
    requestChanges: (
      id: string,
      body: Schemas["RequestChangesRequest"],
      etag: string,
      key: string,
    ) =>
      transition(
        `/api/v1/questionnaire-versions/${encodeURIComponent(id)}/request-changes`,
        etag,
        key,
        body,
      ),
    approve: (id: string, etag: string, key: string) =>
      transition(
        `/api/v1/questionnaire-versions/${encodeURIComponent(id)}/approve`,
        etag,
        key,
      ),
    publish: (id: string, etag: string, key: string) =>
      transition(
        `/api/v1/questionnaire-versions/${encodeURIComponent(id)}/publish`,
        etag,
        key,
      ),
    retire: (id: string, etag: string, key: string) =>
      transition(
        `/api/v1/questionnaire-versions/${encodeURIComponent(id)}/retire`,
        etag,
        key,
      ),
    getActiveQuestionnaire: (stableKey = "tourism_profile") =>
      request<Schemas["PublishedQuestionnaire"]>({
        method: "GET",
        path: `/api/v1/questionnaires/${encodeURIComponent(stableKey)}/active`,
        authenticated: false,
        etag: true,
      }),
    submitSurveyResponse: (
      body: Schemas["SurveySubmissionRequest"],
      capability: string,
      idempotencyKey: string,
    ) =>
      request<Schemas["SurveySubmissionAccepted"]>({
        method: "POST",
        path: "/api/v1/survey-responses",
        authenticated: false,
        body,
        replay: true,
        headers: {
          "Idempotency-Key": idempotencyKey,
          "Survey-Capability": capability,
        },
      }),
  };
}

export type Phase3Client = ReturnType<typeof createPhase3Client>;
