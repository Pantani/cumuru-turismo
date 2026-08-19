import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, within } from "@testing-library/react";
import axe from "axe-core";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { AnalyticsClient } from "../shared/api/analytics-client";
import { LocaleProvider } from "../shared/i18n/LocaleProvider";
import type { Locale } from "../shared/i18n/locale";
import PublicFoundationPage from "./PublicFoundationPage";

const pending = new Promise<never>(() => undefined);
const client: AnalyticsClient = {
  getSummary: vi.fn(() => pending),
  getPresence: vi.fn(() => pending),
  getPreferences: vi.fn(() => pending),
  getMethodology: vi.fn(() => pending),
  getQuality: vi.fn(() => pending),
  getFunnel: vi.fn(() => pending),
};

function renderPage(locale: Locale = "pt") {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <LocaleProvider initial={locale}>
        <PublicFoundationPage client={client} />
      </LocaleProvider>
    </QueryClientProvider>,
  );
}

describe("capa pública do observatório", () => {
  afterEach(cleanup);

  it("abre com o número da vila como título da página", () => {
    renderPage();

    expect(
      screen.getByRole("heading", {
        level: 1,
        name: "O turismo da nossa praia, finalmente em números.",
      }),
    ).toBeInTheDocument();
  });

  it("mantém o aviso de protótipo não censitário", () => {
    renderPage();

    expect(screen.getByText(/somente dados fictícios/i)).toBeInTheDocument();
    expect(
      screen.getByText(/não substitui estatística oficial nem censo/i),
    ).toBeInTheDocument();
  });

  it("não publica número enquanto o contrato não responde", () => {
    renderPage();

    expect(screen.getAllByText("Carregando").length).toBeGreaterThan(0);
    expect(
      screen.getByText("Atualizando indicadores públicos…"),
    ).toBeInTheDocument();
  });

  it("aponta cada âncora do índice para uma seção que existe", () => {
    const { container } = renderPage();

    const index = screen.getByRole("navigation", {
      name: "Seções desta página",
    });
    const anchors = within(index).getAllByRole("link");
    const targets = anchors
      .map((anchor) => anchor.getAttribute("href"))
      .filter((href): href is string => href?.startsWith("#") === true);

    // O botão de cadastro fecha a lista porque aponta para o cartão que explica
    // o caminho real da conta, não para a tela de login: quem ainda não tem
    // conta não teria o que fazer em `/acesso`.
    expect(targets).toEqual([
      "#numeros",
      "#como",
      "#anfitrioes",
      "#comercio",
      "#privacidade",
      "#sobre",
      "#cadastro",
    ]);
    for (const target of targets) {
      expect(container.querySelector(target)).not.toBeNull();
    }
  });

  // Quem chega pela capa ainda não tem conta: um CTA de cadastro apontando
  // para a tela de login terminava numa parede de credencial. A capa e o menu
  // levam à seção de cadastro, e é o cartão dela que abre o pedido — o que o
  // teste protege é que o caminho exista inteiro e não desemboque no login.
  it("leva o convite ao cadastro até o pedido de acesso, nunca ao login", () => {
    renderPage();

    for (const name of ["Cadastrar minha hospedagem", "Cadastrar hospedagem"]) {
      expect(screen.getByRole("link", { name })).toHaveAttribute(
        "href",
        "#cadastro",
      );
    }

    expect(
      screen.getByRole("link", { name: "Pedir meu acesso" }),
    ).toHaveAttribute("href", "/convite");
  });

  // O login continua alcançável, mas por um link que se anuncia como tal.
  it("oferece a entrada de quem já tem conta separada do pedido", () => {
    renderPage();

    expect(
      screen.getByRole("link", { name: /Já tem acesso\?/ }),
    ).toHaveAttribute("href", "/acesso");
  });

  it("liga os guias aos PDFs servidos pela aplicação", () => {
    renderPage();

    expect(
      screen.getByRole("link", {
        name: /Guia do Observatório para a Prefeitura/,
      }),
    ).toHaveAttribute("href", "/guias/observatorio-prefeitura.pdf");
    expect(
      screen.getByRole("link", { name: /Guia para gerar a chave FNRH/ }),
    ).toHaveAttribute("href", "/guias/chave-fnrh-hospedagens.pdf");
  });

  it("responde às perguntas frequentes sem depender de script", () => {
    renderPage();

    expect(
      screen.getByText("Preciso ter CNPJ para participar?"),
    ).toBeInTheDocument();
    expect(screen.getByText("Quanto custa?")).toBeInTheDocument();
  });

  it("publica a capa inteira no idioma escolhido", () => {
    renderPage("es");

    expect(
      screen.getByRole("heading", {
        level: 1,
        name: "El turismo de nuestra playa, por fin en números.",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Actualizando indicadores públicos…"),
    ).toBeInTheDocument();
  });

  it("não apresenta violações automáticas de acessibilidade", async () => {
    const { container } = renderPage();

    const report = await axe.run(container, {
      rules: { "color-contrast": { enabled: false } },
    });

    expect(report.violations).toEqual([]);
  });
});
