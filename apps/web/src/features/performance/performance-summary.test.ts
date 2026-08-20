import { describe, expect, it } from "vitest";

import {
  comparativeTone,
  type PerformancePoint,
  summarize,
} from "./performance-summary";

function point(
  date: string,
  own: number,
  ownIndex?: number,
  villageIndex?: number,
): PerformancePoint {
  return {
    date,
    own_person_days: own,
    ...(ownIndex === undefined ? {} : { own_index: ownIndex }),
    ...(villageIndex === undefined ? {} : { village_index: villageIndex }),
  };
}

describe("summarize", () => {
  it("reads the change of each side from the last published index", () => {
    const summary = summarize([
      point("2026-07-01", 10, 100, 100),
      point("2026-07-02", 12, 120, 150),
    ]);
    expect(summary.ownChangePercent).toBe(20);
    expect(summary.villageChangePercent).toBe(50);
    expect(summary.ownPersonDays).toBe(22);
    expect(summary.days).toBe(2);
  });

  it("falls back to the last day the village survived suppression", () => {
    const summary = summarize([
      point("2026-07-01", 10, 100, 100),
      point("2026-07-02", 12, 120, 140),
      point("2026-07-03", 15, 150),
    ]);
    expect(summary.ownChangePercent).toBe(50);
    expect(summary.villageChangePercent).toBe(40);
  });

  it("keeps own totals when the comparison is closed", () => {
    const summary = summarize([point("2026-07-01", 9), point("2026-07-02", 11)]);
    expect(summary.ownPersonDays).toBe(20);
    expect(summary.ownChangePercent).toBeNull();
    expect(summary.villageChangePercent).toBeNull();
  });
});

describe("comparativeTone", () => {
  it("names who moved more, and stays silent without both sides", () => {
    expect(
      comparativeTone(summarize([point("d", 1, 100, 100), point("e", 1, 130, 110)])),
    ).toBe("ahead");
    expect(
      comparativeTone(summarize([point("d", 1, 100, 100), point("e", 1, 110, 130)])),
    ).toBe("behind");
    expect(
      comparativeTone(summarize([point("d", 1, 100, 100), point("e", 1, 120, 120)])),
    ).toBe("even");
    expect(comparativeTone(summarize([point("d", 1)]))).toBe("unavailable");
  });
});
