import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, describe, expect, it, vi } from "vitest";

import { LocaleProvider } from "../../../shared/i18n/LocaleProvider";
import { AnalyticsDashboard } from "../AnalyticsDashboard";
import { ExternalContextTab } from "./ExternalContextTab";
import type { ContextClient, ContextResult } from "./context-client";
import {
  CADASTUR_ATTRIBUTION,
  contextDocument,
  FRESH_NOW,
  OPEN_METEO_ATTRIBUTION,
  STALE_NOW,
  WEATHER_OBSERVED_AT,
  WEATHER_RETRIEVED_AT,
} from "./context-fixtures";

const DIGIT = /\d/u;

function result(): ContextResult {
  return {
    data: contextDocument,
    etag: '"sha256-'.concat("0".repeat(64), '"'),
    requestId: "01JCONTEXT0000000000000000",
  };
}

function contextClient(): ContextClient {
  return { getContext: vi.fn().mockResolvedValue(result()) };
}

function renderTab(client: ContextClient = contextClient(), now = FRESH_NOW) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <LocaleProvider initial="pt">
        <ExternalContextTab client={client} now={now} />
      </LocaleProvider>
    </QueryClientProvider>,
  );
}

function requireElement(selector: string): HTMLElement {
  const node = document.querySelector<HTMLElement>(selector);
  if (node === null) {
    throw new Error(`Elemento ${selector} ausente.`);
  }
  return node;
}

function cardByCode(code: string): HTMLElement {
  return requireElement(`[data-card-code="${code}"]`);
}

function textOf(root: HTMLElement, selector: string): string {
  return root.querySelector(selector)?.textContent ?? "";
}

/**
 * O texto do card sem o bloco de proveniência.
 *
 * A proveniência é obrigatória inclusive no ramo indisponível, e ela carrega
 * datas e rótulo de licença — dígitos que a ADR-045 §7 exige na tela. A
 * proibição de numeral incide sobre o que o leitor lê como medida: título,
 * estado, motivo e o lugar do valor. Separar as duas regiões é o que permite
 * exigir as duas coisas ao mesmo tempo, em vez de escolher uma.
 */
function textOutsideProvenance(card: HTMLElement): string {
  const clone = card.cloneNode(true) as HTMLElement;
  for (const block of clone.querySelectorAll("[data-card-provenance]")) {
    block.remove();
  }
  return clone.textContent ?? "";
}

function valueText(card: HTMLElement): string {
  const slot = card.querySelector("[data-card-value]");
  return slot?.textContent ?? "";
}

describe("aba de contexto externo", () => {
  afterEach(cleanup);

  it("declara que é contexto externo e não afirma relação com a presença", async () => {
    renderTab();

    expect(
      await screen.findByRole("heading", {
        name: "Contexto externo, fora da metodologia do Observatório",
      }),
    ).toBeInTheDocument();
    const disclaimer = screen.getByText(/Isto é contexto externo\./u);
    expect(disclaimer).toHaveTextContent(
      /não afirma nenhuma relação entre estes números e a presença/u,
    );
  });

  it("publica valor com unidade e alternativa textual da série", async () => {
    renderTab();
    const card = within(await screen.findByRole("article", { name: "Clima do dia" }));

    expect(card.getByText("26,1")).toBeInTheDocument();
    expect(card.getByText("°C")).toBeInTheDocument();
    expect(
      card.getByRole("columnheader", { name: "Valor na fonte" }),
    ).toBeInTheDocument();
    expect(card.getAllByRole("row").length).toBe(4);
  });

  it("exibe proveniência completa no render inicial, sem interação", async () => {
    renderTab();
    await screen.findByRole("article", { name: "Clima do dia" });
    const card = cardByCode("weather_daily");
    const provenance = card.querySelector("[data-card-provenance]");

    expect(provenance).not.toBeNull();
    const inside = within(provenance as HTMLElement);
    expect(inside.getByText("Open-Meteo")).toBeInTheDocument();
    expect(inside.getByRole("link", { name: "CC-BY-4.0" })).toHaveAttribute(
      "href",
      "https://creativecommons.org/licenses/by/4.0/",
    );
    expect(
      provenance?.querySelector(`time[datetime="${WEATHER_OBSERVED_AT}"]`),
    ).not.toBeNull();
    expect(
      provenance?.querySelector(`time[datetime="${WEATHER_RETRIEVED_AT}"]`),
    ).not.toBeNull();
    expect(
      provenance?.querySelector('[data-card-data-mode="real_source"]'),
    ).not.toBeNull();
    expect(inside.getByText(OPEN_METEO_ATTRIBUTION)).toBeInTheDocument();
    // Nenhum `<details>`, `<dialog>` ou `title` esconde a proveniência: ela
    // está aberta desde o primeiro pixel.
    expect(provenance?.closest("details")).toBeNull();
  });

  it("distingue coleta defasada de coleta recente por texto, não só por cor", async () => {
    const { unmount } = renderTab(contextClient(), FRESH_NOW);
    await screen.findByRole("article", { name: "Clima do dia" });
    const fresh = cardByCode("weather_daily");

    expect(
      fresh.querySelector('[data-card-freshness="fresh"]'),
    ).toHaveTextContent("Coleta recente");
    unmount();
    cleanup();

    renderTab(contextClient(), STALE_NOW);
    await screen.findByRole("article", { name: "Clima do dia" });
    const stale = cardByCode("weather_daily");
    const marker = stale.querySelector('[data-card-freshness="stale"]');

    expect(marker).toHaveTextContent(/Coleta defasada/u);
    expect(marker?.textContent).not.toBe("Coleta recente");
    // A origem continua legível: a defasagem se lê no corpo do card.
    expect(
      stale.querySelector(`time[datetime="${WEATHER_RETRIEVED_AT}"]`),
    ).not.toBeNull();
  });

  it("não escreve nenhum numeral no card indisponível", async () => {
    renderTab();
    await screen.findByRole("article", { name: "Maré" });
    const card = cardByCode("tide");

    expect(DIGIT.test(valueText(card))).toBe(false);
    expect(DIGIT.test(textOutsideProvenance(card))).toBe(false);
    expect(valueText(card)).toContain("—");
    expect(card).toHaveAttribute("data-card-status", "unavailable");
  });

  it("anuncia a indisponibilidade em texto, com motivo em linguagem leiga", async () => {
    renderTab();
    const card = within(await screen.findByRole("article", { name: "Maré" }));

    expect(card.getByText(/^Indisponível\./u)).toHaveTextContent(
      /tábua oficial da Marinha do Brasil/u,
    );
  });

  it("não desenha curva, horário de preamar nem de baixa-mar", async () => {
    renderTab();
    await screen.findByRole("article", { name: "Maré" });
    const card = cardByCode("tide");

    expect(card.querySelector("svg, canvas, path, line, rect, polyline")).toBeNull();
    expect(card.querySelector("table")).toBeNull();
    expect(textOutsideProvenance(card)).not.toMatch(/\d{1,2}:\d{2}/u);
    expect(textOutsideProvenance(card)).toMatch(/não mostra curva/u);
  });

  it("credita o Cadastur sem card, sem contagem e sem série", async () => {
    renderTab();
    await screen.findByRole("heading", { name: "Fontes e licenças" });
    const entry = requireElement('[data-source-code="cadastur"]');

    expect(within(entry).getByText(CADASTUR_ATTRIBUTION)).toBeInTheDocument();
    // O rótulo da licença é o único dígito admitido aqui, e é versão de
    // licença: nome e atribuição não carregam número nenhum, e o Cadastur não
    // ganha caixa de valor.
    expect(DIGIT.test(textOf(entry, ".external-source-name"))).toBe(false);
    expect(DIGIT.test(textOf(entry, ".external-source-attribution"))).toBe(false);
    expect(entry.querySelector("[data-card-value]")).toBeNull();
    expect(document.querySelector('[data-card-code="cadastur"]')).toBeNull();
    expect(
      document.querySelectorAll("[data-card-code]").length,
    ).toBe(contextDocument.cards.length);
  });

  it("não desenha eixo, escala nem legenda, e não cita cobertura", async () => {
    const { container } = renderTab();
    await screen.findByRole("article", { name: "Clima do dia" });

    expect(container.querySelector("svg")).toBeNull();
    expect(container.querySelector(".legend")).toBeNull();
    expect(container.querySelector(".presence-chart")).toBeNull();
    expect(container.textContent).not.toMatch(/cobertura/iu);
    expect(container.textContent).not.toMatch(/pessoas-dia/iu);
    expect(container.textContent).not.toContain("%");
  });

  it("mantém a presença intacta quando a fonte externa não carrega", async () => {
    const failing: ContextClient = {
      getContext: vi.fn().mockRejectedValue(new Error("upstream parado")),
    };
    renderTab(failing);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      /O painel de presença não depende desta camada/u,
    );
    expect(document.querySelector("[data-card-code]")).toBeNull();
  });

  it("não tem violação de acessibilidade detectável", async () => {
    const { container } = renderTab();
    await screen.findByRole("article", { name: "Clima do dia" });

    const report = await axe.run(container, {
      rules: { region: { enabled: false } },
    });

    expect(report.violations).toEqual([]);
  });
});

describe("registro da aba no painel", () => {
  afterEach(cleanup);

  it("troca de camada por teclado e nunca mostra as duas ao mesmo tempo", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const analytics = {
      getSummary: vi.fn().mockRejectedValue(new Error("release ausente")),
      getPresence: vi.fn().mockRejectedValue(new Error("release ausente")),
      getPreferences: vi.fn().mockRejectedValue(new Error("release ausente")),
      getMethodology: vi.fn().mockRejectedValue(new Error("release ausente")),
      getQuality: vi.fn().mockRejectedValue(new Error("release ausente")),
      getFunnel: vi.fn().mockRejectedValue(new Error("release ausente")),
    };
    render(
      <QueryClientProvider client={queryClient}>
        <LocaleProvider initial="pt">
          <AnalyticsDashboard client={analytics} />
        </LocaleProvider>
      </QueryClientProvider>,
    );

    const tab = screen.getByRole("button", { name: "Contexto externo" });
    tab.focus();
    expect(tab).toHaveFocus();
    await userEvent.keyboard("{Enter}");

    // Analytics fora do ar não derruba a camada externa, e a camada externa
    // não empresta número nenhum à presença: elas não coexistem na tela.
    expect(
      await screen.findByRole("heading", {
        name: "Contexto externo, fora da metodologia do Observatório",
      }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Presença ao longo do tempo")).toBeNull();
    expect(document.querySelector(".presence-chart")).toBeNull();
  });
});
