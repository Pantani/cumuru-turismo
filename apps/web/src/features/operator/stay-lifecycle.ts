import type { components } from "../../generated/schema";

export type StayStatus = components["schemas"]["StayStatus"];
export type Stay = components["schemas"]["Stay"];
export type Accommodation = components["schemas"]["Accommodation"];
export type AccommodationCategory = components["schemas"]["AccommodationCategory"];
export type AccommodationInputCategory =
  components["schemas"]["AccommodationInputCategory"];

export type StayAction =
  | "invite"
  | "submit_group"
  | "check_in"
  | "check_out"
  | "cancel"
  | "no_show";

/**
 * Mirrors the server state machine in apps/api/internal/stay/status.go. The
 * interface only offers transitions the backend accepts, so an operator never
 * discovers a rule by being rejected.
 */
const allowedActions: Record<StayStatus, readonly StayAction[]> = {
  draft: ["invite", "submit_group", "cancel"],
  invited: ["invite", "submit_group", "cancel", "no_show"],
  pre_registered: ["check_in", "cancel", "no_show"],
  checked_in: ["check_out"],
  checked_out: [],
  cancelled: [],
  no_show: [],
};

export const stayStatusLabels: Record<StayStatus, string> = {
  draft: "Rascunho",
  invited: "Convite enviado",
  pre_registered: "Pré-registrada",
  checked_in: "Hóspedes na casa",
  checked_out: "Estadia encerrada",
  cancelled: "Cancelada",
  no_show: "Não compareceu",
};

export const stayActionLabels: Record<StayAction, string> = {
  invite: "Gerar convite",
  submit_group: "Registrar visitantes",
  check_in: "Fazer check-in",
  check_out: "Fazer check-out",
  cancel: "Cancelar estadia",
  no_show: "Marcar não comparecimento",
};

export const accommodationCategoryLabels: Record<AccommodationCategory, string> = {
  formal_lodging: "Pousada, hotel ou meio de hospedagem",
  seasonal_rental: "Casa ou imóvel de temporada",
  family_hosting: "Hospedagem familiar",
  camping: "Camping",
  regularizing: "Em regularização",
  other: "Outro",
  unclassified: "Ainda não classificado",
};

export const accommodationCategoryOptions = [
  "formal_lodging",
  "seasonal_rental",
  "family_hosting",
  "camping",
  "regularizing",
  "other",
] as const satisfies readonly AccommodationInputCategory[];

export function actionsFor(status: StayStatus): readonly StayAction[] {
  return allowedActions[status] ?? [];
}

export function isOpenStay(status: StayStatus) {
  return actionsFor(status).length > 0;
}

/**
 * The server derives the entity tag from the row version, so a list payload
 * already carries everything optimistic concurrency needs. This is what removes
 * the ETag field from the interface.
 */
export function entityTagFor(version: number) {
  return `"${version}"`;
}

export function formatCivilDate(value: string) {
  const parsed = new Date(`${value}T12:00:00Z`);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("pt-BR", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    timeZone: "UTC",
  }).format(parsed);
}

const stayCivilDateFormat = new Intl.DateTimeFormat("en", {
  day: "2-digit",
  month: "2-digit",
  timeZone: "America/Bahia",
  year: "numeric",
});

/** Data civil (AAAA-MM-DD) de um instante, lida no fuso da estadia. */
function civilDateAtStayZone(moment: Date) {
  const parts = stayCivilDateFormat.formatToParts(moment);
  const value = (type: Intl.DateTimeFormatPartTypes) =>
    parts.find((part) => part.type === type)?.value ?? "";
  return `${value("year")}-${value("month")}-${value("day")}`;
}

/**
 * Prazos chegam como instante UTC. Cortar os dez primeiros caracteres devolve o
 * dia do calendário UTC, e uma expiração depois das 21h em `America/Bahia` já
 * pertence ao dia seguinte lá — a tela anunciaria um dia a mais de prazo do que
 * o servidor concede. A data civil é lida no fuso da estadia, como manda o
 * AGENTS.md.
 */
export function stayCivilDateOf(instant: string) {
  const parsed = new Date(instant);
  return Number.isNaN(parsed.getTime())
    ? instant.slice(0, 10)
    : civilDateAtStayZone(parsed);
}

/** Default civil dates for a new stay, in the observatory's own timezone. */
export function defaultStayDates(now = new Date()) {
  const base = new Date(`${civilDateAtStayZone(now)}T00:00:00Z`);
  const civilDateAt = (offset: number) => {
    const date = new Date(base);
    date.setUTCDate(date.getUTCDate() + offset);
    return date.toISOString().slice(0, 10);
  };
  return { arrival: civilDateAt(0), departure: civilDateAt(2) };
}
