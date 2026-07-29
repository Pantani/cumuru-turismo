import { describe, expect, it, vi } from "vitest";

import {
  PlatformApiError,
  createPlatformClient,
  resolveApiBaseUrl,
} from "./platform-client";

describe("platform client", () => {
  it("usa a API no mesmo origin por padrão", () => {
    expect(resolveApiBaseUrl(undefined)).toBe("/api/v1");
  });

  it("preserva o override explícito do ambiente", () => {
    expect(resolveApiBaseUrl("https://api.example.test/custom/v1")).toBe(
      "https://api.example.test/custom/v1",
    );
  });

  it("consulta health pelo contrato gerado sem anexar credenciais", async () => {
    const fetcher = vi.fn(async (request: Request) => {
      expect(request.url).toBe(
        "https://example.test/api/v1/platform/health",
      );
      expect(request.headers.has("Authorization")).toBe(false);
      expect(request.credentials).toBe("same-origin");

      return new Response(JSON.stringify({ status: "ok" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    });
    const client = createPlatformClient({
      baseUrl: "https://example.test/api/v1",
      fetch: fetcher,
    });

    await expect(client.getHealth()).resolves.toEqual({ status: "ok" });
    expect(fetcher).toHaveBeenCalledOnce();
  });

  it("converte Problem Details em erro seguro para a interface", async () => {
    const client = createPlatformClient({
      baseUrl: "https://example.test/api/v1",
      fetch: async () =>
        new Response(
          JSON.stringify({
            type: "https://example.test/problems/unavailable",
            title: "Serviço indisponível",
            status: 503,
          }),
          {
            status: 503,
            headers: { "Content-Type": "application/problem+json" },
          },
        ),
    });

    await expect(client.getReadiness()).rejects.toEqual(
      expect.objectContaining<Partial<PlatformApiError>>({
        name: "PlatformApiError",
        message: "Serviço indisponível",
        status: 503,
      }),
    );
  });
});
