import { describe, expect, it } from "vitest";

import { chartBounds, polylinePoints } from "./performance-chart";
import type { PerformancePoint } from "./performance-summary";

function point(
  ownIndex?: number,
  villageIndex?: number,
): PerformancePoint {
  return {
    date: "2026-07-01",
    own_person_days: 1,
    ...(ownIndex === undefined ? {} : { own_index: ownIndex }),
    ...(villageIndex === undefined ? {} : { village_index: villageIndex }),
  };
}

describe("chartBounds", () => {
  it("shares one scale between both curves", () => {
    expect(chartBounds([point(100, 40), point(160, 90)])).toEqual({
      maximum: 160,
      minimum: 40,
    });
  });

  it("keeps a drawable span when every value is identical", () => {
    const bounds = chartBounds([point(100, 100)]);
    expect(bounds.maximum).toBeGreaterThan(bounds.minimum);
  });
});

describe("polylinePoints", () => {
  it("breaks the line where the village cell was suppressed", () => {
    const points = [point(100, 100), point(120), point(140, 130)];
    const segments = polylinePoints(
      points,
      (candidate) => candidate.village_index,
      chartBounds(points),
    );
    expect(segments).toHaveLength(2);
  });

  it("draws one continuous run when nothing is missing", () => {
    const points = [point(100, 100), point(120, 110)];
    const segments = polylinePoints(
      points,
      (candidate) => candidate.own_index,
      chartBounds(points),
    );
    expect(segments).toHaveLength(1);
  });
});
