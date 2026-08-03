import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  captureInviteCapability,
  clearInviteCapability,
  peekInviteCapability,
} from "../../shared/security/invite-capability";
import RegistrationPage from "../../pages/RegistrationPage";
import {
  clearSurveyCapability,
  setSurveyCapability,
} from "../../shared/security/survey-capability";
import {
  clearAllDrafts,
  inspectDraftPresence,
  saveDraft,
} from "../../shared/offline/encrypted-drafts";

const capability = "z".repeat(64);
const inviteContext = {
  accommodation_name: "Pousada Teste",
  planned_arrival_on: "2026-08-01",
  planned_departure_on: "2026-08-03",
  expected_guest_count: 1,
  privacy_notice_version: "2026-07",
};

function phase2Response(body: unknown, init: ResponseInit = {}) {
  const headers = new Headers(init.headers);
  headers.set("Cache-Control", "no-store");
  headers.set("X-Request-ID", "request-invite-registration-test");
  return Response.json(body, { ...init, headers });
}

function captureCapability() {
  captureInviteCapability(
    new URL(`https://registro.invalid/convites/${capability}`),
    vi.fn(),
  );
}

async function completeBrazilResidence(user: ReturnType<typeof userEvent.setup>) {
  await user.type(
    screen.getByLabelText("UF de residência do visitante 1"),
    "BA",
  );
  await user.type(
    screen.getByLabelText("Município IBGE do visitante 1"),
    "2925509",
  );
}

function draftIdFromLocation() {
  return new URLSearchParams(window.location.hash.slice(1)).get("draft");
}

describe("registro por convite", () => {
  beforeEach(async () => {
    clearInviteCapability();
    clearSurveyCapability();
    await clearAllDrafts();
    window.history.replaceState(null, "", "/registro");
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("não renderiza nem inclui a capability no cache do TanStack Query", async () => {
    captureCapability();
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      phase2Response(inviteContext),
    );
    const queryClient = new QueryClient();

    render(
      <QueryClientProvider client={queryClient}>
        <RegistrationPage />
      </QueryClientProvider>,
    );

    await screen.findByRole("heading", { name: "Confirme o grupo da estadia" });
    expect(document.documentElement.innerHTML).not.toContain(capability);
    expect(JSON.stringify(queryClient.getQueryCache().getAll())).not.toContain(
      capability,
    );
    await waitFor(() => expect(globalThis.fetch).toHaveBeenCalledTimes(1));
  });

  it("falha fechada quando não existe capability em memória", () => {
    render(<RegistrationPage />);

    expect(
      screen.getByRole("heading", { name: "Convite necessário" }),
    ).toBeInTheDocument();
  });

  it("preserva a confirmação após consumir o convite", () => {
    setSurveyCapability("payload.signature");

    render(<RegistrationPage />);

    expect(
      screen.getByRole("heading", { name: "Registro concluído" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Responder pesquisa voluntária" }),
    ).toBeInTheDocument();
  });

  it("não apresenta violações axe sem convite", async () => {
    const { container } = render(<RegistrationPage />);

    const report = await axe.run(container, {
      rules: { "color-contrast": { enabled: false } },
    });

    expect(report.violations).toEqual([]);
  });

  it("marca e foca o primeiro campo inválido do formulário útil", async () => {
    const user = userEvent.setup();
    captureCapability();
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      phase2Response(inviteContext),
    );
    render(<RegistrationPage />);
    await screen.findByRole("heading", {
      name: "Confirme o grupo da estadia",
    });

    await user.click(screen.getByRole("button", { name: "Enviar grupo" }));

    const state = screen.getByLabelText(
      "UF de residência do visitante 1",
    );
    await waitFor(() =>
      expect(state).toHaveAttribute("aria-invalid", "true"),
    );
    expect(state).toHaveFocus();
    expect(globalThis.fetch).toHaveBeenCalledTimes(1);
  });

  it.each(["false", "true"] as const)(
    "revalida antes do sync e expurga payload e chave após replay=%s",
    async (replayed) => {
      const user = userEvent.setup();
      captureCapability();
      const fetcher = vi
        .spyOn(globalThis, "fetch")
        .mockResolvedValueOnce(phase2Response(inviteContext))
        .mockResolvedValueOnce(phase2Response(inviteContext))
        .mockResolvedValueOnce(
          phase2Response(
            { submission_id: "0190aabb-7ccd-7eef-8abc-001122334455", status: "accepted" },
            {
              status: 200,
              headers: {
                ETag: '"1"',
                "Idempotency-Replayed": replayed,
                "Survey-Capability": "payload.signature",
              },
            },
          ),
        );
      render(<RegistrationPage />);
      await screen.findByRole("heading", {
        name: "Confirme o grupo da estadia",
      });
      await completeBrazilResidence(user);
      await user.click(
        screen.getByRole("button", { name: "Salvar rascunho cifrado" }),
      );
      await screen.findByText("Rascunho cifrado salvo por até 24 horas.");
      const draftId = draftIdFromLocation();
      expect(draftId).not.toBeNull();

      window.dispatchEvent(new Event("online"));

      await screen.findByText("Grupo enviado com sucesso.");
      expect(
        screen.getByRole("button", { name: "Responder pesquisa voluntária" }),
      ).toBeInTheDocument();
      const requests = fetcher.mock.calls.map(
        (call) => call[0] as Request,
      );
      expect(requests[1]?.method).toBe("GET");
      expect(requests[2]?.method).toBe("POST");
      await expect(inspectDraftPresence(draftId!)).resolves.toEqual({
        draft: false,
        key: false,
      });
    },
  );

  it("mantém a capability em memória ao avançar para a pesquisa", async () => {
    const user = userEvent.setup();
    captureCapability();
    vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(phase2Response(inviteContext))
      .mockResolvedValueOnce(phase2Response(inviteContext))
      .mockResolvedValueOnce(
        phase2Response(
          { submission_id: "0190aabb-7ccd-7eef-8abc-001122334455", status: "accepted" },
          {
            headers: {
              ETag: '"1"',
              "Idempotency-Replayed": "false",
              "Survey-Capability": "payload.signature",
            },
          },
        ),
      );
    render(<RegistrationPage />);
    await screen.findByRole("heading", {
      name: "Confirme o grupo da estadia",
    });
    await completeBrazilResidence(user);
    await user.click(screen.getByRole("button", { name: "Enviar grupo" }));
    await user.click(
      await screen.findByRole("button", {
        name: "Responder pesquisa voluntária",
      }),
    );

    expect(window.location.pathname).toBe("/pesquisa");
  });

  it("expurga rascunho, chave e capability quando a revalidação retorna 404", async () => {
    const user = userEvent.setup();
    captureCapability();
    vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(phase2Response(inviteContext))
      .mockResolvedValueOnce(
        phase2Response(
          {
            type: "urn:cumuru:problem:invite-not-found",
            title: "Convite expirado.",
            status: 404,
          },
          {
            status: 404,
            headers: { "Content-Type": "application/problem+json" },
          },
        ),
      );
    render(<RegistrationPage />);
    await screen.findByRole("heading", {
      name: "Confirme o grupo da estadia",
    });
    await completeBrazilResidence(user);
    await user.click(
      screen.getByRole("button", { name: "Salvar rascunho cifrado" }),
    );
    await screen.findByText("Rascunho cifrado salvo por até 24 horas.");
    const draftId = draftIdFromLocation();

    window.dispatchEvent(new Event("online"));

    await screen.findByText("Convite expirado.");
    expect(peekInviteCapability()).toBeNull();
    await expect(inspectDraftPresence(draftId!)).resolves.toEqual({
      draft: false,
      key: false,
    });
  });

  it("expurga draft restaurado quando o GET inicial retorna 404", async () => {
    const saved = await saveDraft({
      client_submission_id: "018f4e59-7a2a-7b12-8fd7-5d2e8dc99b80",
      privacy_notice_version: "2026-07",
      visitors: [
        {
          client_id: "018f4e59-7a2a-7c12-8fd7-5d2e8dc99b80",
          role: "responsible",
          age_band: "25_34",
          residence_country: "AR",
        },
      ],
    });
    window.history.replaceState(null, "", `/registro#draft=${saved.id}`);
    captureCapability();
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      phase2Response(
        {
          type: "urn:cumuru:problem:invite-not-found",
          title: "Convite expirado.",
          status: 404,
        },
        {
          status: 404,
          headers: { "Content-Type": "application/problem+json" },
        },
      ),
    );

    render(<RegistrationPage />);

    await screen.findByText("Convite expirado.");
    expect(peekInviteCapability()).toBeNull();
    await expect(inspectDraftPresence(saved.id)).resolves.toEqual({
      draft: false,
      key: false,
    });
  });

  it("não apresenta violações axe no formulário carregado", async () => {
    captureCapability();
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      phase2Response(inviteContext),
    );
    const { container } = render(<RegistrationPage />);
    await screen.findByRole("heading", {
      name: "Confirme o grupo da estadia",
    });

    const report = await axe.run(container, {
      rules: { "color-contrast": { enabled: false } },
    });

    expect(report.violations).toEqual([]);
  });
});
