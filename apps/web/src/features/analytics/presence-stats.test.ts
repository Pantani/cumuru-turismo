import { describe, expect, it } from "vitest";

import {
  centralValue,
  isWeekend,
  movingAverage,
  percentFromAverage,
  seriesStats,
  weekdayAverages,
  type PresencePoint,
} from "./presence-stats";

function observed(date: string, value: number): PresencePoint {
  return { date, kind: "observed", status: "published", value };
}

function protectedDay(date: string): PresencePoint {
  return { date, kind: "observed", status: "protected" };
}

function forecast(date: string): PresencePoint {
  return {
    date,
    kind: "forecast",
    status: "published",
    lower: 90,
    central: 100,
    upper: 130,
  };
}

describe("estatísticas da série de presença", () => {
  it("usa a estimativa central da previsão e ignora dia protegido", () => {
    expect(centralValue(observed("2026-07-20", 100))).toBe(100);
    expect(centralValue(forecast("2026-07-21"))).toBe(100);
    expect(centralValue(protectedDay("2026-07-22"))).toBeNull();
  });

  it("resume total, média, extremos e dias retidos", () => {
    const stats = seriesStats([
      observed("2026-07-20", 100),
      observed("2026-07-21", 120),
      protectedDay("2026-07-22"),
      observed("2026-07-23", 80),
    ]);

    expect(stats.days).toBe(4);
    expect(stats.withheld).toBe(1);
    expect(stats.total).toBe(300);
    expect(stats.average).toBe(100);
    expect(stats.peak).toEqual({ date: "2026-07-21", value: 120 });
    expect(stats.trough).toEqual({ date: "2026-07-23", value: 80 });
  });

  it("compara metades iguais pulando dias protegidos", () => {
    const stats = seriesStats([
      observed("2026-07-20", 100),
      observed("2026-07-21", 100),
      protectedDay("2026-07-22"),
      observed("2026-07-23", 150),
      observed("2026-07-24", 150),
    ]);

    expect(stats.trendSize).toBe(2);
    expect(stats.trendPercent).toBe(50);
  });

  it("não inventa tendência nem média sem dado publicado", () => {
    const stats = seriesStats([protectedDay("2026-07-20")]);

    expect(stats.average).toBeNull();
    expect(stats.peak).toBeNull();
    expect(stats.trendPercent).toBeNull();
    expect(stats.trendSize).toBe(0);
  });

  it("classifica o fim de semana pela data civil, sem fuso do runtime", () => {
    expect(isWeekend("2026-07-25")).toBe(true);
    expect(isWeekend("2026-07-26")).toBe(true);
    expect(isWeekend("2026-07-27")).toBe(false);
  });

  it("só suaviza com a janela completa e com metade dos dias publicados", () => {
    const smoothed = movingAverage(
      [
        observed("2026-07-20", 100),
        observed("2026-07-21", 200),
        protectedDay("2026-07-22"),
        protectedDay("2026-07-23"),
        observed("2026-07-24", 60),
        protectedDay("2026-07-25"),
      ],
      4,
    );

    expect(smoothed.slice(0, 3)).toEqual([null, null, null]);
    expect(smoothed[3]).toBe(150);
    expect(smoothed[4]).toBe(130);
    expect(smoothed[5]).toBeNull();
  });

  it("média por dia da semana conta só os dias publicados", () => {
    const weekdays = weekdayAverages([
      observed("2026-07-25", 100),
      observed("2026-08-01", 200),
      protectedDay("2026-08-08"),
      observed("2026-07-27", 50),
    ]);

    expect(weekdays).toHaveLength(7);
    expect(weekdays[6]).toEqual({ average: 150, days: 2, weekday: 6 });
    expect(weekdays[1]).toEqual({ average: 50, days: 1, weekday: 1 });
    expect(weekdays[0]).toEqual({ average: null, days: 0, weekday: 0 });
  });

  it("mede a distância da média e recusa divisão por zero", () => {
    expect(percentFromAverage(120, 100)).toBe(20);
    expect(percentFromAverage(120, 0)).toBeNull();
    expect(percentFromAverage(120, null)).toBeNull();
  });
});
