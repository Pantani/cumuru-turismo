import { cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, describe, expect, it, vi } from "vitest";

const { toCanvas } = vi.hoisted(() => ({ toCanvas: vi.fn() }));
vi.mock("qrcode", () => ({ default: { toCanvas } }));

import { renderWithSession } from "../../test/session";
import { AdminWorkspace } from "./AdminWorkspace";

const accommodationId = "019fae14-0000-7000-8000-0000000000a1";
const otherAccommodationId = "019fae14-0000-7000-8000-0000000000a2";

function apiResponse(body: unknown, init: ResponseInit = {}) {
  const headers = new Headers(init.headers);
  headers.set("Cache-Control", "no-store");
  headers.set("X-Request-ID", "request-admin-workspace-test");
  return Response.json(body, { ...init, headers });
}

function accommodation(overrides: Record<string, unknown> = {}) {
  return {
    id: accommodationId,
    organization_id: accommodationId,
    name: "Pousada Farol Fictícia",
    category: "formal_lodging",
    status: "active",
    capacity: 12,
    version: 3,
    created_at: "2026-08-01T00:00:00Z",
    updated_at: "2026-08-01T00:00:00Z",
    ...overrides,
  };
}

interface RouteTable {
  accommodations?: unknown[];
  onRequest?: (request: Request) => Response | undefined;
}

function routedResponse(input: Request, accommodations: readonly unknown[]) {
  const url = new URL(input.url);
  if (input.method === "GET" && url.pathname.endsWith("/accommodations")) {
    return apiResponse({ items: accommodations, next_cursor: null });
  }
  return apiResponse({ items: [], next_cursor: null });
}

function stubApi({ accommodations = [accommodation()], onRequest }: RouteTable = {}) {
  const calls: Request[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((input: Request) => {
      calls.push(input);
      return Promise.resolve(
        onRequest?.(input) ?? routedResponse(input, accommodations),
      );
    }),
  );
  return calls;
}

/** Deixa os efeitos de montagem dispararem antes de afirmar que nada saiu. */
async function settleEffects(ticks = 6) {
  for (let tick = 0; tick < ticks; tick += 1) {
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  toCanvas.mockReset();
});

describe("área da administração", () => {
  /**
   * O que o usuário relatou: entrar como administrador e ver "Suas hospedagens".
   * A administração não opera lugar nenhum, então nem o quadro de estadias nem o
   * cartaz de autocadastro nascem aqui — eles pertencem à conta da hospedagem.
   */
  it("não abre quadro de estadias nem trata a lista como hospedagem própria", async () => {
    stubApi();
    renderWithSession(<AdminWorkspace />);

    await screen.findByRole("heading", { name: "Cadastro de hospedagens" });
    await settleEffects();

    expect(screen.queryByRole("heading", { name: "Suas hospedagens" })).toBeNull();
    expect(screen.queryByRole("heading", { name: /Estadias de/u })).toBeNull();
    expect(
      screen.queryByRole("heading", { name: "Cartaz de autocadastro" }),
    ).toBeNull();
    expect(
      screen.queryByRole("heading", { name: "Aguardando aprovação" }),
    ).toBeNull();
  });

  it("reúne o cadastro e a fila de pedidos na mesma tela", async () => {
    stubApi();
    renderWithSession(<AdminWorkspace />);

    expect(
      await screen.findByRole("button", { name: "Cadastrar hospedagem" }),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole("heading", {
        name: "Pedidos de acesso de hospedagens",
      }),
    ).toBeInTheDocument();
  });

  it("cadastra hospedagem sem pedir documento fiscal", async () => {
    const user = userEvent.setup();
    stubApi({ accommodations: [] });
    renderWithSession(<AdminWorkspace />);

    await user.click(
      await screen.findByRole("button", { name: "Cadastrar hospedagem" }),
    );

    const form = screen
      .getByRole("heading", { name: "Cadastrar hospedagem" })
      .closest("form");
    expect(form).not.toBeNull();
    const scoped = within(form as HTMLFormElement);
    expect(scoped.getByLabelText("Como o local é conhecido")).toBeInTheDocument();
    expect(scoped.queryByLabelText(/CNPJ/i)).toBeNull();
    expect(scoped.queryByLabelText(/CPF/i)).toBeNull();
    expect(scoped.queryByLabelText(/Cadastur/i)).toBeNull();
    expect(form?.textContent).toContain("Não pedimos CPF, CNPJ, Cadastur");
  });

  /**
   * Entregar o acesso é o único ato que a administração tem sobre uma hospedagem
   * já cadastrada, e ele nasce fechado: abrir o formulário de acesso de uma
   * hospedagem que ninguém escolheu seria emitir credencial por engano.
   */
  it("só oferece a emissão do acesso da hospedagem escolhida", async () => {
    const user = userEvent.setup();
    stubApi();
    renderWithSession(<AdminWorkspace />);

    const card = await screen.findByRole("button", {
      name: /Pousada Farol Fictícia/,
    });
    expect(
      screen.queryByRole("heading", { name: /^Acesso da hospedagem/ }),
    ).toBeNull();

    await user.click(card);

    expect(
      await screen.findByRole("heading", {
        name: "Acesso da hospedagem Pousada Farol Fictícia",
      }),
    ).toBeInTheDocument();
  });

  it("emite o acesso com o If-Match da versão listada", async () => {
    const user = userEvent.setup();
    const calls = stubApi({
      onRequest: (request) =>
        request.method === "POST" && request.url.endsWith("/activation")
          ? apiResponse({
              url: "https://exemplo.invalid/ativacao#token",
              version: 4,
            })
          : undefined,
    });
    renderWithSession(<AdminWorkspace />);

    await user.click(
      await screen.findByRole("button", { name: /Pousada Farol Fictícia/ }),
    );
    await user.type(
      screen.getByLabelText("Como chamar essa pessoa"),
      "Marina Fictícia",
    );
    await user.type(
      screen.getByLabelText("E-mail de acesso"),
      "marina@exemplo.invalid",
    );
    await user.click(screen.getByRole("button", { name: "Emitir acesso" }));

    await waitFor(() => {
      const issued = calls.find((call) => call.url.endsWith("/activation"));
      expect(issued?.headers.get("If-Match")).toBe('"3"');
    });
  });

  /**
   * O `version` de `AccommodationAccess` nasce de `useState(accommodation.version)`
   * e só é reinicializado se o componente remontar. Sem `key` no ponto de uso, a
   * troca de hospedagem selecionada reaproveita a instância e a versão da
   * hospedagem anterior vazaria para o `If-Match` da nova — que então falharia
   * com precondição, ou pior, colidiria por acaso com uma versão válida de outra
   * linha.
   */
  it("usa a versão da hospedagem recém-selecionada ao trocar a seleção", async () => {
    const user = userEvent.setup();
    const calls = stubApi({
      accommodations: [
        accommodation({ version: 3 }),
        accommodation({
          id: otherAccommodationId,
          name: "Casa Fictícia da Duna",
          version: 9,
        }),
      ],
      onRequest: (request) =>
        request.method === "POST" && request.url.endsWith("/activation")
          ? apiResponse({
              url: "https://exemplo.invalid/ativacao#token",
              version: 10,
            })
          : undefined,
    });
    renderWithSession(<AdminWorkspace />);

    // A seleção não é automática na administração: a primeira troca precisa
    // acontecer para existir um "antes" cuja versão poderia vazar.
    await user.click(
      await screen.findByRole("button", { name: /Pousada Farol Fictícia/ }),
    );
    await screen.findByRole("heading", {
      name: "Acesso da hospedagem Pousada Farol Fictícia",
    });

    await user.click(
      screen.getByRole("button", { name: /Casa Fictícia da Duna/ }),
    );
    await screen.findByRole("heading", {
      name: "Acesso da hospedagem Casa Fictícia da Duna",
    });
    await user.type(
      screen.getByLabelText("Como chamar essa pessoa"),
      "Marina Fictícia",
    );
    await user.type(
      screen.getByLabelText("E-mail de acesso"),
      "marina@exemplo.invalid",
    );
    await user.click(screen.getByRole("button", { name: "Emitir acesso" }));

    await waitFor(() => {
      const issued = calls.find((call) => call.url.endsWith("/activation"));
      expect(issued?.headers.get("If-Match")).toBe('"9"');
    });
  });

  it("não apresenta violações axe na área da administração", async () => {
    stubApi();
    const { container } = renderWithSession(<AdminWorkspace />);

    await screen.findByRole("button", { name: /Pousada Farol Fictícia/ });
    const report = await axe.run(container, {
      rules: { "color-contrast": { enabled: false } },
    });

    expect(report.violations).toEqual([]);
  });
});
