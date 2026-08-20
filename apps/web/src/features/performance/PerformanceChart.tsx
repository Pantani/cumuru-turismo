import {
  CHART_HEIGHT,
  CHART_WIDTH,
  chartBounds,
  polylinePoints,
} from "./performance-chart";
import type { PerformancePoint } from "./performance-summary";

interface CurveProps {
  className: string;
  points: readonly PerformancePoint[];
  read: (point: PerformancePoint) => number | undefined;
}

function Curve({ className, points, read }: CurveProps) {
  const segments = polylinePoints(points, read, chartBounds(points));
  return (
    <>
      {segments.map((segment) => (
        <polyline className={className} key={segment} points={segment} />
      ))}
    </>
  );
}

export function PerformanceChart({
  label,
  points,
  showVillage,
}: {
  label: string;
  points: readonly PerformancePoint[];
  showVillage: boolean;
}) {
  return (
    <svg
      aria-label={label}
      className="performance-chart"
      preserveAspectRatio="none"
      role="img"
      viewBox={`0 0 ${CHART_WIDTH} ${CHART_HEIGHT}`}
    >
      {showVillage ? (
        <Curve
          className="performance-curve-village"
          points={points}
          read={(point) => point.village_index}
        />
      ) : null}
      <Curve
        className="performance-curve-own"
        points={points}
        read={(point) => point.own_index}
      />
    </svg>
  );
}
