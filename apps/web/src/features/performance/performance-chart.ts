import type { PerformancePoint } from "./performance-summary";

export const CHART_WIDTH = 720;
export const CHART_HEIGHT = 180;

const PADDING = 8;
const DEFAULT_SPAN = 1;

interface Bounds {
  maximum: number;
  minimum: number;
}

function indexValues(
  points: readonly PerformancePoint[],
  read: (point: PerformancePoint) => number | undefined,
): number[] {
  const values: number[] = [];
  for (const point of points) {
    const value = read(point);
    if (value !== undefined) {
      values.push(value);
    }
  }
  return values;
}

/**
 * As duas curvas dividem a mesma escala. Escalas separadas fariam duas
 * variações diferentes parecerem iguais, que é justamente o que o painel existe
 * para não fazer.
 */
export function chartBounds(points: readonly PerformancePoint[]): Bounds {
  const values = [
    ...indexValues(points, (point) => point.own_index),
    ...indexValues(points, (point) => point.village_index),
  ];
  if (values.length === 0) {
    return { maximum: DEFAULT_SPAN, minimum: 0 };
  }
  const maximum = Math.max(...values);
  const minimum = Math.min(...values);
  return maximum === minimum
    ? { maximum: maximum + DEFAULT_SPAN, minimum }
    : { maximum, minimum };
}

function horizontal(index: number, count: number): number {
  if (count <= 1) {
    return PADDING;
  }
  const span = CHART_WIDTH - PADDING * 2;
  return PADDING + (index * span) / (count - 1);
}

function vertical(value: number, bounds: Bounds): number {
  const span = CHART_HEIGHT - PADDING * 2;
  const ratio = (value - bounds.minimum) / (bounds.maximum - bounds.minimum);
  return CHART_HEIGHT - PADDING - ratio * span;
}

/**
 * Um dia suprimido no lado da vila interrompe a linha em vez de ser interpolado:
 * ligar os dois vizinhos desenharia um valor que a publicação recusou.
 */
export function polylinePoints(
  points: readonly PerformancePoint[],
  read: (point: PerformancePoint) => number | undefined,
  bounds: Bounds,
): string[] {
  const segments: string[] = [];
  let current: string[] = [];
  points.forEach((point, index) => {
    const value = read(point);
    if (value === undefined) {
      if (current.length > 0) {
        segments.push(current.join(" "));
        current = [];
      }
      return;
    }
    const x = horizontal(index, points.length).toFixed(1);
    const y = vertical(value, bounds).toFixed(1);
    current.push(`${x},${y}`);
  });
  if (current.length > 0) {
    segments.push(current.join(" "));
  }
  return segments;
}
