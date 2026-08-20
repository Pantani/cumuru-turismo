import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useState, type CSSProperties, type ReactNode } from "react";

import type { components } from "../../generated/schema";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import type { MessageKey, Translate } from "../../shared/i18n/translate";
import {
  publicAnalyticsClient,
  type AnalyticsClient,
} from "../../shared/api/analytics-client";
import { coverageText } from "./coverage";
import { usePresenceFormat, type PresenceFormat } from "./presence-format";
import {
  clampMonth,
  monthRange,
  monthWithin,
  shiftMonth,
  type MonthRange,
} from "./presence-months";
import { PresenceChart, weekendBandsVisible } from "./PresenceChart";
import { PUBLIC_STALE_TIME, usePublicSummary } from "./public-summary";
import {
  centralValue,
  percentFromAverage,
  seriesStats,
  forecastTotals,
  weekdayAverages,
  type PresencePoint,
  type SeriesStats,
  type WeekdayAverage,
} from "./presence-stats";
import { contextCopyFor } from "./external-context/context-copy";
import { ExternalContextTab } from "./external-context/ExternalContextTab";

type Schemas = components["schemas"];
type PresenceWindow = components["parameters"]["PresenceWindow"];
type ForecastPeak = Schemas["ForecastPeak"];
type Metadata = Schemas["PublicMetadata"];
type PreferenceCategory = Schemas["PreferenceCategory"];

/** Média móvel desenhada pelo gráfico; a legenda precisa citar o mesmo número. */
const SMOOTH_DAYS = 7;

interface AnalyticsDashboardProps {
  client?: AnalyticsClient;
}

/**
 * Camada medida e camada copiada. Não compartilham eixo, escala nem legenda
 * porque não compartilham tela.
 */
const DASHBOARD_LAYERS = ["presence", "external"] as const;

type DashboardLayer = (typeof DASHBOARD_LAYERS)[number];

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

/**
 * O acumulado previsto para o horizonte. É a pergunta de quem dimensiona
 * equipe e estoque — "quantas pessoas-dia o mês inteiro traz" — e o dia a dia
 * sozinho obriga o leitor a somar trinta números de cabeça.
 */
function ForecastTotalCard({
  copy,
  predicted,
}: {
  copy: Copy;
  predicted: readonly PresencePoint[];
}) {
  const { format, t } = copy;
  const totals = forecastTotals(predicted);
  return (
    <article className="summary-card" data-kind="forecast">
      <p className="metric-label">{t("analytics.summary.forecastTotal")}</p>
      <p className="metric-kind">{t("analytics.kind.forecast")}</p>
      {totals === null ? (
        <span className="data-state data-state-unavailable">
          {t("analytics.state.unavailable")}
        </span>
      ) : (
        <span className="forecast-value">
          <strong>
            {t("analytics.value.central", {
              value: format.count(totals.central),
            })}
          </strong>
          <span>{format.band(totals.lower, totals.upper)}</span>
        </span>
      )}
      <p className="metric-hint">
        {totals === null
          ? t("analytics.summary.forecastTotalNone")
          : t("analytics.summary.forecastTotalHint", { count: totals.days })}
      </p>
    </article>
  );
}

function SummaryCards({
  copy,
  predicted,
  summary,
}: {
  copy: Copy;
  predicted: readonly PresencePoint[];
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
      <ForecastTotalCard copy={copy} predicted={predicted} />
    </section>
  );
}

interface StatTile {
  hint: string;
  label: string;
  value: ReactNode;
}

/** Janelas que recortam a série observada; o mês civil pede a data junto. */
type HistoryWindow = Exclude<PresenceWindow, "next_30_days">;

/**
 * O que o gráfico mostra, escolhido separadamente do quanto de histórico é
 * carregado. Antes as duas perguntas dividiam um `<select>` só, e por isso
 * "últimos 30 dias" e "com previsão" não podiam ser respondidas juntas para
 * nenhuma janela além de trinta dias.
 */
type SeriesView = "observed" | "combined" | "forecast";

interface HistorySelection {
  window: HistoryWindow;
  month: string;
}

const HISTORY_WINDOWS: readonly HistoryWindow[] = [
  "recent_30_days",
  "recent_90_days",
  "recent_365_days",
  "recent_730_days",
  "month",
];

const HISTORY_LABELS: Record<HistoryWindow, MessageKey> = {
  recent_30_days: "analytics.history.recent30",
  recent_90_days: "analytics.history.recent90",
  recent_365_days: "analytics.history.recent365",
  recent_730_days: "analytics.history.recent730",
  month: "analytics.history.month",
};

const SERIES_VIEWS: readonly SeriesView[] = ["observed", "combined", "forecast"];

const VIEW_LABELS: Record<SeriesView, MessageKey> = {
  observed: "analytics.view.observed",
  combined: "analytics.view.combined",
  forecast: "analytics.view.forecast",
};

/** Nomeia o escopo dos indicadores, que nunca é a metade prevista de uma junção. */
const VIEW_SCOPES: Record<SeriesView, MessageKey> = {
  observed: "analytics.window.scope.recent",
  combined: "analytics.window.scope.combined",
  forecast: "analytics.window.scope.next",
};

function displayedSeries(
  view: SeriesView,
  observed: readonly PresencePoint[],
  predicted: readonly PresencePoint[],
): readonly PresencePoint[] {
  if (view === "observed") {
    return observed;
  }
  if (view === "forecast") {
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
  view: SeriesView,
  observed: readonly PresencePoint[],
  predicted: readonly PresencePoint[],
): readonly PresencePoint[] {
  return view === "forecast" ? predicted : observed;
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
      <span>
        {copy.t("analytics.value.observed", {
          value: copy.format.count(day.value),
        })}
      </span>
      <time className="stat-when" dateTime={day.date}>
        {copy.format.date(day.date)}
      </time>
    </>
  );
}

function averageTile(copy: Copy, stats: SeriesStats): StatTile {
  const { format, t } = copy;
  return {
    label: t("analytics.tile.average"),
    value:
      stats.average === null
        ? t("analytics.empty")
        : t("analytics.value.observed", {
            value: format.count(Math.round(stats.average)),
          }),
    hint: t("analytics.tile.averageHint", { count: stats.published.length }),
  };
}

/**
 * O acumulado soma apenas os dias publicados. Quando algum dia foi suprimido a
 * soma é um piso, e dizê-lo é obrigatório: apresentar parcial como total é
 * exatamente o que a política de publicação proíbe.
 */
function totalTile(copy: Copy, stats: SeriesStats): StatTile {
  const { format, t } = copy;
  return {
    label: t("analytics.tile.total"),
    value: t("analytics.value.observed", { value: format.count(stats.total) }),
    hint:
      stats.withheld === 0
        ? t("analytics.tile.totalHint")
        : t("analytics.tile.totalPartialHint", { count: stats.withheld }),
  };
}

function medianTile(copy: Copy, stats: SeriesStats): StatTile {
  const { format, t } = copy;
  return {
    label: t("analytics.tile.median"),
    value:
      stats.median === null
        ? t("analytics.empty")
        : t("analytics.value.observed", {
            value: format.count(Math.round(stats.median)),
          }),
    hint: t("analytics.tile.medianHint"),
  };
}

function weekendTile(copy: Copy, stats: SeriesStats): StatTile {
  const { format, t } = copy;
  const lift = stats.weekendLiftPercent;
  return {
    label: t("analytics.tile.weekend"),
    value: lift === null ? t("analytics.empty") : format.signedPercent(lift),
    hint:
      lift === null
        ? t("analytics.tile.weekendNone")
        : t("analytics.tile.weekendHint"),
  };
}

function variationTile(copy: Copy, stats: SeriesStats): StatTile {
  const { format, t } = copy;
  const variation = stats.variationPercent;
  return {
    label: t("analytics.tile.variation"),
    value:
      variation === null ? t("analytics.empty") : format.plainPercent(variation),
    hint:
      variation === null
        ? t("analytics.tile.variationNone")
        : t("analytics.tile.variationHint"),
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

/** O nível da janela: o que um dia comum foi, e o quanto os extremos se afastam. */
function levelTiles(copy: Copy, stats: SeriesStats): StatTile[] {
  return [
    averageTile(copy, stats),
    medianTile(copy, stats),
    peakTile(copy, stats),
    troughTile(copy, stats),
  ];
}

/** O ritmo e o alcance: para onde a janela anda e sobre quanto ela se apoia. */
function rhythmTiles(copy: Copy, stats: SeriesStats): StatTile[] {
  return [
    trendTile(copy, stats),
    weekendTile(copy, stats),
    variationTile(copy, stats),
    totalTile(copy, stats),
    publishedTile(copy, stats),
  ];
}

function StatList({
  className,
  label,
  tiles,
}: {
  className: string;
  label: string;
  tiles: readonly StatTile[];
}) {
  return (
    <ul className={className} aria-label={label}>
      {tiles.map((tile) => (
        <li className="stat-tile" key={tile.label}>
          <p className="metric-label">{tile.label}</p>
          <p className="stat-value">{tile.value}</p>
          <p className="metric-hint">{tile.hint}</p>
        </li>
      ))}
    </ul>
  );
}

/**
 * Duas listas em vez de uma malha de nove: com tudo no mesmo peso, "quantos
 * dias foram publicados" competia visualmente com "quantas pessoas por dia".
 */
function WindowStats({
  copy,
  stats,
  view,
}: {
  copy: Copy;
  stats: SeriesStats;
  view: SeriesView;
}) {
  const { t } = copy;
  const scope = t(VIEW_SCOPES[view]);
  return (
    <>
      <StatList
        className="stat-grid"
        label={t("analytics.stats.aria", { scope })}
        tiles={levelTiles(copy, stats)}
      />
      <StatList
        className="stat-strip"
        label={t("analytics.stats.rhythmAria", { scope })}
        tiles={rhythmTiles(copy, stats)}
      />
    </>
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
  const { format, t } = copy;
  if (entry.average === null) {
    return <span className="stat-when">{t("analytics.weekday.none")}</span>;
  }
  return (
    <span>
      <strong>
        {t("analytics.value.observed", {
          value: format.count(Math.round(entry.average)),
        })}
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

function SeriesLegend({ days, t }: { days: number; t: Translate }) {
  return (
    <p className="legend" aria-label={t("analytics.legend.aria")}>
      <span className="legend-observed">{t("analytics.legend.observed")}</span>
      <span className="legend-forecast">{t("analytics.legend.forecast")}</span>
      <span className="legend-gap">{t("analytics.legend.gap")}</span>
      <span className="legend-average">{t("analytics.legend.average")}</span>
      <span className="legend-trend">
        {t("analytics.legend.trend", { days: SMOOTH_DAYS })}
      </span>
      {weekendBandsVisible(days) ? (
        <span className="legend-weekend">{t("analytics.legend.weekend")}</span>
      ) : null}
    </p>
  );
}

/**
 * Controle segmentado em vez de `<select>`: as opções são poucas, fixas e
 * comparáveis entre si, e um menu fechado escondia justamente a informação de
 * que existe mais de trinta dias de histórico para pedir.
 */
function SegmentedControl<Option extends string>({
  label,
  onChange,
  options,
  optionLabel,
  value,
}: {
  label: string;
  onChange: (next: Option) => void;
  options: readonly Option[];
  optionLabel: (option: Option) => string;
  value: Option;
}) {
  return (
    <div className="segmented" role="group" aria-label={label}>
      {options.map((option) => (
        <button
          aria-pressed={option === value}
          className="segment"
          key={option}
          onClick={() => onChange(option)}
          type="button"
        >
          {optionLabel(option)}
        </button>
      ))}
    </div>
  );
}

function MonthPicker({
  copy,
  month,
  onChange,
  range,
}: {
  copy: Copy;
  month: string;
  onChange: (next: string) => void;
  range: MonthRange;
}) {
  const { t } = copy;
  const step = (offset: number) =>
    onChange(clampMonth(shiftMonth(month, offset), range));
  return (
    <div className="month-picker">
      <button
        type="button"
        className="month-step"
        disabled={month === range.min}
        onClick={() => step(-1)}
        aria-label={t("analytics.history.monthPrevious")}
      >
        ‹
      </button>
      <label className="month-field">
        <span className="visually-hidden">{t("analytics.history.monthLabel")}</span>
        <input
          type="month"
          value={month}
          min={range.min}
          max={range.max}
          onChange={(event) =>
            monthWithin(event.target.value, range) &&
            onChange(event.target.value)
          }
        />
      </label>
      <button
        type="button"
        className="month-step"
        disabled={month === range.max}
        onClick={() => step(1)}
        aria-label={t("analytics.history.monthNext")}
      >
        ›
      </button>
    </div>
  );
}

/**
 * Escolher o mês civil sem data é escolher nada. A data mais recente do
 * histórico é o padrão porque é a que o leitor acabou de ver nas outras
 * janelas.
 */
function nextHistory(
  current: HistorySelection,
  window: HistoryWindow,
  range: MonthRange,
): HistorySelection {
  if (window !== "month") {
    return { ...current, window };
  }
  return { window, month: current.month === "" ? range.max : current.month };
}

interface PeriodControlsProps {
  copy: Copy;
  history: HistorySelection;
  onHistoryChange: (next: HistorySelection) => void;
  onViewChange: (next: SeriesView) => void;
  range: MonthRange;
  view: SeriesView;
}

/**
 * Duas perguntas separadas: quanto histórico carregar e o que desenhar. Juntas
 * num controle só, escolher "com previsão" custava voltar para trinta dias.
 */
function PeriodControls({
  copy,
  history,
  onHistoryChange,
  onViewChange,
  range,
  view,
}: PeriodControlsProps) {
  const { t } = copy;
  return (
    <div className="period-controls">
      <div className="period-field">
        <span className="period-label">{t("analytics.history.label")}</span>
        <SegmentedControl
          label={t("analytics.history.label")}
          onChange={(window) => onHistoryChange(nextHistory(history, window, range))}
          options={HISTORY_WINDOWS}
          optionLabel={(window) => t(HISTORY_LABELS[window])}
          value={history.window}
        />
        {history.window === "month" ? (
          <MonthPicker
            copy={copy}
            month={history.month}
            onChange={(month) => onHistoryChange({ ...history, month })}
            range={range}
          />
        ) : null}
      </div>
      <div className="period-field">
        <span className="period-label">{t("analytics.view.label")}</span>
        <SegmentedControl
          label={t("analytics.view.label")}
          onChange={onViewChange}
          options={SERIES_VIEWS}
          optionLabel={(option) => t(VIEW_LABELS[option])}
          value={view}
        />
      </div>
    </div>
  );
}

interface PresenceSectionProps {
  controls: PeriodControlsProps;
  displayed: readonly PresencePoint[];
  copy: Copy;
  observed: readonly PresencePoint[];
  stale: boolean;
  stats: SeriesStats;
  view: SeriesView;
}

function PresenceSection({
  controls,
  displayed,
  copy,
  observed,
  stale,
  stats,
  view,
}: PresenceSectionProps) {
  const { t } = copy;
  return (
    <section className="analytics-section" aria-labelledby="presence-title">
      <div className="section-heading">
        <div>
          <p className="section-kicker">{t("analytics.presence.kicker")}</p>
          <h3 id="presence-title">{t("analytics.presence.title")}</h3>
        </div>
        <p className="series-status" role="status" aria-live="polite">
          {stale ? t("analytics.updating") : ""}
        </p>
      </div>
      <PeriodControls {...controls} />
      <SeriesLegend days={displayed.length} t={t} />
      <WindowStats copy={copy} stats={stats} view={view} />
      <div className="presence-panels">
        <PresenceChart series={displayed} stats={stats} />
        <WeekdayPattern copy={copy} series={observed} />
      </div>
      <details className="series-details">
        <summary>{t("analytics.details")}</summary>
        <PresenceTable copy={copy} series={displayed} stats={stats} />
      </details>
    </section>
  );
}

function usePublicAnalytics(client: AnalyticsClient, history: HistorySelection) {
  const summary = usePublicSummary(client);
  const presence = useQuery({
    queryKey: ["analytics", "public", "presence", history.window, history.month],
    queryFn: () =>
      client.getPresence(history.window, history.month || undefined),
    staleTime: PUBLIC_STALE_TIME,
    // Um mês sem data escolhida não nomeia documento; o seletor preenche a
    // data assim que a metodologia diz qual histórico existe.
    enabled: history.window !== "month" || history.month !== "",
    // A janela anterior fica na tela enquanto a nova carrega: apagar o painel
    // inteiro a cada troca faria "ver o ano passado" piscar a página.
    placeholderData: keepPreviousData,
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

function PresencePanel({ client }: { client: AnalyticsClient }) {
  const { t } = useLocale();
  const format = usePresenceFormat();
  const [history, setHistory] = useState<HistorySelection>({
    window: "recent_30_days",
    month: "",
  });
  const [view, setView] = useState<SeriesView>("observed");
  const sources = usePublicAnalytics(client, history);
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
  const methodology = loaded.methodology.data;
  const range = monthRange(
    methodology.metadata.period.start,
    methodology.presence_history_days,
  );
  const observed = loaded.observed.data.series;
  const predicted = loaded.predicted.data.series;
  const displayed = displayedSeries(view, observed, predicted);
  const stats = seriesStats(referenceSeries(view, observed, predicted));

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
      <SummaryCards
        copy={copy}
        predicted={predicted}
        summary={loaded.summary.data}
      />
      <PresenceSection
        controls={{
          copy,
          history,
          onHistoryChange: setHistory,
          onViewChange: setView,
          range,
          view,
        }}
        displayed={displayed}
        copy={copy}
        observed={observed}
        stale={sources.presence.isFetching}
        stats={stats}
        view={view}
      />
      <Preferences preferences={loaded.preferences.data} t={t} />
      <Methodology methodology={methodology} t={t} />
    </section>
  );
}

/**
 * As duas camadas do painel, e só uma na tela por vez.
 *
 * A aba é o registro da camada externa neste arquivo, e é tudo o que ele sabe
 * dela: o card mora em `external-context/`, com cliente, allowlist e texto
 * próprios. Alternar em vez de empilhar não é preferência de layout — dois
 * gráficos visíveis ao mesmo tempo produzem leitura causal mesmo sem nenhuma
 * frase de correlação, e nenhum aviso desfaz isso (ADR-045 §2 e §4).
 *
 * A escolha fica acima do carregamento da série medida de propósito: analytics
 * fora do ar não pode derrubar a camada externa, e fonte externa morta não
 * atrasa a presença (ADR-045 §3).
 */
export function AnalyticsDashboard({
  client = publicAnalyticsClient,
}: AnalyticsDashboardProps) {
  const { locale, t } = useLocale();
  const [layer, setLayer] = useState<DashboardLayer>("presence");
  const contextCopy = contextCopyFor(locale);
  return (
    <div className="analytics-layers">
      <SegmentedControl
        label={contextCopy.tabsLabel}
        onChange={setLayer}
        options={DASHBOARD_LAYERS}
        optionLabel={(option) =>
          option === "presence" ? t("analytics.title") : contextCopy.tabLabel
        }
        value={layer}
      />
      {layer === "presence" ? (
        <PresencePanel client={client} />
      ) : (
        <ExternalContextTab />
      )}
    </div>
  );
}
