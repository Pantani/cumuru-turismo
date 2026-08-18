import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  clearSurveyCapability,
  peekSurveyCapability,
  setSurveyCapability,
} from "../../shared/security/survey-capability";
import {
  clearAllDrafts,
  inspectDraftRecord,
} from "../../shared/offline/encrypted-drafts";
import { LocaleProvider } from "../../shared/i18n/LocaleProvider";
import { SurveyForm } from "./SurveyForm";

const id = "019f0000-0000-7000-8000-000000000051";
const headers = {
  "Cache-Control": "no-store",
  "X-Request-ID": "request-survey-form-test",
};
const published = {
  id,
  questionnaire_id: "019f0000-0000-7000-8000-000000000052",
  stable_key: "tourism_profile",
  version_number: 1,
  revision: 1,
  title: "Pesquisa turística",
  introduction: "Participação voluntária.",
  privacy_notice_version: "notice-v1",
  questions: [
    {
      id: "019f0000-0000-7000-8000-000000000053",
      stable_key: "first_visit",
      prompt: "Primeira visita?",
      answer_type: "boolean",
      required: false,
      purpose_code: "tourism_planning",
      display_order: 1,
      options: [],
    },
  ],
  consent_requirements: [
    {
      purpose_code: "tourism_planning",
      notice_version: "notice-v1",
      prompt: "Aceito participar.",
      required_for_answers: true,
      display_order: 1,
    },
  ],
};

function renderSurvey() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <LocaleProvider initial="pt">
        <SurveyForm />
      </LocaleProvider>
    </QueryClientProvider>,
  );
}

describe("pesquisa pública", () => {
  afterEach(async () => {
    cleanup();
    clearSurveyCapability();
    window.history.replaceState(null, "", "/pesquisa");
    await clearAllDrafts();
    vi.unstubAllGlobals();
  });

  // Tela de hóspede: `problem.title` é escrito para quem opera o serviço.
  it("não mostra ao hóspede o texto de erro do servidor que não sabe explicar", async () => {
    const engineeringTitle = "Upstream dependency returned an unexpected shape.";
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValue(
        Response.json(
          { type: "about:blank", title: engineeringTitle, status: 503 },
          {
            status: 503,
            headers: { ...headers, "Content-Type": "application/problem+json" },
          },
        ),
      ),
    );
    setSurveyCapability("payload.signature");

    renderSurvey();

    expect(
      await screen.findByText(/Não conseguimos falar com o serviço/u),
    ).toBeInTheDocument();
    expect(document.documentElement.innerHTML).not.toContain(engineeringTitle);
  });

  it.each([
    [429, "Muitas tentativas", 60, "Já houve envios demais desta rede", "rate-limited"],
    [409, "Requisição em processamento", 3, "Já recebemos este envio", "idempotency-in-progress"],
  ])(
    "status %s com Retry-After usa a cópia própria e mantém o prazo",
    async (status, serverTitle, seconds, expectedCopy, problemCode) => {
      vi.stubGlobal(
        "fetch",
        vi.fn<typeof fetch>().mockResolvedValue(
          Response.json(
            {
            type: `https://turismo.prado.ba.gov.br/problems/${problemCode}`,
            title: serverTitle,
            status,
          },
            {
              status,
              headers: {
                ...headers,
                "Content-Type": "application/problem+json",
                "Retry-After": String(seconds),
              },
            },
          ),
        ),
      );
      setSurveyCapability("payload.signature");

      renderSurvey();

      expect(
        await screen.findByText(new RegExp(expectedCopy, "u")),
      ).toBeInTheDocument();
      expect(screen.getByText(new RegExp(`${seconds} segundos`, "u")))
        .toBeInTheDocument();
      expect(document.documentElement.innerHTML).not.toContain(serverTitle);
    },
  );

  it("falha fechada sem capability e não consulta a API", () => {
    const fetcher = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetcher);

    renderSurvey();

    expect(
      screen.getByRole("heading", { name: "Pesquisa disponível após o registro" }),
    ).toBeInTheDocument();
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("registra recusa sem respostas nem consentimentos e elimina authority", async () => {
    const user = userEvent.setup();
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(Response.json(published, { headers: { ...headers, ETag: '"1"' } }))
      .mockResolvedValueOnce(
        Response.json(
          { submission_id: id, participation: "declined", status: "accepted" },
          { headers: { ...headers, "Idempotency-Replayed": "false" } },
        ),
      );
    vi.stubGlobal("fetch", fetcher);
    setSurveyCapability("payload.signature");
    renderSurvey();

    await screen.findByRole("heading", { name: "Pesquisa turística", level: 2 });
    await user.click(screen.getByRole("button", { name: "Não quero participar" }));

    expect(
      await screen.findByRole("heading", { name: "Participação registrada" }),
    ).toBeInTheDocument();
    const request = fetcher.mock.calls[1]?.[0] as Request;
    const body = await request.clone().json();
    expect(body).toMatchObject({
      participation: "declined",
      answers: [],
      consent_decisions: [],
    });
    expect(request.headers.get("Survey-Capability")).toBe("payload.signature");
    expect(peekSurveyCapability()).toBeNull();
  });

  it("não apresenta violações axe no formulário publicado", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockImplementation(() =>
        Promise.resolve(
          Response.json(published, { headers: { ...headers, ETag: '"1"' } }),
        ),
      ),
    );
    setSurveyCapability("payload.signature");
    const { container } = renderSurvey();
    await screen.findByRole("heading", { name: "Pesquisa turística", level: 2 });

    const report = await axe.run(container, {
      rules: { "color-contrast": { enabled: false } },
    });

    expect(report.violations).toEqual([]);
  });

  it("renderiza canários HTML e script somente como texto", async () => {
    const user = userEvent.setup();
    const canary = `<img src=x onerror="globalThis.surveyCanary=true"><script>surveyCanary()</script>`;
    const unsafePublished = {
      ...published,
      title: canary,
      introduction: canary,
      questions: [{ ...published.questions[0], prompt: canary, help_text: canary }],
    };
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValue(
        Response.json(unsafePublished, { headers: { ...headers, ETag: '"1"' } }),
      ),
    );
    setSurveyCapability("payload.signature");
    const { container } = renderSurvey();

    await screen.findByRole("heading", { name: canary, level: 2 });
    await user.click(screen.getByRole("checkbox", { name: /Aceito participar/ }));
    expect(screen.getAllByText(canary).length).toBeGreaterThanOrEqual(4);
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("script")).toBeNull();
    expect(
      (globalThis as typeof globalThis & { surveyCanary?: boolean }).surveyCanary,
    ).toBeUndefined();
  });

  it("recupera rascunho cifrado por índice não sensível sem capability persistida", async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockImplementation(() =>
        Promise.resolve(
          Response.json(published, { headers: { ...headers, ETag: '"1"' } }),
        ),
      ),
    );
    setSurveyCapability("payload.signature");
    const first = renderSurvey();
    await screen.findByRole("heading", { name: "Pesquisa turística", level: 2 });
    await user.click(screen.getByRole("checkbox", { name: /Aceito participar/ }));
    await user.selectOptions(screen.getByRole("combobox"), "true");
    await user.click(screen.getByRole("button", { name: "Salvar rascunho cifrado" }));
    await screen.findByText(/sem a capability/);

    const draftId = new URLSearchParams(window.location.hash.slice(1)).get(
      "survey-draft",
    );
    expect(draftId).toMatch(/^[0-9a-f-]{36}$/i);
    const record = await inspectDraftRecord(draftId!);
    expect(JSON.stringify(record)).not.toContain("payload.signature");

    first.unmount();
    renderSurvey();
    await screen.findByText("Rascunho cifrado recuperado neste dispositivo.");
    expect(screen.getByRole("checkbox", { name: /Aceito participar/ })).toBeChecked();
    expect(screen.getByRole("combobox")).toHaveValue("true");
  });
});
