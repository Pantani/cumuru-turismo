import { cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { toCanvas } = vi.hoisted(() => ({ toCanvas: vi.fn() }));
vi.mock("qrcode", () => ({ default: { toCanvas } }));

import type { AccessRequest } from "../../shared/api/invite-request-client";
import {
  renderWithSession,
  stubAuthClient,
  testAccountScopes,
  testSession,
} from "../../test/session";
import { OperatorWorkspace } from "../operator/OperatorWorkspace";
import { AccessRequestQueue } from "./AccessRequestQueue";

const requestId = "019fae14-0000-7000-8000-0000000000e1";
const accommodationId = "019fae14-0000-7000-8000-0000000000e2";
const idempotencyKey = "019fae14-0000-7000-8000-0000000000ff";

/** Uma hora atrás, para a espera ser dita e não apenas insinuada. */
const createdAt = new Date(Date.now() - 3_600_000).toISOString();

const pendingRequest = {
  id: requestId,
  accommodation_name: "Pousada Fictícia da Barra",
  category: "formal_lodging",
  capacity: 12,
  contact_name: "Marina Fictícia",
  contact_email: "marina@exemplo.invalid",
  contact_phone: "+55 73 90000-0000",
  city_label: "Prado",
  state_code: "BA",
  approval_state: "pending",
  expires_at: "2026-09-15T12:00:00Z",
  accommodation_id: null,
  rejection_reason_code: null,
  version: 3,
  created_at: createdAt,
  updated_at: createdAt,
} as unknown as AccessRequest;

const approvedRequest = {
  ...pendingRequest,
  approval_state: "approved",
  accommodation_id: accommodationId,
  version: 4,
} as unknown as AccessRequest;

const accommodation = {
  id: accommodationId,
  organization_id: "019fae14-0000-7000-8000-0000000000e3",
  name: "Pousada Fictícia da Barra",
  category: "formal_lodging",
  status: "active",
  capacity: 12,
  version: 7,
  created_at: createdAt,
  updated_at: createdAt,
};

function apiResponse(body: unknown, init: ResponseInit = {}) {
  const headers = new Headers(init.headers);
  headers.set("Cache-Control", "no-store");
  headers.set("X-Request-ID", "request-access-request-queue-test");
  return Response.json(body, { ...init, headers });
}

function queuePage(items: readonly AccessRequest[] = [pendingRequest]) {
  return apiResponse({ items, next_cursor: null });
}

function decisionResult(request: AccessRequest) {
  return apiResponse(request, {
    status: 200,
    headers: {
      ETag: `"${request.version}"`,
      "Idempotency-Replayed": "false",
    },
  });
}

function problem(status: number, title: string) {
  return apiResponse(
    { type: "urn:cumuru:problem:conflict", title, status },
    { status, headers: { "Content-Type": "application/problem+json" } },
  );
}

function accommodationResult() {
  return apiResponse(accommodation, {
    status: 200,
    headers: { ETag: '"7"' },
  });
}

function activationResult() {
  return apiResponse(
    {
      activation_id: "019fae14-0000-7000-8000-0000000000e4",
      account_id: "019fae14-0000-7000-8000-0000000000e5",
      url: "https://cumuru.invalid/ativacao#token",
      expires_at: "2026-09-01T12:00:00Z",
      version: 8,
    },
    {
      status: 201,
      headers: {
        ETag: '"8"',
        Location: `/api/v1/accommodations/${accommodationId}/activation`,
        "Idempotency-Replayed": "false",
      },
    },
  );
}

function stubFetch() {
  return vi.spyOn(globalThis, "fetch").mockResolvedValue(queuePage());
}

async function renderQueue(fetcher = stubFetch()) {
  const view = renderWithSession(<AccessRequestQueue />);
  await screen.findByRole("heading", {
    name: "Pedidos de acesso de hospedagens",
  });
  await waitFor(() => expect(fetcher).toHaveBeenCalled());
  return { fetcher, view };
}

function requestAt(fetcher: ReturnType<typeof stubFetch>, index: number) {
  return fetcher.mock.calls[index]?.[0] as Request;
}

/** Aprova e devolve o `fetcher` já posicionado na emissão do acesso. */
async function approveAndFollowUp(user: ReturnType<typeof userEvent.setup>) {
  const fetcher = stubFetch();
  await renderQueue(fetcher);
  fetcher
    .mockResolvedValueOnce(decisionResult(approvedRequest))
    .mockResolvedValueOnce(queuePage([]))
    .mockResolvedValueOnce(accommodationResult())
    .mockResolvedValue(activationResult());

  await user.click(
    await screen.findByRole("button", { name: "Aprovar e cadastrar" }),
  );
  return fetcher;
}

describe("fila de pedidos de acesso de hospedagens", () => {
  beforeEach(() => {
    vi.stubGlobal("crypto", {
      ...globalThis.crypto,
      randomUUID: () => idempotencyKey,
    });
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    toCanvas.mockReset();
  });

  it("lista o pedido com tudo que ele trouxe e há quanto tempo espera", async () => {
    const { fetcher } = await renderQueue();

    const url = new URL(requestAt(fetcher, 0).url);
    expect(url.pathname).toBe("/api/v1/accommodation-access-requests");
    expect(url.searchParams.get("approval_state")).toBe("pending");

    expect(
      await screen.findByText("Pousada Fictícia da Barra"),
    ).toBeInTheDocument();
    expect(screen.getByText("Pousada ou hotel formal")).toBeInTheDocument();
    expect(screen.getByText("12 pessoas")).toBeInTheDocument();
    expect(screen.getByText("Prado (BA)")).toBeInTheDocument();
    expect(screen.getByText("Marina Fictícia")).toBeInTheDocument();
    expect(screen.getByText("marina@exemplo.invalid")).toBeInTheDocument();
    expect(screen.getByText("+55 73 90000-0000")).toBeInTheDocument();
    expect(screen.getByText("Esperando há 1 hora")).toBeInTheDocument();
  });

  it("aprova com If-Match derivado da versão e chave idempotente", async () => {
    const user = userEvent.setup();
    const fetcher = await approveAndFollowUp(user);

    await waitFor(() => expect(fetcher.mock.calls.length).toBeGreaterThan(1));
    const approval = requestAt(fetcher, 1);
    expect(approval.method).toBe("POST");
    expect(new URL(approval.url).pathname).toBe(
      `/api/v1/accommodation-access-requests/${requestId}/approve`,
    );
    expect(approval.headers.get("If-Match")).toBe('"3"');
    expect(approval.headers.get("Idempotency-Key")).toBe(idempotencyKey);
  });

  it("encaminha para a emissão do acesso já preenchida com o contato do pedido", async () => {
    const user = userEvent.setup();
    const fetcher = await approveAndFollowUp(user);

    const heading = await screen.findByRole("heading", {
      name: "Agora entregue o acesso de Pousada Fictícia da Barra",
    });
    await waitFor(() => expect(heading).toHaveFocus());
    expect(
      screen.getByText(/o sistema não conferiu se esse e-mail/iu),
    ).toBeInTheDocument();

    const name = await screen.findByLabelText("Como chamar essa pessoa");
    expect(name).toHaveValue("Marina Fictícia");
    expect(screen.getByLabelText("E-mail de acesso")).toHaveValue(
      "marina@exemplo.invalid",
    );

    await user.click(screen.getByRole("button", { name: "Emitir acesso" }));

    await waitFor(() => {
      const issued = requestAt(fetcher, fetcher.mock.calls.length - 1);
      expect(new URL(issued.url).pathname).toBe(
        `/api/v1/accommodations/${accommodationId}/activation`,
      );
      expect(issued.headers.get("If-Match")).toBe('"7"');
    });
  });

  it("recusa somente com motivo de lista fechada, sem texto livre", async () => {
    const user = userEvent.setup();
    const fetcher = stubFetch();
    const { view } = await renderQueue(fetcher);
    fetcher
      .mockResolvedValueOnce(
        decisionResult({
          ...pendingRequest,
          approval_state: "rejected",
        } as unknown as AccessRequest),
      )
      .mockResolvedValue(queuePage([]));

    await user.click(await screen.findByRole("button", { name: "Recusar" }));
    const form = screen.getByRole("group", { name: "Motivo da recusa" });
    expect(within(form).getByRole("combobox")).toBeInTheDocument();
    expect(within(form).queryByRole("textbox")).not.toBeInTheDocument();
    expect(view.container.querySelector("textarea")).toBeNull();
    expect(form.textContent).toMatch(/são apagados na hora/u);

    await user.selectOptions(
      within(form).getByRole("combobox"),
      "not_a_lodging",
    );
    await user.click(
      within(form).getByRole("button", { name: "Confirmar recusa" }),
    );

    await waitFor(() => expect(fetcher.mock.calls.length).toBeGreaterThan(1));
    const rejection = requestAt(fetcher, 1);
    expect(new URL(rejection.url).pathname).toBe(
      `/api/v1/accommodation-access-requests/${requestId}/reject`,
    );
    expect(rejection.headers.get("If-Match")).toBe('"3"');
    await expect(rejection.clone().json()).resolves.toEqual({
      reason_code: "not_a_lodging",
    });
  });

  it("explica o conflito de versão sem oferecer a emissão", async () => {
    const user = userEvent.setup();
    const fetcher = stubFetch();
    await renderQueue(fetcher);
    fetcher
      .mockResolvedValueOnce(problem(412, "Versão desatualizada"))
      .mockResolvedValue(queuePage());

    await user.click(
      await screen.findByRole("button", { name: "Aprovar e cadastrar" }),
    );

    expect(
      await screen.findByText(/Alguém decidiu sobre este pedido/u),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: /Agora entregue o acesso/u }),
    ).toBeNull();
    expect(
      screen.getByRole("button", { name: "Aprovar e cadastrar" }),
    ).toHaveAttribute("aria-describedby", "access-request-status");
  });

  it("liga o 409 à mesma explicação de conflito", async () => {
    const user = userEvent.setup();
    const fetcher = stubFetch();
    await renderQueue(fetcher);
    fetcher
      .mockResolvedValueOnce(problem(409, "Pedido já decidido"))
      .mockResolvedValue(queuePage());

    await user.click(
      await screen.findByRole("button", { name: "Aprovar e cadastrar" }),
    );

    expect(
      await screen.findByText(/Alguém decidiu sobre este pedido/u),
    ).toBeInTheDocument();
  });

  it("não apresenta violações axe na fila carregada", async () => {
    const { view } = await renderQueue();
    await screen.findByRole("button", { name: "Aprovar e cadastrar" });

    const report = await axe.run(view.container, {
      rules: { "color-contrast": { enabled: false } },
    });

    expect(report.violations).toEqual([]);
  });
});

describe("escopo da fila de pedidos de acesso", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  /** Aprovar cria acomodação, então a fila custa `accommodations:onboard`. */
  it("não monta o painel nem consulta a fila sem o escopo de onboarding", async () => {
    const calls: Request[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn((input: Request) => {
        calls.push(input);
        return Promise.resolve(apiResponse({ items: [], next_cursor: null }));
      }),
    );
    const withoutOnboard = testAccountScopes.filter(
      (scope) => scope !== "accommodations:onboard",
    );

    renderWithSession(<OperatorWorkspace />, {
      authClient: stubAuthClient(testSession(withoutOnboard)),
    });
    await screen.findByRole("heading", { name: "Suas hospedagens" });
    for (let tick = 0; tick < 6; tick += 1) {
      await new Promise((resolve) => setTimeout(resolve, 0));
    }

    expect(
      screen.queryByRole("heading", {
        name: "Pedidos de acesso de hospedagens",
      }),
    ).toBeNull();
    expect(
      calls.filter((call) =>
        call.url.includes("/accommodation-access-requests"),
      ),
    ).toEqual([]);
  });

  it("monta o painel quando a conta carrega o escopo de onboarding", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() =>
        Promise.resolve(apiResponse({ items: [], next_cursor: null })),
      ),
    );

    renderWithSession(<OperatorWorkspace />);

    expect(
      await screen.findByRole("heading", {
        name: "Pedidos de acesso de hospedagens",
      }),
    ).toBeInTheDocument();
  });
});
