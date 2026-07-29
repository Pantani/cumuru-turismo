import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { components } from "../../generated/schema";
import type {
  Phase4Client,
  Phase4Result,
} from "../../shared/api/phase4-client";
import { AnalyticsDashboard } from "./AnalyticsDashboard";

type Schemas = components["schemas"];

const metadata: Schemas["PublicMetadata"] = {
  period: {
    start: "2026-07-01",
    end: "2026-07-31",
    end_exclusive: true,
    time_zone: "America/Bahia",
  },
  unit: "person_day",
  data_mode: "prototype_fixtures",
  updated_at: "2026-07-28T12:00:00Z",
  privacy_policy_version: "prototype-v1",
  methodology_version: "explainable-baseline-v1",
  coverage: { status: "published", ratio: 65 },
};
const summary: Schemas["PublicSummary"] = {
  metadata,
  presence_today: {
    date: "2026-07-28",
    kind: "observed",
    status: "published",
    value: 120,
  },
  forecast_peak_next_30_days: {
    date: "2026-08-02",
    kind: "forecast",
    status: "published",
    lower: 130,
    central: 150,
    upper: 180,
  },
};
const recentPresence: Schemas["PublicPresence"] = {
  metadata,
  window: "recent_30_days",
  series: [
    {
      date: "2026-07-27",
      kind: "observed",
      status: "published",
      value: 110,
    },
    {
      date: "2026-07-28",
      kind: "observed",
      status: "protected",
    },
  ],
};
const forecastPresence: Schemas["PublicPresence"] = {
  metadata: {
    ...metadata,
    period: { ...metadata.period, start: "2026-07-29", end: "2026-08-28" },
  },
  window: "next_30_days",
  series: [
    {
      date: "2026-08-02",
      kind: "forecast",
      status: "published",
      lower: 130,
      central: 150,
      upper: 180,
    },
    {
      date: "2026-08-03",
      kind: "forecast",
      status: "unavailable",
    },
  ],
};
const preferences: Schemas["PublicPreferences"] = {
  metadata: { ...metadata, unit: "survey_response" },
  period: "last_complete_month",
  metrics: [
    {
      metric_code: "first_visit_share",
      dimension_code: "visit_profile",
      categories: [
        {
          category_code: "first_visit",
          status: "published",
          share_percent: 60,
        },
        { category_code: "returning", status: "protected" },
      ],
    },
  ],
};
const methodology: Schemas["PublicMethodology"] = {
  metadata,
  presence_interval: "[arrival,departure)",
  time_zone: "America/Bahia",
  observed_definition_code: "checked_presence_through_as_of",
  forecast_definition_code: "explainable-baseline-v1",
  forecast_bounds_percent: [85, 115],
  forecast_fallback_bounds_percent: [70, 130],
  primary_threshold: 10,
  minimum_reporting_accommodations: 3,
  complementary_suppression: true,
  rounding_base: 10,
  rounding_mode: "stable-half-up",
  allowed_presence_windows: ["recent_30_days", "next_30_days"],
  allowed_preference_periods: ["last_complete_month"],
};

function result<T>(data: T): Promise<Phase4Result<T>> {
  return Promise.resolve({
    data,
    etag: `"sha256-${"a".repeat(64)}"`,
    requestId: "request-dashboard-test",
  });
}

function client(
  overrides: Partial<Phase4Client> = {},
): Phase4Client {
  return {
    getSummary: vi.fn(() => result(summary)),
    getPresence: vi.fn((window) =>
      result(window === "recent_30_days" ? recentPresence : forecastPresence),
    ),
    getPreferences: vi.fn(() => result(preferences)),
    getMethodology: vi.fn(() => result(methodology)),
    getQuality: vi.fn(),
    ...overrides,
  };
}

function renderDashboard(phase4Client = client()) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <AnalyticsDashboard client={phase4Client} />
    </QueryClientProvider>,
  );
}

describe("dashboard público da Fase 4", () => {
  afterEach(cleanup);

  it("explica protótipo, cobertura, atualização e limites sem depender de cor", async () => {
    const { container } = renderDashboard();

    expect(
      await screen.findByRole("heading", { name: "Indicadores públicos" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("120 pessoas-dia")).toBeInTheDocument();
    expect(screen.getByText(/Faixa provável: 130 a 180/)).toBeInTheDocument();
    expect(screen.getAllByText("Dado protegido").length).toBeGreaterThan(0);
    expect(screen.getByText("Cobertura estimada: 65%")).toBeInTheDocument();
    expect(
      screen.getAllByText(/dados fictícios de protótipo/i).length,
    ).toBeGreaterThan(0);
    expect(screen.getByText(/não representa um censo/i)).toBeInTheDocument();
    expect(
      screen.getByText(/faixa normal usa limites de 85% a 115%/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/fallback mais amplo, de 70% a 130%/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/não identifica qual faixa foi aplicada a cada ponto/i),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("table", { name: "Presença observada e prevista" }),
    ).toBeInTheDocument();
    expect(screen.getAllByText("● Observado").length).toBeGreaterThan(0);

    const report = await axe.run(container, {
      rules: { "color-contrast": { enabled: false } },
    });
    expect(report.violations).toEqual([]);
  });

  it("troca somente entre as janelas catalogadas e descreve forecast", async () => {
    const user = userEvent.setup();
    const phase4Client = client();
    renderDashboard(phase4Client);
    await screen.findByText("110 pessoas-dia");

    await user.selectOptions(
      screen.getByRole("combobox", { name: "Janela da presença" }),
      "next_30_days",
    );

    expect(
      (await screen.findAllByText("Estimativa central: 150 pessoas-dia"))
        .length,
    ).toBeGreaterThan(0);
    expect(screen.getAllByText("◇ Previsto").length).toBeGreaterThan(0);
    expect(screen.getByText("Dado indisponível")).toBeInTheDocument();
    expect(phase4Client.getPresence).toHaveBeenCalledWith("next_30_days");
  });

  it("anuncia carregamento e oferece retry seguro após erro", async () => {
    let resolveSummary:
      | ((value: Phase4Result<Schemas["PublicSummary"]>) => void)
      | undefined;
    const pendingSummary = new Promise<
      Phase4Result<Schemas["PublicSummary"]>
    >((resolve) => {
      resolveSummary = resolve;
    });
    const getSummary = vi
      .fn<Phase4Client["getSummary"]>()
      .mockRejectedValueOnce(new Error("upstream"))
      .mockImplementationOnce(() => pendingSummary);
    const phase4Client = client({ getSummary });
    const user = userEvent.setup();
    renderDashboard(phase4Client);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Não foi possível carregar os indicadores públicos.",
    );
    await user.click(screen.getByRole("button", { name: "Tentar novamente" }));
    expect(screen.getByRole("status")).toHaveTextContent(
      "Atualizando indicadores públicos",
    );

    resolveSummary?.({
      data: summary,
      etag: `"sha256-${"b".repeat(64)}"`,
      requestId: "request-dashboard-retry",
    });
    expect(await screen.findByText("120 pessoas-dia")).toBeInTheDocument();
    expect(getSummary).toHaveBeenCalledTimes(2);
  });
});
