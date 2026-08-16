import type { components } from "../../generated/schema";

type Schemas = components["schemas"];
type PresencePoint =
  | Schemas["ObservedPresencePoint"]
  | Schemas["ForecastPresencePoint"];

interface PresenceChartProps {
  series: readonly PresencePoint[];
}

const VIEW_WIDTH = 960;
const VIEW_HEIGHT = 260;
const PADDING_LEFT = 46;
const PADDING_RIGHT = 12;
const PADDING_TOP = 18;
const BASELINE = VIEW_HEIGHT - 34;
const GRID_STEPS = 4;
const PLOT_WIDTH = VIEW_WIDTH - PADDING_LEFT - PADDING_RIGHT;

const axisDateFormatter = new Intl.DateTimeFormat("pt-BR", {
  day: "2-digit",
  month: "2-digit",
  timeZone: "America/Bahia",
});

interface Plotted {
  point: PresencePoint;
  x: number;
  width: number;
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
  return series.map((point, index) => ({
    point,
    x: PADDING_LEFT + index * span,
    width: Math.max(span - 2, 1),
  }));
}

function axisDate(value: string) {
  return axisDateFormatter.format(new Date(`${value}T12:00:00-03:00`));
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

/** Only the ends of the window are labelled; a tick per day would be noise. */
function DateAxis({ slots }: { slots: readonly Plotted[] }) {
  const first = slots[0];
  const last = slots[slots.length - 1];
  if (first === undefined || last === undefined) {
    return null;
  }
  return (
    <g aria-hidden="true">
      <text className="chart-tick" x={PADDING_LEFT} y={BASELINE + 18}>
        {axisDate(first.point.date)}
      </text>
      <text
        className="chart-tick"
        x={VIEW_WIDTH - PADDING_RIGHT}
        y={BASELINE + 18}
        textAnchor="end"
      >
        {axisDate(last.point.date)}
      </text>
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

function summarize(series: readonly PresencePoint[]) {
  const published = series.filter((point) => point.status === "published");
  const withheld = series.length - published.length;
  if (published.length === 0) {
    return "Nenhum dia da janela tem valor publicável; toda a série está protegida ou indisponível.";
  }
  return `Série de ${series.length} dias em pessoas-dia. ${published.length} dias com valor publicado e ${withheld} protegidos ou indisponíveis, exibidos como falha na base do gráfico.`;
}

export function PresenceChart({ series }: PresenceChartProps) {
  const bound = upperBound(series);
  const slots = plot(series);

  return (
    <figure className="presence-chart">
      <svg
        viewBox={`0 0 ${VIEW_WIDTH} ${VIEW_HEIGHT}`}
        role="img"
        aria-label={summarize(series)}
      >
        <ValueGrid bound={bound} />
        {slots.map((slot) => (
          <g key={`${slot.point.date}-${slot.point.kind}`}>
            <ForecastBand bound={bound} slot={slot} />
            <ObservedBar bound={bound} slot={slot} />
            <ProtectedTick slot={slot} />
          </g>
        ))}
        <line
          className="chart-axis"
          x1={PADDING_LEFT}
          y1={BASELINE}
          x2={VIEW_WIDTH - PADDING_RIGHT}
          y2={BASELINE}
        />
        <DateAxis slots={slots} />
      </svg>
      <figcaption>
        Máximo da escala: {bound} pessoas-dia. Dias protegidos aparecem como
        falha, sem valor substituto.
      </figcaption>
    </figure>
  );
}
