import { describe, expect, it } from "vitest";

import {
  clampMonth,
  isCivilMonth,
  monthRange,
  monthWithin,
  shiftMonth,
} from "./presence-months";

describe("aritmética de mês civil", () => {
  it("cobre o histórico publicado a partir do dia de referência", () => {
    // 730 dias contados a partir de 2026-07-15 alcançam 2024-07-17, ainda
    // dentro de julho de 2024: o mês mais antigo é oferecido parcial.
    expect(monthRange("2026-07-15", 730)).toEqual({
      max: "2026-07",
      min: "2024-07",
    });
    expect(monthRange("2026-01-01", 30)).toEqual({
      max: "2026-01",
      min: "2025-12",
    });
  });

  it("atravessa a virada do ano nos dois sentidos", () => {
    expect(shiftMonth("2026-01", -1)).toBe("2025-12");
    expect(shiftMonth("2025-12", 1)).toBe("2026-01");
    expect(shiftMonth("2026-07", -12)).toBe("2025-07");
  });

  it("prende a escolha ao histórico que existe", () => {
    const range = { max: "2026-07", min: "2024-07" };
    expect(clampMonth("2024-01", range)).toBe("2024-07");
    expect(clampMonth("2027-01", range)).toBe("2026-07");
    expect(clampMonth("2025-03", range)).toBe("2025-03");
    expect(monthWithin("2025-03", range)).toBe(true);
    expect(monthWithin("2027-01", range)).toBe(false);
  });

  // O campo `month` do navegador aceita digitação livre e pode entregar texto
  // vazio ou incompleto; nada disso nomeia um documento publicado.
  it("recusa qualquer coisa que não seja YYYY-MM", () => {
    for (const value of ["", "2026", "2026-13", "2026-00", "2026-7", "2026-07-01"]) {
      expect(isCivilMonth(value)).toBe(false);
    }
    expect(isCivilMonth("2026-07")).toBe(true);
  });
});
