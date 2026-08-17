import type { components, operations } from "../../generated/schema";
import {
  createHttpClient,
  type HttpClientOptions,
  segment,
} from "./http-client";

type Schemas = components["schemas"];

/**
 * The open self-registration channel and the account activation capability.
 *
 * Each purpose owns its own header, because separate purposes are separate
 * cryptographic domains: a leaked invite token must not be usable to activate
 * an account (ADR-039, ADR-041).
 */
export const INVITE_TOKEN_HEADER = "X-Cumuru-Invite-Token";
export const ACTIVATION_TOKEN_HEADER = "X-Cumuru-Activation-Token";

export const selfServiceOperationNames = [
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

export function createSelfServiceClient(options: HttpClientOptions) {
  const request = createHttpClient(options);

  const accommodation = (id: string) => `/api/v1/accommodations/${segment(id)}`;
  const conditional = (etag: string, idempotencyKey: string) => ({
    "If-Match": etag,
    "Idempotency-Key": idempotencyKey,
  });

  const stayCommand = (
    stayId: string,
    action: string,
    body: object,
    etag: string,
    idempotencyKey: string,
  ) =>
    request<Schemas["StayMutationResult"]>({
      method: "POST",
      path: `/api/v1/stays/${segment(stayId)}/${action}`,
      body,
      etag: true,
      replay: true,
      headers: conditional(etag, idempotencyKey),
    });

  const withCapability = <T,>(
    header: string,
    value: string,
    spec: Omit<Parameters<typeof request>[0], "capability" | "authenticated">,
  ) =>
    request<T>({ ...spec, authenticated: false, capability: { header, value } });

  return {
    createAccommodationActivation: (
      accommodationId: string,
      body: Schemas["CreateAccommodationActivationRequest"],
      etag: string,
      idempotencyKey: string,
    ) =>
      request<Schemas["AccommodationActivationCreated"]>({
        method: "POST",
        path: `${accommodation(accommodationId)}/activation`,
        body,
        etag: true,
        location: true,
        replay: true,
        status: 201,
        headers: conditional(etag, idempotencyKey),
      }),
    getAccommodationActivation: (capability: string) =>
      withCapability<Schemas["AccommodationActivationContext"]>(
        ACTIVATION_TOKEN_HEADER,
        capability,
        { method: "GET", path: "/api/v1/activation" },
      ),
    activateAccommodationAccount: (
      capability: string,
      body: Schemas["ActivateAccountRequest"],
    ) =>
      withCapability<null>(ACTIVATION_TOKEN_HEADER, capability, {
        method: "POST",
        path: "/api/v1/activation/complete",
        body,
        noContent: true,
        status: 204,
      }),
    getAccommodationInvite: (accommodationId: string) =>
      request<Schemas["AccommodationInviteStatus"]>({
        method: "GET",
        path: `${accommodation(accommodationId)}/invite`,
      }),
    createAccommodationInvite: (
      accommodationId: string,
      body: Schemas["CreateAccommodationInviteRequest"],
      etag: string,
      idempotencyKey: string,
    ) =>
      request<Schemas["AccommodationInviteCreated"]>({
        method: "POST",
        path: `${accommodation(accommodationId)}/invite`,
        body,
        etag: true,
        location: true,
        replay: true,
        status: 201,
        headers: conditional(etag, idempotencyKey),
      }),
    revokeAccommodationInvite: (
      accommodationId: string,
      etag: string,
      idempotencyKey: string,
    ) =>
      request<Schemas["AccommodationInviteStatus"]>({
        method: "POST",
        path: `${accommodation(accommodationId)}/invite/revoke`,
        body: {},
        etag: true,
        replay: true,
        headers: conditional(etag, idempotencyKey),
      }),
    getAccommodationInviteContext: (capability: string) =>
      withCapability<Schemas["AccommodationInviteContext"]>(
        INVITE_TOKEN_HEADER,
        capability,
        { method: "GET", path: "/api/v1/accommodation-invite" },
      ),
    submitAccommodationSelfRegistration: (
      capability: string,
      body: Schemas["SelfRegistrationRequest"],
      idempotencyKey: string,
    ) =>
      withCapability<Schemas["SelfRegistrationAccepted"]>(
        INVITE_TOKEN_HEADER,
        capability,
        {
          method: "POST",
          path: "/api/v1/accommodation-invite/submit",
          body,
          etag: true,
          replay: true,
          headers: { "Idempotency-Key": idempotencyKey },
        },
      ),
    approveStay: (stayId: string, etag: string, idempotencyKey: string) =>
      stayCommand(stayId, "approve", {}, etag, idempotencyKey),
    rejectStay: (
      stayId: string,
      body: Schemas["RejectStayRequest"],
      etag: string,
      idempotencyKey: string,
    ) => stayCommand(stayId, "reject", body, etag, idempotencyKey),
  };
}

export type SelfServiceClient = ReturnType<typeof createSelfServiceClient>;
