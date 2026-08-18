import { useMemo, useState, type KeyboardEvent } from "react";

import { useLocale } from "../../shared/i18n/LocaleProvider";
import type { Translate } from "../../shared/i18n/translate";
import { usePresenceFormat, type PresenceFormat } from "./presence-format";
import {
  isWeekend,
  movingAverage,
  type PresencePoint,
  type SeriesStats,
} from "./presence-stats";

interface PresenceChartProps {
  series: readonly PresencePoint[];
  stats: SeriesStats;
}

const VIEW_WIDTH = 960;
const VIEW_HEIGHT = 280;
const PADDING_LEFT = 46;
const PADDING_RIGHT = 12;
const PADDING_TOP = 18;
const BASELINE = VIEW_HEIGHT - 40;
const GRID_STEPS = 4;
const PLOT_WIDTH = VIEW_WIDTH - PADDING_LEFT - PADDING_RIGHT;
const AXIS_LABELS = 6;
/** Keeps the tooltip inside the frame when the active day sits at an edge. */
const TOOLTIP_MARGIN_PERCENT = 12;
/** A week of trailing days: shorter keeps the weekday noise it should remove. */
const SMOOTH_DAYS = 7;

interface Plotted {
  point: PresencePoint;
  x: number;
  width: number;
}

/**
 * Eixo horizontal. Dentro de um ano civil, dia e mês bastam e o ano só faria os
 * rótulos colidirem; atravessando o ano, omiti-lo faria "05/01" aparecer duas
 * vezes na mesma série sem dizer de qual ano se trata.
 */
function axisDateFormatter(tag: string, spansYears: boolean) {
  const formatter = new Intl.DateTimeFormat(tag, {
    ...(spansYears ? { month: "short", year: "2-digit" } : {
      day: "2-digit",
      month: "2-digit",
    }),
    timeZone: "America/Bahia",
  });
  return (value: string) => formatter.format(new Date(`${value}T12:00:00-03:00`));
}

/** Uma série que atravessa o ano civil precisa nomear o ano no eixo. */
function spansCalendarYears(series: readonly PresencePoint[]) {
  const first = series[0]?.date.slice(0, 4);
  const last = series[series.length - 1]?.date.slice(0, 4);
  return first !== undefined && first !== last;
}

/**
 * Abaixo de quatro pixels por dia a coluna vira um traço: o vão entre barras e
 * a faixa de fim de semana deixam de separar dias e passam a texturizar o
 * gráfico inteiro.
 */
const DENSE_SLOT_WIDTH = 4;

/**
 * A legenda só pode nomear o que o gráfico desenha. Numa janela longa a faixa
 * de fim de semana some, e citá-la mandaria o leitor procurar uma marca que
 * não existe.
 */
export function weekendBandsVisible(days: number): boolean {
  return PLOT_WIDTH / Math.max(days, 1) >= DENSE_SLOT_WIDTH;
}

const NICE_STEPS = [1, 2, 3, 4, 5, 6, 8, 10, 15, 20, 25, 30, 40, 50, 75, 100];

function highestPublished(series: readonly PresencePoint[]) {
  let highest = 0;
  for (const point of series) {
    if (point.status !== "published") {
      continue;
    }
    const candidate = point.kind === "observed" ? point.value : point.upper;
    highest = Math.max(highest, candidate);
  }
  return highest;
}

/** Rounds the gridline interval up to a value a reader can add in their head. */
function niceStep(rough: number) {
  const step = NICE_STEPS.find((candidate) => candidate >= rough);
  return step ?? Math.ceil(rough / 100) * 100;
}

/**
 * upperBound picks the scale from the widest published value, forecast range
 * included, so a forecast band is never clipped. Protected points contribute
 * nothing: they carry no value by design. A small headroom keeps a full-height
 * bar from reading as a clipped one, and the bound is a whole multiple of the
 * gridline interval so every axis label stays a round number.
 */
function upperBound(series: readonly PresencePoint[]) {
  const highest = highestPublished(series);
  if (highest === 0) {
    return GRID_STEPS * 5;
  }
  return niceStep((highest * 1.05) / GRID_STEPS) * GRID_STEPS;
}

function scaleY(value: number, bound: number) {
  const usable = BASELINE - PADDING_TOP;
  return BASELINE - (value / bound) * usable;
}

function plot(series: readonly PresencePoint[]): Plotted[] {
  const span = PLOT_WIDTH / Math.max(series.length, 1);
  const gap = span < DENSE_SLOT_WIDTH ? 0 : 2;
  return series.map((point, index) => ({
    point,
    x: PADDING_LEFT + index * span,
    width: Math.max(span - gap, 1),
  }));
}

function denseSlots(slots: readonly Plotted[]) {
  return (slots[0]?.width ?? 0) < DENSE_SLOT_WIDTH;
}

function centreOf(slot: Plotted) {
  return slot.x + slot.width / 2;
}

/** Weekends shaded behind the series: the weekly rhythm reads without a table. */
function WeekendBands({ slots }: { slots: readonly Plotted[] }) {
  if (denseSlots(slots)) {
    return null;
  }
  return (
    <g aria-hidden="true">
      {slots
        .filter((slot) => isWeekend(slot.point.date))
        .map((slot) => (
          <rect
            className="chart-weekend"
            key={slot.point.date}
            x={slot.x}
            y={PADDING_TOP}
            width={slot.width}
            height={BASELINE - PADDING_TOP}
          />
        ))}
    </g>
  );
}

/** Horizontal rules with their value, so a bar can be read without the table. */
function ValueGrid({ bound }: { bound: number }) {
  const steps = Array.from({ length: GRID_STEPS + 1 }, (_, index) => index);
  return (
    <g aria-hidden="true">
      {steps.map((step) => {
        const value = (bound / GRID_STEPS) * step;
        const y = scaleY(value, bound);
        return (
          <g key={step}>
            <line
              className="chart-grid"
              x1={PADDING_LEFT}
              y1={y}
              x2={VIEW_WIDTH - PADDING_RIGHT}
              y2={y}
            />
            <text className="chart-tick" x={PADDING_LEFT - 8} y={y + 4} textAnchor="end">
              {Math.round(value)}
            </text>
          </g>
        );
      })}
    </g>
  );
}

/** The window average as a reference every bar can be compared against. */
function AverageLine({
  average,
  bound,
}: {
  average: number | null;
  bound: number;
}) {
  const { t } = useLocale();
  if (average === null) {
    return null;
  }
  const y = scaleY(average, bound);
  return (
    <g aria-hidden="true">
      <line
        className="chart-average"
        x1={PADDING_LEFT}
        y1={y}
        x2={VIEW_WIDTH - PADDING_RIGHT}
        y2={y}
      />
      <text className="chart-average-label" x={PADDING_LEFT + 6} y={y - 6}>
        {t("analytics.chart.average", { value: Math.round(average) })}
      </text>
    </g>
  );
}

/** Roughly six labels: one tick per day would collide at any usable width. */
function DateAxis({ slots }: { slots: readonly Plotted[] }) {
  const { tag } = useLocale();
  const spansYears = spansCalendarYears(slots.map((slot) => slot.point));
  const axisDate = useMemo(
    () => axisDateFormatter(tag, spansYears),
    [spansYears, tag],
  );
  const step = Math.max(Math.ceil(slots.length / AXIS_LABELS), 1);
  return (
    <g aria-hidden="true">
      {slots
        .filter((_, index) => index % step === 0)
        .map((slot) => (
          <text
            className="chart-tick"
            key={slot.point.date}
            x={centreOf(slot)}
            y={BASELINE + 18}
            textAnchor="middle"
          >
            {axisDate(slot.point.date)}
          </text>
        ))}
    </g>
  );
}

function ObservedBar({ bound, slot }: { bound: number; slot: Plotted }) {
  if (slot.point.status !== "published" || slot.point.kind !== "observed") {
    return null;
  }
  const y = scaleY(slot.point.value, bound);
  return (
    <rect
      className="chart-observed"
      x={slot.x}
      y={y}
      width={slot.width}
      height={Math.max(BASELINE - y, 1)}
      rx={Math.min(slot.width / 2, 2)}
    />
  );
}

function ForecastBand({ bound, slot }: { bound: number; slot: Plotted }) {
  if (slot.point.status !== "published" || slot.point.kind === "observed") {
    return null;
  }
  const top = scaleY(slot.point.upper, bound);
  const bottom = scaleY(slot.point.lower, bound);
  const centre = scaleY(slot.point.central, bound);
  return (
    <>
      <rect
        className="chart-forecast-band"
        x={slot.x}
        y={top}
        width={slot.width}
        height={Math.max(bottom - top, 1)}
        rx={Math.min(slot.width / 2, 2)}
      />
      <rect
        className="chart-forecast-centre"
        x={slot.x}
        y={centre}
        width={slot.width}
        height={2}
      />
    </>
  );
}

/** A protected or unavailable day gets a baseline tick, never a substitute value. */
function ProtectedTick({ slot }: { slot: Plotted }) {
  if (slot.point.status === "published") {
    return null;
  }
  return (
    <rect
      className={`chart-gap chart-gap-${slot.point.status}`}
      x={slot.x}
      y={BASELINE - 4}
      width={slot.width}
      height={4}
    />
  );
}

type Run = [number, number][];

/**
 * Contiguous runs of smoothed values. A day without a smoothed value breaks the
 * line instead of being bridged: a drawn segment would imply a level nobody
 * measured.
 */
function runsOf(values: readonly (number | null)[]): Run[] {
  const runs: Run[] = [];
  let current: Run = [];
  values.forEach((value, index) => {
    if (value === null) {
      current = [];
      return;
    }
    if (current.length === 0) {
      runs.push(current);
    }
    current.push([index, value]);
  });
  return runs;
}

function runPoints(run: Run, slots: readonly Plotted[], bound: number) {
  return run
    .map(([index, value]) => {
      const slot = slots[index];
      return slot === undefined
        ? ""
        : `${centreOf(slot)},${scaleY(value, bound)}`;
    })
    .join(" ");
}

/** Seven-day trailing mean: the weekly level under the day-to-day noise. */
function TrendLine({
  bound,
  series,
  slots,
}: {
  bound: number;
  series: readonly PresencePoint[];
  slots: readonly Plotted[];
}) {
  const runs = runsOf(movingAverage(series, SMOOTH_DAYS));
  return (
    <g aria-hidden="true">
      {runs.map((run) => (
        <polyline
          className="chart-trend"
          key={run[0]?.[0]}
          points={runPoints(run, slots, bound)}
        />
      ))}
    </g>
  );
}

/** Boundary between the observed stretch and the forecast one, when both show. */
function TodayMarker({ slots }: { slots: readonly Plotted[] }) {
  const { t } = useLocale();
  const index = slots.findIndex((slot) => slot.point.kind === "forecast");
  const slot = index > 0 ? slots[index] : undefined;
  if (slot === undefined) {
    return null;
  }
  return (
    <g aria-hidden="true">
      <line
        className="chart-today"
        x1={slot.x}
        y1={PADDING_TOP - 8}
        x2={slot.x}
        y2={BASELINE}
      />
      <text className="chart-today-label" x={slot.x + 4} y={PADDING_TOP - 10}>
        {t("analytics.chart.forecastFrom")}
      </text>
    </g>
  );
}

function ActiveGuide({ slot }: { slot: Plotted | null }) {
  if (slot === null) {
    return null;
  }
  return (
    <g aria-hidden="true">
      <rect
        className="chart-active"
        x={slot.x}
        y={PADDING_TOP}
        width={slot.width}
        height={BASELINE - PADDING_TOP}
      />
      <line
        className="chart-active-line"
        x1={centreOf(slot)}
        y1={PADDING_TOP}
        x2={centreOf(slot)}
        y2={BASELINE}
      />
    </g>
  );
}

/** Full-height targets: a short bar stays as easy to point at as a tall one. */
function HoverTargets({
  slots,
  onEnter,
}: {
  slots: readonly Plotted[];
  onEnter: (index: number) => void;
}) {
  return (
    <g aria-hidden="true">
      {slots.map((slot, index) => (
        <rect
          className="chart-hit"
          key={slot.point.date}
          x={slot.x}
          y={PADDING_TOP}
          width={slot.width}
          height={BASELINE - PADDING_TOP}
          onMouseEnter={() => onEnter(index)}
        />
      ))}
    </g>
  );
}

function tooltipPercent(slot: Plotted) {
  const raw = (centreOf(slot) / VIEW_WIDTH) * 100;
  return Math.min(
    Math.max(raw, TOOLTIP_MARGIN_PERCENT),
    100 - TOOLTIP_MARGIN_PERCENT,
  );
}

/**
 * Decoration for pointer users: the same facts reach assistive technology
 * through the live readout below the chart.
 */
function ChartTooltip({
  average,
  format,
  slot,
}: {
  average: number | null;
  format: PresenceFormat;
  slot: Plotted | null;
}) {
  if (slot === null) {
    return null;
  }
  return (
    <div
      aria-hidden="true"
      className="chart-tooltip"
      style={{ left: `${tooltipPercent(slot)}%` }}
    >
      <strong>{format.day(slot.point.date)}</strong>
      {format.slotLines(slot.point, average).map((line) => (
        <span key={line}>{line}</span>
      ))}
    </div>
  );
}

function summarize(t: Translate, series: readonly PresencePoint[]) {
  const published = series.filter((point) => point.status === "published");
  const withheld = series.length - published.length;
  if (published.length === 0) {
    return t("analytics.chart.empty");
  }
  return t("analytics.chart.summary", {
    days: series.length,
    published: published.length,
    withheld,
  });
}

const KEY_STEPS: Record<string, number> = {
  ArrowRight: 1,
  ArrowLeft: -1,
};

function clampIndex(index: number, length: number) {
  return Math.min(Math.max(index, 0), length - 1);
}

/** Entering the series from the left starts at the first day, and vice versa. */
function startIndex(step: number, length: number) {
  return step > 0 ? -1 : length;
}

/** Returns null when the key is not a navigation key and the event stands. */
function movedIndex(key: string, current: number | null, length: number) {
  if (key === "Home") {
    return 0;
  }
  if (key === "End") {
    return length - 1;
  }
  const step = KEY_STEPS[key];
  if (step === undefined) {
    return null;
  }
  return clampIndex((current ?? startIndex(step, length)) + step, length);
}

function readout(
  t: Translate,
  format: PresenceFormat,
  slot: Plotted | null,
  average: number | null,
) {
  if (slot === null) {
    return t("analytics.chart.readoutIdle");
  }
  return t("analytics.chart.readoutDay", {
    day: format.day(slot.point.date),
    lines: format.slotLines(slot.point, average).join(". "),
  });
}

export function PresenceChart({ series, stats }: PresenceChartProps) {
  const { t } = useLocale();
  const format = usePresenceFormat();
  const [active, setActive] = useState<number | null>(null);
  const bound = upperBound(series);
  const slots = plot(series);
  const activeSlot = active === null ? null : (slots[active] ?? null);

  function handleKeyDown(event: KeyboardEvent<SVGSVGElement>) {
    if (event.key === "Escape") {
      setActive(null);
      return;
    }
    const index = movedIndex(event.key, active, slots.length);
    if (index === null) {
      return;
    }
    event.preventDefault();
    setActive(index);
  }

  return (
    <figure className="presence-chart">
      <div className="chart-frame">
        <svg
          aria-label={summarize(t, series)}
          onBlur={() => setActive(null)}
          onKeyDown={handleKeyDown}
          onMouseLeave={() => setActive(null)}
          role="img"
          tabIndex={0}
          viewBox={`0 0 ${VIEW_WIDTH} ${VIEW_HEIGHT}`}
        >
          <WeekendBands slots={slots} />
          <ValueGrid bound={bound} />
          <ActiveGuide slot={activeSlot} />
          {slots.map((slot) => (
            <g key={`${slot.point.date}-${slot.point.kind}`}>
              <ForecastBand bound={bound} slot={slot} />
              <ObservedBar bound={bound} slot={slot} />
              <ProtectedTick slot={slot} />
            </g>
          ))}
          <AverageLine average={stats.average} bound={bound} />
          <TrendLine bound={bound} series={series} slots={slots} />
          <TodayMarker slots={slots} />
          <line
            className="chart-axis"
            x1={PADDING_LEFT}
            y1={BASELINE}
            x2={VIEW_WIDTH - PADDING_RIGHT}
            y2={BASELINE}
          />
          <DateAxis slots={slots} />
          <HoverTargets slots={slots} onEnter={setActive} />
        </svg>
        <ChartTooltip
          average={stats.average}
          format={format}
          slot={activeSlot}
        />
      </div>
      <p className="chart-readout" role="status">
        {readout(t, format, activeSlot, stats.average)}
      </p>
      <figcaption>
        {t("analytics.chart.caption", { bound, days: SMOOTH_DAYS })}
      </figcaption>
    </figure>
  );
}
