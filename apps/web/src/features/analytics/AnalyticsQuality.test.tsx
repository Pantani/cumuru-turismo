import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { components } from "../../generated/schema";
import {
  Phase4ApiError,
  type Phase4Client,
  type Phase4Result,
} from "../../shared/api/phase4-client";
import { AnalyticsQuality } from "./AnalyticsQuality";

type QualitySnapshot = components["schemas"]["QualitySnapshot"];

const snapshot: QualitySnapshot = {
  window: "last_30_days",
  updated_at: "2026-07-28T12:00:00Z",
  incomplete_stays: { status: "available", value: 4 },
  overdue_planned_departures: { status: "available", value: 2 },
  silent_accommodations: { status: "available", value: 1 },
  aggregation_failures: { status: "available", value: 0 },
  suspected_duplicates: {
    status: "not_available",
    reason_code: "pseudonym_not_approved",
  },
  fnrh_failures: {
    status: "not_available",
    reason_code: "phase_not_implemented",
  },
  coverage_by_category: [
    { category_code: "pousada", status: "available", ratio: 0.75 },
    { category_code: "camping", status: "not_available" },
  ],
};

function qualityResult(): Promise<Phase4Result<QualitySnapshot>> {
  return Promise.resolve({
    data: snapshot,
    etag: null,
    requestId: "request-quality-test",
  });
}

function client(getQuality: Phase4Client["getQuality"]): Phase4Client {
  return {
    getSummary: vi.fn(),
    getPresence: vi.fn(),
    getPreferences: vi.fn(),
    getMethodology: vi.fn(),
    getQuality,
  };
}

function renderQuality(phase4Client: Phase4Client) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <AnalyticsQuality client={phase4Client} />
    </QueryClientProvider>,
  );
}

describe("painel interno agregado de qualidade", () => {
  afterEach(cleanup);

  it("exibe somente agregados e N/A honesto para lacunas da fase", async () => {
    const { container } = renderQuality(client(vi.fn(qualityResult)));

    expect(
      await screen.findByRole("heading", { name: "Resumo dos últimos 30 dias" }),
    ).toBeInTheDocument();
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(screen.getAllByText("N/A")).toHaveLength(3);
    expect(screen.getByText(/pseudônimo transversal/i)).toBeInTheDocument();
    expect(screen.getByText(/FNRH pertence à Fase 5/i)).toBeInTheDocument();
    expect(screen.getByText("75%")).toBeInTheDocument();
    expect(document.body.textContent).not.toMatch(
      /stay_id|visitor_id|accommodation_id/i,
    );

    const report = await axe.run(container, {
      rules: { "color-contrast": { enabled: false } },
    });
    expect(report.violations).toEqual([]);
  });

  it("nega o conteúdo para 403 e permite tentar novamente", async () => {
    const denied = new Phase4ApiError(
      403,
      {
        type: "about:blank",
        title: "Acesso negado",
        status: 403,
      },
      null,
    );
    const getQuality = vi
      .fn<Phase4Client["getQuality"]>()
      .mockRejectedValueOnce(denied)
      .mockImplementationOnce(qualityResult);
    const user = userEvent.setup();
    renderQuality(client(getQuality));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Sua sessão não possui o escopo analytics:read:internal.",
    );
    expect(screen.queryByText("Cadastros incompletos")).toBeNull();

    await user.click(screen.getByRole("button", { name: "Tentar novamente" }));
    expect(
      await screen.findByText("Cadastros incompletos"),
    ).toBeInTheDocument();
    expect(getQuality).toHaveBeenCalledTimes(2);
  });
});
