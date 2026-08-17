import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { App } from "../../app/App";
import { LocaleProvider } from "../../shared/i18n/LocaleProvider";
import {
  captureSelfServiceCapability,
  clearSelfServiceCapability,
  peekSelfServiceCapability,
} from "../../shared/security/self-service-capability";
import { clearSurveyCapability } from "../../shared/security/survey-capability";
import { clearAllDrafts } from "../../shared/offline/encrypted-drafts";

/**
 * A jornada montada dentro do `App` real, e não a página isolada.
 *
 * É essa a diferença que deixou passar a regressão do e2e: `App` assina
 * `CAPABILITY_CHANGE_EVENT` (`useCapabilityRevision`) e se redesenha quando uma
 * capability muda. A página isolada não assina nada, então ela nunca reavaliava
 * `peekSelfServiceCapability()` depois do sucesso e a conclusão aparecia.
 *
 * Sob o `App`, descartar o token — que a ADR-019 exige no sucesso — dispara o
 * redesenho, a página reavalia, não encontra token e troca o formulário pela
 * tela "Cartaz necessário", destruindo o estado de conclusão junto. O teste
 * precisa cobrir a **sequência**, não o estado final isolado.
 */

const capability = "j".repeat(96);
const inviteContext = {
  accommodation_name: "Pousada Fictícia da Barra",
  privacy_notice_version: "2026-07",
  proof_of_work: {
    algorithm: "sha256-leading-zero-bits",
    challenge: "Y3VtdXJ1LXBvdy1qb3VybmV5LWNoYWxsZW5nZS0wMDAwMDAx",
    difficulty_bits: 4,
    expires_at: "2026-08-17T23:59:00Z",
  },
};

function apiResponse(body: unknown, init: ResponseInit = {}) {
  const headers = new Headers(init.headers);
  headers.set("Cache-Control", "no-store");
  headers.set("X-Request-ID", "request-self-registration-journey");
  return Response.json(body, { ...init, headers });
}

function acceptedResponse() {
  return apiResponse(
    {
      submission_id: "018f4e59-7a2a-7b12-8fd7-5d2e8dc99b80",
      stay_id: "018f4e59-7a2a-7b12-8fd7-5d2e8dc99b81",
      status: "accepted",
      stay_status: "pre_registered",
      approval_state: "pending",
    },
    { status: 200, headers: { ETag: '"1"', "Idempotency-Replayed": "false" } },
  );
}

/**
 * O passo de setup carrega a página por `import()` dinâmico e ainda espera o
 * contexto do cartaz. O padrão de 1s do `findBy*` é orçamento de asserção, não
 * de partida: sob carga isso estourava e a falha aparecia no setup, escondendo
 * o que o teste realmente afirma. As asserções seguintes continuam com o
 * padrão — só a largada ganhou folga.
 */
const FORM_READY_TIMEOUT_MS = 10_000;

function formReady() {
  return screen.findByRole(
    "heading",
    { name: "Confirme os dados da estadia" },
    { timeout: FORM_READY_TIMEOUT_MS },
  );
}

function renderApp() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <LocaleProvider initial="pt">
        <App />
      </LocaleProvider>
    </QueryClientProvider>,
  );
}

describe("jornada do autocadastro dentro do App", () => {
  beforeEach(async () => {
    clearSelfServiceCapability();
    clearSurveyCapability();
    await clearAllDrafts();
    window.history.replaceState(null, "", "/i");
    captureSelfServiceCapability(
      new URL(`https://cumuru.invalid/i#${capability}`),
      vi.fn(),
    );
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("mostra a confirmação depois do 200 e não regride para a tela sem cartaz", async () => {
    const user = userEvent.setup();
    const fetcher = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(apiResponse(inviteContext))
      .mockResolvedValueOnce(acceptedResponse());
    renderApp();
    await formReady();
    await user.type(
      screen.getByLabelText("UF de residência do visitante 1"),
      "BA",
    );
    await user.type(
      screen.getByLabelText("Município IBGE do visitante 1"),
      "2925509",
    );

    await user.click(screen.getByRole("button", { name: "Enviar autocadastro" }));

    expect(
      await screen.findByRole("heading", { name: "Autocadastro enviado" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Cartaz necessário" }),
    ).not.toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("descarta o token no sucesso, como a ADR-019 exige", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(apiResponse(inviteContext))
      .mockResolvedValueOnce(acceptedResponse());
    renderApp();
    await formReady();
    await user.type(
      screen.getByLabelText("UF de residência do visitante 1"),
      "BA",
    );
    await user.type(
      screen.getByLabelText("Município IBGE do visitante 1"),
      "2925509",
    );

    await user.click(screen.getByRole("button", { name: "Enviar autocadastro" }));
    await screen.findByRole("heading", { name: "Autocadastro enviado" });

    // A confirmação não pode custar a sobrevivência do token: o cartaz é
    // credencial bearer e o sucesso tem de consumi-lo.
    expect(peekSelfServiceCapability()).toBeNull();
    expect(document.documentElement.innerHTML).not.toContain(capability);
  });
});
