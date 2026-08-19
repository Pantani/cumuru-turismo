import { cleanup, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { renderWithSession } from "../../test/session";
import type { Accommodation } from "../operator/stay-lifecycle";
import { PerformancePanel } from "./PerformancePanel";

const accommodationId = "018f4e59-7a2a-7b12-8fd7-5d2e8dc99b80";

const accommodation: Accommodation = {
  id: accommodationId,
  organization_id: accommodationId,
  name: "Pousada Farol Fictícia",
  category: "formal_lodging",
  status: "active",
  capacity: 12,
  version: 1,
  created_at: "2026-08-01T00:00:00Z",
  updated_at: "2026-08-01T00:00:00Z",
};

function payload(overrides: Record<string, unknown> = {}) {
  return {
    metadata: {
      period: {
        start: "2026-05-01",
        end: "2026-08-01",
        end_exclusive: true,
        time_zone: "America/Bahia",
      },
      unit: "person_day",
      data_mode: "prototype_fixtures",
      updated_at: "2026-08-01T03:00:00Z",
      privacy_policy_version: "prototype-v1",
      methodology_version: "explainable-baseline-v1",
      coverage: { status: "published", ratio: 65 },
    },
    window: "recent_90_days",
    comparison: { status: "available" },
    occupancy: { own_percent: 62, village_percent: 70 },
    series: [
      { date: "2026-07-01", own_person_days: 8, own_index: 100, village_index: 100 },
      { date: "2026-07-02", own_person_days: 12, own_index: 150, village_index: 120 },
    ],
    ...overrides,
  };
}

function stubApi(body: unknown) {
  const urls: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((input: Request) => {
      urls.push(input.url);
      const headers = new Headers({
        "Cache-Control": "no-store",
        "X-Request-ID": "request-performance-test",
      });
      return Promise.resolve(Response.json(body, { headers }));
    }),
  );
  return urls;
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("comparativo da hospedagem", () => {
  it("mostra a cobertura junto do número e compara as duas variações", async () => {
    stubApi(payload());
    renderWithSession(<PerformancePanel accommodation={accommodation} />);

    expect(await screen.findByText("Cobertura estimada: 65%")).toBeTruthy();
    expect(screen.getByText("62%")).toBeTruthy();
    expect(screen.getByText("70%")).toBeTruthy();
    expect(screen.getByText("+50%")).toBeTruthy();
    expect(screen.getByText("+20%")).toBeTruthy();
    expect(screen.getByText("Você cresceu mais que a vila nesta janela.")).toBeTruthy();
  });

  it("esconde a curva da vila quando o servidor fecha o comparativo", async () => {
    stubApi(
      payload({
        comparison: {
          status: "unavailable",
          reason: "few_reporting_accommodations",
        },
        // O servidor retira a taxa da vila junto com a curva; a própria fica.
        occupancy: { own_percent: 62 },
        series: [
          { date: "2026-07-01", own_person_days: 8 },
          { date: "2026-07-02", own_person_days: 12 },
        ],
      }),
    );
    const { container } = renderWithSession(
      <PerformancePanel accommodation={accommodation} />,
    );

    expect(
      await screen.findByText(/Poucas hospedagens reportaram nesta janela/u),
    ).toBeTruthy();
    expect(container.querySelector(".performance-curve-village")).toBeNull();
    expect(screen.getByText("62%")).toBeTruthy();
    expect(screen.queryByText("70%")).toBeNull();
    // O dado próprio continua inteiro: fechar a comparação não apaga o que é dela.
    expect(screen.getByText("20")).toBeTruthy();
  });

  it("relê a janela escolhida no mesmo catálogo fechado do contrato", async () => {
    const urls = stubApi(payload());
    renderWithSession(<PerformancePanel accommodation={accommodation} />);
    await screen.findByText("Cobertura estimada: 65%");

    await userEvent.click(screen.getByRole("button", { name: "2 anos" }));

    await waitFor(() => {
      expect(
        urls.some((url) => url.includes("window=recent_730_days")),
      ).toBe(true);
    });
  });
});
