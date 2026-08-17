import { cleanup, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AccommodationAccessPanel } from "./AccommodationAccessPanel";
import { renderWithSession } from "../../test/session";
import type { Accommodation } from "../operator/stay-lifecycle";

const { toCanvas } = vi.hoisted(() => ({ toCanvas: vi.fn() }));

vi.mock("qrcode", () => ({ default: { toCanvas } }));

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

const posterUrl = `https://cumuru.invalid/i#${"z".repeat(96)}`;
const activationUrl = `https://cumuru.invalid/ativacao#${"y".repeat(96)}`;

function apiResponse(body: unknown, init: ResponseInit = {}) {
  const headers = new Headers(init.headers);
  headers.set("Cache-Control", "no-store");
  headers.set("X-Request-ID", "request-accommodation-access-test");
  return Response.json(body, { ...init, headers });
}

function noPoster() {
  return apiResponse(
    {
      type: "urn:cumuru:problem:invite-not-found",
      title: "Sem cartaz ativo.",
      status: 404,
    },
    { status: 404, headers: { "Content-Type": "application/problem+json" } },
  );
}

function posterStatus(revokedAt: string | null = null) {
  return apiResponse({
    invite_id: "019fae14-0000-7000-8000-0000000000e1",
    expires_at: "2027-08-01T12:00:00Z",
    max_uses: null,
    use_count: 4,
    revoked_at: revokedAt,
  });
}

function posterCreated() {
  return apiResponse(
    {
      invite_id: "019fae14-0000-7000-8000-0000000000e1",
      url: posterUrl,
      expires_at: "2027-08-01T12:00:00Z",
      max_uses: null,
      use_count: 0,
    },
    {
      status: 201,
      headers: {
        ETag: '"2"',
        Location: `/api/v1/accommodations/${accommodation.id}/invite`,
        "Idempotency-Replayed": "false",
      },
    },
  );
}

async function renderPanel(fetcher: ReturnType<typeof vi.spyOn>) {
  const view = renderWithSession(
    <AccommodationAccessPanel accommodation={accommodation} />,
  );
  await screen.findByRole("heading", { name: "Cartaz de autocadastro" });
  await waitFor(() => expect(fetcher).toHaveBeenCalled());
  return view;
}

function stubFetch(first: Response) {
  return vi.spyOn(globalThis, "fetch").mockResolvedValue(first);
}

describe("cartaz e acesso da acomodação", () => {
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

  it("mostra ausência de cartaz sem tratar o 404 como falha", async () => {
    const fetcher = stubFetch(noPoster());

    await renderPanel(fetcher);

    expect(await screen.findByText(/Nenhum cartaz ativo/u)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Emitir cartaz" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("emite o cartaz com If-Match da acomodação e só mostra o QR", async () => {
    const user = userEvent.setup();
    const fetcher = stubFetch(noPoster());
    await renderPanel(fetcher);
    fetcher.mockResolvedValueOnce(posterCreated()).mockResolvedValue(posterStatus());

    await user.click(screen.getByRole("button", { name: "Emitir cartaz" }));

    const created = fetcher.mock.calls[1]?.[0] as Request;
    expect(created.method).toBe("POST");
    expect(new URL(created.url).pathname).toBe(
      `/api/v1/accommodations/${accommodation.id}/invite`,
    );
    expect(created.headers.get("If-Match")).toBe('"1"');
    expect(created.headers.get("Idempotency-Key")).toBe(
      "019fae14-0000-7000-8000-0000000000ff",
    );
    expect(
      await screen.findByRole("img", {
        name: "Código QR do cartaz de autocadastro",
      }),
    ).toBeInTheDocument();
    expect(document.documentElement.innerHTML).not.toContain(posterUrl);
    expect(toCanvas).toHaveBeenCalledWith(
      expect.any(HTMLCanvasElement),
      posterUrl,
      expect.any(Object),
    );
  });

  it("avisa que trocar o cartaz invalida o anterior", async () => {
    const fetcher = stubFetch(posterStatus());

    await renderPanel(fetcher);

    expect(
      await screen.findByRole("button", { name: "Trocar o cartaz" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/invalida imediatamente o anterior/u),
    ).toBeInTheDocument();
  });

  it("revoga o cartaz ativo pelo comando próprio", async () => {
    const user = userEvent.setup();
    const fetcher = stubFetch(posterStatus());
    await renderPanel(fetcher);
    fetcher.mockResolvedValueOnce(
      apiResponse(
        {
          invite_id: "019fae14-0000-7000-8000-0000000000e1",
          expires_at: "2027-08-01T12:00:00Z",
          max_uses: null,
          use_count: 4,
          revoked_at: "2026-08-16T12:00:00Z",
        },
        {
          status: 200,
          headers: { ETag: '"2"', "Idempotency-Replayed": "false" },
        },
      ),
    );

    await user.click(await screen.findByRole("button", { name: "Revogar cartaz" }));

    await waitFor(() => expect(fetcher.mock.calls.length).toBeGreaterThan(1));
    const revoke = fetcher.mock.calls[1]?.[0] as Request;
    expect(new URL(revoke.url).pathname).toBe(
      `/api/v1/accommodations/${accommodation.id}/invite/revoke`,
    );
  });

  it("emite o acesso da hospedagem sem enviar e-mail e sem exibir a URL", async () => {
    const user = userEvent.setup();
    const fetcher = stubFetch(noPoster());
    await renderPanel(fetcher);
    fetcher.mockResolvedValueOnce(
      apiResponse(
        {
          activation_id: "019fae14-0000-7000-8000-0000000000f1",
          account_id: "019fae14-0000-7000-8000-0000000000f2",
          url: activationUrl,
          expires_at: "2026-08-19T12:00:00Z",
          version: 3,
        },
        {
          status: 201,
          headers: {
            ETag: '"3"',
            Location: `/api/v1/accommodations/${accommodation.id}/activation`,
            "Idempotency-Replayed": "false",
          },
        },
      ),
    );

    await user.type(
      screen.getByLabelText("Como chamar essa pessoa"),
      "Maria da Recepção",
    );
    await user.type(
      screen.getByLabelText("E-mail de acesso"),
      "recepcao@exemplo.invalid",
    );
    await user.click(screen.getByRole("button", { name: "Emitir acesso" }));

    expect(
      await screen.findByRole("img", {
        name: "Código QR do acesso da hospedagem",
      }),
    ).toBeInTheDocument();
    const created = fetcher.mock.calls[1]?.[0] as Request;
    expect(new URL(created.url).pathname).toBe(
      `/api/v1/accommodations/${accommodation.id}/activation`,
    );
    expect(document.documentElement.innerHTML).not.toContain(activationUrl);
    expect(screen.getByText(/Nada é enviado por e-mail/u)).toBeInTheDocument();
  });

  it("não apresenta violações axe nos painéis carregados", async () => {
    const fetcher = stubFetch(posterStatus());
    const view = await renderPanel(fetcher);
    await screen.findByRole("button", { name: "Revogar cartaz" });

    const report = await axe.run(view.container, {
      rules: { "color-contrast": { enabled: false } },
    });

    expect(report.violations).toEqual([]);
  });
});
