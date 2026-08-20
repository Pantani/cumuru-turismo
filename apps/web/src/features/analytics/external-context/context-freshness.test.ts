import { describe, expect, it } from "vitest";

import {
  collectionAgeSeconds,
  freshnessOf,
  STALE_GRACE_SECONDS,
} from "./context-freshness";

const RETRIEVED_AT = "2026-08-18T09:00:00Z";
const RETRIEVED = Date.parse(RETRIEVED_AT);
const DECLARED_LAG = 10800;

describe("frescor da coleta externa", () => {
  it("mede a idade da coleta em segundos", () => {
    expect(collectionAgeSeconds(RETRIEVED_AT, RETRIEVED + 60_000)).toBe(60);
  });

  it("recusa instante ilegível em vez de devolver NaN", () => {
    expect(collectionAgeSeconds("ontem de manhã", RETRIEVED)).toBeNull();
    expect(freshnessOf("ontem de manhã", DECLARED_LAG, RETRIEVED)).toBe("stale");
  });

  it("é fresco dentro da defasagem declarada mais a folga do ciclo", () => {
    const limit = RETRIEVED + (DECLARED_LAG + STALE_GRACE_SECONDS) * 1000;

    expect(freshnessOf(RETRIEVED_AT, DECLARED_LAG, limit)).toBe("fresh");
    expect(freshnessOf(RETRIEVED_AT, DECLARED_LAG, limit + 1000)).toBe("stale");
  });

  it("não corrige, não interpola e não inventa valor: só classifica", () => {
    // Fonte sem defasagem declarada continua tendo a folga do ciclo, e nada
    // mais: o módulo devolve um rótulo, nunca um número servido ao leitor.
    expect(freshnessOf(RETRIEVED_AT, 0, RETRIEVED)).toBe("fresh");
    expect(
      freshnessOf(RETRIEVED_AT, 0, RETRIEVED + (STALE_GRACE_SECONDS + 1) * 1000),
    ).toBe("stale");
  });
});
