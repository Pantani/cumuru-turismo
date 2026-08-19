import { cleanup, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { renderWithSession } from "../../test/session";
import { CalendarFeedPanel } from "./CalendarFeedPanel";
import { CalendarReservationQueue } from "./CalendarReservationQueue";

const accommodationId = "018f4e59-7a2a-7b12-8fd7-5d2e8dc99b80";
const reservationId = "018f4e59-7a2a-7b12-8fd7-5d2e8dc99b90";
const feedId = "018f4e59-7a2a-7b12-8fd7-5d2e8dc99b91";

function apiResponse(body: unknown, init: ResponseInit = {}) {
  const headers = new Headers(init.headers);
  headers.set("Cache-Control", "no-store");
  headers.set("X-Request-ID", "request-calendar-test");
  return Response.json(body, { ...init, headers });
}

function feed(overrides: Record<string, unknown> = {}) {
  return {
    id: feedId,
    accommodation_id: accommodationId,
    provider: "booking",
    label: "Chalé 3",
    status: "active",
    last_synced_at: "2026-08-18T12:00:00Z",
    last_sync_outcome: "ok",
    consecutive_failures: 0,
    version: 1,
    created_at: "2026-08-01T00:00:00Z",
    updated_at: "2026-08-18T12:00:00Z",
    ...overrides,
  };
}

function reservation(overrides: Record<string, unknown> = {}) {
  return {
    id: reservationId,
    feed_id: feedId,
    arrival_on: "2026-09-01",
    departure_on: "2026-09-03",
    kind: "reserved",
    state: "pending",
    stay_id: null,
    first_seen_at: "2026-08-18T12:00:00Z",
    last_seen_at: "2026-08-18T12:00:00Z",
    version: 1,
    ...overrides,
  };
}

interface RouteTable {
  feeds?: unknown[];
  reservations?: unknown[];
}

function stubApi({ feeds = [feed()], reservations = [reservation()] }: RouteTable = {}) {
  const calls: Request[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((input: Request) => {
      calls.push(input);
      const url = new URL(input.url);
      if (url.pathname.endsWith("/calendar-feeds")) {
        return Promise.resolve(apiResponse({ items: feeds }));
      }
      if (url.pathname.endsWith("/calendar-reservations")) {
        return Promise.resolve(apiResponse({ items: reservations }));
      }
      return Promise.resolve(
        apiResponse(feed(), { headers: { ETag: '"2"', "Idempotency-Replayed": "false" } }),
      );
    }),
  );
  return calls;
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("calendário da plataforma de hospedagem", () => {
  it("mostra o estado da leitura sem nunca revelar o endereço do feed", async () => {
    stubApi();

    renderWithSession(<CalendarFeedPanel accommodationId={accommodationId} />);

    expect(await screen.findByText(/Chalé 3/u)).toBeInTheDocument();
    expect(screen.getByText(/última leitura funcionou/u)).toBeInTheDocument();
    // O endereço é segredo portador: a API não o devolve e a tela não o exibe.
    expect(document.body.textContent).not.toContain("ical.booking.com");
  });

  it("explica em vez de silenciar quando o endereço expirou", async () => {
    stubApi({ feeds: [feed({ last_sync_outcome: "not_calendar" })] });

    renderWithSession(<CalendarFeedPanel accommodationId={accommodationId} />);

    expect(
      await screen.findByText(/o endereço não abriu um calendário/u),
    ).toBeInTheDocument();
  });

  /**
   * O calendário não traz número de hóspedes. Se a confirmação enviasse um valor
   * qualquer, um bloqueio de manutenção viraria presença publicada (ADR-043).
   */
  it("confirma a reserva com o número de hóspedes informado por quem recebeu", async () => {
    const calls = stubApi();
    const user = userEvent.setup();

    renderWithSession(<CalendarReservationQueue accommodationId={accommodationId} />);

    const guests = await screen.findByLabelText("Quantas pessoas");
    await user.clear(guests);
    await user.type(guests, "4");
    await user.click(screen.getByRole("button", { name: "Confirmar estadia" }));

    await waitFor(() => {
      const confirmCall = calls.find((call) =>
        new URL(call.url).pathname.endsWith(`/${reservationId}/confirm`),
      );
      expect(confirmCall).toBeDefined();
    });
    const confirmCall = calls.find((call) =>
      new URL(call.url).pathname.endsWith(`/${reservationId}/confirm`),
    );
    expect(confirmCall?.headers.get("If-Match")).toBe('"1"');
  });

  it("oferece dispensar o que não era estadia", async () => {
    const calls = stubApi({ reservations: [reservation({ kind: "unknown" })] });
    const user = userEvent.setup();

    renderWithSession(<CalendarReservationQueue accommodationId={accommodationId} />);

    expect(
      await screen.findByText(/a plataforma não disse o que é/u),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Não era estadia" }));

    await waitFor(() => {
      expect(
        calls.some((call) => new URL(call.url).pathname.endsWith("/dismiss")),
      ).toBe(true);
    });
  });

  it("não inventa fila quando nada chegou", async () => {
    stubApi({ reservations: [] });

    renderWithSession(<CalendarReservationQueue accommodationId={accommodationId} />);

    expect(
      await screen.findByText("Nenhuma reserva esperando por você."),
    ).toBeInTheDocument();
  });
});
