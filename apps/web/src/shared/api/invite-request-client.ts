import type { components, operations } from "../../generated/schema";
import {
  createHttpClient,
  queryString,
  segment,
  type HttpClientOptions,
} from "./http-client";

type Schemas = components["schemas"];

/**
 * Canal público pelo qual uma hospedagem pede acesso ao Observatório.
 *
 * O pedido é anônimo em transporte: não há sessão, não há capability e o token
 * de idempotência é o próprio `client_submission_id` do corpo, para que reenviar
 * o formulário numa rede instável não crie duas linhas na fila de aprovação.
 *
 * Nominal em conteúdo, ao contrário do autocadastro do hóspede: sem nome e
 * e-mail não há a quem devolver o acesso depois da aprovação. O 201 devolve só
 * `id` e `created_at` — a rota é aberta, e ecoar o que foi enviado transformaria
 * a criação em consulta de dado de contato alheio.
 */

export type AccessRequestContext = Schemas["AccommodationAccessRequestContext"];
export type AccessRequestBody = Schemas["CreateAccommodationAccessRequest"];
export type AccessRequestCreated =
  Schemas["AccommodationAccessRequestCreated"];
export type AccessRequestCategory = Schemas["AccommodationInputCategory"];

/**
 * Lado administrativo do mesmo recurso. Fica neste cliente, e não em outro,
 * porque fila e formulário falam do mesmo `accommodation-access-requests`: um
 * segundo cliente duplicaria o caminho da rota e deixaria as duas metades
 * livres para divergir do contrato uma sem a outra.
 */
export type AccessRequest = Schemas["AccommodationAccessRequest"];
export type AccessRequestPage = Schemas["AccommodationAccessRequestPage"];
export type AccessRequestState =
  Schemas["AccommodationAccessRequestApprovalState"];
export type AccessRequestRejectionReason =
  Schemas["AccommodationAccessRequestRejectionReason"];
export type RejectAccessRequestBody =
  Schemas["RejectAccommodationAccessRequest"];

export interface AccessRequestQuery {
  approvalState?: AccessRequestState;
  cursor?: string;
  limit?: number;
}

/** Mesmo vocabulário de `AccommodationInputCategory` no contrato. */
export const accessRequestCategories = [
  "formal_lodging",
  "seasonal_rental",
  "family_hosting",
  "camping",
  "regularizing",
  "other",
] as const satisfies readonly AccessRequestCategory[];

export const inviteRequestOperationNames = [
  "getAccommodationAccessRequestContext",
  "createAccommodationAccessRequest",
  "listAccommodationAccessRequests",
  "approveAccommodationAccessRequest",
  "rejectAccommodationAccessRequest",
] as const satisfies readonly (keyof operations)[];

const ACCESS_REQUESTS_PATH = "/api/v1/accommodation-access-requests";

export function createInviteRequestClient(options: HttpClientOptions) {
  const request = createHttpClient(options);

  const decision = (
    requestId: string,
    action: string,
    body: object,
    etag: string,
    idempotencyKey: string,
  ) =>
    request<AccessRequest>({
      method: "POST",
      path: `${ACCESS_REQUESTS_PATH}/${segment(requestId)}/${action}`,
      body,
      etag: true,
      replay: true,
      headers: { "If-Match": etag, "Idempotency-Key": idempotencyKey },
    });

  return {
    getAccessRequestContext: () =>
      request<AccessRequestContext>({
        authenticated: false,
        method: "GET",
        path: `${ACCESS_REQUESTS_PATH}/context`,
      }),
    createAccessRequest: (body: AccessRequestBody, idempotencyKey: string) =>
      request<AccessRequestCreated>({
        authenticated: false,
        body,
        headers: { "Idempotency-Key": idempotencyKey },
        method: "POST",
        path: ACCESS_REQUESTS_PATH,
        replay: true,
        status: 201,
      }),
    listAccessRequests: (query: AccessRequestQuery = {}) =>
      request<AccessRequestPage>({
        method: "GET",
        path: `${ACCESS_REQUESTS_PATH}${queryString({
          approval_state: query.approvalState,
          cursor: query.cursor,
          limit: query.limit,
        })}`,
      }),
    approveAccessRequest: (
      requestId: string,
      etag: string,
      idempotencyKey: string,
    ) => decision(requestId, "approve", {}, etag, idempotencyKey),
    rejectAccessRequest: (
      requestId: string,
      body: RejectAccessRequestBody,
      etag: string,
      idempotencyKey: string,
    ) => decision(requestId, "reject", body, etag, idempotencyKey),
  };
}

export type InviteRequestClient = ReturnType<typeof createInviteRequestClient>;
