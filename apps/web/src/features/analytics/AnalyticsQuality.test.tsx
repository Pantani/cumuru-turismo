import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { components } from "../../generated/schema";
import { ApiError } from "../../shared/api/http-client";
import { type AnalyticsClient, type AnalyticsResult } from "../../shared/api/analytics-client";
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
    reason_code: "not_implemented",
  },
  coverage_by_category: [
    { category_code: "formal_lodging", status: "available", ratio: 0.75 },
    { category_code: "camping", status: "not_available" },
  ],
};

function qualityResult(): Promise<AnalyticsResult<QualitySnapshot>> {
  return Promise.resolve({
    data: snapshot,
    etag: null,
    requestId: "request-quality-test",
  });
}

const funnel: components["schemas"]["AdoptionFunnel"] = {
  window: "last_30_days",
  as_of: "2026-08-19T03:00:00Z",
  invite: {
    issued: 40,
    submitted: 30,
    expired_unused: 8,
    revoked: 2,
    median_hours_to_submit: 5,
  },
  // Sem mediana: o servidor a retira abaixo de dez submissões na janela.
  survey: { issued: 30, answered: 9, declined: 4, expired_unanswered: 17 },
  self_registration: {
    started: 12,
    pending: 2,
    approved: 8,
    rejected: 1,
    expired: 1,
  },
};

function client(
  getQuality: AnalyticsClient["getQuality"],
  getFunnel: AnalyticsClient["getFunnel"] = () =>
    Promise.resolve({
      data: funnel,
      etag: null,
      requestId: "request-funnel-test",
    }),
): AnalyticsClient {
  return {
    getSummary: vi.fn(),
    getPresence: vi.fn(),
    getPreferences: vi.fn(),
    getMethodology: vi.fn(),
    getFunnel,
    getQuality,
  };
}

function renderQuality(analyticsClient: AnalyticsClient) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <AnalyticsQuality client={analyticsClient} />
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
    expect(
      screen.getByText(/FNRH ainda não foi implementada/i),
    ).toBeInTheDocument();
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
    const denied = new ApiError(
      403,
      {
        type: "about:blank",
        title: "Acesso negado",
        status: 403,
      },
      null,
    );
    const getQuality = vi
      .fn<AnalyticsClient["getQuality"]>()
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

  it("mostra o funil e respeita a mediana retirada pelo servidor", async () => {
    renderQuality(client(qualityResult));

    const section = within(
      (
        await screen.findByRole("heading", {
          name: "Funil de adesão nos últimos 30 dias",
        })
      ).closest("section") as HTMLElement,
    );
    // 30 de 40 convites submetidos; 9 de 30 pesquisas respondidas.
    expect(section.getByText("75%")).toBeTruthy();
    expect(section.getByText("30%")).toBeTruthy();
    expect(section.getByText("5 h")).toBeTruthy();
    // A pesquisa e o autocadastro não têm mediana publicável nesta janela.
    expect(section.getAllByText("—")).toHaveLength(2);
  });
});
