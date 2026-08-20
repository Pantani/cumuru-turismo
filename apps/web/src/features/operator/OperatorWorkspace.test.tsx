import { cleanup, screen, waitFor } from "@testing-library/react";
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
    public_listing: {
      enabled: false,
      phone: null,
      whatsapp: false,
      website: null,
      consented_at: null,
    },
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

/**
 * O comparativo da hospedagem é lido junto com o quadro de estadias. A
 * publicação fictícia entra com os dois lados liberados para que a área monte
 * como monta em runtime.
 */
function performancePayload() {
  return {
    metadata: {
      period: {
        start: "2026-05-01",
        end: "2026-08-01",
        end_exclusive: true,
        time_zone: "America/Bahia",
      },
      unit: "person_day",
      data_mode: "prototype_fixtures",
      updated_at: "2026-08-01T03:00:00Z",
      privacy_policy_version: "prototype-v1",
      methodology_version: "explainable-baseline-v1",
      coverage: { status: "published", ratio: 65 },
    },
    window: "recent_90_days",
    comparison: { status: "available" },
    occupancy: { own_percent: 62, village_percent: 70 },
    series: [
      { date: "2026-07-01", own_person_days: 8, own_index: 100, village_index: 100 },
      { date: "2026-07-02", own_person_days: 12, own_index: 150, village_index: 120 },
    ],
  };
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
  if (url.pathname.endsWith("/performance")) {
    return apiResponse(performancePayload());
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

/**
 * O rótulo exato, e não /Cadastrar/: a área da hospedagem tem outros botões
 * que começam com o mesmo verbo — cadastrar calendário é ato dela —, e o que
 * estes dois testes negam é o cadastro de hospedagem, que é ato da
 * administração.
 */
const ONBOARDING_BUTTON = "Cadastrar";

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

  /**
   * Cadastrar hospedagem é ato da administração, e a área da hospedagem não o
   * oferece mais a ninguém — nem a quem carrega `accommodations:onboard`, porque
   * para essa conta a tela é outra (`AuthenticatedPage`). A tela vazia diz por
   * onde se pede o cadastro: quem chega aqui tem conta e nenhuma hospedagem, e
   * uma tela sem saída seria pior que a afordância morta.
   */
  it("orienta em vez de oferecer cadastro na tela vazia", async () => {
    stubApi({ accommodations: [] });

    renderWithSession(<OperatorWorkspace />);

    expect(
      await screen.findByRole("link", { name: "página de pedido de acesso" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: ONBOARDING_BUTTON }),
    ).toBeNull();
  });

  it("não oferece cadastro de hospedagem nem com o escopo de onboarding", async () => {
    stubApi({});

    renderWithSession(<OperatorWorkspace />, {
      authClient: stubAuthClient(testSession(testAccountScopes)),
    });

    await screen.findByRole("button", { name: /Pousada Farol Fictícia/ });
    expect(screen.queryByRole("button", { name: ONBOARDING_BUTTON })).toBeNull();
  });

  /**
   * Publicar é ato da hospedagem: o painel escreve o mesmo `PATCH` das demais
   * edições, e lê a versão corrente na hora de salvar porque os painéis
   * vizinhos escrevem na mesma linha.
   */
  it("publica o contato lendo a versão corrente antes de escrever", async () => {
    const user = userEvent.setup();
    const calls = stubApi({
      onRequest: (input) => {
        if (input.method === "PATCH") {
          return apiResponse(accommodation({ version: 2 }), {
            headers: { ETag: '"2"' },
          });
        }
        return new URL(input.url).pathname.endsWith(accommodationId)
          ? apiResponse(accommodation(), { headers: { ETag: '"1"' } })
          : undefined;
      },
    });
    renderWithSession(<OperatorWorkspace />);

    await screen.findByRole("heading", { name: "Aparecer na lista pública" });
    await user.type(
      screen.getByLabelText("Telefone com país e DDD"),
      "+5573999990001",
    );
    await user.click(
      screen.getByLabelText("Publicar minha hospedagem na lista"),
    );
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    await waitFor(() => {
      expect(calls.some((call) => call.method === "PATCH")).toBe(true);
    });
    const patch = calls.find((call) => call.method === "PATCH");
    expect(patch?.headers.get("If-Match")).toBe('"1"');
  });

  // O 409 do banco custa uma ida ao servidor para dizer o que a tela já sabe.
  it("recusa publicar sem telefone antes de chamar o servidor", async () => {
    const user = userEvent.setup();
    const calls = stubApi({});
    renderWithSession(<OperatorWorkspace />);

    await screen.findByRole("heading", { name: "Aparecer na lista pública" });
    await user.click(
      screen.getByLabelText("Publicar minha hospedagem na lista"),
    );
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      /informe o telefone/u,
    );
    expect(calls.some((call) => call.method === "PATCH")).toBe(false);
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
