import { useMemo } from "react";

import { useLocale } from "../../shared/i18n/LocaleProvider";
import type { Translate } from "../../shared/i18n/translate";
import {
  centralValue,
  percentFromAverage,
  type PresencePoint,
} from "./presence-stats";

const TIME_ZONE = "America/Bahia";

interface Formatters {
  count: Intl.NumberFormat;
  date: Intl.DateTimeFormat;
  dateTime: Intl.DateTimeFormat;
  day: Intl.DateTimeFormat;
  weekday: Intl.DateTimeFormat;
}

/**
 * Domingo, 7 de janeiro de 2024, em UTC: âncora para nomear o dia da semana
 * pelo `Intl` do idioma ativo em vez de manter sete traduções por idioma.
 */
const WEEKDAY_ANCHOR_DAY = 7;

function weekdayDate(weekday: number): Date {
  return new Date(Date.UTC(2024, 0, WEEKDAY_ANCHOR_DAY + weekday));
}

/**
 * `Intl` é caro para instanciar e o painel formata centenas de células por
 * render. O cache é por tag e não por render: são três tags no total.
 */
const FORMATTERS = new Map<string, Formatters>();

function buildFormatters(tag: string): Formatters {
  return {
    count: new Intl.NumberFormat(tag),
    date: new Intl.DateTimeFormat(tag, {
      dateStyle: "medium",
      timeZone: TIME_ZONE,
    }),
    dateTime: new Intl.DateTimeFormat(tag, {
      dateStyle: "long",
      timeStyle: "short",
      timeZone: TIME_ZONE,
    }),
    day: new Intl.DateTimeFormat(tag, {
      weekday: "short",
      day: "2-digit",
      month: "2-digit",
      timeZone: TIME_ZONE,
    }),
    weekday: new Intl.DateTimeFormat(tag, {
      weekday: "long",
      timeZone: "UTC",
    }),
  };
}

function formattersFor(tag: string): Formatters {
  const cached = FORMATTERS.get(tag);
  if (cached !== undefined) {
    return cached;
  }
  const created = buildFormatters(tag);
  FORMATTERS.set(tag, created);
  return created;
}

/** Meio-dia ancora a data civil longe da virada do dia em America/Bahia. */
function civilDate(value: string) {
  return new Date(`${value}T12:00:00-03:00`);
}

/**
 * "Referência" e não "janela": uma visão combinada mede a previsão contra o
 * nível observado, então nomear a janela aqui declararia o escopo errado.
 */
function deltaText(t: Translate, percent: number): string {
  const rounded = Math.round(percent);
  if (rounded === 0) {
    return t("analytics.delta.same");
  }
  const key = rounded > 0 ? "analytics.delta.above" : "analytics.delta.below";
  return t(key, { percent: Math.abs(rounded) });
}

function trendText(t: Translate, percent: number, size: number): string {
  const rounded = Math.round(percent);
  return t("analytics.trend.value", {
    percent: rounded,
    sign: rounded > 0 ? "+" : "",
    size,
  });
}

function bandLine(t: Translate, point: PresencePoint): string[] {
  if (point.status !== "published" || point.kind !== "forecast") {
    return [];
  }
  return [t("analytics.slot.band", { lower: point.lower, upper: point.upper })];
}

function deltaLine(
  t: Translate,
  value: number,
  average: number | null,
): string[] {
  const percent = percentFromAverage(value, average);
  return percent === null ? [] : [deltaText(t, percent)];
}

function withheldLine(t: Translate, point: PresencePoint): string[] {
  return [
    point.status === "protected"
      ? t("analytics.slot.protected")
      : t("analytics.slot.unavailable"),
  ];
}

/**
 * Uma descrição do dia, reaproveitada pelo tooltip e pela leitura viva, para
 * que ponteiro e leitor de tela nunca recebam fatos diferentes.
 */
function slotLinesOf(
  t: Translate,
  point: PresencePoint,
  average: number | null,
): string[] {
  const value = centralValue(point);
  if (value === null) {
    return withheldLine(t, point);
  }
  const kindKey =
    point.kind === "observed" ? "analytics.slot.observed" : "analytics.slot.forecast";
  return [
    t(kindKey, { value }),
    ...bandLine(t, point),
    ...deltaLine(t, value, average),
  ];
}

export interface PresenceFormat {
  count(value: number): string;
  date(value: string): string;
  dateTime(value: string): string;
  day(value: string): string;
  delta(percent: number): string;
  slotLines(point: PresencePoint, average: number | null): string[];
  trend(percent: number, size: number): string;
  weekdayName(weekday: number): string;
}

export function presenceFormat(tag: string, t: Translate): PresenceFormat {
  const formatters = formattersFor(tag);
  return {
    count: (value) => formatters.count.format(value),
    date: (value) => formatters.date.format(civilDate(value)),
    dateTime: (value) => formatters.dateTime.format(new Date(value)),
    day: (value) => formatters.day.format(civilDate(value)),
    delta: (percent) => deltaText(t, percent),
    slotLines: (point, average) => slotLinesOf(t, point, average),
    trend: (percent, size) => trendText(t, percent, size),
    weekdayName: (weekday) => formatters.weekday.format(weekdayDate(weekday)),
  };
}

export function usePresenceFormat(): PresenceFormat {
  const { t, tag } = useLocale();
  return useMemo(() => presenceFormat(tag, t), [t, tag]);
}
