import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, describe, expect, it, vi } from "vitest";

import InviteRequestPage from "../../pages/InviteRequestPage";
import { LocaleProvider } from "../../shared/i18n/LocaleProvider";

const accessContext = {
  privacy_notice_version: "2026-07",
  proof_of_work: {
    algorithm: "sha256-leading-zero-bits",
    challenge: "Y3VtdXJ1LWFjY2Vzcy1yZXF1ZXN0LWNoYWxsZW5nZS0wMQ",
    difficulty_bits: 4,
    expires_at: "2026-08-19T23:59:00Z",
  },
};

function apiResponse(body: unknown, init: ResponseInit = {}) {
  const headers = new Headers(init.headers);
  headers.set("Cache-Control", "no-store");
  headers.set("X-Request-ID", "request-invite-request-test");
  return Response.json(body, { ...init, headers });
}

function acceptedResponse() {
  return apiResponse(
    {
      id: "018f4e59-7a2a-7b12-8fd7-5d2e8dc99b80",
      created_at: "2026-08-18T12:00:00Z",
    },
    { status: 201, headers: { "Idempotency-Replayed": "false" } },
  );
}

function problemResponse(status: number, type: string) {
  return apiResponse(
    { type, title: "Conflito de estado.", status },
    { status, headers: { "Content-Type": "application/problem+json" } },
  );
}

/**
 * `vi.spyOn` sem implementação própria cai de volta no `fetch` original assim
 * que a fila de `…Once` acaba, e uma requisição não prevista sairia de verdade
 * do processo de teste. A implementação padrão recusa: o teste falha na chamada
 * inesperada, não no tempo limite.
 */
function stubFetch() {
  return vi
    .spyOn(globalThis, "fetch")
    .mockImplementation((input) =>
      Promise.reject(
        new Error(`Requisição não prevista: ${String((input as Request).url)}`),
      ),
    );
}

function renderPage(locale: "pt" | "en" | "es" = "pt") {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <LocaleProvider initial={locale}>
        <InviteRequestPage />
      </LocaleProvider>
    </QueryClientProvider>,
  );
}

async function renderLoadedForm() {
  const fetcher = stubFetch().mockResolvedValueOnce(apiResponse(accessContext));
  const view = renderPage();
  await screen.findByRole("heading", {
    name: "Dados da hospedagem e do contato",
  });
  return { fetcher, view };
}

async function fillForm(user: ReturnType<typeof userEvent.setup>) {
  await user.type(
    screen.getByLabelText("Nome da hospedagem"),
    "Pousada Fictícia da Barra",
  );
  await user.selectOptions(
    screen.getByLabelText("Tipo de hospedagem"),
    "seasonal_rental",
  );
  await user.type(
    screen.getByLabelText("Quantas pessoas você hospeda de uma vez"),
    "12",
  );
  await user.type(screen.getByLabelText("Cidade"), "Prado");
  await user.type(screen.getByLabelText("Estado (UF)"), "ba");
  await user.type(screen.getByLabelText("Seu nome"), "Marta Ribeiro");
  await user.type(screen.getByLabelText("Seu e-mail"), "marta@exemplo.com");
}

describe("pedido de acesso da hospedagem", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("busca o contexto público sem sessão e sem token", async () => {
    const { fetcher } = await renderLoadedForm();

    const request = fetcher.mock.calls[0]?.[0] as Request;
    expect(request.method).toBe("GET");
    expect(new URL(request.url).pathname).toBe(
      "/api/v1/accommodation-access-requests/context",
    );
    expect(request.headers.get("Authorization")).toBeNull();
    expect(
      screen.getByRole("heading", {
        level: 1,
        name: "Peça seu acesso ao Observatório",
      }),
    ).toBeInTheDocument();
  });

  it("mostra os dois grupos de campos e o telefone como opcional", async () => {
    await renderLoadedForm();

    expect(
      screen.getByRole("group", { name: "Sobre a hospedagem" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("group", { name: "Sobre quem responde" }),
    ).toBeInTheDocument();
    expect(
      screen.getByLabelText("Seu telefone ou WhatsApp (opcional)"),
    ).not.toBeRequired();
    expect(
      screen.getByRole("option", { name: "Em processo de regularização" }),
    ).toBeInTheDocument();
  });

  it("recusa o formulário vazio antes de gastar a prova de trabalho", async () => {
    const user = userEvent.setup();
    const { fetcher } = await renderLoadedForm();

    await user.click(screen.getByRole("button", { name: "Enviar pedido" }));

    const name = screen.getByLabelText("Nome da hospedagem");
    await waitFor(() => expect(name).toHaveAttribute("aria-invalid", "true"));
    expect(name).toHaveFocus();
    expect(name).toHaveAccessibleDescription(
      "Escreva o nome da hospedagem, com até 200 letras.",
    );
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("recusa e-mail malformado apontando o campo do e-mail", async () => {
    const user = userEvent.setup();
    const { fetcher } = await renderLoadedForm();
    await fillForm(user);
    await user.clear(screen.getByLabelText("Seu e-mail"));
    await user.type(screen.getByLabelText("Seu e-mail"), "marta arroba casa");

    await user.click(screen.getByRole("button", { name: "Enviar pedido" }));

    const email = screen.getByLabelText("Seu e-mail");
    await waitFor(() => expect(email).toHaveAttribute("aria-invalid", "true"));
    expect(email).toHaveFocus();
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("resolve a prova de trabalho e envia o pedido com chave de idempotência", async () => {
    const user = userEvent.setup();
    const { fetcher } = await renderLoadedForm();
    fetcher.mockResolvedValueOnce(acceptedResponse());
    await fillForm(user);

    await user.click(screen.getByRole("button", { name: "Enviar pedido" }));

    await screen.findByRole("heading", { name: "Pedido enviado" });
    const submission = fetcher.mock.calls[1]?.[0] as Request;
    expect(submission.method).toBe("POST");
    expect(new URL(submission.url).pathname).toBe(
      "/api/v1/accommodation-access-requests",
    );
    const body = (await submission.clone().json()) as Record<string, unknown>;
    expect(submission.headers.get("Idempotency-Key")).toBe(
      body["client_submission_id"],
    );
    expect(body).toMatchObject({
      accommodation_name: "Pousada Fictícia da Barra",
      capacity: 12,
      category: "seasonal_rental",
      city_label: "Prado",
      contact_email: "marta@exemplo.com",
      contact_name: "Marta Ribeiro",
      privacy_notice_version: "2026-07",
      state_code: "BA",
    });
    // Opcional em branco não vira string vazia no corpo.
    expect(Object.keys(body)).not.toContain("contact_phone");
    expect(body["proof_of_work"]).toMatchObject({
      challenge: accessContext.proof_of_work.challenge,
    });
  });

  it("explica o que acontece depois sem prometer prazo", async () => {
    const user = userEvent.setup();
    const { fetcher } = await renderLoadedForm();
    fetcher.mockResolvedValueOnce(acceptedResponse());
    await fillForm(user);

    await user.click(screen.getByRole("button", { name: "Enviar pedido" }));

    const done = await screen.findByRole("heading", { name: "Pedido enviado" });
    expect(done).toHaveFocus();
    expect(screen.getByRole("status")).toHaveTextContent("marta@exemplo.com");
    expect(
      screen.getByText(/Não há data certa para a resposta/u),
    ).toBeInTheDocument();
    expect(
      screen.queryByLabelText("Nome da hospedagem"),
    ).not.toBeInTheDocument();
  });

  it("dá mensagem própria ao 409 e devolve o foco ao campo de e-mail", async () => {
    const user = userEvent.setup();
    const { fetcher } = await renderLoadedForm();
    fetcher.mockResolvedValueOnce(
      problemResponse(409, "urn:cumuru:problem:access-request-pending"),
    );
    await fillForm(user);

    await user.click(screen.getByRole("button", { name: "Enviar pedido" }));

    expect(
      await screen.findByText(/Já existe um pedido em análise para este e-mail/u),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Seu e-mail")).toHaveFocus();
    expect(
      screen.queryByRole("heading", { name: "Pedido enviado" }),
    ).not.toBeInTheDocument();
    // `problem.title` é escrito para quem opera o serviço, não para a pousada.
    expect(document.documentElement.innerHTML).not.toContain(
      "Conflito de estado.",
    );
  });

  it("oferece tentar de novo quando o contexto não carrega", async () => {
    const user = userEvent.setup();
    const fetcher = stubFetch().mockRejectedValueOnce(
      new TypeError("Failed to fetch"),
    );
    renderPage();
    await screen.findByText(/Sem conexão agora/u);

    fetcher.mockResolvedValueOnce(apiResponse(accessContext));
    await user.click(screen.getByRole("button", { name: "Tentar de novo" }));

    expect(
      await screen.findByRole("heading", {
        name: "Dados da hospedagem e do contato",
      }),
    ).toBeInTheDocument();
  });

  it("não apresenta violações axe no formulário carregado", async () => {
    const { view } = await renderLoadedForm();

    const report = await axe.run(view.container, {
      rules: { "color-contrast": { enabled: false } },
    });

    expect(report.violations).toEqual([]);
  });
});

// A tela é pública e chega a quem nunca falou com o Observatório, então a
// paridade PT/EN/ES é asserida sobre o texto traduzido de verdade: um
// componente que ignorasse o idioma e renderizasse sempre português passaria
// num teste de "não lançou erro" e falharia aqui.
describe.each([
  {
    capacityLabel: "Quantas pessoas você hospeda de uma vez",
    cityLabel: "Cidade",
    completionTitle: "Pedido enviado",
    contactEmailLabel: "Seu e-mail",
    contactNameLabel: "Seu nome",
    categoryLabel: "Tipo de hospedagem",
    formTitle: "Dados da hospedagem e do contato",
    locale: "pt" as const,
    nameLabel: "Nome da hospedagem",
    stateLabel: "Estado (UF)",
    submitLabel: "Enviar pedido",
  },
  {
    capacityLabel: "How many people you host at once",
    cityLabel: "City",
    completionTitle: "Request sent",
    contactEmailLabel: "Your email",
    contactNameLabel: "Your name",
    categoryLabel: "Type of lodging",
    formTitle: "Lodging and contact details",
    locale: "en" as const,
    nameLabel: "Lodging name",
    stateLabel: "State (two letters)",
    submitLabel: "Send request",
  },
  {
    capacityLabel: "Cuántas personas aloja a la vez",
    cityLabel: "Ciudad",
    completionTitle: "Solicitud enviada",
    contactEmailLabel: "Su correo electrónico",
    contactNameLabel: "Su nombre",
    categoryLabel: "Tipo de alojamiento",
    formTitle: "Datos del alojamiento y del contacto",
    locale: "es" as const,
    nameLabel: "Nombre del alojamiento",
    stateLabel: "Estado (dos letras)",
    submitLabel: "Enviar solicitud",
  },
])("i18n do pedido de acesso ($locale)", (labels) => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("mostra rótulos e confirmação no idioma escolhido", async () => {
    const user = userEvent.setup();
    const fetcher = stubFetch().mockResolvedValueOnce(
      apiResponse(accessContext),
    );
    renderPage(labels.locale);
    await screen.findByRole("heading", { name: labels.formTitle });

    fetcher.mockResolvedValueOnce(acceptedResponse());
    await user.type(screen.getByLabelText(labels.nameLabel), "Casa da Duna");
    await user.selectOptions(
      screen.getByLabelText(labels.categoryLabel),
      "family_hosting",
    );
    await user.type(screen.getByLabelText(labels.capacityLabel), "6");
    await user.type(screen.getByLabelText(labels.cityLabel), "Prado");
    await user.type(screen.getByLabelText(labels.stateLabel), "BA");
    await user.type(screen.getByLabelText(labels.contactNameLabel), "Marta");
    await user.type(
      screen.getByLabelText(labels.contactEmailLabel),
      "marta@exemplo.com",
    );
    await user.click(screen.getByRole("button", { name: labels.submitLabel }));

    expect(
      await screen.findByRole("heading", { name: labels.completionTitle }),
    ).toBeInTheDocument();
  });
});
