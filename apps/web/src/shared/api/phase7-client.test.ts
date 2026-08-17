import type { operations } from "../../generated/schema";
import { describe, expect, expectTypeOf, it, vi } from "vitest";

import {
  ACTIVATION_TOKEN_HEADER,
  INVITE_TOKEN_HEADER,
  Phase7ApiError,
  createPhase7Client,
  phase7OperationNames,
} from "./phase7-client";

const accessToken = "access-token-used-only-in-memory";
const requestId = "request-phase7-client-test";
const identifier = "018f4e59-7a2a-7b12-8fd7-5d2e8dc99b80";
const capability = "p".repeat(96);
const responseHeaders = {
  "Cache-Control": "no-store",
  "X-Request-ID": requestId,
} as const;

const selfRegistrationBody = {
  client_submission_id: identifier,
  privacy_notice_version: "2026-07",
  planned_arrival_on: "2026-08-01",
  planned_departure_on: "2026-08-03",
  visitors: [
    {
      client_id: identifier,
      role: "responsible" as const,
      age_band: "25_34" as const,
      residence_country: "BR",
      residence_state: "BA",
      residence_city_code: "2925509",
    },
  ],
  proof_of_work: { challenge: "c".repeat(40), solution: "AAAAAAAAAAA" },
};

type ContractClient = ReturnType<typeof createPhase7Client>;
type ContractInvoke = (client: ContractClient) => Promise<unknown>;

function jsonResponse(
  body: unknown,
  status: number,
  headers: Readonly<Record<string, string>> = {},
) {
  return Response.json(body, {
    status,
    headers: { ...responseHeaders, ...headers },
  });
}

function contractCases(): ReadonlyArray<
  readonly [
    keyof operations,
    string,
    string,
    number,
    Readonly<Record<string, string>>,
    ContractInvoke,
  ]
> {
  const etag = { ETag: '"1"', "Idempotency-Replayed": "false" } as const;
  return [
    [
      "createAccommodationActivation",
      "POST",
      `/api/v1/accommodations/${identifier}/activation`,
      201,
      { ...etag, Location: `/api/v1/accommodations/${identifier}/activation` },
      (client) =>
        client.createAccommodationActivation(
          identifier,
          { email: "pousada@exemplo.invalid", display_name: "Pousada" },
          '"1"',
          "idem-12345678",
        ),
    ],
    [
      "getAccommodationActivation",
      "GET",
      "/api/v1/activation",
      200,
      {},
      (client) => client.getAccommodationActivation(capability),
    ],
    [
      "getAccommodationInvite",
      "GET",
      `/api/v1/accommodations/${identifier}/invite`,
      200,
      {},
      (client) => client.getAccommodationInvite(identifier),
    ],
    [
      "createAccommodationInvite",
      "POST",
      `/api/v1/accommodations/${identifier}/invite`,
      201,
      { ...etag, Location: `/api/v1/accommodations/${identifier}/invite` },
      (client) =>
        client.createAccommodationInvite(
          identifier,
          { privacy_notice_version: "2026-07", max_uses: null },
          '"1"',
          "idem-12345678",
        ),
    ],
    [
      "revokeAccommodationInvite",
      "POST",
      `/api/v1/accommodations/${identifier}/invite/revoke`,
      200,
      etag,
      (client) =>
        client.revokeAccommodationInvite(identifier, '"1"', "idem-12345678"),
    ],
    [
      "getAccommodationInviteContext",
      "GET",
      "/api/v1/accommodation-invite",
      200,
      {},
      (client) => client.getAccommodationInviteContext(capability),
    ],
    [
      "submitAccommodationSelfRegistration",
      "POST",
      "/api/v1/accommodation-invite/submit",
      200,
      { ...etag, "Survey-Capability": "payload.signature" },
      (client) =>
        client.submitAccommodationSelfRegistration(
          capability,
          selfRegistrationBody,
          "idem-12345678",
        ),
    ],
    [
      "approveStay",
      "POST",
      `/api/v1/stays/${identifier}/approve`,
      200,
      etag,
      (client) => client.approveStay(identifier, '"1"', "idem-12345678"),
    ],
    [
      "rejectStay",
      "POST",
      `/api/v1/stays/${identifier}/reject`,
      200,
      etag,
      (client) =>
        client.rejectStay(
          identifier,
          { reason_code: "not_a_guest" },
          '"1"',
          "idem-12345678",
        ),
    ],
  ];
}

describe("cliente tipado da Fase 7", () => {
  it("declara exatamente as 10 operações do contrato da fase", () => {
    expectTypeOf(phase7OperationNames).toMatchTypeOf<
      ReadonlyArray<keyof operations>
    >();
    expect(phase7OperationNames).toHaveLength(10);
    expect(new Set(phase7OperationNames).size).toBe(10);
  });

  it.each(contractCases())(
    "%s honra método, caminho e metadados obrigatórios",
    async (_operation, method, path, status, headers, invoke) => {
      const fetcher = vi
        .fn<typeof fetch>()
        .mockResolvedValue(jsonResponse({ ok: true }, status, headers));
      const client = createPhase7Client({
        baseUrl: "https://api.invalid",
        fetcher,
        getAccessToken: () => accessToken,
      });

      await invoke(client);

      const request = fetcher.mock.calls[0]?.[0] as Request;
      expect(request.method).toBe(method);
      expect(new URL(request.url).pathname).toBe(path);
    },
  );

  it("ativa a conta com 204 e sem corpo", async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(new Response(null, { status: 204, headers: responseHeaders }));
    const client = createPhase7Client({
      fetcher,
      getAccessToken: () => null,
    });

    const result = await client.activateAccommodationAccount(capability, {
      password: "senha-de-teste-longa",
    });

    expect(result.data).toBeNull();
    const request = fetcher.mock.calls[0]?.[0] as Request;
    expect(request.method).toBe("POST");
    expect(new URL(request.url).pathname).toBe("/api/v1/activation/complete");
  });

  it.each([
    [
      "cartaz",
      INVITE_TOKEN_HEADER,
      (client: ContractClient) => client.getAccommodationInviteContext(capability),
    ],
    [
      "ativação",
      ACTIVATION_TOKEN_HEADER,
      (client: ContractClient) => client.getAccommodationActivation(capability),
    ],
  ] as const)(
    "envia o token do %s por cabeçalho, nunca no caminho nem em query string",
    async (_label, header, invoke) => {
      const fetcher = vi
        .fn<typeof fetch>()
        .mockResolvedValue(jsonResponse({ ok: true }, 200));
      const client = createPhase7Client({
        baseUrl: "https://api.invalid",
        fetcher,
        getAccessToken: () => null,
      });

      await invoke(client);

      const request = fetcher.mock.calls[0]?.[0] as Request;
      const url = new URL(request.url);
      expect(request.headers.get(header)).toBe(capability);
      expect(url.pathname).not.toContain(capability);
      expect(url.search).toBe("");
      expect(url.hash).toBe("");
      expect(request.headers.get("Authorization")).toBeNull();
    },
  );

  it("recusa capability fora do formato antes de qualquer requisição", async () => {
    const fetcher = vi.fn<typeof fetch>();
    const client = createPhase7Client({ fetcher, getAccessToken: () => null });

    await expect(
      client.getAccommodationInviteContext("curto-demais"),
    ).rejects.toBeInstanceOf(Phase7ApiError);
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("exige sessão nas operações autenticadas", async () => {
    const fetcher = vi.fn<typeof fetch>();
    const client = createPhase7Client({ fetcher, getAccessToken: () => null });

    await expect(
      client.approveStay(identifier, '"1"', "idem-12345678"),
    ).rejects.toBeInstanceOf(Phase7ApiError);
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("propaga Retry-After do 429 do formulário aberto", async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
      Response.json(
        {
          type: "urn:cumuru:problem:rate-limited",
          title: "Muitas tentativas.",
          status: 429,
        },
        {
          status: 429,
          headers: { ...responseHeaders, "Retry-After": "45" },
        },
      ),
    );
    const client = createPhase7Client({ fetcher, getAccessToken: () => null });

    await expect(
      client.getAccommodationInviteContext(capability),
    ).rejects.toMatchObject({ status: 429, retryAfterSeconds: 45 });
  });

  it("recusa resposta sem X-Request-ID ou sem no-store", async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(Response.json({ ok: true }, { status: 200 }));
    const client = createPhase7Client({ fetcher, getAccessToken: () => null });

    await expect(
      client.getAccommodationInviteContext(capability),
    ).rejects.toMatchObject({ status: 502 });
  });

  it("recusa 201 sem ETag forte", async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        jsonResponse({ ok: true }, 201, {
          Location: `/api/v1/accommodations/${identifier}/invite`,
          "Idempotency-Replayed": "false",
        }),
      );
    const client = createPhase7Client({
      fetcher,
      getAccessToken: () => accessToken,
    });

    await expect(
      client.createAccommodationInvite(
        identifier,
        { privacy_notice_version: "2026-07" },
        '"1"',
        "idem-12345678",
      ),
    ).rejects.toMatchObject({ status: 502 });
  });

  it("recusa If-Match que não seja ETag forte canônico", async () => {
    const fetcher = vi.fn<typeof fetch>();
    const client = createPhase7Client({
      fetcher,
      getAccessToken: () => accessToken,
    });

    await expect(
      client.approveStay(identifier, "W/\"1\"", "idem-12345678"),
    ).rejects.toBeInstanceOf(Phase7ApiError);
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("devolve a capability de pesquisa emitida pelo autocadastro", async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
      jsonResponse(
        {
          submission_id: identifier,
          stay_id: identifier,
          status: "accepted",
          stay_status: "pre_registered",
          approval_state: "pending",
        },
        200,
        {
          ETag: '"1"',
          "Idempotency-Replayed": "false",
          "Survey-Capability": "payload.signature",
        },
      ),
    );
    const client = createPhase7Client({ fetcher, getAccessToken: () => null });

    const result = await client.submitAccommodationSelfRegistration(
      capability,
      selfRegistrationBody,
      "idem-12345678",
    );

    expect(result.surveyCapability).toBe("payload.signature");
    expect(result.replayed).toBe(false);
    expect(JSON.stringify(result.data)).not.toContain(capability);
  });
});
