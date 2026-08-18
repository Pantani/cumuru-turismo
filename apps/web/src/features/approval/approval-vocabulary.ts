import type { components } from "../../generated/schema";
import { formatCivilDate, stayCivilDateOf } from "../operator/stay-lifecycle";

type Schemas = components["schemas"];
type RejectionReasonCode = Schemas["RejectStayRequest"]["reason_code"];
type StayApprovalState = Schemas["StayApprovalState"];

/**
 * Closed list, in the precedent of `questionnaire_change_reason_valid`. Free
 * text is refused on purpose: `platform.audit_events` is append-only with no
 * `UPDATE` and no `DELETE`, so a typed sentence there would become permanent
 * personal data inside the very path designed to erase personal data.
 */
export const rejectionReasonOptions = [
  "not_a_guest",
  "identity_not_verified",
  "duplicate",
  "data_incorrect",
  "other",
] as const satisfies readonly RejectionReasonCode[];

export const rejectionReasonLabels: Record<RejectionReasonCode, string> = {
  not_a_guest: "Não é hóspede desta acomodação",
  identity_not_verified: "Não foi possível reconhecer quem se cadastrou",
  duplicate: "Cadastro repetido",
  data_incorrect: "Datas ou dados não conferem",
  other: "Outro motivo",
};

export const approvalStateLabels: Record<StayApprovalState, string> = {
  pending: "Aguardando aprovação",
  approved: "Aprovada",
  rejected: "Recusada",
  expired: "Expirada",
};

export function formatApprovalDeadline(expiresAt: string | null) {
  if (expiresAt === null) {
    return null;
  }
  const parsed = new Date(expiresAt);
  if (Number.isNaN(parsed.getTime())) {
    return null;
  }
  return `Expira em ${formatCivilDate(stayCivilDateOf(parsed))}`;
}
