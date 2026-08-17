import { cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  renderWithSession,
  stubAuthClient,
  testAccountScopes,
  testSession,
} from "../../test/session";
import { OperatorWorkspace } from "./OperatorWorkspace";

const { toCanvas } = vi.hoisted(() => ({ toCanvas: vi.fn() }));

vi.mock("qrcode", () => ({ default: { toCanvas } }));

const accommodationId = "018f4e59-7a2a-7b12-8fd7-5d2e8dc99b80";
const stayId = "018f4e59-7a2a-7b12-8fd7-5d2e8dc99b81";

function apiResponse(body: unknown, init: ResponseInit = {}) {
  const headers = new Headers(init.headers);
  headers.set("Cache-Control", "no-store");
  headers.set("X-Request-ID", "request-operator-workspace-test");
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
    version: 1,
    created_at: "2026-08-01T00:00:00Z",
    updated_at: "2026-08-01T00:00:00Z",
    ...overrides,
  };
}

function stay(overrides: Record<string, unknown> = {}) {
  return {
    id: stayId,
    accommodation_id: accommodationId,
    status: "pre_registered",
    planned_arrival_on: "2026-08-20",
    planned_departure_on: "2026-08-23",
    expected_guest_count: 3,
    visitor_count: 0,
    version: 4,
    created_at: "2026-08-01T00:00:00Z",
    updated_at: "2026-08-01T00:00:00Z",
    ...overrides,
  };
}

/**
 * O escopo `stays:approve` é o único concedido exclusivamente pelo caminho de
 * ativação do autoatendimento (`activation.go:97`), e nenhuma conta semeada o tem. Narrar
 * a lista de escopos é como o resto da suíte simula "esta capacidade está
 * desligada" — ver `test/session.tsx`.
 */
const selfServiceScopes = [...testAccountScopes, "stays:approve"];

/** Deixa os efeitos de montagem dispararem antes de afirmar que nada saiu. */
async function settleEffects(ticks = 6) {
  for (let tick = 0; tick < ticks; tick += 1) {
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
}

function stayBoardHeading() {
  return screen.getByRole("heading", { name: /Estadias de/u });
}

/** The accommodation starts with no reusable poster; self-service answers 404. */
function noActivePoster() {
  return apiResponse(
    {
      type: "urn:cumuru:problem:invite-not-found",
      title: "Sem cartaz ativo.",
      status: 404,
    },
    { status: 404, headers: { "Content-Type": "application/problem+json" } },
  );
}

/** The approval queue reads the same listing behind `approval_state`. */
function staysFor(url: URL, stays: readonly unknown[]) {
  return url.searchParams.has("approval_state") ? [] : stays;
}

interface RouteTable {
  accommodations?: unknown[];
  stays?: unknown[];
  onRequest?: (request: Request) => Response | undefined;
}

function isPosterLookup(input: Request, url: URL) {
  return input.method === "GET" && url.pathname.endsWith("/invite");
}

function routedResponse(input: Request, table: Required<Omit<RouteTable, "onRequest">>) {
  const url = new URL(input.url);
  if (url.pathname.endsWith("/accommodations")) {
    return apiResponse({ items: table.accommodations, next_cursor: null });
  }
  if (isPosterLookup(input, url)) {
    return noActivePoster();
  }
  if (url.pathname.endsWith("/stays")) {
    return apiResponse({
      items: staysFor(url, table.stays),
      next_cursor: null,
    });
  }
  return apiResponse({ items: [], next_cursor: null });
}

function stubApi({ accommodations = [accommodation()], stays = [], onRequest }: RouteTable) {
  const calls: Request[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((input: Request) => {
      calls.push(input);
      const override = onRequest?.(input);
      return Promise.resolve(
        override ?? routedResponse(input, { accommodations, stays }),
      );
    }),
  );
  return calls;
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  toCanvas.mockReset();
});

describe("área da hospedagem", () => {
  it("lista as hospedagens do operador sem pedir identificador", async () => {
    stubApi({});
    renderWithSession(<OperatorWorkspace />);

    expect(
      await screen.findByRole("button", { name: /Pousada Farol Fictícia/ }),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText(/ID da acomodação/i)).toBeNull();
    expect(screen.queryByLabelText(/ETag/i)).toBeNull();
    expect(screen.queryByLabelText(/Emissor OIDC/i)).toBeNull();
  });

  it("convida ao cadastro quando não há hospedagem", async () => {
    stubApi({ accommodations: [] });
    renderWithSession(<OperatorWorkspace />);

    expect(
      await screen.findByRole("button", { name: "Cadastrar minha hospedagem" }),
    ).toBeInTheDocument();
  });

  it("oferece só as transições que o servidor aceita para o estado", async () => {
    stubApi({ stays: [stay({ status: "pre_registered" })] });
    renderWithSession(<OperatorWorkspace />);

    expect(
      await screen.findByRole("button", { name: "Fazer check-in" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancelar estadia" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Fazer check-out" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Gerar convite" })).toBeNull();
  });

  it("não oferece nenhuma ação em estadia encerrada", async () => {
    stubApi({ stays: [stay({ status: "checked_out" })] });
    renderWithSession(<OperatorWorkspace />);

    expect(await screen.findByText("Estadia encerrada")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Fazer/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /Cancelar estadia/ })).toBeNull();
  });

  // The entity tag comes from the row version already listed, which is what
  // removes the ETag field from the interface without weakening concurrency.
  it("deriva o If-Match da versão listada ao fazer check-in", async () => {
    const user = userEvent.setup();
    const calls = stubApi({
      stays: [stay({ status: "pre_registered", version: 7 })],
      onRequest: (request) =>
        request.method === "POST" && request.url.includes("/check-in")
          ? apiResponse({ id: stayId, status: "checked_in", version: 8 })
          : undefined,
    });
    renderWithSession(<OperatorWorkspace />);

    await user.click(await screen.findByRole("button", { name: "Fazer check-in" }));

    await waitFor(() => {
      const checkIn = calls.find((call) => call.url.includes("/check-in"));
      expect(checkIn?.headers.get("If-Match")).toBe('"7"');
      expect(checkIn?.headers.get("Idempotency-Key")).toMatch(/^[0-9a-f-]{36}$/);
    });
  });

  it("explica em português quando a versão está desatualizada", async () => {
    const user = userEvent.setup();
    stubApi({
      stays: [stay({ status: "pre_registered" })],
      onRequest: (request) =>
        request.method === "POST" && request.url.includes("/check-in")
          ? apiResponse(
              {
                type: "https://turismo.prado.ba.gov.br/problems/precondition-failed",
                title: "Versão desatualizada",
                status: 412,
              },
              {
                status: 412,
                headers: { "Content-Type": "application/problem+json" },
              },
            )
          : undefined,
    });
    renderWithSession(<OperatorWorkspace />);

    await user.click(await screen.findByRole("button", { name: "Fazer check-in" }));

    expect(
      await screen.findByText(/Alguém alterou esta estadia/),
    ).toBeInTheDocument();
  });

  it("cadastra hospedagem sem pedir documento fiscal", async () => {
    const user = userEvent.setup();
    stubApi({ accommodations: [] });
    renderWithSession(<OperatorWorkspace />);

    await user.click(
      await screen.findByRole("button", { name: "Cadastrar minha hospedagem" }),
    );

    const form = screen.getByRole("heading", { name: "Cadastrar hospedagem" })
      .closest("form");
    expect(form).not.toBeNull();
    const scoped = within(form as HTMLFormElement);
    expect(scoped.getByLabelText("Como o local é conhecido")).toBeInTheDocument();
    expect(scoped.queryByLabelText(/CNPJ/i)).toBeNull();
    expect(scoped.queryByLabelText(/CPF/i)).toBeNull();
    expect(scoped.queryByLabelText(/Cadastur/i)).toBeNull();
    expect(form?.textContent).toContain("Não pedimos CPF, CNPJ, Cadastur");
  });

  // Regressão D-02. `SELF_SERVICE_ENABLED` tem default `false` e não está em nenhum
  // compose, então em todo runtime de hoje o servidor não registra as rotas da
  // fase. Montar os painéis sem condição fazia o operador tomar `404` a cada
  // acomodação aberta — foi o que derrubou `make local-demo-e2e`.
  it("não consulta o cartaz quando a conta não tem o escopo do autoatendimento", async () => {
    const calls = stubApi({ stays: [stay()] });

    renderWithSession(<OperatorWorkspace />);
    await screen.findByRole("button", { name: /Pousada Farol Fictícia/ });
    await settleEffects();

    expect(calls.filter((call) => call.url.endsWith("/invite"))).toEqual([]);
    expect(
      screen.queryByRole("heading", { name: "Cartaz de autocadastro" }),
    ).toBeNull();
    expect(
      screen.queryByRole("heading", { name: "Aguardando aprovação" }),
    ).toBeNull();
    expect(stayBoardHeading()).toBeInTheDocument();
  });

  it("monta os painéis do autoatendimento quando o servidor concede o escopo próprio", async () => {
    const calls = stubApi({ stays: [stay()] });

    renderWithSession(<OperatorWorkspace />, {
      authClient: stubAuthClient(testSession(selfServiceScopes)),
    });

    expect(
      await screen.findByRole("heading", { name: "Cartaz de autocadastro" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Aguardando aprovação" }),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(calls.some((call) => call.url.endsWith("/invite"))).toBe(true),
    );
  });

  it("não apresenta violações axe na área da hospedagem", async () => {
    stubApi({ stays: [stay()] });
    const { container } = renderWithSession(<OperatorWorkspace />);

    await screen.findByRole("button", { name: /Pousada Farol Fictícia/ });
    const report = await axe.run(container, {
      rules: { "color-contrast": { enabled: false } },
    });

    expect(report.violations).toEqual([]);
  });
});
