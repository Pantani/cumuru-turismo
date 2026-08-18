import { describe, expect, it } from "vitest";

import { formatApprovalDeadline } from "./approval-vocabulary";

describe("prazo de aprovação", () => {
  it("nomeia o dia do prazo em America/Bahia, não em UTC", () => {
    // 23:00 do dia 18 na Bahia; UTC já virou para o dia 19.
    expect(formatApprovalDeadline("2026-08-19T02:00:00Z")).toBe(
      "Expira em 18 de ago. de 2026",
    );
  });

  it("mantém o dia quando o instante não cruza a virada", () => {
    expect(formatApprovalDeadline("2026-08-19T12:00:00Z")).toBe(
      "Expira em 19 de ago. de 2026",
    );
  });

  it("recusa prazo ausente ou ilegível", () => {
    expect(formatApprovalDeadline(null)).toBeNull();
    expect(formatApprovalDeadline("não é uma data")).toBeNull();
  });
});
