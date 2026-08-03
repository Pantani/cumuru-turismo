import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import {
  AuthSessionProvider,
  localDemoAccessToken,
  useAuthSession,
} from "./AuthSession";
import AuthenticatedPage from "../../pages/AuthenticatedPage";
import {
  inspectDraftPresence,
  saveDraft,
} from "../offline/encrypted-drafts";

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
      <QueryClientProvider client={queryClient}>
        {children}
      </QueryClientProvider>,
    ),
    queryClient,
  };
}

describe("fronteira de autenticação institucional", () => {
  it("habilita o principal fictício somente com o sinal local explícito", () => {
    expect(localDemoAccessToken(false, "fixture-token", "localhost")).toBeNull();
    expect(localDemoAccessToken(true, undefined, "localhost")).toBeNull();
    expect(localDemoAccessToken(true, "fixture-token", "example.org")).toBeNull();
    for (const hostname of ["localhost", "127.0.0.1", "::1", "[::1]"]) {
      expect(localDemoAccessToken(true, "fixture-token", hostname)).toBe(
        "fixture-token",
      );
    }
  });

  it("falha fechada sem token entregue pelo provedor OIDC", () => {
    renderSession(
      <AuthSessionProvider>
        <AuthenticatedPage />
      </AuthSessionProvider>,
    );

    expect(
      screen.getByRole("heading", { name: "Acesso institucional necessário" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: /senha/i })).toBeNull();
  });

  it("libera o workspace com token mantido somente em memória", () => {
    renderSession(
      <AuthSessionProvider accessToken="opaque-test-token">
        <AuthenticatedPage />
      </AuthSessionProvider>,
    );

    expect(
      screen.getByRole("heading", { name: "Operação de estadias" }),
    ).toBeInTheDocument();
    expect(document.body.textContent).not.toContain("opaque-test-token");
  });

  it("não apresenta violações axe na fronteira fail-closed", async () => {
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

  it("expõe logout aguardável que elimina payload e chave", async () => {
    const user = userEvent.setup();
    const completed = vi.fn();
    const saved = await saveDraft({ privacy_notice_version: "2026-07" });
    const { queryClient } = renderSession(
      <AuthSessionProvider accessToken="opaque-test-token">
        <LogoutProbe completed={completed} />
      </AuthSessionProvider>,
    );
    queryClient.setQueryData(["authenticated"], { sensitive: true });

    await user.click(screen.getByRole("button", { name: "Encerrar" }));
    await waitFor(() => expect(completed).toHaveBeenCalledOnce());

    await expect(inspectDraftPresence(saved.id)).resolves.toEqual({
      draft: false,
      key: false,
    });
    expect(queryClient.getQueryData(["authenticated"])).toBeUndefined();
  });

  it("mantém o cliente de analytics fail-closed sem token", async () => {
    const user = userEvent.setup();
    const completed = vi.fn();
    const fetcher = vi.spyOn(globalThis, "fetch");
    renderSession(
      <AuthSessionProvider>
        <AnalyticsProbe completed={completed} />
      </AuthSessionProvider>,
    );

    await user.click(screen.getByRole("button", {
      name: "Consultar qualidade",
    }));

    await waitFor(() => expect(completed).toHaveBeenCalledWith(401));
    expect(fetcher).not.toHaveBeenCalled();
  });
});
