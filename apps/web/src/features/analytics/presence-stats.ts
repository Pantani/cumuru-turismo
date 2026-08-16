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
  peak: PublishedDay | null;
  trough: PublishedDay | null;
  trendPercent: number | null;
  trendSize: number;
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

function mean(days: readonly PublishedDay[]): number | null {
  if (days.length === 0) {
    return null;
  }
  return days.reduce((sum, day) => sum + day.value, 0) / days.length;
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
  return {
    days: series.length,
    published,
    withheld: series.length - published.length,
    total: published.reduce((sum, day) => sum + day.value, 0),
    average: mean(published),
    peak: extreme(published, (candidate, current) => candidate > current),
    trough: extreme(published, (candidate, current) => candidate < current),
    trendPercent: trendPercent(published, size),
    trendSize: size,
  };
}

/** Civil weekday of a YYYY-MM-DD date, independent of the runtime time zone. */
export function isWeekend(date: string): boolean {
  const weekday = new Date(`${date}T00:00:00Z`).getUTCDay();
  return weekday === 0 || weekday === 6;
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
