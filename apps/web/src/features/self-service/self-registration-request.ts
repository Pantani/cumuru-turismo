import type { components } from "../../generated/schema";
import type { SelfRegistrationDraft } from "../../shared/validation/self-service-validation";
import { normalizedVisitor } from "../visitors/visitor-input";

type Schemas = components["schemas"];
type AccommodationInviteContext = Schemas["AccommodationInviteContext"];
type SelfServiceVisitorInput = Schemas["SelfServiceVisitorInput"];
type VisitorInput = Schemas["VisitorInput"];

export interface SelfRegistrationFormState {
  clientSubmissionId: string;
  plannedArrivalOn: string;
  plannedDepartureOn: string;
  visitors: VisitorInput[];
}

/**
 * Explicit failure, never a silent rewrite: `validateSelfRegistrationDraft`
 * refuses `minor` before this runs, so reaching here means the guard was
 * bypassed and the submission must not be quietly relabelled as a companion.
 */
function selfServiceRole(
  role: VisitorInput["role"],
): SelfServiceVisitorInput["role"] {
  if (role === "minor") {
    throw new Error(
      "O canal aberto não aceita menores; a validação deveria ter recusado.",
    );
  }
  return role;
}

function selfServiceVisitor(visitor: VisitorInput): SelfServiceVisitorInput {
  const normalized = normalizedVisitor(visitor);
  return { ...normalized, role: selfServiceRole(normalized.role) };
}

/**
 * The body carries no consent decision. `survey.consent_decisions` requires a
 * `response_id` pointing at a real questionnaire response, so recording one
 * here would mean fabricating a survey answer for someone who answered nothing
 * — and that fabricated row would later collide on
 * `UNIQUE (stay_id, questionnaire_version_id)` when the guest opens the real
 * survey with the `Survey-Capability` this very flow emits.
 *
 * The requirement is not lost. ADR-040 limits the open channel to generalised
 * data, so there is no questionnaire purpose to consent to at submit time. The
 * versioned notice stays on screen and the acceptance is pinned by
 * `privacy_notice_version`, exactly as in the `assisted` and `invite` channels;
 * a poster printed against an older notice still fails with 409.
 */
export function selfRegistrationDraftFrom(
  state: SelfRegistrationFormState,
  context: AccommodationInviteContext,
): SelfRegistrationDraft {
  return {
    client_submission_id: state.clientSubmissionId,
    privacy_notice_version: context.privacy_notice_version,
    planned_arrival_on: state.plannedArrivalOn,
    planned_departure_on: state.plannedDepartureOn,
    visitors: state.visitors.map(selfServiceVisitor),
  };
}
