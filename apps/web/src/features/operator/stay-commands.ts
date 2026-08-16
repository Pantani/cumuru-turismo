import type { Phase2Client } from "../../shared/api/phase2-client";
import { entityTagFor, type Stay, type StayAction } from "./stay-lifecycle";

/**
 * PRIVACY_NOTICE_VERSION pins the notice the operator is acting under. It is
 * carried on every invite and group submission so a stored answer keeps the
 * consent text it was given with.
 */
export const PRIVACY_NOTICE_VERSION = "prototype-v1";

export type DirectStayAction = Exclude<StayAction, "invite" | "submit_group">;

interface CommandContext {
  client: Phase2Client;
  stay: Stay;
}

/**
 * runDirectAction issues the lifecycle transitions that need no extra input.
 * The entity tag comes from the row version already in hand, so concurrency is
 * enforced without ever asking the operator for one.
 */
export function runDirectAction(
  { client, stay }: CommandContext,
  action: DirectStayAction,
) {
  const etag = entityTagFor(stay.version);
  const key = crypto.randomUUID();
  if (action === "check_in") {
    return client.checkInStay(stay.id, {}, etag, key);
  }
  if (action === "check_out") {
    return client.checkOutStay(stay.id, {}, etag, key);
  }
  if (action === "no_show") {
    return client.markStayNoShow(stay.id, {}, etag, key);
  }
  return client.cancelStay(
    stay.id,
    { reason_code: "guest_request", correction: false },
    etag,
    key,
  );
}

export function createInvite({ client, stay }: CommandContext) {
  return client.createStayInvite(
    stay.id,
    { privacy_notice_version: PRIVACY_NOTICE_VERSION },
    entityTagFor(stay.version),
    crypto.randomUUID(),
  );
}
