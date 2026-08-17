import type { components } from "../../generated/schema";
import {
  validatePlannedDates,
  validateSubmitGroup,
  type ValidationIssue,
} from "./phase2-validation";

type SelfRegistrationRequest = components["schemas"]["SelfRegistrationRequest"];
type SelfServiceVisitorInput = components["schemas"]["SelfServiceVisitorInput"];

export const ACTIVATION_MIN_PASSWORD_LENGTH = 12;
export const ACTIVATION_MAX_PASSWORD_LENGTH = 256;

/**
 * ADR-040 refuses `role='minor'` in the open channel: there is no way to verify
 * a guardian in an anonymous, unauthenticated form. The generated type already
 * narrows the role, so this guard exists for values that re-enter the app from
 * outside the type system — a restored IndexedDB draft, above all.
 */
function openChannelRoleIssues(
  visitors: readonly SelfServiceVisitorInput[],
): ValidationIssue[] {
  return visitors.flatMap((visitor, index) =>
    String(visitor.role) === "minor"
      ? [
          {
            field: `visitors.${index}.role`,
            message:
              "Menores não são aceitos neste formulário aberto. Use o canal assistido pela hospedagem.",
          },
        ]
      : [],
  );
}

function proofOfWorkIssues(
  proof: SelfRegistrationRequest["proof_of_work"],
): ValidationIssue[] {
  if (proof.challenge.length > 0 && proof.solution.length > 0) {
    return [];
  }
  return [
    {
      field: "proof_of_work",
      message: "A verificação de segurança ainda não terminou. Aguarde um instante.",
    },
  ];
}

export type SelfRegistrationDraft = Omit<
  SelfRegistrationRequest,
  "proof_of_work"
>;

/**
 * The group rules of Phase 2 apply unchanged; only the channel rules are new.
 * The draft variant exists so an invalid form is refused before the guest's
 * device spends any work on the proof.
 */
export function validateSelfRegistrationDraft(
  value: SelfRegistrationDraft,
): ValidationIssue[] {
  return [
    ...openChannelRoleIssues(value.visitors),
    ...validatePlannedDates(value),
    ...validateSubmitGroup({
      client_submission_id: value.client_submission_id,
      privacy_notice_version: value.privacy_notice_version,
      visitors: value.visitors,
    }),
  ];
}

export function validateSelfRegistration(
  value: SelfRegistrationRequest,
): ValidationIssue[] {
  return [
    ...validateSelfRegistrationDraft(value),
    ...proofOfWorkIssues(value.proof_of_work),
  ];
}

export interface ActivationPasswordIssues {
  confirmation?: string;
  password?: string;
}

function passwordLengthIssue(password: string) {
  if (password.length < ACTIVATION_MIN_PASSWORD_LENGTH) {
    return `A senha precisa ter no mínimo ${ACTIVATION_MIN_PASSWORD_LENGTH} caracteres.`;
  }
  if (password.length > ACTIVATION_MAX_PASSWORD_LENGTH) {
    return `A senha precisa ter no máximo ${ACTIVATION_MAX_PASSWORD_LENGTH} caracteres.`;
  }
  return undefined;
}

/**
 * Mirrors the contract bounds so an obviously invalid attempt never spends the
 * single-use capability. The confirmation field exists only here: a secret that
 * cannot be read back is easy to mistype, and there is no second capability.
 */
export function validateActivationPassword(
  password: string,
  confirmation: string,
): ActivationPasswordIssues {
  const length = passwordLengthIssue(password);
  const mismatch =
    confirmation === password
      ? undefined
      : "A confirmação não confere com a senha escolhida.";
  return {
    ...(length === undefined ? {} : { password: length }),
    ...(mismatch === undefined ? {} : { confirmation: mismatch }),
  };
}
