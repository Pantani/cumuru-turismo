import { cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApprovalQueue } from "./ApprovalQueue";

const { toCanvas } = vi.hoisted(() => ({ toCanvas: vi.fn() }));

vi.mock("qrcode", () => ({ default: { toCanvas } }));
import { renderWithSession } from "../../test/session";
import type { Accommodation, Stay } from "../operator/stay-lifecycle";

const accommodation = {
  id: "019fae14-0000-7000-8000-0000000000a1",
  organization_id: "019fae14-0000-7000-8000-0000000000b1",
  name: "Pousada Fictícia da Barra",
  category: "formal_lodging",
  capacity: 10,
  status: "active",
  version: 1,
  created_at: "2026-08-01T12:00:00Z",
  updated_at: "2026-08-01T12:00:00Z",
} as unknown as Accommodation;

const pendingStay = {
  id: "019fae14-0000-7000-8000-0000000000c1",
  accommodation_id: accommodation.id,
  status: "pre_registered",
  planned_arrival_on: "2026-08-20",
  planned_departure_on: "2026-08-23",
  expected_guest_count: 2,
  visitor_count: 2,
  provenance: "self_service",
  approval_state: "pending",
  approval_expires_at: "2026-08-19T12:00:00Z",
  version: 3,
  created_at: "2026-08-16T12:00:00Z",
  updated_at: "2026-08-16T12:00:00Z",
} as unknown as Stay;

function apiResponse(body: unknown, init: ResponseInit = {}) {
  const headers = new Headers(init.headers);
  headers.set("Cache-Control", "no-store");
  headers.set("X-Request-ID", "request-approval-queue-test");
  return Response.json(body, { ...init, headers });
}

function queuePage(items: readonly Stay[] = [pendingStay]) {
  return apiResponse({ items, next_cursor: null });
}

function commandResult() {
  return apiResponse(
    { id: pendingStay.id, status: "pre_registered", version: 4 },
    { status: 200, headers: { ETag: '"4"', "Idempotency-Replayed": "false" } },
  );
}

function stubFetch() {
  return vi.spyOn(globalThis, "fetch").mockResolvedValue(queuePage());
}

async function renderQueue(fetcher = stubFetch()) {
  const view = renderWithSession(<ApprovalQueue accommodation={accommodation} />);
  await screen.findByRole("heading", { name: /Aguardando aprovação/u });
  await waitFor(() => expect(fetcher).toHaveBeenCalled());
  return { fetcher, view };
}

function lastRequest(fetcher: ReturnType<typeof stubFetch>) {
  const calls = fetcher.mock.calls;
  return calls[calls.length - 1]?.[0] as Request;
}

describe("fila de aprovação da acomodação", () => {
  beforeEach(() => {
    vi.stubGlobal("crypto", {
      ...globalThis.crypto,
      randomUUID: () => "019fae14-0000-7000-8000-0000000000ff",
    });
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("consulta a fila pelos filtros de aprovação e proveniência", async () => {
    const { fetcher } = await renderQueue();

    const url = new URL(lastRequest(fetcher).url);
    expect(url.pathname).toBe("/api/v1/stays");
    expect(url.searchParams.get("accommodation_id")).toBe(accommodation.id);
    expect(url.searchParams.get("approval_state")).toBe("pending");
    expect(url.searchParams.get("provenance")).toBe("self_service");
  });

  it("mostra o prazo de 72 horas de cada pendência", async () => {
    await renderQueue();

    expect(
      await screen.findByText(/Expira em 19 de ago\. de 2026/u),
    ).toBeInTheDocument();
  });

  it("aprova com If-Match derivado da versão e chave idempotente", async () => {
    const user = userEvent.setup();
    const fetcher = stubFetch();
    await renderQueue(fetcher);
    fetcher.mockResolvedValueOnce(commandResult()).mockResolvedValue(queuePage([]));

    await user.click(await screen.findByRole("button", { name: "Aprovar" }));

    await waitFor(() => expect(fetcher.mock.calls.length).toBeGreaterThan(1));
    const approval = fetcher.mock.calls[1]?.[0] as Request;
    expect(approval.method).toBe("POST");
    expect(new URL(approval.url).pathname).toBe(
      `/api/v1/stays/${pendingStay.id}/approve`,
    );
    expect(approval.headers.get("If-Match")).toBe('"3"');
    expect(approval.headers.get("Idempotency-Key")).toBe(
      "019fae14-0000-7000-8000-0000000000ff",
    );
  });

  it("rejeita apenas com motivo de lista fechada, sem texto livre", async () => {
    const user = userEvent.setup();
    const fetcher = stubFetch();
    const { view } = await renderQueue(fetcher);
    fetcher.mockResolvedValueOnce(commandResult()).mockResolvedValue(queuePage([]));

    await user.click(await screen.findByRole("button", { name: "Recusar" }));
    const form = screen.getByRole("group", { name: "Motivo da recusa" });
    expect(within(form).getByRole("combobox")).toBeInTheDocument();
    expect(within(form).queryByRole("textbox")).not.toBeInTheDocument();
    expect(view.container.querySelector("textarea")).toBeNull();

    await user.selectOptions(within(form).getByRole("combobox"), "duplicate");
    await user.click(within(form).getByRole("button", { name: "Confirmar recusa" }));

    await waitFor(() => expect(fetcher.mock.calls.length).toBeGreaterThan(1));
    const rejection = fetcher.mock.calls[1]?.[0] as Request;
    expect(new URL(rejection.url).pathname).toBe(
      `/api/v1/stays/${pendingStay.id}/reject`,
    );
    await expect(rejection.clone().json()).resolves.toEqual({
      reason_code: "duplicate",
    });
  });

  it("oferece o convite nominal do núcleo depois de aprovar", async () => {
    const user = userEvent.setup();
    const fetcher = stubFetch();
    await renderQueue(fetcher);
    fetcher
      .mockResolvedValueOnce(commandResult())
      .mockResolvedValueOnce(queuePage([]))
      .mockResolvedValueOnce(
        apiResponse(
          {
            invite_id: "019fae14-0000-7000-8000-0000000000d1",
            url: "https://cumuru.invalid/convites/token",
            expires_at: "2026-08-19T12:00:00Z",
          },
          {
            status: 201,
            headers: {
              ETag: '"5"',
              Location: `/api/v1/stays/${pendingStay.id}/invite`,
              "Idempotency-Replayed": "false",
            },
          },
        ),
      );

    await user.click(await screen.findByRole("button", { name: "Aprovar" }));
    const offer = await screen.findByRole("button", {
      name: "Emitir convite nominal",
    });
    await user.click(offer);

    expect(
      await screen.findByRole("img", { name: "Código QR do convite" }),
    ).toBeInTheDocument();
    expect(new URL(lastRequest(fetcher).url).pathname).toBe(
      `/api/v1/stays/${pendingStay.id}/invite`,
    );
  });

  it("informa a fila vazia sem alarmar", async () => {
    const fetcher = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(queuePage([]));

    await renderQueue(fetcher);

    expect(
      await screen.findByText(/Nenhum autocadastro aguardando/u),
    ).toBeInTheDocument();
  });

  it("não apresenta violações axe na fila carregada", async () => {
    const { view } = await renderQueue();
    await screen.findByRole("button", { name: "Aprovar" });

    const report = await axe.run(view.container, {
      rules: { "color-contrast": { enabled: false } },
    });

    expect(report.violations).toEqual([]);
  });
});
