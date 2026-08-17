import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import SelfRegistrationPage from "../../pages/SelfRegistrationPage";
import {
  captureSelfServiceCapability,
  clearSelfServiceCapability,
} from "../../shared/security/self-service-capability";
import { clearSurveyCapability } from "../../shared/security/survey-capability";
import {
  clearAllDrafts,
  inspectDraftRecord,
  listDraftIds,
  loadDraft,
} from "../../shared/offline/encrypted-drafts";

const capability = "s".repeat(96);
const inviteContext = {
  accommodation_name: "Pousada Fictícia da Barra",
  privacy_notice_version: "2026-07",
  consent_requirements: [
    {
      purpose_code: "tourism_research",
      notice_version: "2026-07",
      prompt: "Aceito responder a pesquisa voluntária depois da aprovação.",
      required_for_answers: false,
      display_order: 1,
    },
  ],
  proof_of_work: {
    algorithm: "sha256-leading-zero-bits",
    challenge: "Y3VtdXJ1LXBvdy1zZWxmLXNlcnZpY2UtY2hhbGxlbmdlLTAx",
    difficulty_bits: 4,
    expires_at: "2026-08-16T23:59:00Z",
  },
};

function apiResponse(body: unknown, init: ResponseInit = {}) {
  const headers = new Headers(init.headers);
  headers.set("Cache-Control", "no-store");
  headers.set("X-Request-ID", "request-self-registration-test");
  return Response.json(body, { ...init, headers });
}

function capture() {
  captureSelfServiceCapability(
    new URL(`https://cumuru.invalid/i#${capability}`),
    vi.fn(),
  );
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
    {
      status: 200,
      headers: { ETag: '"1"', "Idempotency-Replayed": "false" },
    },
  );
}

/**
 * Dá ao loop de eventos tiques de macrotask suficientes para que qualquer envio
 * concorrente já tenha resolvido a prova de trabalho e alcançado o `fetch`. Com
 * dificuldade 4 a solução sai dentro do primeiro lote, sem nenhum yield, então
 * poucos tiques bastam e o resultado é determinístico.
 */
async function settleEventLoop(ticks = 12) {
  for (let tick = 0; tick < ticks; tick += 1) {
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
}

function postCount(fetcher: { mock: { calls: unknown[][] } }) {
  return fetcher.mock.calls.filter(
    ([request]) => (request as Request).method === "POST",
  ).length;
}

async function fillResidence(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText("UF de residência do visitante 1"), "BA");
  await user.type(
    screen.getByLabelText("Município IBGE do visitante 1"),
    "2925509",
  );
}

async function renderLoadedForm() {
  capture();
  const fetcher = vi
    .spyOn(globalThis, "fetch")
    .mockResolvedValueOnce(apiResponse(inviteContext));
  const view = render(<SelfRegistrationPage />);
  await screen.findByRole("heading", { name: /Autocadastro/u });
  await screen.findByText(inviteContext.accommodation_name);
  return { fetcher, view };
}

describe("autocadastro pelo cartaz da acomodação", () => {
  beforeEach(async () => {
    clearSelfServiceCapability();
    clearSurveyCapability();
    await clearAllDrafts();
    window.history.replaceState(null, "", "/i");
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("falha fechada e não chama a API sem o token do cartaz", () => {
    const fetcher = vi.spyOn(globalThis, "fetch");

    render(<SelfRegistrationPage />);

    expect(
      screen.getByRole("heading", { name: "Cartaz necessário" }),
    ).toBeInTheDocument();
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("busca o contexto com o token em cabeçalho e nunca o revela na página", async () => {
    const { fetcher } = await renderLoadedForm();

    const request = fetcher.mock.calls[0]?.[0] as Request;
    expect(request.headers.get("X-Cumuru-Invite-Token")).toBe(capability);
    expect(new URL(request.url).pathname).toBe("/api/v1/accommodation-invite");
    expect(new URL(request.url).search).toBe("");
    expect(document.documentElement.innerHTML).not.toContain(capability);
  });

  it("exibe o aviso de privacidade versionado antes do primeiro campo", async () => {
    await renderLoadedForm();

    const notice = screen.getByRole("region", {
      name: "Aviso de privacidade",
    });
    const firstField = screen.getByLabelText("Papel do visitante 1");
    expect(notice).toBeInTheDocument();
    expect(notice).toHaveTextContent("2026-07");
    expect(
      notice.compareDocumentPosition(firstField) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("não oferece papel de menor nem qualquer campo de identidade", async () => {
    await renderLoadedForm();

    expect(
      screen.queryByRole("option", { name: "Menor" }),
    ).not.toBeInTheDocument();
    for (const label of [/nome/iu, /documento/iu, /cpf/iu, /e-?mail/iu, /telefone/iu]) {
      expect(screen.queryByLabelText(label)).not.toBeInTheDocument();
    }
  });

  // Metade cliente do par com TestPosterContextEmitsNoConsentField do backend.
  // O contexto do cartaz ainda declara consent_requirements — a fixture acima
  // traz um —, mas não existe onde gravar a decisão: survey.consent_decisions
  // exige response_id de uma resposta de questionário real. Oferecer o controle
  // aqui seria afordância falsa, e um registro de consentimento que ninguém
  // grava é pior do que nenhum, porque simula conformidade.
  it("não oferece controle de consentimento que não teria onde ser gravado", async () => {
    await renderLoadedForm();

    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    expect(
      screen.queryByText(inviteContext.consent_requirements[0]!.prompt),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("region", { name: "Aviso de privacidade" }),
    ).toHaveTextContent(inviteContext.privacy_notice_version);
  });

  it("resolve a prova de trabalho e envia somente dados generalizados", async () => {
    const user = userEvent.setup();
    const { fetcher } = await renderLoadedForm();
    fetcher.mockResolvedValueOnce(acceptedResponse());
    await fillResidence(user);

    await user.click(screen.getByRole("button", { name: "Enviar autocadastro" }));

    await screen.findByRole("heading", { name: "Autocadastro enviado" });
    const submission = fetcher.mock.calls[1]?.[0] as Request;
    expect(submission.method).toBe("POST");
    expect(submission.headers.get("X-Cumuru-Invite-Token")).toBe(capability);
    const body = (await submission.clone().json()) as Record<string, unknown>;
    // Conjunto EXATO, nunca subconjunto: se esta asserção virar "contém", a
    // minimização deixa de ser provada e um campo de identidade poderia
    // reaparecer sem quebrar teste nenhum.
    expect(Object.keys(body).sort()).toEqual([
      "client_submission_id",
      "planned_arrival_on",
      "planned_departure_on",
      "privacy_notice_version",
      "proof_of_work",
      "visitors",
    ]);
    expect(JSON.stringify(body)).not.toContain(capability);
    expect(body["proof_of_work"]).toMatchObject({
      challenge: inviteContext.proof_of_work.challenge,
    });
  });

  // Reproduz o envio sobreposto: a rede oscila, o ouvinte de `online` dispara
  // enquanto a prova de trabalho ainda roda, e sem a guarda de `ref` o segundo
  // envio resolve a prova de novo e gasta o mesmo desafio, perdendo por replay.
  // A `Idempotency-Key` estável protegeria o dado no servidor, mas quem paga a
  // CPU e a bateria é o aparelho fraco do canal aberto.
  it("ignora o retry de rede enquanto um envio está em voo", async () => {
    const user = userEvent.setup();
    const { fetcher } = await renderLoadedForm();
    let releaseSubmission!: (response: Response) => void;
    fetcher.mockReturnValueOnce(
      new Promise<Response>((resolve) => {
        releaseSubmission = resolve;
      }),
    );
    await fillResidence(user);

    await user.click(screen.getByRole("button", { name: "Enviar autocadastro" }));

    // O primeiro POST segue pendurado. Sem a guarda, cada `online` inicia outro
    // envio que resolve a prova e chega ao fetch em poucos tiques — por isso o
    // teste assenta o event loop ANTES de liberar a resposta, em vez de correr
    // contra a conclusão do primeiro.
    await act(async () => {
      window.dispatchEvent(new Event("online"));
      window.dispatchEvent(new Event("online"));
      window.dispatchEvent(new Event("online"));
      await settleEventLoop();
    });

    expect(postCount(fetcher)).toBe(1);

    await act(async () => {
      releaseSubmission(acceptedResponse());
      await settleEventLoop();
    });

    await screen.findByRole("heading", { name: "Autocadastro enviado" });
    expect(postCount(fetcher)).toBe(1);
  });

  // D-06. O sinal de conclusão do R6 sobrevive ao token de propósito, mas
  // precisa morrer com ele: sem isso, descartar a capability deixa a aba
  // afirmando que houve um autocadastro que ela não pode mais comprovar. Sob
  // ordem embaralhada isso vazava para o teste de fail-closed, em 2 de 5
  // execuções; aqui a sequência é explícita e o resultado não depende de sorte.
  it("esquece a conclusão quando a capability é descartada", async () => {
    const user = userEvent.setup();
    const { fetcher } = await renderLoadedForm();
    fetcher.mockResolvedValueOnce(acceptedResponse());
    await fillResidence(user);
    await user.click(screen.getByRole("button", { name: "Enviar autocadastro" }));
    await screen.findByRole("heading", { name: "Autocadastro enviado" });

    // É o que o fim de sessão e um boot sem fragmento fazem.
    clearSelfServiceCapability();
    cleanup();
    render(<SelfRegistrationPage />);

    expect(
      screen.getByRole("heading", { name: "Cartaz necessário" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Autocadastro enviado" }),
    ).not.toBeInTheDocument();
  });

  // O hóspede não pode receber `problem.title`: ele é escrito para quem opera o
  // serviço. Com a API fora do ar, a tela pública chegou a mostrar
  // "O serviço respondeu fora do contrato." a quem só queria se cadastrar.
  it("não mostra ao hóspede o texto de erro do servidor que não sabe explicar", async () => {
    capture();
    const engineeringTitle = "Upstream dependency returned an unexpected shape.";
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      apiResponse(
        { type: "about:blank", title: engineeringTitle, status: 503 },
        { status: 503, headers: { "Content-Type": "application/problem+json" } },
      ),
    );

    render(<SelfRegistrationPage />);

    // A mesma região `role="status"` mostra "Validando o cartaz…" na primeira
    // renderização e só troca de texto depois que a promessa do fetch rejeita.
    // findByRole resolve assim que o elemento existe, não quando o texto certo
    // chega — aguardar pelo texto evita a corrida contra essa atualização
    // assíncrona.
    await screen.findByText(/Não conseguimos falar com o serviço/u);
    expect(document.documentElement.innerHTML).not.toContain(engineeringTitle);
  });

  it("anuncia a espera de aprovação de 72 horas na conclusão", async () => {
    const user = userEvent.setup();
    const { fetcher } = await renderLoadedForm();
    fetcher.mockResolvedValueOnce(acceptedResponse());
    await fillResidence(user);

    await user.click(screen.getByRole("button", { name: "Enviar autocadastro" }));

    expect(
      await screen.findByRole("heading", { name: "Autocadastro enviado" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(/72 horas/u);
  });

  it("recusa antes da prova de trabalho quando o formulário é inválido", async () => {
    const user = userEvent.setup();
    const { fetcher } = await renderLoadedForm();

    await user.click(screen.getByRole("button", { name: "Enviar autocadastro" }));

    const state = screen.getByLabelText("UF de residência do visitante 1");
    await waitFor(() => expect(state).toHaveAttribute("aria-invalid", "true"));
    expect(state).toHaveFocus();
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("informa a espera do 429 sem descartar o que foi preenchido", async () => {
    const user = userEvent.setup();
    const { fetcher } = await renderLoadedForm();
    fetcher.mockResolvedValueOnce(
      apiResponse(
        {
          type: "urn:cumuru:problem:rate-limited",
          title: "Muitas tentativas.",
          status: 429,
        },
        { status: 429, headers: { "Retry-After": "30" } },
      ),
    );
    await fillResidence(user);

    await user.click(screen.getByRole("button", { name: "Enviar autocadastro" }));

    expect(await screen.findByText(/30 segundos/u)).toBeInTheDocument();
    expect(screen.getByLabelText("UF de residência do visitante 1")).toHaveValue(
      "BA",
    );
  });

  it("guarda o rascunho cifrado sem token quando a rede falha", async () => {
    const user = userEvent.setup();
    const { fetcher } = await renderLoadedForm();
    fetcher.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    await fillResidence(user);

    await user.click(screen.getByRole("button", { name: "Enviar autocadastro" }));

    await screen.findByText(/cifrado, neste dispositivo/u);
    const [draftId] = await listDraftIds();
    expect(draftId).toBeDefined();
    const record = await inspectDraftRecord(draftId!);
    expect(JSON.stringify(record)).not.toContain(capability);
    const draft = await loadDraft<Record<string, unknown>>(draftId!);
    expect(JSON.stringify(draft)).not.toContain(capability);
    expect(Object.keys(draft ?? {})).not.toContain("proof_of_work");
  });

  it("descarta o rascunho quando o cartaz deixa de valer", async () => {
    const user = userEvent.setup();
    const { fetcher } = await renderLoadedForm();
    fetcher.mockResolvedValueOnce(
      apiResponse(
        {
          type: "urn:cumuru:problem:invite-not-found",
          title: "Cartaz inválido.",
          status: 404,
        },
        { status: 404, headers: { "Content-Type": "application/problem+json" } },
      ),
    );
    await fillResidence(user);

    await user.click(screen.getByRole("button", { name: "Enviar autocadastro" }));

    await screen.findByText(/não é mais válido/u);
    await expect(listDraftIds()).resolves.toEqual([]);
  });

  it("não apresenta violações axe no formulário carregado", async () => {
    const { view } = await renderLoadedForm();

    const report = await axe.run(view.container, {
      rules: { "color-contrast": { enabled: false } },
    });

    expect(report.violations).toEqual([]);
  });
});
