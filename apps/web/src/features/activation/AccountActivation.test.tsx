import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import ActivationPage from "../../pages/ActivationPage";
import {
  captureActivationCapability,
  clearActivationCapability,
  peekActivationCapability,
} from "../../shared/security/activation-capability";

const capability = "k".repeat(96);
const activationContext = {
  accommodation_name: "Pousada Fictícia da Barra",
  display_name: "Maria da Recepção",
  expires_at: "2026-08-19T12:00:00Z",
  password_policy: { min_length: 12, max_length: 256 },
};

function apiResponse(body: unknown, init: ResponseInit = {}) {
  const headers = new Headers(init.headers);
  headers.set("Cache-Control", "no-store");
  headers.set("X-Request-ID", "request-activation-test");
  return Response.json(body, { ...init, headers });
}

function noContent() {
  return new Response(null, {
    status: 204,
    headers: {
      "Cache-Control": "no-store",
      "X-Request-ID": "request-activation-test",
    },
  });
}

function capture() {
  captureActivationCapability(
    new URL(`https://cumuru.invalid/ativacao#${capability}`),
    vi.fn(),
  );
}

async function renderLoaded() {
  capture();
  const fetcher = vi
    .spyOn(globalThis, "fetch")
    .mockResolvedValueOnce(apiResponse(activationContext));
  const view = render(<ActivationPage />);
  await screen.findByText(activationContext.accommodation_name);
  return { fetcher, view };
}

describe("ativação da conta da hospedagem", () => {
  beforeEach(() => {
    clearActivationCapability();
    window.history.replaceState(null, "", "/ativacao");
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("falha fechada e não chama a API sem a capability", () => {
    const fetcher = vi.spyOn(globalThis, "fetch");

    render(<ActivationPage />);

    expect(
      screen.getByRole("heading", { name: "Link de acesso necessário" }),
    ).toBeInTheDocument();
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("consulta o contexto por cabeçalho próprio e nunca revela a capability", async () => {
    const { fetcher } = await renderLoaded();

    const request = fetcher.mock.calls[0]?.[0] as Request;
    expect(request.headers.get("X-Cumuru-Activation-Token")).toBe(capability);
    expect(request.headers.get("X-Cumuru-Invite-Token")).toBeNull();
    expect(new URL(request.url).pathname).toBe("/api/v1/activation");
    expect(document.documentElement.innerHTML).not.toContain(capability);
  });

  it("recusa senha curta e confirmação divergente antes de gastar o link", async () => {
    const user = userEvent.setup();
    const { fetcher } = await renderLoaded();

    const password = screen.getByLabelText("Nova senha");
    await user.type(password, "curta");
    await user.type(screen.getByLabelText("Confirme a nova senha"), "outra");
    await user.click(screen.getByRole("button", { name: "Definir senha" }));

    expect(password).toHaveAttribute("aria-invalid", "true");
    expect(await screen.findByText(/mínimo 12 caracteres/u)).toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("define a senha, esquece a capability e não emite sessão", async () => {
    const user = userEvent.setup();
    const { fetcher } = await renderLoaded();
    fetcher.mockResolvedValueOnce(noContent());

    await user.type(screen.getByLabelText("Nova senha"), "senha-de-teste-longa");
    await user.type(
      screen.getByLabelText("Confirme a nova senha"),
      "senha-de-teste-longa",
    );
    await user.click(screen.getByRole("button", { name: "Definir senha" }));

    expect(
      await screen.findByRole("heading", { name: "Senha definida" }),
    ).toBeInTheDocument();
    const request = fetcher.mock.calls[1]?.[0] as Request;
    expect(new URL(request.url).pathname).toBe("/api/v1/activation/complete");
    expect(peekActivationCapability()).toBeNull();
    expect(screen.getByRole("status")).toHaveTextContent(/entrar/iu);
  });

  it("trata o link consumido ou inválido com a mesma mensagem", async () => {
    capture();
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      apiResponse(
        {
          type: "urn:cumuru:problem:activation-not-found",
          title: "Link inválido.",
          status: 404,
        },
        { status: 404, headers: { "Content-Type": "application/problem+json" } },
      ),
    );

    render(<ActivationPage />);

    expect(
      await screen.findByText(/não é mais válido/u),
    ).toBeInTheDocument();
  });

  it("não apresenta violações axe no formulário carregado", async () => {
    const { view } = await renderLoaded();

    const report = await axe.run(view.container, {
      rules: { "color-contrast": { enabled: false } },
    });

    expect(report.violations).toEqual([]);
  });
});
