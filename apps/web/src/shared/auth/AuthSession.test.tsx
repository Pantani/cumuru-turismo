import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { useEffect, useRef, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AuthSessionProvider, useAuthSession } from "./AuthSession";
import AuthenticatedPage from "../../pages/AuthenticatedPage";
import { AuthError, type AuthClient } from "../api/auth-client";
import { inspectDraftPresence, saveDraft } from "../offline/encrypted-drafts";
import {
  establishmentScopes,
  renderWithSession,
  stubAuthClient,
  testSession,
} from "../../test/session";

function LogoutProbe({ completed }: { completed: () => void }) {
  const { endSession } = useAuthSession();
  return (
    <button
      type="button"
      onClick={() => {
        void endSession().then(completed);
      }}
    >
      Encerrar
    </button>
  );
}

function AnalyticsProbe({ completed }: { completed: (status: number) => void }) {
  const { analyticsClient } = useAuthSession();
  return (
    <button
      type="button"
      onClick={() => {
        void analyticsClient
          .getQuality()
          .then(() => completed(200))
          .catch((error: unknown) => {
            const status =
              typeof error === "object" && error !== null && "status" in error
                ? Number(error.status)
                : 0;
            completed(status);
          });
      }}
    >
      Consultar qualidade
    </button>
  );
}

function renderSession(children: ReactNode) {
  const queryClient = new QueryClient();
  return {
    ...render(
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>,
    ),
    queryClient,
  };
}

afterEach(cleanup);

describe("fronteira de autenticação local", () => {
  it("apresenta o formulário de login sem sessão", () => {
    renderSession(
      <AuthSessionProvider>
        <AuthenticatedPage />
      </AuthSessionProvider>,
    );

    expect(
      screen.getByRole("heading", { name: "Entrar na área da hospedagem" }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("E-mail")).toBeInTheDocument();
    expect(screen.getByLabelText("Senha")).toBeInTheDocument();
  });

  it("não anuncia exigência de CNPJ nem de credencial federal", () => {
    renderSession(
      <AuthSessionProvider>
        <AuthenticatedPage />
      </AuthSessionProvider>,
    );

    const text = document.body.textContent ?? "";
    expect(text).toContain("Não é preciso CNPJ");
    expect(text).not.toContain("OIDC");
  });

  it("abre o workspace mantendo o token fora do DOM", async () => {
    renderWithSession(<AuthenticatedPage />, {
      authClient: stubAuthClient(testSession(establishmentScopes)),
    });

    expect(
      await screen.findByRole("heading", { name: "Suas hospedagens" }),
    ).toBeInTheDocument();
    expect(document.body.textContent).not.toContain("cms_test-session-token");
  });

  it("informa credencial rejeitada sem revelar se a conta existe", async () => {
    const user = userEvent.setup();
    const rejecting: AuthClient = {
      ...stubAuthClient(),
      login: async () => {
        throw new AuthError(401, "E-mail ou senha incorretos.");
      },
    };
    renderSession(
      <AuthSessionProvider authClient={rejecting}>
        <AuthenticatedPage />
      </AuthSessionProvider>,
    );

    await user.type(screen.getByLabelText("E-mail"), "quem@cumuru.local");
    await user.type(screen.getByLabelText("Senha"), "senha-de-teste-longa");
    await user.click(screen.getByRole("button", { name: "Entrar" }));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("E-mail ou senha incorretos.");
    expect(alert.textContent).not.toContain("quem@cumuru.local");
  });

  it("bloqueia envio com senha abaixo do mínimo do contrato", async () => {
    const user = userEvent.setup();
    const login = vi.fn(async () => testSession());
    renderSession(
      <AuthSessionProvider authClient={{ ...stubAuthClient(), login }}>
        <AuthenticatedPage />
      </AuthSessionProvider>,
    );

    await user.type(screen.getByLabelText("E-mail"), "operador@cumuru.local");
    await user.type(screen.getByLabelText("Senha"), "curta");
    await user.click(screen.getByRole("button", { name: "Entrar" }));

    expect(login).not.toHaveBeenCalled();
    expect(
      screen.getByText("A senha tem no mínimo 12 caracteres."),
    ).toBeInTheDocument();
  });

  it("não apresenta violações axe na tela de login", async () => {
    const { container } = renderSession(
      <AuthSessionProvider>
        <AuthenticatedPage />
      </AuthSessionProvider>,
    );

    const report = await axe.run(container, {
      rules: { "color-contrast": { enabled: false } },
    });

    expect(report.violations).toEqual([]);
  });

  it("revoga no servidor e elimina payload, chave e cache no logout", async () => {
    const user = userEvent.setup();
    const completed = vi.fn();
    const logout = vi.fn(async () => undefined);
    const saved = await saveDraft({ privacy_notice_version: "2026-07" });
    const queryClient = new QueryClient();
    render(
      <QueryClientProvider client={queryClient}>
        <AuthSessionProvider authClient={{ ...stubAuthClient(), logout }}>
          <SignedInProbe completed={completed} />
        </AuthSessionProvider>
      </QueryClientProvider>,
    );
    queryClient.setQueryData(["authenticated"], { sensitive: true });

    await user.click(await screen.findByRole("button", { name: "Encerrar" }));
    await waitFor(() => expect(completed).toHaveBeenCalledOnce());

    expect(logout).toHaveBeenCalledWith("cms_test-session-token");
    await expect(inspectDraftPresence(saved.id)).resolves.toEqual({
      draft: false,
      key: false,
    });
    expect(queryClient.getQueryData(["authenticated"])).toBeUndefined();
  });

  it("mantém o cliente de analytics fail-closed sem sessão", async () => {
    const user = userEvent.setup();
    const completed = vi.fn();
    const fetcher = vi.spyOn(globalThis, "fetch");
    renderSession(
      <AuthSessionProvider>
        <AnalyticsProbe completed={completed} />
      </AuthSessionProvider>,
    );

    await user.click(
      screen.getByRole("button", { name: "Consultar qualidade" }),
    );

    await waitFor(() => expect(completed).toHaveBeenCalledWith(401));
    expect(fetcher).not.toHaveBeenCalled();
  });
});

function SignedInProbe({ completed }: { completed: () => void }) {
  const { authenticated, signIn } = useAuthSession();
  const started = useRef(false);
  useEffect(() => {
    if (started.current) {
      return;
    }
    started.current = true;
    void signIn("operador@cumuru.local", "fixture-fixture-");
  }, [signIn]);
  return authenticated ? <LogoutProbe completed={completed} /> : null;
}
