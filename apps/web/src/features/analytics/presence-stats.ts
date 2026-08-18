import type { components } from "../../generated/schema";

type Schemas = components["schemas"];

export type PresencePoint =
  | Schemas["ObservedPresencePoint"]
  | Schemas["ForecastPresencePoint"];

export interface PublishedDay {
  date: string;
  value: number;
}

export interface SeriesStats {
  days: number;
  published: readonly PublishedDay[];
  withheld: number;
  total: number;
  average: number | null;
  median: number | null;
  peak: PublishedDay | null;
  trough: PublishedDay | null;
  trendPercent: number | null;
  trendSize: number;
  weekendLiftPercent: number | null;
  variationPercent: number | null;
}

/** Longest half the trend compares, so the reading stays about a recent week. */
const TREND_MAX_DAYS = 7;

/**
 * The single value a reader compares across days: a forecast contributes its
 * central estimate, and a protected day contributes nothing at all.
 */
export function centralValue(point: PresencePoint): number | null {
  if (point.status !== "published") {
    return null;
  }
  return point.kind === "observed" ? point.value : point.central;
}

export function publishedDays(
  series: readonly PresencePoint[],
): PublishedDay[] {
  return series.flatMap((point) => {
    const value = centralValue(point);
    return value === null ? [] : [{ date: point.date, value }];
  });
}

function averageOf(values: readonly number[]): number | null {
  if (values.length === 0) {
    return null;
  }
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

function mean(days: readonly PublishedDay[]): number | null {
  return averageOf(days.map((day) => day.value));
}

function extreme(
  days: readonly PublishedDay[],
  wins: (candidate: number, current: number) => boolean,
): PublishedDay | null {
  return days.reduce<PublishedDay | null>(
    (best, day) => (best === null || wins(day.value, best.value) ? day : best),
    null,
  );
}

/** Each half of the trend uses the same number of days, or the trend is absent. */
export function trendSize(published: number): number {
  return Math.min(Math.floor(published / 2), TREND_MAX_DAYS);
}

function growth(recent: number | null, previous: number | null): number | null {
  if (recent === null || previous === null || previous === 0) {
    return null;
  }
  return ((recent - previous) / previous) * 100;
}

/**
 * Compares the last `size` published days with the `size` published days before
 * them. Protected days are skipped rather than counted as zero, so a suppressed
 * cell never reads as a drop in presence.
 */
export function trendPercent(
  days: readonly PublishedDay[],
  size: number,
): number | null {
  if (size === 0) {
    return null;
  }
  return growth(mean(days.slice(-size)), mean(days.slice(-size * 2, -size)));
}

export function seriesStats(series: readonly PresencePoint[]): SeriesStats {
  const published = publishedDays(series);
  const size = trendSize(published.length);
  const average = mean(published);
  return {
    days: series.length,
    published,
    withheld: series.length - published.length,
    total: published.reduce((sum, day) => sum + day.value, 0),
    average,
    median: medianOf(published.map((day) => day.value)),
    peak: extreme(published, (candidate, current) => candidate > current),
    trough: extreme(published, (candidate, current) => candidate < current),
    trendPercent: trendPercent(published, size),
    trendSize: size,
    weekendLiftPercent: weekendLift(published),
    variationPercent: variation(published, average),
  };
}

/**
 * Mediana da janela. A média sozinha responde mal a um feriado: um único dia
 * atípico desloca a referência que todos os deltas usam, e a mediana diz qual
 * era o dia comum.
 */
function medianOf(values: readonly number[]): number | null {
  if (values.length === 0) {
    return null;
  }
  const ordered = [...values].sort((first, second) => first - second);
  const middle = Math.floor(ordered.length / 2);
  const lower = ordered[middle - 1];
  const upper = ordered[middle];
  if (upper === undefined) {
    return null;
  }
  return ordered.length % 2 === 1 || lower === undefined
    ? upper
    : (lower + upper) / 2;
}

/**
 * Quanto o fim de semana pesa acima — ou abaixo — do dia útil, em pontos
 * percentuais. É a métrica que decide escala de equipe e compra de estoque, e
 * ela não existe enquanto um dos dois lados não tiver nenhum dia publicado.
 */
function weekendLift(days: readonly PublishedDay[]): number | null {
  const weekend = mean(days.filter((day) => isWeekend(day.date)));
  const working = mean(days.filter((day) => !isWeekend(day.date)));
  return growth(weekend, working);
}

/**
 * Coeficiente de variação: o desvio-padrão como percentual da média. Uma
 * cidade com 20% varia pouco de um dia para o outro; com 80% a demanda é
 * imprevisível e a média engana quem planeja por ela.
 */
function variation(
  days: readonly PublishedDay[],
  average: number | null,
): number | null {
  if (average === null || average === 0 || days.length < 2) {
    return null;
  }
  const squares = days.map((day) => (day.value - average) ** 2);
  const deviation = Math.sqrt(
    squares.reduce((sum, value) => sum + value, 0) / days.length,
  );
  return (deviation / average) * 100;
}

export interface ForecastTotals {
  lower: number;
  central: number;
  upper: number;
  days: number;
}

/**
 * Soma da previsão publicada no horizonte, em pessoa-dia. É o número que uma
 * pousada ou um restaurante usa para dimensionar equipe e estoque, e o dia a
 * dia sozinho não o entrega. Dias protegidos não entram — e por isso `days`
 * diz sobre quantos a soma se apoia.
 */
export function forecastTotals(
  series: readonly PresencePoint[],
): ForecastTotals | null {
  const bands = series.flatMap((point) =>
    point.status === "published" && point.kind === "forecast" ? [point] : [],
  );
  if (bands.length === 0) {
    return null;
  }
  return {
    lower: bands.reduce((sum, point) => sum + point.lower, 0),
    central: bands.reduce((sum, point) => sum + point.central, 0),
    upper: bands.reduce((sum, point) => sum + point.upper, 0),
    days: bands.length,
  };
}

/** Civil weekday of a YYYY-MM-DD date, independent of the runtime time zone. */
export function weekdayOf(date: string): number {
  return new Date(`${date}T00:00:00Z`).getUTCDay();
}

export function isWeekend(date: string): boolean {
  const weekday = weekdayOf(date);
  return weekday === 0 || weekday === 6;
}

/**
 * Trailing mean over `size` days. It reports nothing until the window is
 * complete, and nothing when fewer than half its days carry a value: averaging
 * one surviving day would dress a suppressed stretch up as a measured level.
 */
function windowMean(
  values: readonly (number | null)[],
  index: number,
  size: number,
): number | null {
  if (index + 1 < size) {
    return null;
  }
  const slice = values.slice(index + 1 - size, index + 1);
  const known = slice.filter((value): value is number => value !== null);
  return known.length * 2 >= size ? averageOf(known) : null;
}

export function movingAverage(
  series: readonly PresencePoint[],
  size: number,
): (number | null)[] {
  const values = series.map(centralValue);
  return values.map((_, index) => windowMean(values, index, size));
}

export interface WeekdayAverage {
  average: number | null;
  days: number;
  weekday: number;
}

/** Average per civil weekday, so the weekly rhythm reads as a number. */
export function weekdayAverages(
  series: readonly PresencePoint[],
): WeekdayAverage[] {
  const buckets: number[][] = Array.from({ length: 7 }, () => []);
  for (const point of series) {
    const value = centralValue(point);
    if (value !== null) {
      buckets[weekdayOf(point.date)]?.push(value);
    }
  }
  return buckets.map((values, weekday) => ({
    average: averageOf(values),
    days: values.length,
    weekday,
  }));
}

export function percentFromAverage(
  value: number,
  average: number | null,
): number | null {
  if (average === null || average === 0) {
    return null;
  }
  return ((value - average) / average) * 100;
}
