import type { components, operations } from "../../generated/schema";
import {
  createHttpClient,
  type HttpClientOptions,
  pageQuery,
  segment,
} from "./http-client";

type Schemas = components["schemas"];

/**
 * Questionnaire authoring and the public survey submission. Authoring is a
 * review workflow — draft, review, approve, publish, retire — and every
 * transition is conditional on the version ETag.
 */
export const questionnaireOperationNames = [
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

const questionnaires = "/api/v1/questionnaires";
const versions = "/api/v1/questionnaire-versions";

export function createQuestionnaireClient(options: HttpClientOptions) {
  const request = createHttpClient(options);

  const transition = (
    id: string,
    action: string,
    etag: string,
    idempotencyKey: string,
    body: object = {},
  ) =>
    request<Schemas["QuestionnaireVersionMutationResult"]>({
      method: "POST",
      path: `${versions}/${segment(id)}/${action}`,
      body,
      etag: true,
      replay: true,
      headers: { "If-Match": etag, "Idempotency-Key": idempotencyKey },
    });

  const create = <T,>(path: string, body: unknown, idempotencyKey: string) =>
    request<T>({
      method: "POST",
      path,
      body,
      etag: true,
      replay: true,
      status: 201,
      headers: { "Idempotency-Key": idempotencyKey },
    });

  return {
    listQuestionnaires: (cursor?: string, limit?: number) =>
      request<Schemas["QuestionnairePage"]>({
        method: "GET",
        path: `${questionnaires}${pageQuery(cursor, limit)}`,
      }),
    createQuestionnaire: (
      body: Schemas["CreateQuestionnaireRequest"],
      idempotencyKey: string,
    ) =>
      create<Schemas["QuestionnaireVersionMutationResult"]>(
        questionnaires,
        body,
        idempotencyKey,
      ),
    getQuestionnaire: (id: string) =>
      request<Schemas["Questionnaire"]>({
        method: "GET",
        path: `${questionnaires}/${segment(id)}`,
      }),
    listQuestionnaireVersions: (
      questionnaireId: string,
      cursor?: string,
      limit?: number,
    ) =>
      request<Schemas["QuestionnaireVersionPage"]>({
        method: "GET",
        path: `${questionnaires}/${segment(questionnaireId)}/versions${pageQuery(cursor, limit)}`,
      }),
    cloneQuestionnaireVersion: (
      questionnaireId: string,
      body: Schemas["CloneQuestionnaireVersionRequest"],
      idempotencyKey: string,
    ) =>
      create<Schemas["QuestionnaireVersionMutationResult"]>(
        `${questionnaires}/${segment(questionnaireId)}/versions`,
        body,
        idempotencyKey,
      ),
    getQuestionnaireVersion: (id: string) =>
      request<Schemas["QuestionnaireVersionAdmin"]>({
        method: "GET",
        path: `${versions}/${segment(id)}`,
        etag: true,
      }),
    updateQuestionnaireVersion: (
      id: string,
      body: Schemas["UpdateQuestionnaireVersionRequest"],
      etag: string,
    ) =>
      request<Schemas["QuestionnaireVersionAdmin"]>({
        method: "PUT",
        path: `${versions}/${segment(id)}`,
        body,
        etag: true,
        headers: { "If-Match": etag },
      }),
    submitReview: (id: string, etag: string, key: string) =>
      transition(id, "submit-review", etag, key),
    requestChanges: (
      id: string,
      body: Schemas["RequestChangesRequest"],
      etag: string,
      key: string,
    ) => transition(id, "request-changes", etag, key, body),
    approve: (id: string, etag: string, key: string) =>
      transition(id, "approve", etag, key),
    publish: (id: string, etag: string, key: string) =>
      transition(id, "publish", etag, key),
    retire: (id: string, etag: string, key: string) =>
      transition(id, "retire", etag, key),
    getActiveQuestionnaire: (stableKey = "tourism_profile") =>
      request<Schemas["PublishedQuestionnaire"]>({
        method: "GET",
        path: `${questionnaires}/${segment(stableKey)}/active`,
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

export type QuestionnaireClient = ReturnType<typeof createQuestionnaireClient>;
