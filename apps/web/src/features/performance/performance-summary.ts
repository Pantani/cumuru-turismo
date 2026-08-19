import type { components } from "../../generated/schema";

type Schemas = components["schemas"];
export type PerformancePoint = Schemas["OwnPresencePoint"];
export type PerformancePayload = Schemas["AccommodationPerformance"];

/**
 * O índice publicado pela API vale 100 no dia-base da janela, então a variação
 * de cada lado é a distância do último índice até 100. Ler assim evita repetir
 * no cliente a escolha da base, que é decisão do servidor.
 */
const INDEX_BASE = 100;

export interface PerformanceSummary {
  days: number;
  ownChangePercent: number | null;
  ownPersonDays: number;
  villageChangePercent: number | null;
}

function lastDefined(
  points: readonly PerformancePoint[],
  read: (point: PerformancePoint) => number | undefined,
): number | null {
  for (let index = points.length - 1; index >= 0; index -= 1) {
    const value = read(points[index] as PerformancePoint);
    if (value !== undefined) {
      return value;
    }
  }
  return null;
}

function changeFrom(index: number | null): number | null {
  return index === null ? null : index - INDEX_BASE;
}

export function summarize(
  points: readonly PerformancePoint[],
): PerformanceSummary {
  const ownPersonDays = points.reduce(
    (total, point) => total + point.own_person_days,
    0,
  );
  return {
    days: points.length,
    ownChangePercent: changeFrom(lastDefined(points, (point) => point.own_index)),
    ownPersonDays,
    villageChangePercent: changeFrom(
      lastDefined(points, (point) => point.village_index),
    ),
  };
}

/**
 * A leitura em uma frase: quem subiu mais desde o começo da janela. Sem os dois
 * lados não há comparação a narrar, e a frase simplesmente não aparece.
 */
export function comparativeTone(
  summary: PerformanceSummary,
): "ahead" | "behind" | "even" | "unavailable" {
  const { ownChangePercent, villageChangePercent } = summary;
  if (ownChangePercent === null || villageChangePercent === null) {
    return "unavailable";
  }
  if (ownChangePercent > villageChangePercent) {
    return "ahead";
  }
  return ownChangePercent < villageChangePercent ? "behind" : "even";
}
