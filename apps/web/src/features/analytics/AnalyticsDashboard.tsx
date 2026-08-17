import { useQuery } from "@tanstack/react-query";
import { useState, type CSSProperties, type ReactNode } from "react";

import type { components } from "../../generated/schema";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import type { MessageKey, Translate } from "../../shared/i18n/translate";
import {
  phase4PublicClient,
  type Phase4Client,
} from "../../shared/api/phase4-client";
import { coverageText } from "./coverage";
import { usePresenceFormat, type PresenceFormat } from "./presence-format";
import { PresenceChart } from "./PresenceChart";
import { PUBLIC_STALE_TIME, usePublicSummary } from "./public-summary";
import {
  centralValue,
  percentFromAverage,
  seriesStats,
  weekdayAverages,
  type PresencePoint,
  type SeriesStats,
  type WeekdayAverage,
} from "./presence-stats";

type Schemas = components["schemas"];
type PresenceWindow = components["parameters"]["PresenceWindow"];
type ForecastPeak = Schemas["ForecastPeak"];
type Metadata = Schemas["PublicMetadata"];
type PreferenceCategory = Schemas["PreferenceCategory"];

/** Média móvel desenhada pelo gráfico; a legenda precisa citar o mesmo número. */
const SMOOTH_DAYS = 7;

interface AnalyticsDashboardProps {
  client?: Phase4Client;
}

/** Reúne o que todo painel interno precisa sem repetir dois hooks por bloco. */
interface Copy {
  format: PresenceFormat;
  t: Translate;
}

function MetadataPanel({
  copy,
  metadata,
}: {
  copy: Copy;
  metadata: Metadata;
}) {
  const { format, t } = copy;
  return (
    <dl className="analytics-metadata" aria-label={t("analytics.metadata.aria")}>
      <div>
        <dt>{t("analytics.metadata.updated")}</dt>
        <dd>{format.dateTime(metadata.updated_at)}</dd>
      </div>
      <div>
        <dt>{t("analytics.metadata.coverage")}</dt>
        <dd>{coverageText(t, metadata.coverage)}</dd>
      </div>
      <div>
        <dt>{t("analytics.metadata.unit")}</dt>
        <dd>
          {metadata.unit === "person_day"
            ? t("analytics.unit.personDay")
            : t("analytics.unit.surveyAnswer")}
        </dd>
      </div>
      <div>
        <dt>{t("analytics.metadata.mode")}</dt>
        <dd>{t("analytics.prototypeBadge")}</dd>
      </div>
    </dl>
  );
}

function kindLabel(t: Translate, kind: PresencePoint["kind"]) {
  return kind === "observed"
    ? t("analytics.kind.observed")
    : t("analytics.kind.forecast");
}

function protectedLabel(t: Translate, status: "protected" | "unavailable") {
  return status === "protected"
    ? t("analytics.state.protected")
    : t("analytics.state.unavailable");
}

function PointValue({
  point,
  t,
}: {
  point: PresencePoint | ForecastPeak;
  t: Translate;
}) {
  if (point.status !== "published") {
    return (
      <span className={`data-state data-state-${point.status}`}>
        {protectedLabel(t, point.status)}
      </span>
    );
  }
  if (point.kind === "observed") {
    return <strong>{t("analytics.value.observed", { value: point.value })}</strong>;
  }
  return (
    <span className="forecast-value">
      <strong>{t("analytics.value.central", { value: point.central })}</strong>
      <span>
        {t("analytics.value.band", { lower: point.lower, upper: point.upper })}
      </span>
    </span>
  );
}

function SummaryCards({
  copy,
  summary,
}: {
  copy: Copy;
  summary: Schemas["PublicSummary"];
}) {
  const { format, t } = copy;
  const peak = summary.forecast_peak_next_30_days;
  return (
    <section className="summary-grid" aria-labelledby="summary-title">
      <h3 id="summary-title" className="visually-hidden">
        {t("analytics.summary.aria")}
      </h3>
      <article className="summary-card" data-kind={summary.presence_today.kind}>
        <p className="metric-label">{t("analytics.summary.today")}</p>
        <p className="metric-kind">
          {kindLabel(t, summary.presence_today.kind)}
        </p>
        <PointValue point={summary.presence_today} t={t} />
        <p className="metric-hint">{t("analytics.summary.todayHint")}</p>
      </article>
      <article className="summary-card" data-kind="forecast">
        <p className="metric-label">{t("analytics.summary.peak")}</p>
        <p className="metric-kind">{t("analytics.kind.forecast")}</p>
        {"date" in peak ? (
          <time dateTime={peak.date}>{format.date(peak.date)}</time>
        ) : null}
        <PointValue point={peak} t={t} />
        <p className="metric-hint">{t("analytics.summary.peakHint")}</p>
      </article>
    </section>
  );
}

interface StatTile {
  hint: string;
  label: string;
  value: ReactNode;
}

/** "combined" is composed in the client; the contract still serves one window. */
type DisplayWindow = PresenceWindow | "combined";

/** Names the scope of the tiles, which is never the forecast half of a join. */
const WINDOW_SCOPES: Record<DisplayWindow, MessageKey> = {
  recent_30_days: "analytics.window.scope.recent",
  next_30_days: "analytics.window.scope.next",
  combined: "analytics.window.scope.combined",
};

function displayedSeries(
  window: DisplayWindow,
  observed: readonly PresencePoint[],
  predicted: readonly PresencePoint[],
): readonly PresencePoint[] {
  if (window === "recent_30_days") {
    return observed;
  }
  if (window === "next_30_days") {
    return predicted;
  }
  return [...observed, ...predicted];
}

/**
 * Reference the tiles, the average line and every delta are measured against.
 * A joined view keeps the observed level as the reference: the useful question
 * there is how the forecast sits against what was actually measured.
 */
function referenceSeries(
  window: DisplayWindow,
  observed: readonly PresencePoint[],
  predicted: readonly PresencePoint[],
): readonly PresencePoint[] {
  return window === "next_30_days" ? predicted : observed;
}

function DayValue({
  day,
  copy,
}: {
  day: { date: string; value: number };
  copy: Copy;
}) {
  return (
    <>
      <span>{copy.t("analytics.value.observed", { value: day.value })}</span>
      <time className="stat-when" dateTime={day.date}>
        {copy.format.date(day.date)}
      </time>
    </>
  );
}

function averageTile(copy: Copy, stats: SeriesStats): StatTile {
  const { t } = copy;
  return {
    label: t("analytics.tile.average"),
    value:
      stats.average === null
        ? t("analytics.empty")
        : t("analytics.value.observed", { value: Math.round(stats.average) }),
    hint: t("analytics.tile.averageHint", { count: stats.published.length }),
  };
}

function totalTile(copy: Copy, stats: SeriesStats): StatTile {
  const { format, t } = copy;
  return {
    label: t("analytics.tile.total"),
    value: t("analytics.value.observed", { value: format.count(stats.total) }),
    hint: t("analytics.tile.totalHint"),
  };
}

function peakTile(copy: Copy, stats: SeriesStats): StatTile {
  return {
    label: copy.t("analytics.tile.peak"),
    value:
      stats.peak === null ? (
        copy.t("analytics.empty")
      ) : (
        <DayValue day={stats.peak} copy={copy} />
      ),
    hint: copy.t("analytics.tile.peakHint"),
  };
}

function troughTile(copy: Copy, stats: SeriesStats): StatTile {
  return {
    label: copy.t("analytics.tile.trough"),
    value:
      stats.trough === null ? (
        copy.t("analytics.empty")
      ) : (
        <DayValue day={stats.trough} copy={copy} />
      ),
    hint: copy.t("analytics.tile.troughHint"),
  };
}

function trendTile(copy: Copy, stats: SeriesStats): StatTile {
  const { format, t } = copy;
  const size = stats.trendSize;
  return {
    label: t("analytics.tile.trend"),
    value:
      stats.trendPercent === null
        ? t("analytics.empty")
        : format.trend(stats.trendPercent, size),
    hint:
      stats.trendPercent === null
        ? t("analytics.tile.trendNone")
        : t("analytics.tile.trendHint", { count: size }),
  };
}

function withheldHint(t: Translate, withheld: number) {
  if (withheld === 0) {
    return t("analytics.withheld.none");
  }
  return withheld === 1
    ? t("analytics.withheld.one")
    : t("analytics.withheld.other", { count: withheld });
}

function publishedTile(copy: Copy, stats: SeriesStats): StatTile {
  const { t } = copy;
  return {
    label: t("analytics.tile.published"),
    value: t("analytics.tile.publishedValue", {
      days: stats.days,
      published: stats.published.length,
    }),
    hint: withheldHint(t, stats.withheld),
  };
}

function statTiles(copy: Copy, stats: SeriesStats): StatTile[] {
  return [
    averageTile(copy, stats),
    peakTile(copy, stats),
    troughTile(copy, stats),
    trendTile(copy, stats),
    totalTile(copy, stats),
    publishedTile(copy, stats),
  ];
}

function WindowStats({
  copy,
  stats,
  window,
}: {
  copy: Copy;
  stats: SeriesStats;
  window: DisplayWindow;
}) {
  const { t } = copy;
  return (
    <ul
      className="stat-grid"
      aria-label={t("analytics.stats.aria", {
        scope: t(WINDOW_SCOPES[window]),
      })}
    >
      {statTiles(copy, stats).map((tile) => (
        <li className="stat-tile" key={tile.label}>
          <p className="metric-label">{tile.label}</p>
          <p className="stat-value">{tile.value}</p>
          <p className="metric-hint">{tile.hint}</p>
        </li>
      ))}
    </ul>
  );
}

/** Distance from the window average, so a row is read without doing the math. */
function ComparisonCell({
  average,
  copy,
  point,
}: {
  average: number | null;
  copy: Copy;
  point: PresencePoint;
}) {
  const value = centralValue(point);
  const percent = value === null ? null : percentFromAverage(value, average);
  if (percent === null) {
    return <span className="stat-when">{copy.t("analytics.empty")}</span>;
  }
  return <span>{copy.format.delta(percent)}</span>;
}

/** The bar is proportional to the busiest weekday, filled through --share. */
function weekdayShare(entry: WeekdayAverage, busiest: number): CSSProperties {
  const share =
    entry.average === null || busiest === 0
      ? 0
      : (entry.average / busiest) * 100;
  return { "--share": share } as CSSProperties;
}

function WeekdayValue({
  entry,
  copy,
}: {
  entry: WeekdayAverage;
  copy: Copy;
}) {
  const { t } = copy;
  if (entry.average === null) {
    return <span className="stat-when">{t("analytics.weekday.none")}</span>;
  }
  return (
    <span>
      <strong>
        {t("analytics.value.observed", { value: Math.round(entry.average) })}
      </strong>
      <span className="stat-when">
        {entry.days === 1
          ? t("analytics.weekday.days.one")
          : t("analytics.weekday.days.other", { count: entry.days })}
      </span>
    </span>
  );
}

function WeekdayPattern({
  copy,
  series,
}: {
  copy: Copy;
  series: readonly PresencePoint[];
}) {
  const { format, t } = copy;
  const averages = weekdayAverages(series);
  const busiest = Math.max(...averages.map((entry) => entry.average ?? 0));
  return (
    <div className="weekday-pattern">
      <h4>{t("analytics.weekday.title")}</h4>
      <p className="metric-hint">{t("analytics.weekday.hint")}</p>
      <ul className="weekday-list" aria-label={t("analytics.weekday.aria")}>
        {averages.map((entry) => (
          <li key={entry.weekday} style={weekdayShare(entry, busiest)}>
            <span>{format.weekdayName(entry.weekday)}</span>
            <WeekdayValue entry={entry} copy={copy} />
          </li>
        ))}
      </ul>
    </div>
  );
}

function PresenceTable({
  copy,
  series,
  stats,
}: {
  copy: Copy;
  series: readonly PresencePoint[];
  stats: SeriesStats;
}) {
  const { format, t } = copy;
  return (
    <div className="table-scroll">
      <table aria-label={t("analytics.table.aria")}>
        <thead>
          <tr>
            <th scope="col">{t("analytics.table.date")}</th>
            <th scope="col">{t("analytics.table.kind")}</th>
            <th scope="col">{t("analytics.table.result")}</th>
            <th scope="col">{t("analytics.table.delta")}</th>
          </tr>
        </thead>
        <tbody>
          {series.map((point) => (
            <tr key={`${point.date}-${point.kind}`}>
              <th scope="row">
                <time dateTime={point.date}>{format.date(point.date)}</time>
              </th>
              <td>{kindLabel(t, point.kind)}</td>
              <td>
                <PointValue point={point} t={t} />
              </td>
              <td>
                <ComparisonCell
                  average={stats.average}
                  copy={copy}
                  point={point}
                />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function categoryLabel(
  t: Translate,
  category: PreferenceCategory["category_code"],
) {
  return category === "first_visit"
    ? t("analytics.preferences.firstVisit")
    : t("analytics.preferences.returning");
}

/**
 * The row draws its own proportional bar from --share. A protected category has
 * no share to draw, so it reports zero and the bar stays absent.
 */
function shareStyle(category: PreferenceCategory): CSSProperties {
  const share = category.status === "published" ? category.share_percent : 0;
  return { "--share": share } as CSSProperties;
}

function PreferenceValue({
  category,
  t,
}: {
  category: PreferenceCategory;
  t: Translate;
}) {
  if (category.status === "published") {
    return (
      <strong>
        {t("analytics.preferences.share", { percent: category.share_percent })}
      </strong>
    );
  }
  return (
    <span className={`data-state data-state-${category.status}`}>
      {protectedLabel(t, category.status)}
    </span>
  );
}

function PreferenceList({
  metric,
  t,
}: {
  metric: Schemas["PublicPreferences"]["metrics"][number] | undefined;
  t: Translate;
}) {
  if (metric === undefined) {
    return (
      <p className="data-state data-state-unavailable">
        {t("analytics.state.unavailable")}
      </p>
    );
  }
  return (
    <ul className="preference-list">
      {metric.categories.map((category) => (
        <li key={category.category_code} style={shareStyle(category)}>
          <span>{categoryLabel(t, category.category_code)}</span>
          <PreferenceValue category={category} t={t} />
        </li>
      ))}
    </ul>
  );
}

function Preferences({
  preferences,
  t,
}: {
  preferences: Schemas["PublicPreferences"];
  t: Translate;
}) {
  return (
    <section className="analytics-section" aria-labelledby="preferences-title">
      <div className="section-heading">
        <div>
          <p className="section-kicker">{t("analytics.preferences.kicker")}</p>
          <h3 id="preferences-title">{t("analytics.preferences.title")}</h3>
        </div>
        <label>
          {t("analytics.preferences.periodLabel")}
          <select value={preferences.period} disabled>
            <option value="last_complete_month">
              {t("analytics.preferences.lastCompleteMonth")}
            </option>
          </select>
        </label>
      </div>
      <p>{t("analytics.preferences.lead")}</p>
      <PreferenceList metric={preferences.metrics[0]} t={t} />
    </section>
  );
}

function forecastBody(t: Translate, methodology: Schemas["PublicMethodology"]) {
  return t("analytics.methodology.forecastBody", {
    fallbackHigh: methodology.forecast_fallback_bounds_percent[1],
    fallbackLow: methodology.forecast_fallback_bounds_percent[0],
    high: methodology.forecast_bounds_percent[1],
    low: methodology.forecast_bounds_percent[0],
  });
}

function Methodology({
  methodology,
  t,
}: {
  methodology: Schemas["PublicMethodology"];
  t: Translate;
}) {
  return (
    <section
      className="analytics-section methodology"
      aria-labelledby="methodology-title"
    >
      <p className="section-kicker">{t("analytics.methodology.kicker")}</p>
      <h3 id="methodology-title">{t("analytics.methodology.title")}</h3>
      <div className="methodology-grid">
        <article>
          <h4>{t("analytics.methodology.observed")}</h4>
          <p>{t("analytics.methodology.observedBody")}</p>
        </article>
        <article>
          <h4>{t("analytics.methodology.forecast")}</h4>
          <p>{forecastBody(t, methodology)}</p>
        </article>
        <article>
          <h4>{t("analytics.methodology.protection")}</h4>
          <p>
            {t("analytics.methodology.protectionBody", {
              accommodations: methodology.minimum_reporting_accommodations,
              rounding: methodology.rounding_base,
              threshold: methodology.primary_threshold,
            })}
          </p>
        </article>
        <article>
          <h4>{t("analytics.methodology.limits")}</h4>
          <p>{t("analytics.methodology.limitsBody")}</p>
        </article>
      </div>
    </section>
  );
}

type DashboardStage = "loading" | "failed";

interface QueryStage {
  isPending: boolean;
  isFetching: boolean;
  isError: boolean;
}

function dashboardStage(queries: readonly QueryStage[]): DashboardStage {
  return queries.some((query) => query.isError) ? "failed" : "loading";
}

interface DashboardPayloads<S, P, F, M> {
  summary: S;
  observed: P;
  predicted: P;
  preferences: F;
  methodology: M;
}

/**
 * Narrows every payload together so the render below reads them without a
 * per-panel undefined check. Both presence windows load up front: the combined
 * view needs the two at once, and each is a cacheable public document.
 */
function loadedPayloads<S, P, F, M>(
  parts: DashboardPayloads<
    S | undefined,
    P | undefined,
    F | undefined,
    M | undefined
  >,
): DashboardPayloads<S, P, F, M> | null {
  const complete = Object.values(parts).every((value) => value !== undefined);
  return complete ? (parts as DashboardPayloads<S, P, F, M>) : null;
}

function DashboardPlaceholder({
  onRetry,
  stage,
  t,
}: {
  onRetry: () => void;
  stage: DashboardStage;
  t: Translate;
}) {
  return (
    <section className="analytics-dashboard" aria-labelledby="analytics-title">
      <h2 id="analytics-title">{t("analytics.title")}</h2>
      {stage === "failed" ? (
        <div className="analytics-error" role="alert">
          <p>{t("analytics.error")}</p>
          <button type="button" onClick={onRetry}>
            {t("analytics.retry")}
          </button>
        </div>
      ) : (
        <p role="status" aria-live="polite">
          {t("analytics.loading")}
        </p>
      )}
    </section>
  );
}

function SeriesLegend({ t }: { t: Translate }) {
  return (
    <p className="legend" aria-label={t("analytics.legend.aria")}>
      <span className="legend-observed">{t("analytics.legend.observed")}</span>
      <span className="legend-forecast">{t("analytics.legend.forecast")}</span>
      <span className="legend-gap">{t("analytics.legend.gap")}</span>
      <span className="legend-average">{t("analytics.legend.average")}</span>
      <span className="legend-trend">
        {t("analytics.legend.trend", { days: SMOOTH_DAYS })}
      </span>
      <span className="legend-weekend">{t("analytics.legend.weekend")}</span>
    </p>
  );
}

interface PresenceSectionProps {
  displayed: readonly PresencePoint[];
  copy: Copy;
  observed: readonly PresencePoint[];
  onWindowChange: (next: DisplayWindow) => void;
  stats: SeriesStats;
  window: DisplayWindow;
}

function PresenceSection({
  displayed,
  copy,
  observed,
  onWindowChange,
  stats,
  window,
}: PresenceSectionProps) {
  const { t } = copy;
  return (
    <section className="analytics-section" aria-labelledby="presence-title">
      <div className="section-heading">
        <div>
          <p className="section-kicker">{t("analytics.presence.kicker")}</p>
          <h3 id="presence-title">{t("analytics.presence.title")}</h3>
        </div>
        <label>
          {t("analytics.window.label")}
          <select
            value={window}
            onChange={(event) =>
              onWindowChange(event.target.value as DisplayWindow)
            }
          >
            <option value="recent_30_days">{t("analytics.window.recent")}</option>
            <option value="next_30_days">{t("analytics.window.next")}</option>
            <option value="combined">{t("analytics.window.combined")}</option>
          </select>
        </label>
      </div>
      <SeriesLegend t={t} />
      <WindowStats copy={copy} stats={stats} window={window} />
      <PresenceChart series={displayed} stats={stats} />
      <WeekdayPattern copy={copy} series={observed} />
      <details className="series-details">
        <summary>{t("analytics.details")}</summary>
        <PresenceTable copy={copy} series={displayed} stats={stats} />
      </details>
    </section>
  );
}

function usePublicAnalytics(client: Phase4Client) {
  const summary = usePublicSummary(client);
  const presence = useQuery({
    queryKey: ["analytics", "public", "presence", "recent_30_days"],
    queryFn: () => client.getPresence("recent_30_days"),
    staleTime: PUBLIC_STALE_TIME,
  });
  const forecast = useQuery({
    queryKey: ["analytics", "public", "presence", "next_30_days"],
    queryFn: () => client.getPresence("next_30_days"),
    staleTime: PUBLIC_STALE_TIME,
  });
  const preferences = useQuery({
    queryKey: ["analytics", "public", "preferences", "last_complete_month"],
    queryFn: () => client.getPreferences(),
    staleTime: PUBLIC_STALE_TIME,
  });
  const methodology = useQuery({
    queryKey: ["analytics", "public", "methodology"],
    queryFn: () => client.getMethodology(),
    staleTime: PUBLIC_STALE_TIME,
  });
  return { forecast, methodology, preferences, presence, summary };
}

export function AnalyticsDashboard({
  client = phase4PublicClient,
}: AnalyticsDashboardProps) {
  const { t } = useLocale();
  const format = usePresenceFormat();
  const [window, setWindow] = useState<DisplayWindow>("recent_30_days");
  const sources = usePublicAnalytics(client);
  const queries = Object.values(sources);
  const copy: Copy = { format, t };

  function retry() {
    void Promise.all(queries.map((query) => query.refetch()));
  }

  const loaded = loadedPayloads({
    summary: sources.summary.data,
    observed: sources.presence.data,
    predicted: sources.forecast.data,
    preferences: sources.preferences.data,
    methodology: sources.methodology.data,
  });
  if (loaded === null) {
    return (
      <DashboardPlaceholder
        onRetry={retry}
        stage={dashboardStage(queries)}
        t={t}
      />
    );
  }
  const observed = loaded.observed.data.series;
  const predicted = loaded.predicted.data.series;
  const displayed = displayedSeries(window, observed, predicted);
  const stats = seriesStats(referenceSeries(window, observed, predicted));

  return (
    <section className="analytics-dashboard" aria-labelledby="analytics-title">
      <div className="dashboard-heading">
        <div>
          <p className="section-kicker">{t("analytics.kicker")}</p>
          <h2 id="analytics-title">{t("analytics.title")}</h2>
          <p>{t("analytics.lead")}</p>
        </div>
        <span className="prototype-badge">{t("analytics.prototypeBadge")}</span>
      </div>
      <MetadataPanel copy={copy} metadata={loaded.summary.data.metadata} />
      <SummaryCards copy={copy} summary={loaded.summary.data} />
      <PresenceSection
        displayed={displayed}
        copy={copy}
        observed={observed}
        onWindowChange={setWindow}
        stats={stats}
        window={window}
      />
      <Preferences preferences={loaded.preferences.data} t={t} />
      <Methodology methodology={loaded.methodology.data} t={t} />
    </section>
  );
}
