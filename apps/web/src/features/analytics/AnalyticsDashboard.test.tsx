import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { components } from "../../generated/schema";
import { LocaleProvider } from "../../shared/i18n/LocaleProvider";
import type {
  AnalyticsClient,
  AnalyticsResult,
} from "../../shared/api/analytics-client";
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
const richPresence: Schemas["PublicPresence"] = {
  metadata,
  window: "recent_30_days",
  series: [
    { date: "2026-07-20", kind: "observed", status: "published", value: 100 },
    { date: "2026-07-21", kind: "observed", status: "published", value: 120 },
    { date: "2026-07-22", kind: "observed", status: "published", value: 80 },
    { date: "2026-07-23", kind: "observed", status: "protected" },
    { date: "2026-07-24", kind: "observed", status: "published", value: 140 },
    { date: "2026-07-25", kind: "observed", status: "published", value: 160 },
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
  presence_history_days: 730,
  allowed_presence_windows: [
    "recent_30_days",
    "recent_90_days",
    "recent_365_days",
    "recent_730_days",
    "next_30_days",
    "month",
  ],
  allowed_preference_periods: ["last_complete_month"],
};

function result<T>(data: T): Promise<AnalyticsResult<T>> {
  return Promise.resolve({
    data,
    etag: `"sha256-${"a".repeat(64)}"`,
    requestId: "request-dashboard-test",
  });
}

function client(
  overrides: Partial<AnalyticsClient> = {},
): AnalyticsClient {
  return {
    getSummary: vi.fn(() => result(summary)),
    getPresence: vi.fn((window) =>
      result(window === "next_30_days" ? forecastPresence : recentPresence),
    ),
    getPreferences: vi.fn(() => result(preferences)),
    getMethodology: vi.fn(() => result(methodology)),
    getQuality: vi.fn(),
    ...overrides,
  };
}

function renderDashboard(analyticsClient = client()) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <LocaleProvider initial="pt">
        <AnalyticsDashboard client={analyticsClient} />
      </LocaleProvider>
    </QueryClientProvider>,
  );
}

describe("dashboard público de analytics", () => {
  afterEach(cleanup);

  it("explica protótipo, cobertura, atualização e limites sem depender de cor", async () => {
    const { container } = renderDashboard();

    expect(
      await screen.findByRole("heading", { name: "Indicadores públicos" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("120 pessoas-dia")).toBeInTheDocument();
    expect(screen.getAllByText(/Faixa provável: 130 a 180/).length).toBe(2);
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
    const analyticsClient = client();
    renderDashboard(analyticsClient);
    await screen.findAllByText("110 pessoas-dia");

    await user.click(screen.getByRole("button", { name: "Só previsão" }));

    expect(
      (await screen.findAllByText("Estimativa central: 150 pessoas-dia"))
        .length,
    ).toBeGreaterThan(0);
    expect(screen.getAllByText("◇ Previsto").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Dado indisponível").length).toBeGreaterThan(0);
    expect(analyticsClient.getPresence).toHaveBeenCalledWith("next_30_days");
  });

  it("pede cada janela histórica pelo nome catalogado", async () => {
    const user = userEvent.setup();
    const analyticsClient = client();
    renderDashboard(analyticsClient);
    await screen.findAllByText("110 pessoas-dia");

    for (const [label, window] of [
      ["90 dias", "recent_90_days"],
      ["1 ano", "recent_365_days"],
      ["2 anos", "recent_730_days"],
    ] as const) {
      await user.click(screen.getByRole("button", { name: label }));
      expect(analyticsClient.getPresence).toHaveBeenCalledWith(
        window,
        undefined,
      );
    }
  });

  // O mês civil é a consulta histórica nominal: sem data escolhida ele não
  // nomeia documento nenhum, então o seletor abre no mês mais recente.
  it("navega mês a mês dentro do histórico publicado", async () => {
    const user = userEvent.setup();
    const analyticsClient = client();
    renderDashboard(analyticsClient);
    await screen.findAllByText("110 pessoas-dia");

    await user.click(screen.getByRole("button", { name: "Mês" }));
    expect(analyticsClient.getPresence).toHaveBeenCalledWith(
      "month",
      "2026-07",
    );

    await user.click(screen.getByRole("button", { name: "Mês anterior" }));
    expect(analyticsClient.getPresence).toHaveBeenCalledWith(
      "month",
      "2026-06",
    );

    // O histórico termina no dia de referência: não há mês seguinte a pedir.
    await user.click(screen.getByRole("button", { name: "Mês seguinte" }));
    expect(screen.getByRole("button", { name: "Mês seguinte" })).toBeDisabled();
    expect(analyticsClient.getPresence).not.toHaveBeenCalledWith(
      "month",
      "2026-08",
    );
  });

  it("deriva estatísticas da janela e explica cada indicador", async () => {
    renderDashboard(
      client({ getPresence: vi.fn(() => result(richPresence)) }),
    );

    const tiles = await screen.findByRole("list", {
      name: "Estatísticas dos dias observados da janela",
    });
    const average = within(tiles).getByText("Média diária").closest("li");
    expect(
      within(average as HTMLElement).getByText("120 pessoas-dia"),
    ).toBeInTheDocument();
    expect(within(tiles).getByText("160 pessoas-dia")).toBeInTheDocument();
    expect(within(tiles).getByText("80 pessoas-dia")).toBeInTheDocument();
    // Mediana de 100, 120, 80, 140 e 160: a média é a mesma 120, e é o valor
    // atípico que separaria as duas se a janela tivesse um.
    const median = within(tiles).getByText("Dia comum").closest("li");
    expect(
      within(median as HTMLElement).getByText("120 pessoas-dia"),
    ).toBeInTheDocument();

    const rhythm = screen.getByRole("list", {
      name: "Ritmo e alcance dos dias observados da janela",
    });
    expect(within(rhythm).getByText("600 pessoas-dia")).toBeInTheDocument();
    expect(
      within(rhythm).getByText("+50% ante os 2 dias anteriores"),
    ).toBeInTheDocument();
    expect(within(rhythm).getByText("5 de 6")).toBeInTheDocument();
    expect(
      within(rhythm).getByText(/Dias protegidos são pulados/),
    ).toBeInTheDocument();
    // Um dia suprimido torna o acumulado um piso, e o painel precisa dizê-lo.
    expect(
      within(rhythm).getByText(/Soma parcial: 1 dias da janela/),
    ).toBeInTheDocument();
  });

  it("junta observado e previsto sem misturar a referência dos indicadores", async () => {
    const user = userEvent.setup();
    renderDashboard();
    await screen.findAllByText("110 pessoas-dia");

    await user.click(screen.getByRole("button", { name: "Observado e previsto" }));

    expect(
      await screen.findByRole("img", { name: /Série de 4 dias/ }),
    ).toBeInTheDocument();
    const tiles = screen.getByRole("list", {
      name: "Estatísticas dos dias observados da janela",
    });
    // A média do observado é 110; se a referência escorregasse para a série
    // combinada, o previsto de 150 puxaria o valor para 130.
    const average = within(tiles).getByText("Média diária").closest("li");
    expect(average).not.toBeNull();
    expect(
      within(average as HTMLElement).getByText("110 pessoas-dia"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("list", { name: "Média por dia da semana" }),
    ).toBeInTheDocument();
  });

  it("permite ler cada dia do gráfico pelo teclado", async () => {
    const user = userEvent.setup();
    renderDashboard(
      client({ getPresence: vi.fn(() => result(richPresence)) }),
    );

    const chart = await screen.findByRole("img", {
      name: /Série de 6 dias em pessoas-dia/,
    });
    // A leitura viva vive dentro da figura do gráfico; o painel tem outra
    // região viva para o carregamento da janela.
    const figure = within(chart.closest("figure") as HTMLElement);
    chart.focus();
    await user.keyboard("{ArrowRight}");
    expect(figure.getByRole("status")).toHaveTextContent(
      "Observado: 100 pessoas-dia. 17% abaixo da média de referência.",
    );

    await user.keyboard("{End}");
    expect(figure.getByRole("status")).toHaveTextContent(
      "Observado: 160 pessoas-dia. 33% acima da média de referência.",
    );

    await user.keyboard("{Escape}");
    expect(figure.getByRole("status")).toHaveTextContent(
      "use as setas ← → do teclado",
    );
  });

  it("anuncia carregamento e oferece retry seguro após erro", async () => {
    let resolveSummary:
      | ((value: AnalyticsResult<Schemas["PublicSummary"]>) => void)
      | undefined;
    const pendingSummary = new Promise<
      AnalyticsResult<Schemas["PublicSummary"]>
    >((resolve) => {
      resolveSummary = resolve;
    });
    const getSummary = vi
      .fn<AnalyticsClient["getSummary"]>()
      .mockRejectedValueOnce(new Error("upstream"))
      .mockImplementationOnce(() => pendingSummary);
    const analyticsClient = client({ getSummary });
    const user = userEvent.setup();
    renderDashboard(analyticsClient);

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
