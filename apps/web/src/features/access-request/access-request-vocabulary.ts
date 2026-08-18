import type {
  AccessRequest,
  AccessRequestCategory,
  AccessRequestRejectionReason,
  AccessRequestState,
} from "../../shared/api/invite-request-client";
import type { MessageKey, Translate } from "../../shared/i18n/translate";

/**
 * Vocabulário da fila. Cada valor do contrato tem aqui uma chave de mensagem,
 * nunca uma frase literal: quem lê esta tela é a administração municipal, e o
 * enum do OpenAPI (`duplicate_request`, `not_a_lodging`) não é português.
 */

export const accessRequestStates = [
  "pending",
  "approved",
  "rejected",
  "expired",
] as const satisfies readonly AccessRequestState[];

export const accessRequestStateKeys: Record<AccessRequestState, MessageKey> = {
  pending: "accessRequest.state.pending",
  approved: "accessRequest.state.approved",
  rejected: "accessRequest.state.rejected",
  expired: "accessRequest.state.expired",
};

/** Lista fechada do contrato (ADR-042): texto livre viraria dado permanente. */
export const rejectionReasonOptions = [
  "duplicate_request",
  "not_a_lodging",
  "insufficient_information",
  "abuse",
] as const satisfies readonly AccessRequestRejectionReason[];

export const rejectionReasonKeys: Record<
  AccessRequestRejectionReason,
  MessageKey
> = {
  duplicate_request: "accessRequest.reason.duplicateRequest",
  not_a_lodging: "accessRequest.reason.notALodging",
  insufficient_information: "accessRequest.reason.insufficientInformation",
  abuse: "accessRequest.reason.abuse",
};

/** Mesmo vocabulário do formulário público, para os dois lados dizerem o mesmo. */
export const categoryKeys: Record<AccessRequestCategory, MessageKey> = {
  formal_lodging: "inviteRequest.category.formalLodging",
  seasonal_rental: "inviteRequest.category.seasonalRental",
  family_hosting: "inviteRequest.category.familyHosting",
  camping: "inviteRequest.category.camping",
  regularizing: "inviteRequest.category.regularizing",
  other: "inviteRequest.category.other",
};

/** Contato presente. Nulo em `rejected` e `expired`, quando já foi eliminado. */
export interface AccessRequestContact {
  email: string;
  name: string;
  phone: string | null;
}

export function contactOf(request: AccessRequest): AccessRequestContact | null {
  if (request.contact_name === null || request.contact_email === null) {
    return null;
  }
  return {
    email: request.contact_email,
    name: request.contact_name,
    phone: request.contact_phone,
  };
}

const HOUR_IN_MS = 3_600_000;

function elapsedHours(createdAt: string, now: number): number | null {
  const started = new Date(createdAt).getTime();
  if (Number.isNaN(started)) {
    return null;
  }
  return Math.floor(Math.max(0, now - started) / HOUR_IN_MS);
}

function pluralKey(one: MessageKey, other: MessageKey, count: number) {
  return count === 1 ? one : other;
}

/**
 * A espera é o que faz a administração priorizar, então ela é dita em dias e
 * horas — não em carimbo de tempo, que obriga o leitor a fazer a conta.
 */
export function waitingLabel(
  t: Translate,
  createdAt: string,
  now = Date.now(),
): string | null {
  const hours = elapsedHours(createdAt, now);
  if (hours === null) {
    return null;
  }
  if (hours < 1) {
    return t("accessRequest.waiting.now");
  }
  if (hours < 24) {
    const key = pluralKey(
      "accessRequest.waiting.hour.one",
      "accessRequest.waiting.hour.other",
      hours,
    );
    return t(key, { count: hours });
  }
  const days = Math.floor(hours / 24);
  const key = pluralKey(
    "accessRequest.waiting.day.one",
    "accessRequest.waiting.day.other",
    days,
  );
  return t(key, { count: days });
}

export function capacityLabel(t: Translate, capacity: number) {
  const key = pluralKey(
    "accessRequest.capacityValue.one",
    "accessRequest.capacityValue.other",
    capacity,
  );
  return t(key, { count: capacity });
}
