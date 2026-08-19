import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AccommodationDirectory } from "./AccommodationDirectory";
import type {
  AccommodationDirectory as Listing,
  DirectoryClient,
} from "../../shared/api/directory-client";
import { LocaleProvider } from "../../shared/i18n/LocaleProvider";

function listing(): Listing {
  return {
    updated_at: "2026-08-18T09:00:00Z",
    count: 2,
    entries: [
      {
        id: "0198f000-0000-7000-8000-000000000001",
        name: "Pousada da Vila",
        category: "formal_lodging",
        capacity: 24,
        area_code: "Centro",
        phone: "+5573999990001",
        whatsapp: true,
        website: "https://pousada.invalid/",
      },
      {
        id: "0198f000-0000-7000-8000-000000000002",
        name: "Camping do Rio",
        category: "camping",
        capacity: null,
        area_code: "Praia do Norte",
        phone: "+5573999990002",
        whatsapp: false,
        website: null,
      },
    ],
  };
}

function renderDirectory(client: DirectoryClient) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <LocaleProvider initial="pt">
        <AccommodationDirectory client={client} />
      </LocaleProvider>
    </QueryClientProvider>,
  );
}

function clientReturning(data: Listing): DirectoryClient {
  return {
    listAccommodations: vi.fn(() =>
      Promise.resolve({ data, etag: null, requestId: "request-1" }),
    ),
  };
}

afterEach(cleanup);

describe("lista pública de hospedagens", () => {
  it("mostra o telefone como link discável e o WhatsApp de quem atende por lá", async () => {
    renderDirectory(clientReturning(listing()));

    const first = await screen.findByRole("heading", {
      name: "Pousada da Vila",
    });
    const card = first.closest("li");
    expect(card).not.toBeNull();
    const scope = within(card as HTMLElement);
    expect(
      scope.getByRole("link", { name: "Ligar para Pousada da Vila" }),
    ).toHaveAttribute("href", "tel:+5573999990001");
    expect(
      scope.getByRole("link", { name: "WhatsApp de Pousada da Vila" }),
    ).toHaveAttribute("href", "https://wa.me/5573999990001");
    expect(
      scope.getByRole("link", { name: "Site de Pousada da Vila" }),
    ).toHaveAttribute("href", "https://pousada.invalid/");
  });

  // Quem não marcou WhatsApp não ganha link de WhatsApp: o número é o mesmo, e
  // oferecer a conversa mandaria o hóspede para um canal que ninguém lê.
  it("não inventa WhatsApp nem site para quem não publicou", async () => {
    renderDirectory(clientReturning(listing()));

    const second = await screen.findByRole("heading", { name: "Camping do Rio" });
    const scope = within(second.closest("li") as HTMLElement);
    expect(
      scope.queryByRole("link", { name: /WhatsApp/u }),
    ).not.toBeInTheDocument();
    expect(scope.queryByRole("link", { name: /Site/u })).not.toBeInTheDocument();
  });

  it("filtra por tipo e por busca livre", async () => {
    const user = userEvent.setup();
    renderDirectory(clientReturning(listing()));
    await screen.findByRole("heading", { name: "Pousada da Vila" });

    await user.selectOptions(
      screen.getByLabelText("Tipo de hospedagem"),
      "camping",
    );
    expect(
      screen.queryByRole("heading", { name: "Pousada da Vila" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Camping do Rio" }),
    ).toBeInTheDocument();

    await user.selectOptions(screen.getByLabelText("Tipo de hospedagem"), "all");
    await user.type(
      screen.getByLabelText("Buscar por nome ou localidade"),
      "praia do norte",
    );
    expect(
      screen.getByRole("heading", { name: "Camping do Rio" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Pousada da Vila" }),
    ).not.toBeInTheDocument();
  });

  it("explica a lista vazia em vez de mostrar uma tela em branco", async () => {
    renderDirectory(
      clientReturning({ updated_at: "2026-08-18T09:00:00Z", count: 0, entries: [] }),
    );

    expect(
      await screen.findByText(/Nenhuma hospedagem publicou o contato ainda/u),
    ).toBeInTheDocument();
  });

  it("avisa quando a lista não pode ser carregada", async () => {
    const failing: DirectoryClient = {
      listAccommodations: vi.fn(() => Promise.reject(new Error("offline"))),
    };
    renderDirectory(failing);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      /Não foi possível carregar a lista agora/u,
    );
  });
});
