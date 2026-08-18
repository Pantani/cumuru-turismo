import {
  accessRequestCategories,
  type AccessRequestBody,
  type AccessRequestCategory,
  type AccessRequestContext,
} from "../../shared/api/invite-request-client";
import type { MessageKey } from "../../shared/i18n/translate";
import { createUuidV7 } from "../../shared/identity/uuid-v7";

/**
 * Estado e regras do pedido de acesso.
 *
 * As chaves do estado são as do corpo enviado, e não uma versão camelCase: o
 * formulário, a validação, o `id` do campo e o JSON passam a falar o mesmo nome,
 * então não existe tabela de tradução para sair de sincronia quando um campo
 * novo aparecer.
 *
 * A mensagem de cada recusa é uma chave de tradução, nunca uma frase pronta:
 * esta é uma tela pública em três idiomas, e um texto fixo aqui apareceria em
 * português para quem escolheu espanhol.
 */

export interface InviteRequestFormState {
  accommodation_name: string;
  capacity: string;
  category: string;
  city_label: string;
  client_submission_id: string;
  contact_email: string;
  contact_name: string;
  contact_phone: string;
  state_code: string;
}

export type InviteRequestField = Exclude<
  keyof InviteRequestFormState,
  "client_submission_id"
>;

export interface InviteRequestIssue {
  field: InviteRequestField;
  message: MessageKey;
}

export const ACCOMMODATION_NAME_MAX = 200;
export const CITY_MAX = 120;
export const CONTACT_NAME_MAX = 120;
export const CONTACT_EMAIL_MAX = 254;
export const CONTACT_PHONE_MAX = 40;
const CAPACITY_MIN = 1;
const CAPACITY_MAX = 10_000;

const digitsPattern = /^\d+$/u;
const statePattern = /^[A-Z]{2}$/u;
/** Só o que separa um endereço plausível de um erro de digitação óbvio. */
const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/u;

const categories = new Set<string>(accessRequestCategories);

export function initialInviteRequestForm(): InviteRequestFormState {
  return {
    accommodation_name: "",
    capacity: "",
    category: "",
    city_label: "",
    client_submission_id: createUuidV7(),
    contact_email: "",
    contact_name: "",
    contact_phone: "",
    state_code: "",
  };
}

export function fieldElementId(field: InviteRequestField) {
  return `invite-request-${field.replaceAll("_", "-")}`;
}

function issue(
  field: InviteRequestField,
  message: MessageKey,
): InviteRequestIssue[] {
  return [{ field, message }];
}

function textIssues(
  field: InviteRequestField,
  value: string,
  maxLength: number,
  message: MessageKey,
) {
  const trimmed = value.trim();
  const accepted = trimmed.length > 0 && trimmed.length <= maxLength;
  return accepted ? [] : issue(field, message);
}

function categoryIssues(value: string) {
  return categories.has(value)
    ? []
    : issue("category", "inviteRequest.error.category");
}

function capacityIssues(value: string) {
  const trimmed = value.trim();
  const accepted =
    digitsPattern.test(trimmed) &&
    Number(trimmed) >= CAPACITY_MIN &&
    Number(trimmed) <= CAPACITY_MAX;
  return accepted ? [] : issue("capacity", "inviteRequest.error.capacity");
}

function stateIssues(value: string) {
  return statePattern.test(normalizedState(value))
    ? []
    : issue("state_code", "inviteRequest.error.state");
}

function emailIssues(value: string) {
  const trimmed = value.trim();
  const accepted =
    emailPattern.test(trimmed) && trimmed.length <= CONTACT_EMAIL_MAX;
  return accepted ? [] : issue("contact_email", "inviteRequest.error.email");
}

/** Opcional de verdade: em branco não é erro; preenchido demais, sim. */
function phoneIssues(value: string) {
  const accepted = value.trim().length <= CONTACT_PHONE_MAX;
  return accepted ? [] : issue("contact_phone", "inviteRequest.error.phone");
}

export function normalizedState(value: string) {
  return value.trim().toUpperCase();
}

export function validateInviteRequest(
  state: InviteRequestFormState,
): InviteRequestIssue[] {
  return [
    ...textIssues(
      "accommodation_name",
      state.accommodation_name,
      ACCOMMODATION_NAME_MAX,
      "inviteRequest.error.name",
    ),
    ...categoryIssues(state.category),
    ...capacityIssues(state.capacity),
    ...textIssues(
      "city_label",
      state.city_label,
      CITY_MAX,
      "inviteRequest.error.city",
    ),
    ...stateIssues(state.state_code),
    ...textIssues(
      "contact_name",
      state.contact_name,
      CONTACT_NAME_MAX,
      "inviteRequest.error.contactName",
    ),
    ...emailIssues(state.contact_email),
    ...phoneIssues(state.contact_phone),
  ];
}

/**
 * Falha explícita, nunca reescrita silenciosa: `validateInviteRequest` recusa
 * um tipo fora da lista antes daqui, então chegar aqui com outro valor significa
 * que a guarda foi contornada e o pedido não pode ser gravado como "outro".
 */
function requiredCategory(value: string): AccessRequestCategory {
  const category = accessRequestCategories.find((known) => known === value);
  if (category === undefined) {
    throw new Error("Tipo de hospedagem fora da lista aceita pelo contrato.");
  }
  return category;
}

/** O telefone só entra no corpo quando existe; vazio não vira string vazia. */
function phoneField(value: string) {
  const trimmed = value.trim();
  return trimmed.length === 0 ? {} : { contact_phone: trimmed };
}

export function inviteRequestBodyFrom(
  state: InviteRequestFormState,
  context: AccessRequestContext,
  solution: string,
): AccessRequestBody {
  return {
    accommodation_name: state.accommodation_name.trim(),
    capacity: Number(state.capacity.trim()),
    category: requiredCategory(state.category),
    city_label: state.city_label.trim(),
    client_submission_id: state.client_submission_id,
    contact_email: state.contact_email.trim(),
    contact_name: state.contact_name.trim(),
    ...phoneField(state.contact_phone),
    privacy_notice_version: context.privacy_notice_version,
    proof_of_work: {
      challenge: context.proof_of_work.challenge,
      solution,
    },
    state_code: normalizedState(state.state_code),
  };
}
