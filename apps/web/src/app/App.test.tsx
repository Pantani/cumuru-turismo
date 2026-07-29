import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { lazy, type ReactElement } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AuthSessionProvider } from "../shared/auth/AuthSession";
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
      {app}
    </QueryClientProvider>,
  );
}

describe("App", () => {
  beforeEach(() => {
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
    ["/", "Observatório Turístico de Cumuruxatiba"],
    ["/registro", "Registro de estadias"],
    ["/pesquisa", "Pesquisa turística"],
    ["/acesso", "Área autenticada"],
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
      name: "Observatório Turístico de Cumuruxatiba",
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

  it("oculta a rota de qualidade sem sessão e a mostra com sessão", () => {
    const { unmount } = renderApp();

    expect(screen.queryByRole("link", { name: "Qualidade" })).toBeNull();
    unmount();

    renderApp(
      <AuthSessionProvider accessToken="opaque-test-token">
        <App />
      </AuthSessionProvider>,
    );

    expect(
      screen.getByRole("link", { name: "Qualidade" }),
    ).toBeInTheDocument();
    expect(document.body.textContent).not.toContain("opaque-test-token");
  });
});
