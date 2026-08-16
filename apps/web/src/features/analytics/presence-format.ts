import {
  centralValue,
  percentFromAverage,
  type PresencePoint,
} from "./presence-stats";

const TIME_ZONE = "America/Bahia";

const dateFormatter = new Intl.DateTimeFormat("pt-BR", {
  dateStyle: "medium",
  timeZone: TIME_ZONE,
});
const dayFormatter = new Intl.DateTimeFormat("pt-BR", {
  weekday: "short",
  day: "2-digit",
  month: "2-digit",
  timeZone: TIME_ZONE,
});
const dateTimeFormatter = new Intl.DateTimeFormat("pt-BR", {
  dateStyle: "long",
  timeStyle: "short",
  timeZone: TIME_ZONE,
});
const countFormatter = new Intl.NumberFormat("pt-BR");

/** Noon anchors the civil date away from the day boundary in America/Bahia. */
function civilDate(value: string) {
  return new Date(`${value}T12:00:00-03:00`);
}

export function formatDate(value: string) {
  return dateFormatter.format(civilDate(value));
}

export function formatDay(value: string) {
  return dayFormatter.format(civilDate(value));
}

export function formatDateTime(value: string) {
  return dateTimeFormatter.format(new Date(value));
}

export function formatCount(value: number) {
  return countFormatter.format(value);
}

/**
 * "Referência" and not "janela": a joined view measures the forecast against
 * the observed level, so naming the window here would state the wrong scope.
 */
export function formatDelta(percent: number) {
  const rounded = Math.round(percent);
  if (rounded === 0) {
    return "no mesmo nível da média de referência";
  }
  const direction = rounded > 0 ? "acima" : "abaixo";
  return `${Math.abs(rounded)}% ${direction} da média de referência`;
}

export function formatTrend(percent: number, size: number) {
  const rounded = Math.round(percent);
  const sign = rounded > 0 ? "+" : "";
  return `${sign}${rounded}% ante os ${size} dias anteriores`;
}

function kindText(kind: PresencePoint["kind"]) {
  return kind === "observed" ? "Observado" : "Previsto";
}

function bandLine(point: PresencePoint): string[] {
  if (point.status !== "published" || point.kind !== "forecast") {
    return [];
  }
  return [`Faixa provável: ${point.lower} a ${point.upper} pessoas-dia`];
}

function deltaLine(value: number, average: number | null): string[] {
  const percent = percentFromAverage(value, average);
  return percent === null ? [] : [formatDelta(percent)];
}

function withheldLine(point: PresencePoint): string[] {
  return [
    point.status === "protected"
      ? "Protegido pela política de publicação, sem valor substituto"
      : "Sem dado disponível para este dia",
  ];
}

/**
 * One description of a day, reused by the tooltip and by the live readout, so
 * pointer and keyboard readers never receive different facts.
 */
export function slotLines(
  point: PresencePoint,
  average: number | null,
): string[] {
  const value = centralValue(point);
  if (value === null) {
    return withheldLine(point);
  }
  return [
    `${kindText(point.kind)}: ${value} pessoas-dia`,
    ...bandLine(point),
    ...deltaLine(value, average),
  ];
}
