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
  "getAccommodationPerformance",
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
  "listCalendarFeeds",
  "createCalendarFeed",
  "removeCalendarFeed",
  "listCalendarReservations",
  "confirmCalendarReservation",
  "dismissCalendarReservation",
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
  // O comparativo é dado de um inquilino só: sai com no-store e sem ETag,
  // porque validador é promessa de cache compartilhado.
  getAccommodationPerformance: { status: 200 },
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
  listCalendarFeeds: { status: 200 },
  createCalendarFeed: { status: 201, etag: true, replay: true },
  removeCalendarFeed: command,
  listCalendarReservations: { status: 200 },
  confirmCalendarReservation: command,
  dismissCalendarReservation: command,
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

/**
 * `month` só acompanha `window=month`. O servidor recusa o par inconsistente em
 * vez de ignorá-lo, então mandá-lo em outra janela renderia 400 — e o cliente
 * tipado seria uma promessa que o contrato não cumpre. Mesma regra de
 * `presenceQuery` no cliente de analytics.
 */
function performanceQuery(window: PerformanceWindow, month?: string) {
  const query = new URLSearchParams({ window });
  if (window === "month" && month !== undefined) {
    query.set("month", month);
  }
  return `?${query.toString()}`;
}

export type PerformanceWindow =
  operations["getAccommodationPerformance"]["parameters"]["query"]["window"];

/**
 * A fila do calendário não usa cursor: ela é limitada ao calendário de uma
 * acomodação, e um token assinado numa lista que cabe na tela seria peso sem
 * ganho.
 */
export interface CalendarReservationFilters {
  limit?: number;
  state?: Schemas["CalendarReservationState"];
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

  const calendarReservationCommand = <Operation extends CoreOperation>(
    operation: Operation,
    reservationId: string,
    path: string,
    body: unknown,
    etag: string,
    idempotencyKey: string,
  ) =>
    request(operation, {
      method: "POST",
      path: `/api/v1/calendar-reservations/${segment(reservationId)}/${path}`,
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
    getAccommodationPerformance: (
      id: string,
      window: PerformanceWindow,
      month?: string,
    ) =>
      read(
        "getAccommodationPerformance",
        `${accommodation(id)}/performance${performanceQuery(window, month)}`,
      ),
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
    listCalendarFeeds: (accommodationId: string) =>
      read("listCalendarFeeds", `${accommodation(accommodationId)}/calendar-feeds`),
    createCalendarFeed: (
      accommodationId: string,
      body: JsonRequest<"createCalendarFeed">,
      idempotencyKey: string,
    ) =>
      request("createCalendarFeed", {
        method: "POST",
        path: `${accommodation(accommodationId)}/calendar-feeds`,
        body,
        headers: { "Idempotency-Key": idempotencyKey },
      }),
    removeCalendarFeed: (feedId: string, etag: string, idempotencyKey: string) =>
      request("removeCalendarFeed", {
        method: "POST",
        path: `/api/v1/calendar-feeds/${segment(feedId)}/remove`,
        body: {},
        headers: concurrencyHeaders(etag, idempotencyKey),
      }),
    listCalendarReservations: (
      accommodationId: string,
      filters: CalendarReservationFilters = {},
    ) =>
      read(
        "listCalendarReservations",
        `${accommodation(accommodationId)}/calendar-reservations${queryString({
          state: filters.state,
          limit: filters.limit,
        })}`,
      ),
    confirmCalendarReservation: (
      reservationId: string,
      body: JsonRequest<"confirmCalendarReservation">,
      etag: string,
      idempotencyKey: string,
    ) =>
      calendarReservationCommand(
        "confirmCalendarReservation",
        reservationId,
        "confirm",
        body,
        etag,
        idempotencyKey,
      ),
    dismissCalendarReservation: (
      reservationId: string,
      etag: string,
      idempotencyKey: string,
    ) =>
      calendarReservationCommand(
        "dismissCalendarReservation",
        reservationId,
        "dismiss",
        {},
        etag,
        idempotencyKey,
      ),
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
