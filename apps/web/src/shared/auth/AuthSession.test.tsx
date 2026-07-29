import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { describe, expect, it, vi } from "vitest";

import { AuthSessionProvider, useAuthSession } from "./AuthSession";
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

describe("fronteira de autenticação institucional", () => {
  it("falha fechada sem token entregue pelo provedor OIDC", () => {
    render(
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
    render(
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
    const { container } = render(
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
    render(
      <AuthSessionProvider accessToken="opaque-test-token">
        <LogoutProbe completed={completed} />
      </AuthSessionProvider>,
    );

    await user.click(screen.getByRole("button", { name: "Encerrar" }));
    await waitFor(() => expect(completed).toHaveBeenCalledOnce());

    await expect(inspectDraftPresence(saved.id)).resolves.toEqual({
      draft: false,
      key: false,
    });
  });

  it("mantém o cliente de analytics fail-closed sem token", async () => {
    const user = userEvent.setup();
    const completed = vi.fn();
    const fetcher = vi.spyOn(globalThis, "fetch");
    render(
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
