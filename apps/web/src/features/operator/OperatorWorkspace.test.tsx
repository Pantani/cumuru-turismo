import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AuthSessionProvider } from "../../shared/auth/AuthSession";
import { peekInviteCapability } from "../../shared/security/invite-capability";
import { OperatorWorkspace } from "./OperatorWorkspace";

const { toCanvas } = vi.hoisted(() => ({ toCanvas: vi.fn() }));

vi.mock("qrcode", () => ({ default: { toCanvas } }));

const uuid = "018f4e59-7a2a-7b12-8fd7-5d2e8dc99b80";

function phase2Response(body: unknown, init: ResponseInit = {}) {
  const headers = new Headers(init.headers);
  headers.set("Cache-Control", "no-store");
  headers.set("X-Request-ID", "request-operator-workspace-test");
  return Response.json(body, { ...init, headers });
}

function renderWorkspace(localDemo = false) {
  const queryClient = new QueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <AuthSessionProvider
        accessToken="opaque-test-token"
        localDemo={localDemo}
      >
        <OperatorWorkspace />
      </AuthSessionProvider>
    </QueryClientProvider>,
  );
}

function stayResponse() {
  return {
    id: uuid,
    status: "draft",
    version: 1,
  };
}

function accommodationResponse(
  category: "formal_lodging" | "family_hosting",
  cadasturId: string | null = null,
) {
  return {
    id: uuid,
    organization_id: uuid,
    name: category === "formal_lodging" ? "Pousada Demo" : "Casa de Família",
    category,
    status: "active" as const,
    cadastur_id: cadasturId,
    capacity: 8,
    version: 1,
    created_at: "2026-08-02T12:00:00Z",
    updated_at: "2026-08-02T12:00:00Z",
  };
}

async function revealOnboarding(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "Listar acomodações" }));
  return screen.findByRole("form", { name: "Cadastrar meu local" });
}

async function fillOnboarding(
  user: ReturnType<typeof userEvent.setup>,
  form: HTMLElement,
  category: "formal_lodging" | "family_hosting",
) {
  await user.type(within(form).getByLabelText("Nome do local"), "Local Fictício");
  await user.selectOptions(within(form).getByLabelText("Tipo"), category);
  await user.clear(within(form).getByLabelText("Capacidade aproximada"));
  await user.type(within(form).getByLabelText("Capacidade aproximada"), "8");
}

describe("workspace operacional", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it("renderiza projeção mínima e captura o ETag retornado", async () => {
    const user = userEvent.setup();
    const fetcher = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        phase2Response({ items: [], next_cursor: null }),
      )
      .mockResolvedValueOnce(
        phase2Response(
          {
            id: uuid,
            organization_id: uuid,
            name: "Pousada",
            category: "formal_lodging",
            status: "active",
            version: 7,
            created_at: "2026-07-28T12:00:00Z",
            updated_at: "2026-07-28T12:00:00Z",
          },
          { headers: { ETag: '"7"' } },
        ),
      );
    renderWorkspace();
    const section = screen.getByRole("region", {
      name: "Acomodações e vínculos",
    });

    await user.click(within(section).getByRole("button", {
      name: "Listar acomodações",
    }));
    expect(
      await within(section).findByText("Nenhuma acomodação disponível."),
    ).toBeInTheDocument();

    await user.type(within(section).getByLabelText("ID da acomodação"), uuid);
    await user.click(within(section).getByRole("button", {
      name: "Consultar acomodação",
    }));

    expect(await within(section).findByText(/Acomodação .*active/)).toHaveTextContent(
      `Acomodação ${uuid}: active, versão 7.`,
    );
    expect(within(section).getByLabelText("ETag da acomodação")).toHaveValue(
      '"7"',
    );
    expect(within(section).getByLabelText("ETag do vínculo")).toHaveValue("");
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("oferece cadastro simples na lista vazia e explica os dois trilhos", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      phase2Response({ items: [], next_cursor: null }),
    );
    renderWorkspace();

    const form = await revealOnboarding(user);

    const name = within(form).getByLabelText("Nome do local");
    const category = within(form).getByLabelText("Tipo");
    expect(name).toBeRequired();
    expect(name).toHaveAccessibleDescription(
      /não inclua CPF, CNPJ, documento, contato, chave FNRH ou outro dado pessoal/i,
    );
    expect(category).toBeRequired();
    expect(within(form).getByLabelText("Capacidade aproximada")).toBeRequired();
    expect(within(form).queryByLabelText(
      /CPF|CNPJ|Cadastur|FNRH|issuer|subject|tenant|organiza[cç][aã]o/i,
    )).not.toBeInTheDocument();
    expect(screen.getByText(
      "Observatório local: funciona sem CNPJ, Cadastur ou chave",
    )).toBeInTheDocument();
    expect(screen.getByText(
      "FNRH opcional: processo federal separado, quando aplicável",
    )).toBeInTheDocument();
    await user.click(within(form).getByRole("button", { name: "Cadastrar local" }));
    expect(within(form).getByText("Informe o nome do local.")).toBeInTheDocument();
    expect(name).toHaveFocus();
    expect(name).toHaveAttribute("aria-invalid", "true");
    expect(category).toHaveAttribute("aria-invalid", "true");

    await user.type(name, "Local sem dados pessoais");

    expect(name).toHaveAttribute("aria-invalid", "false");
    expect(category).toHaveAttribute("aria-invalid", "true");
    expect(within(form).queryByText("Informe o nome do local."))
      .not.toBeInTheDocument();
  });

  it("mostra Cadastur apenas como informação existente e permite outro local", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      phase2Response({
        items: [
          accommodationResponse(
            "formal_lodging",
            "CADASTUR-FICTICIO-NAO-VALIDO",
          ),
          { ...accommodationResponse("family_hosting"), id: `${uuid.slice(0, -1)}1` },
        ],
        next_cursor: null,
      }),
    );
    renderWorkspace();

    await user.click(screen.getByRole("button", { name: "Listar acomodações" }));

    expect(await screen.findByText(
      "Cadastur informado no cadastro existente: CADASTUR-FICTICIO-NAO-VALIDO",
    )).toBeInTheDocument();
    expect(screen.getByText("Cadastur: Não informado")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Cadastrar outro local" }));
    expect(screen.getByRole("form", { name: "Cadastrar meu local" })).toBeInTheDocument();
  });

  it("mantém capacidade vazia sem enviar NaN ao input controlado", async () => {
    const user = userEvent.setup();
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      phase2Response({ items: [], next_cursor: null }),
    );
    renderWorkspace();
    const form = await revealOnboarding(user);
    await user.type(within(form).getByLabelText("Nome do local"), "Local Fictício");
    await user.selectOptions(within(form).getByLabelText("Tipo"), "family_hosting");
    const capacity = within(form).getByLabelText("Capacidade aproximada");

    await user.clear(capacity);
    await user.click(within(form).getByRole("button", { name: "Cadastrar local" }));

    expect(capacity).toHaveValue(null);
    expect(capacity).toHaveFocus();
    expect(within(form).getByText(
      "Informe uma capacidade entre 1 e 10.000 pessoas.",
    )).toBeInTheDocument();
    expect(consoleError.mock.calls.some((call) =>
      call.some((value) => String(value).includes("NaN")),
    )).toBe(false);
  });

  it.each([
    ["formal_lodging", "Pousada, hotel ou meio de hospedagem"],
    ["family_hosting", "Hospedagem familiar"],
  ] as const)(
    "cadastra %s sem documentos e seleciona a acomodação criada",
    async (category, categoryLabel) => {
      const user = userEvent.setup();
      const fetcher = vi.spyOn(globalThis, "fetch")
        .mockResolvedValueOnce(phase2Response({ items: [], next_cursor: null }))
        .mockResolvedValueOnce(
          phase2Response(accommodationResponse(category), {
            status: 201,
            headers: {
              ETag: '"1"',
              Location: `/api/v1/accommodations/${uuid}`,
              "Idempotency-Replayed": "false",
            },
          }),
        );
      renderWorkspace();
      const form = await revealOnboarding(user);
      await fillOnboarding(user, form, category);

      expect(within(form).getByRole("option", { name: categoryLabel })).toBeInTheDocument();
      await user.click(within(form).getByRole("button", { name: "Cadastrar local" }));

      const request = fetcher.mock.calls[1]?.[0] as Request;
      const body = await request.json();
      expect(body).toEqual({
        name: "Local Fictício",
        category,
        capacity: 8,
        client_submission_id: expect.stringMatching(
          /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
        ),
      });
      expect(body).not.toHaveProperty("cpf");
      expect(body).not.toHaveProperty("cnpj");
      expect(body).not.toHaveProperty("cadastur_id");
      expect(body).not.toHaveProperty("fnrh_key");
      expect(body).not.toHaveProperty("organization_id");
      expect(body).not.toHaveProperty("oidc_subject");
      expect(request.headers.get("Idempotency-Key")).toBeTruthy();
      await waitFor(() => expect(
        within(screen.getByRole("region", { name: "Estadias" }))
          .getByLabelText("ID da acomodação"),
      ).toHaveValue(uuid));
    },
  );

  it("mantém os identificadores do cadastro no retry e renova após sucesso", async () => {
    const user = userEvent.setup();
    const fetcher = vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(phase2Response({ items: [], next_cursor: null }))
      .mockResolvedValueOnce(phase2Response(
        { type: "urn:cumuru:problem:conflict", title: "Conflito", status: 409 },
        { status: 409, headers: { "Content-Type": "application/problem+json" } },
      ))
      .mockResolvedValueOnce(phase2Response(accommodationResponse("family_hosting"), {
        status: 201,
        headers: {
          ETag: '"1"',
          Location: `/api/v1/accommodations/${uuid}`,
          "Idempotency-Replayed": "false",
        },
      }))
      .mockResolvedValueOnce(phase2Response(accommodationResponse("formal_lodging"), {
        status: 201,
        headers: {
          ETag: '"1"',
          Location: `/api/v1/accommodations/${uuid}`,
          "Idempotency-Replayed": "false",
        },
      }));
    renderWorkspace();
    const firstForm = await revealOnboarding(user);
    await fillOnboarding(user, firstForm, "family_hosting");
    const submit = within(firstForm).getByRole("button", { name: "Cadastrar local" });

    await user.click(submit);
    await within(firstForm).findByText("Conflito");
    await user.click(submit);
    await screen.findByText("Cadastrar local: concluído.");
    await user.click(screen.getByRole("button", { name: "Cadastrar outro local" }));
    const secondForm = screen.getByRole("form", { name: "Cadastrar meu local" });
    await fillOnboarding(user, secondForm, "formal_lodging");
    await user.click(within(secondForm).getByRole("button", { name: "Cadastrar local" }));

    const first = fetcher.mock.calls[1]?.[0] as Request;
    const retry = fetcher.mock.calls[2]?.[0] as Request;
    const next = fetcher.mock.calls[3]?.[0] as Request;
    expect(retry.headers.get("Idempotency-Key")).toBe(
      first.headers.get("Idempotency-Key"),
    );
    expect((await retry.clone().json()).client_submission_id).toBe(
      (await first.clone().json()).client_submission_id,
    );
    expect(next.headers.get("Idempotency-Key")).not.toBe(
      first.headers.get("Idempotency-Key"),
    );
    expect((await next.clone().json()).client_submission_id).not.toBe(
      (await first.clone().json()).client_submission_id,
    );
  });

  it("seleciona a primeira acomodação listada para criar a estadia", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      phase2Response({
        items: [
          {
            id: uuid,
            organization_id: uuid,
            name: "Pousada Demo",
            category: "formal_lodging",
            status: "active",
            version: 1,
            created_at: "2026-07-28T12:00:00Z",
            updated_at: "2026-07-28T12:00:00Z",
          },
        ],
        next_cursor: null,
      }),
    );
    renderWorkspace();

    await user.click(screen.getByRole("button", {
      name: "Listar acomodações",
    }));

    await waitFor(() =>
      expect(
        within(screen.getByRole("region", { name: "Estadias" }))
          .getByLabelText("ID da acomodação"),
      ).toHaveValue(uuid),
    );
    expect(screen.getByText(/Pousada Demo/)).toBeInTheDocument();
  });

  it("inicia a jornada local com datas da Bahia e aviso de privacidade", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-07-29T01:30:00Z"));

    renderWorkspace(true);

    const stays = screen.getByRole("region", { name: "Estadias" });
    expect(within(stays).getByLabelText("Chegada prevista")).toHaveValue(
      "2026-07-28",
    );
    expect(within(stays).getByLabelText("Saída prevista")).toHaveValue(
      "2026-07-30",
    );
    const lifecycle = screen.getByRole("region", {
      name: "Grupo, convite e ciclo da estadia",
    });
    expect(
      within(lifecycle).getByLabelText("Versão do aviso de privacidade"),
    ).toHaveValue("prototype-v1");
  });

  it.each([
    ["sem padrão sensível", "https://registro.invalid/registro", "/acesso"],
    ["com capability inválida", "https://registro.invalid/convites/curto", "/registro"],
    ["com URL malformada", "url-malformada", "/acesso"],
  ] as const)("preserva o convite quando a URL está %s", async (
    _case,
    inviteUrl,
    expectedPath,
  ) => {
    const user = userEvent.setup();
    window.history.replaceState(null, "", "/acesso");
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      phase2Response(
        {
          invite_id: uuid,
          url: inviteUrl,
          expires_at: "2026-08-03T12:00:00Z",
        },
        {
          status: 201,
          headers: {
            ETag: '"2"',
            Location: `/api/v1/stays/${uuid}/invite`,
            "Idempotency-Replayed": "false",
          },
        },
      ),
    );
    const popstate = vi.fn();
    window.addEventListener("popstate", popstate);
    renderWorkspace(true);
    const lifecycle = screen.getByRole("region", {
      name: "Grupo, convite e ciclo da estadia",
    });
    await user.type(within(lifecycle).getByLabelText("ID da estadia"), uuid);
    await user.type(within(lifecycle).getByLabelText("ETag da estadia"), '"1"');
    await user.click(
      within(lifecycle).getByRole("button", { name: "Criar QR de convite" }),
    );
    await user.click(await within(lifecycle).findByRole("button", {
      name: "Abrir registro neste navegador",
    }));

    expect(peekInviteCapability()).toBeNull();
    expect(popstate).not.toHaveBeenCalled();
    expect(within(lifecycle).getByRole("heading", {
      name: "Convite pronto para leitura local",
    })).toBeInTheDocument();
    expect(within(lifecycle).getByText(/QR permanece disponível/i))
      .toBeInTheDocument();
    expect(window.location.pathname).toBe(expectedPath);
    window.removeEventListener("popstate", popstate);
    window.history.replaceState(null, "", "/");
  });

  it("propaga a estadia criada e seu ETag para o passo de convite", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      phase2Response(stayResponse(), {
        status: 201,
        headers: {
          ETag: '"1"',
          Location: `/api/v1/stays/${uuid}`,
          "Idempotency-Replayed": "false",
        },
      }),
    );
    renderWorkspace();
    const stays = screen.getByRole("region", { name: "Estadias" });
    await user.type(within(stays).getByLabelText("ID da acomodação"), uuid);
    await user.type(
      within(stays).getByLabelText("Chegada prevista"),
      "2026-08-01",
    );
    await user.type(
      within(stays).getByLabelText("Saída prevista"),
      "2026-08-03",
    );
    await user.click(within(stays).getByRole("button", {
      name: "Criar estadia",
    }));

    const lifecycle = screen.getByRole("region", {
      name: "Grupo, convite e ciclo da estadia",
    });
    expect(await within(lifecycle).findByLabelText("ID da estadia")).toHaveValue(
      uuid,
    );
    expect(within(lifecycle).getByLabelText("ETag da estadia")).toHaveValue(
      '"1"',
    );
  });

  it("anuncia validação e foca o primeiro campo inválido", () => {
    renderWorkspace();
    const section = screen.getByRole("region", { name: "Estadias" });
    const button = within(section).getByRole("button", {
      name: "Criar estadia",
    });

    fireEvent.submit(button.closest("form") as HTMLFormElement);

    expect(
      within(section).getByText("Informe um identificador válido."),
    ).toBeInTheDocument();
    expect(within(section).getByLabelText("ID da acomodação")).toHaveFocus();
  });

  it("mantém a chave no retry e rotaciona somente após sucesso", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        phase2Response(
          {
            type: "urn:cumuru:problem:conflict",
            title: "Conflito",
            status: 409,
          },
          {
            status: 409,
            headers: {
              "Content-Type": "application/problem+json",
              "Retry-After": "1",
            },
          },
        ),
      )
      .mockResolvedValue(
        phase2Response(stayResponse(), {
          status: 201,
          headers: {
            ETag: '"1"',
            Location: `/api/v1/stays/${uuid}`,
            "Idempotency-Replayed": "false",
          },
        }),
      );
    renderWorkspace();
    const section = screen.getByRole("region", { name: "Estadias" });
    await user.type(within(section).getByLabelText("ID da acomodação"), uuid);
    await user.type(
      within(section).getByLabelText("Chegada prevista"),
      "2026-08-01",
    );
    await user.type(
      within(section).getByLabelText("Saída prevista"),
      "2026-08-03",
    );
    const create = within(section).getByRole("button", {
      name: "Criar estadia",
    });

    await user.click(create);
    await within(section).findByText(/Conflito/);
    await user.click(create);
    await within(section).findByText("Criar estadia: concluído.");
    await user.click(create);

    const calls = vi.mocked(globalThis.fetch).mock.calls;
    const first = calls[0]?.[0] as Request;
    const second = calls[1]?.[0] as Request;
    const third = calls[2]?.[0] as Request;
    const firstKey = first.headers.get("Idempotency-Key");
    expect(second.headers.get("Idempotency-Key")).toBe(firstKey);
    expect(third.headers.get("Idempotency-Key")).not.toBe(firstKey);
    const firstSubmission = (await first.clone().json()).client_submission_id;
    expect(firstSubmission).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
    );
    expect((await second.clone().json()).client_submission_id).toBe(
      firstSubmission,
    );
    expect((await third.clone().json()).client_submission_id).not.toBe(
      firstSubmission,
    );
  });

  it("mantém alterações concorrentes desabilitadas até receber ETag forte", () => {
    renderWorkspace();
    const accommodation = screen.getByRole("region", {
      name: "Acomodações e vínculos",
    });
    const stays = screen.getByRole("region", { name: "Estadias" });

    expect(
      within(accommodation).getByLabelText("ETag da acomodação"),
    ).toHaveAttribute("aria-invalid", "true");
    expect(
      within(accommodation).getByRole("button", {
        name: "Atualizar acomodação",
      }),
    ).toBeDisabled();
    expect(within(stays).getByLabelText("ETag da estadia")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(
      within(stays).getByRole("button", { name: "Atualizar estadia" }),
    ).toBeDisabled();
  });

  it("envia no fluxo assistido somente os valores generalizados editados", async () => {
    const user = userEvent.setup();
    const fetcher = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      phase2Response(
        {
          submission_id: uuid,
          status: "accepted",
        },
        {
          headers: {
            ETag: '"4"',
            "Idempotency-Replayed": "false",
            "Survey-Capability": "payload.signature",
          },
        },
      ),
    );
    renderWorkspace();
    const section = screen.getByRole("region", {
      name: "Grupo, convite e ciclo da estadia",
    });
    await user.type(within(section).getByLabelText("ID da estadia"), uuid);
    await user.type(
      within(section).getByLabelText("ETag da estadia"),
      '"3"',
    );
    await user.type(
      within(section).getByLabelText("Versão do aviso de privacidade"),
      "2026-07",
    );
    await user.selectOptions(
      within(section).getByLabelText("Faixa etária do visitante 1"),
      "60_plus",
    );
    await user.clear(
      within(section).getByLabelText("País de residência do visitante 1"),
    );
    await user.type(
      within(section).getByLabelText("País de residência do visitante 1"),
      "AR",
    );

    await user.click(
      within(section).getByRole("button", {
        name: "Enviar grupo assistido",
      }),
    );
    await within(section).findByText("Enviar grupo assistido: concluído.");

    const request = fetcher.mock.calls[0]?.[0] as Request;
    const body = await request.json();
    expect(body.client_submission_id).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
    );
    expect(body.visitors).toEqual([
      expect.objectContaining({
        client_id: expect.stringMatching(
          /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
        ),
        role: "responsible",
        age_band: "60_plus",
        residence_country: "AR",
      }),
    ]);
    expect(body.visitors[0]).not.toHaveProperty("residence_state");
    expect(body.visitors[0]).not.toHaveProperty("residence_city_code");
  });

  it("não apresenta violações axe no workspace autenticado", async () => {
    const { container } = renderWorkspace();

    const report = await axe.run(container, {
      rules: { "color-contrast": { enabled: false } },
    });

    expect(report.violations).toEqual([]);
  });
});
