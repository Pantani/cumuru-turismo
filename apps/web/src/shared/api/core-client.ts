import type { components, operations } from "../../generated/schema";
import {
  type ApiResult,
  createHttpClient,
  type HttpClientOptions,
  queryString,
  type RequestSpec,
  segment,
} from "./http-client";

type Schemas = components["schemas"];

/**
 * The assisted registration surface: accommodations, their memberships, stays
 * and the invite an operator issues for a stay. It is the slice every other
 * feature builds on, and it mirrors `config.CoreConfig` on the API side.
 */
export const coreOperationNames = [
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

type CoreOperation = (typeof coreOperationNames)[number];

/**
 * successContracts states, per operation, which response headers the service
 * owes. Deriving the request spec from it keeps the promise in one table
 * instead of repeated at every call site.
 */
interface SuccessContract {
  etag?: true;
  location?: true;
  replay?: true;
  status: 200 | 201;
}

const created = {
  status: 201,
  etag: true,
  location: true,
  replay: true,
} as const satisfies SuccessContract;

const command = { status: 200, etag: true, replay: true } as const;
const versioned = { status: 200, etag: true } as const;

const successContracts = {
  listAccommodations: { status: 200 },
  createAccommodation: created,
  getAccommodation: versioned,
  updateAccommodation: versioned,
  listAccommodationMemberships: { status: 200 },
  createAccommodationMembership: created,
  updateAccommodationMembership: versioned,
  listStays: { status: 200 },
  createStay: created,
  getStay: versioned,
  updateStay: versioned,
  getStayGroup: versioned,
  submitAssistedStayGroup: command,
  createStayInvite: created,
  checkInStay: command,
  checkOutStay: command,
  cancelStay: command,
  markStayNoShow: command,
  getInvite: { status: 200 },
  submitInviteGroup: command,
} as const satisfies Record<CoreOperation, SuccessContract>;

type OperationStatus<Operation extends CoreOperation> =
  (typeof successContracts)[Operation]["status"];
type OperationResponse<Operation extends CoreOperation> =
  operations[Operation]["responses"][
    OperationStatus<Operation> & keyof operations[Operation]["responses"]
  ] extends {
    content: { "application/json": infer Data };
  }
    ? Data
    : never;
type RequestContent<Operation extends CoreOperation> =
  operations[Operation] extends { requestBody: { content: infer Content } }
    ? Content
    : never;
type JsonRequest<Operation extends CoreOperation> =
  RequestContent<Operation> extends { "application/json": infer Data }
    ? Data
    : never;
type MergePatchRequest<Operation extends CoreOperation> =
  RequestContent<Operation> extends {
    "application/merge-patch+json": infer Data;
  }
    ? Data
    : never;

/**
 * Self-service adds the approval queue as two filters on this very listing, so
 * there is no second endpoint: cursor, limit, ordering and membership isolation
 * stay exactly the ones of the core slice.
 */
export interface StayListFilters {
  accommodationId?: string;
  approvalState?: Schemas["StayApprovalState"];
  cursor?: string;
  limit?: number;
  provenance?: Schemas["StayProvenance"];
  status?: Schemas["StayStatus"];
}

const MERGE_PATCH = "application/merge-patch+json";

function concurrencyHeaders(etag: string, idempotencyKey?: string) {
  return {
    "If-Match": etag,
    ...(idempotencyKey === undefined
      ? {}
      : { "Idempotency-Key": idempotencyKey }),
  };
}

export function createCoreClient(options: HttpClientOptions) {
  const send = createHttpClient(options);

  function request<Operation extends CoreOperation>(
    operation: Operation,
    spec: Omit<RequestSpec, "etag" | "location" | "replay" | "status">,
  ): Promise<ApiResult<OperationResponse<Operation>>> {
    const contract: SuccessContract = successContracts[operation];
    return send({ ...spec, ...contract });
  }

  const read = <Operation extends CoreOperation>(
    operation: Operation,
    path: string,
  ) => request(operation, { method: "GET", path });

  const accommodation = (id: string) => `/api/v1/accommodations/${segment(id)}`;
  const stay = (id: string) => `/api/v1/stays/${segment(id)}`;

  const stayCommand = <Operation extends CoreOperation>(
    operation: Operation,
    stayId: string,
    path: string,
    body: unknown,
    etag: string,
    idempotencyKey: string,
  ) =>
    request(operation, {
      method: "POST",
      path: `${stay(stayId)}/${path}`,
      body,
      headers: concurrencyHeaders(etag, idempotencyKey),
    });

  const mergePatch = <Operation extends CoreOperation>(
    operation: Operation,
    path: string,
    body: unknown,
    etag: string,
  ) =>
    request(operation, {
      method: "PATCH",
      path,
      body,
      contentType: MERGE_PATCH,
      headers: concurrencyHeaders(etag),
    });

  return {
    listAccommodations: (cursor?: string, limit?: number) =>
      read(
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
      read("getAccommodation", accommodation(id)),
    updateAccommodation: (
      id: string,
      body: MergePatchRequest<"updateAccommodation">,
      etag: string,
    ) => mergePatch("updateAccommodation", accommodation(id), body, etag),
    listAccommodationMemberships: (
      accommodationId: string,
      cursor?: string,
      limit?: number,
    ) =>
      read(
        "listAccommodationMemberships",
        `${accommodation(accommodationId)}/memberships${queryString({ cursor, limit })}`,
      ),
    createAccommodationMembership: (
      accommodationId: string,
      body: JsonRequest<"createAccommodationMembership">,
      idempotencyKey: string,
    ) =>
      request("createAccommodationMembership", {
        method: "POST",
        path: `${accommodation(accommodationId)}/memberships`,
        body,
        headers: { "Idempotency-Key": idempotencyKey },
      }),
    updateAccommodationMembership: (
      accommodationId: string,
      membershipId: string,
      body: MergePatchRequest<"updateAccommodationMembership">,
      etag: string,
    ) =>
      mergePatch(
        "updateAccommodationMembership",
        `${accommodation(accommodationId)}/memberships/${segment(membershipId)}`,
        body,
        etag,
      ),
    listStays: (filters: StayListFilters = {}) =>
      read(
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
    createStay: (body: JsonRequest<"createStay">, idempotencyKey: string) =>
      request("createStay", {
        method: "POST",
        path: "/api/v1/stays",
        body,
        headers: { "Idempotency-Key": idempotencyKey },
      }),
    getStay: (id: string) => read("getStay", stay(id)),
    updateStay: (
      id: string,
      body: MergePatchRequest<"updateStay">,
      etag: string,
    ) => mergePatch("updateStay", stay(id), body, etag),
    getStayGroup: (stayId: string) =>
      read("getStayGroup", `${stay(stayId)}/group`),
    submitAssistedStayGroup: (
      stayId: string,
      body: JsonRequest<"submitAssistedStayGroup">,
      etag: string,
      idempotencyKey: string,
    ) =>
      stayCommand(
        "submitAssistedStayGroup",
        stayId,
        "group",
        body,
        etag,
        idempotencyKey,
      ),
    createStayInvite: (
      stayId: string,
      body: JsonRequest<"createStayInvite">,
      etag: string,
      idempotencyKey: string,
    ) =>
      stayCommand(
        "createStayInvite",
        stayId,
        "invite",
        body,
        etag,
        idempotencyKey,
      ),
    checkInStay: (
      stayId: string,
      body: JsonRequest<"checkInStay">,
      etag: string,
      idempotencyKey: string,
    ) =>
      stayCommand("checkInStay", stayId, "check-in", body, etag, idempotencyKey),
    checkOutStay: (
      stayId: string,
      body: JsonRequest<"checkOutStay">,
      etag: string,
      idempotencyKey: string,
    ) =>
      stayCommand(
        "checkOutStay",
        stayId,
        "check-out",
        body,
        etag,
        idempotencyKey,
      ),
    cancelStay: (
      stayId: string,
      body: JsonRequest<"cancelStay">,
      etag: string,
      idempotencyKey: string,
    ) =>
      stayCommand("cancelStay", stayId, "cancel", body, etag, idempotencyKey),
    markStayNoShow: (
      stayId: string,
      body: JsonRequest<"markStayNoShow">,
      etag: string,
      idempotencyKey: string,
    ) =>
      stayCommand(
        "markStayNoShow",
        stayId,
        "no-show",
        body,
        etag,
        idempotencyKey,
      ),
    getInvite: (capability: string) =>
      request("getInvite", {
        method: "GET",
        path: `/api/v1/invites/${segment(capability)}`,
        authenticated: false,
      }),
    submitInviteGroup: (
      capability: string,
      body: JsonRequest<"submitInviteGroup">,
      idempotencyKey: string,
    ) =>
      request("submitInviteGroup", {
        method: "POST",
        path: `/api/v1/invites/${segment(capability)}/submit`,
        authenticated: false,
        body,
        headers: { "Idempotency-Key": idempotencyKey },
      }),
  };
}

export type CoreClient = ReturnType<typeof createCoreClient>;
