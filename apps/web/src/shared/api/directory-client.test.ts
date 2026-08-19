import { describe, expect, expectTypeOf, it, vi } from "vitest";

import {
  createDirectoryClient,
  directoryOperationNames,
  type AccommodationDirectory,
} from "./directory-client";
import { ApiError } from "./http-client";
import type { operations } from "../../generated/schema";

const requestId = "request-directory-client-test";
const publicHeaders = {
  "Cache-Control": "public, max-age=300, stale-if-error=86400",
  ETag: `"sha256-${"a".repeat(64)}"`,
  "X-Request-ID": requestId,
} as const;

function listing(): AccommodationDirectory {
  return {
    updated_at: "2026-08-18T09:00:00Z",
    count: 1,
    entries: [
      {
        id: "0198f000-0000-7000-8000-000000000001",
        name: "Pousada da Vila",
        category: "formal_lodging",
        capacity: 24,
        area_code: "cumuruxatiba",
        phone: "+5573999990001",
        whatsapp: true,
        website: "https://pousada.invalid/",
      },
    ],
  };
}

function responding(body: unknown, headers: HeadersInit = publicHeaders) {
  return vi.fn<typeof fetch>().mockResolvedValue(
    Response.json(body, { headers }),
  );
}

function clientWith(fetcher: typeof fetch) {
  return createDirectoryClient({ getAccessToken: () => null, fetcher });
}

describe("cliente tipado da lista pública", () => {
  it("declara exatamente a operação do contrato", () => {
    expectTypeOf(directoryOperationNames).toMatchTypeOf<
      ReadonlyArray<keyof operations>
    >();
    expect(directoryOperationNames).toEqual(["listPublicAccommodations"]);
  });

  it("lê a lista sem token e sem seletor", async () => {
    const fetcher = responding(listing());

    const result = await clientWith(fetcher).listAccommodations();

    expect(result.data.entries[0]?.phone).toBe("+5573999990001");
    const request = fetcher.mock.calls[0]?.[0] as Request;
    expect(new URL(request.url).pathname).toBe("/api/v1/public/accommodations");
    expect(new URL(request.url).search).toBe("");
    expect(request.headers.get("Authorization")).toBeNull();
  });

  // O documento é lido por chamador anônimo e cacheado por cache compartilhado;
  // um corpo fora do allowlist é recusado em vez de virar link discável.
  it.each([
    {
      name: "telefone fora de E.164",
      body: () => {
        const value = listing();
        value.entries[0] = { ...value.entries[0]!, phone: "(73) 99999-0001" };
        return value;
      },
    },
    {
      name: "site em http",
      body: () => {
        const value = listing();
        value.entries[0] = {
          ...value.entries[0]!,
          website: "http://pousada.invalid/",
        };
        return value;
      },
    },
    {
      name: "propriedade fora do contrato",
      body: () => ({
        ...listing(),
        entries: [{ ...listing().entries[0]!, email: "contato@pousada.invalid" }],
      }),
    },
    {
      name: "categoria desconhecida",
      body: () => ({
        ...listing(),
        entries: [{ ...listing().entries[0]!, category: "hotel" }],
      }),
    },
  ])("recusa payload com $name", async ({ body }) => {
    const client = clientWith(responding(body()));

    await expect(client.listAccommodations()).rejects.toBeInstanceOf(ApiError);
  });

  it("recusa resposta pública sem cabeçalho de cache compartilhado", async () => {
    const client = clientWith(
      responding(listing(), {
        "Cache-Control": "no-store",
        "X-Request-ID": requestId,
      }),
    );

    await expect(client.listAccommodations()).rejects.toBeInstanceOf(ApiError);
  });
});
