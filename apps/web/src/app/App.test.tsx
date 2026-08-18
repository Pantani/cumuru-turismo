import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { lazy, type ReactElement } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  renderWithSession,
  stubAuthClient,
  testSession,
} from "../test/session";
import {
  clearInviteCapability,
  captureInviteCapability,
} from "../shared/security/invite-capability";
import {
  clearSurveyCapability,
  setSurveyCapability,
} from "../shared/security/survey-capability";
import { LocaleProvider } from "../shared/i18n/LocaleProvider";
import { App } from "./App";

function renderApp(app: ReactElement = <App />) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <LocaleProvider initial="pt">{app}</LocaleProvider>
    </QueryClientProvider>,
  );
}

describe("App", () => {
  beforeEach(() => {
    clearInviteCapability();
    clearSurveyCapability();
    window.history.replaceState(null, "", "/");
    vi.stubGlobal(
      "fetch",
      vi.fn(() => new Promise<Response>(() => undefined)),
    );
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it.each([
    ["/", "O turismo da nossa praia, finalmente em números."],
    ["/registro", "Registro de estadias"],
    ["/convite", "Peça seu acesso ao Observatório"],
    ["/pesquisa", "Pesquisa turística"],
    ["/acesso", "Área da hospedagem"],
    ["/questionarios", "Questionários"],
    ["/qualidade", "Qualidade dos dados"],
    ["/nao-existe", "Página não encontrada"],
  ])("carrega a rota %s sob demanda", async (path, heading) => {
    window.history.replaceState(null, "", path);
    renderApp();

    expect(
      await screen.findByRole("heading", { level: 1, name: heading }),
    ).toBeInTheDocument();
  });

  it("permite navegar e chegar ao conteúdo apenas com o teclado", async () => {
    const user = userEvent.setup();
    captureInviteCapability(
      new URL(`https://registro.invalid/convites/${"a".repeat(64)}`),
      vi.fn(),
    );
    renderApp();

    const skipLink = screen.getByRole("link", { name: "Ir para o conteúdo" });
    await user.tab();
    expect(skipLink).toHaveFocus();

    await user.keyboard("{Enter}");
    expect(screen.getByRole("main")).toHaveFocus();

    const registrationLink = screen.getByRole("link", {
      name: "Registro",
    });
    registrationLink.focus();
    await user.keyboard("{Enter}");

    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Registro de estadias",
      }),
    ).toHaveFocus();
  });

  it("move o foco somente depois que o chunk lazy da nova rota monta", async () => {
    let resolveRegistration:
      | ((module: { default: () => ReactElement }) => void)
      | undefined;
    const registrationModule = new Promise<{
      default: () => ReactElement;
    }>((resolve) => {
      resolveRegistration = resolve;
    });
    const DelayedRegistrationPage = lazy(() => registrationModule);
    const user = userEvent.setup();
    captureInviteCapability(
      new URL(`https://registro.invalid/convites/${"a".repeat(64)}`),
      vi.fn(),
    );

    renderApp(
      <App
        routeRenderer={(pathname) =>
          pathname === "/registro" ? (
            <DelayedRegistrationPage />
          ) : (
            <h1 tabIndex={-1}>Página inicial de teste</h1>
          )
        }
      />,
    );
    const registrationLink = screen.getByRole("link", { name: "Registro" });
    registrationLink.focus();
    await user.keyboard("{Enter}");

    expect(screen.getByText("Carregando página…")).toBeInTheDocument();

    await act(async () => {
      resolveRegistration?.({
        default: () => (
          <h1 data-route-heading tabIndex={-1}>
            Registro com chunk atrasado
          </h1>
        ),
      });
      await registrationModule;
    });

    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Registro com chunk atrasado",
      }),
    ).toHaveFocus();
  });

  it("não apresenta violações automáticas de acessibilidade no shell", async () => {
    const { container } = renderApp();
    await screen.findByRole("heading", {
      level: 1,
      name: "O turismo da nossa praia, finalmente em números.",
    });

    const report = await axe.run(container, {
      rules: {
        "color-contrast": { enabled: false },
      },
    });

    expect(report.violations).toEqual([]);
  });

  it("anuncia o carregamento dos indicadores públicos", async () => {
    renderApp();

    expect(
      await screen.findByText("Atualizando indicadores públicos…"),
    ).toHaveAttribute("role", "status");
  });

  it("oculta a rota de qualidade sem sessão e a mostra com sessão", async () => {
    const { unmount } = renderApp();

    expect(screen.queryByRole("link", { name: "Qualidade" })).toBeNull();
    unmount();

    renderWithSession(<App />);

    expect(
      await screen.findByRole("link", { name: "Qualidade" }),
    ).toBeInTheDocument();
    expect(document.body.textContent).not.toContain("cms_test-session-token");
  });

  it("mostra registro e pesquisa somente com a capability correspondente", () => {
    const first = renderApp();

    expect(screen.queryByRole("link", { name: "Registro" })).toBeNull();
    expect(screen.queryByRole("link", { name: "Pesquisa" })).toBeNull();
    first.unmount();

    captureInviteCapability(
      new URL(`https://registro.invalid/convites/${"a".repeat(64)}`),
      vi.fn(),
    );
    const second = renderApp();
    expect(screen.getByRole("link", { name: "Registro" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Pesquisa" })).toBeNull();
    second.unmount();

    clearInviteCapability();
    setSurveyCapability("payload.signature");
    renderApp();
    expect(screen.queryByRole("link", { name: "Registro" })).toBeNull();
    expect(screen.getByRole("link", { name: "Pesquisa" })).toBeInTheDocument();
  });

  it("oferece a área da hospedagem sem sessão e esconde as internas", () => {
    renderApp();

    expect(
      screen.getByRole("link", { name: "Área da hospedagem" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Questionários" })).toBeNull();
    expect(screen.queryByRole("link", { name: "Qualidade" })).toBeNull();
  });

  it("mostra superfícies internas somente com o escopo correspondente", async () => {
    renderWithSession(<App />, {
      authClient: stubAuthClient(testSession(["stays:read:own"])),
    });

    expect(
      await screen.findByRole("link", { name: "Área da hospedagem" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Questionários" })).toBeNull();
    expect(screen.queryByRole("link", { name: "Qualidade" })).toBeNull();
  });

  it("apresenta o nome da conta e a saída quando há sessão", async () => {
    renderWithSession(<App />);

    expect(await screen.findByText("Operadora de teste")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Sair" })).toBeInTheDocument();
  });

  it("remove a navegação contextual quando a capability é consumida", () => {
    setSurveyCapability("payload.signature");
    renderApp();
    expect(screen.getByRole("link", { name: "Pesquisa" })).toBeInTheDocument();

    act(() => clearSurveyCapability());

    expect(screen.queryByRole("link", { name: "Pesquisa" })).toBeNull();
  });
});
