import type { operations } from "../../generated/schema";
import { describe, expect, expectTypeOf, it, vi } from "vitest";

import {
  Phase2ApiError,
  createPhase2Client,
  phase2OperationNames,
} from "./phase2-client";

const token = "access-token-used-only-in-memory";
const requestId = "request-phase2-client-test";
const responseHeaders = {
  "Cache-Control": "no-store",
  "X-Request-ID": requestId,
} as const;
const stayId = "018f4e59-7a2a-7b12-8fd7-5d2e8dc99b80";
const createBody = {
  accommodation_id: stayId,
  planned_arrival_on: "2026-08-01",
  planned_departure_on: "2026-08-03",
  expected_guest_count: 2,
  client_submission_id: stayId,
};
const createAccommodationBody = {
  name: "Casa da Praia Fictícia",
  category: "family_hosting" as const,
  capacity: 6,
  client_submission_id: stayId,
};
const groupBody = {
  client_submission_id: stayId,
  privacy_notice_version: "2026-07",
  visitors: [
    {
      client_id: stayId,
      role: "responsible" as const,
      age_band: "25_34" as const,
      residence_country: "BR",
      residence_state: "BA",
      residence_city_code: "2925509",
    },
  ],
};

type ContractClient = ReturnType<typeof createPhase2Client>;
type ContractInvoke = (client: ContractClient) => Promise<unknown>;
type OperationName = keyof operations;

function invalidMetadataCases(): ReadonlyArray<
  readonly [string, Readonly<Record<string, string>>, string | null]
> {
  return [
    ["X-Request-ID ausente", { "Cache-Control": "no-store" }, null],
    [
      "X-Request-ID em branco",
      { "Cache-Control": "no-store", "X-Request-ID": "   " },
      null,
    ],
    ["Cache-Control ausente", { "X-Request-ID": requestId }, requestId],
    [
      "Cache-Control em branco",
      { "Cache-Control": "", "X-Request-ID": requestId },
      requestId,
    ],
    [
      "Cache-Control incorreto",
      { "Cache-Control": "no-cache", "X-Request-ID": requestId },
      requestId,
    ],
  ];
}

function contractCases(): ReadonlyArray<
  readonly [
    OperationName,
    string,
    string,
    number,
    Readonly<Record<string, string>>,
    ContractInvoke,
  ]
> {
  return [
    ["listAccommodations", "GET", "/api/v1/accommodations", 200, {}, (client) => client.listAccommodations()],
    ["createAccommodation", "POST", "/api/v1/accommodations", 201, { ETag: '"1"', Location: `/api/v1/accommodations/${stayId}`, "Idempotency-Replayed": "false" }, (client) => client.createAccommodation(createAccommodationBody, "idem-12345678")],
    ["getAccommodation", "GET", `/api/v1/accommodations/${stayId}`, 200, { ETag: '"1"' }, (client) => client.getAccommodation(stayId)],
    ["updateAccommodation", "PATCH", `/api/v1/accommodations/${stayId}`, 200, { ETag: '"1"' }, (client) => client.updateAccommodation(stayId, { name: "Pousada" }, '"1"')],
    ["listAccommodationMemberships", "GET", `/api/v1/accommodations/${stayId}/memberships`, 200, {}, (client) => client.listAccommodationMemberships(stayId)],
    ["createAccommodationMembership", "POST", `/api/v1/accommodations/${stayId}/memberships`, 201, { ETag: '"1"', Location: `/api/v1/accommodations/${stayId}/memberships/${stayId}`, "Idempotency-Replayed": "false" }, (client) => client.createAccommodationMembership(stayId, { oidc_issuer: "https://id.invalid", oidc_subject: "subject", role: "operator" }, "idem-12345678")],
    ["updateAccommodationMembership", "PATCH", `/api/v1/accommodations/${stayId}/memberships/${stayId}`, 200, { ETag: '"1"' }, (client) => client.updateAccommodationMembership(stayId, stayId, { active: false }, '"1"')],
    ["listStays", "GET", "/api/v1/stays", 200, {}, (client) => client.listStays()],
    ["createStay", "POST", "/api/v1/stays", 201, { ETag: '"1"', Location: `/api/v1/stays/${stayId}`, "Idempotency-Replayed": "false" }, (client) => client.createStay(createBody, "idem-12345678")],
    ["getStay", "GET", `/api/v1/stays/${stayId}`, 200, { ETag: '"1"' }, (client) => client.getStay(stayId)],
    ["updateStay", "PATCH", `/api/v1/stays/${stayId}`, 200, { ETag: '"1"' }, (client) => client.updateStay(stayId, { expected_guest_count: 2 }, '"1"')],
    ["getStayGroup", "GET", `/api/v1/stays/${stayId}/group`, 200, { ETag: '"1"' }, (client) => client.getStayGroup(stayId)],
    ["submitAssistedStayGroup", "POST", `/api/v1/stays/${stayId}/group`, 200, { ETag: '"1"', "Idempotency-Replayed": "false", "Survey-Capability": "payload.signature" }, (client) => client.submitAssistedStayGroup(stayId, groupBody, '"1"', "idem-12345678")],
    ["createStayInvite", "POST", `/api/v1/stays/${stayId}/invite`, 201, { ETag: '"1"', Location: `/api/v1/stays/${stayId}/invite`, "Idempotency-Replayed": "false" }, (client) => client.createStayInvite(stayId, { privacy_notice_version: "2026-07" }, '"1"', "idem-12345678")],
    ["checkInStay", "POST", `/api/v1/stays/${stayId}/check-in`, 200, { ETag: '"1"', "Idempotency-Replayed": "false" }, (client) => client.checkInStay(stayId, {}, '"1"', "idem-12345678")],
    ["checkOutStay", "POST", `/api/v1/stays/${stayId}/check-out`, 200, { ETag: '"1"', "Idempotency-Replayed": "false" }, (client) => client.checkOutStay(stayId, {}, '"1"', "idem-12345678")],
    ["cancelStay", "POST", `/api/v1/stays/${stayId}/cancel`, 200, { ETag: '"1"', "Idempotency-Replayed": "false" }, (client) => client.cancelStay(stayId, { reason_code: "guest_request", correction: false }, '"1"', "idem-12345678")],
    ["markStayNoShow", "POST", `/api/v1/stays/${stayId}/no-show`, 200, { ETag: '"1"', "Idempotency-Replayed": "false" }, (client) => client.markStayNoShow(stayId, { reason_code: "guest_absent" }, '"1"', "idem-12345678")],
    ["getInvite", "GET", `/api/v1/invites/${stayId}`, 200, {}, (client) => client.getInvite(stayId)],
    ["submitInviteGroup", "POST", `/api/v1/invites/${stayId}/submit`, 200, { ETag: '"1"', "Idempotency-Replayed": "false", "Survey-Capability": "payload.signature" }, (client) => client.submitInviteGroup(stayId, groupBody, "idem-12345678")],
  ];
}

describe("cliente tipado da Fase 2", () => {
  it("declara exatamente as 20 operações do contrato da fase", () => {
    expectTypeOf(phase2OperationNames).toMatchTypeOf<
      ReadonlyArray<keyof operations>
    >();
    expect(phase2OperationNames).toHaveLength(20);
    expect(new Set(phase2OperationNames).size).toBe(20);
  });

  it("envia o onboarding fechado com autorização e chave idempotente", async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
      Response.json(
        {
          id: stayId,
          organization_id: stayId,
          ...createAccommodationBody,
          status: "active",
          version: 1,
          created_at: "2026-08-02T12:00:00Z",
          updated_at: "2026-08-02T12:00:00Z",
        },
        {
          status: 201,
          headers: {
            ...responseHeaders,
            ETag: '"1"',
            Location: `/api/v1/accommodations/${stayId}`,
            "Idempotency-Replayed": "false",
          },
        },
      ),
    );
    const client = createPhase2Client({ fetcher, getAccessToken: () => token });

    const result = await client.createAccommodation(
      createAccommodationBody,
      "idem-12345678",
    );

    const request = fetcher.mock.calls[0]?.[0] as Request;
    expect(request.method).toBe("POST");
    expect(new URL(request.url).pathname).toBe("/api/v1/accommodations");
    expect(request.headers.get("authorization")).toBe(`Bearer ${token}`);
    expect(request.headers.get("idempotency-key")).toBe("idem-12345678");
    expect(await request.json()).toEqual(createAccommodationBody);
    expect(result).toMatchObject({
      etag: '"1"',
      location: `/api/v1/accommodations/${stayId}`,
      replayed: false,
      requestId,
    });
  });

  it.each(contractCases())(
    "%s usa %s %s",
    async (
      _operation,
      expectedMethod,
      expectedPath,
      expectedStatus,
      headers,
      invoke,
    ) => {
      const fetcher = vi
        .fn<typeof fetch>()
        .mockResolvedValue(
          Response.json(
            {},
            {
              status: expectedStatus,
              headers: { ...responseHeaders, ...headers },
            },
          ),
        );
      const client = createPhase2Client({
        fetcher,
        getAccessToken: () => token,
      });

      const result = await invoke(client);

      const request = fetcher.mock.calls[0]?.[0] as Request;
      expect(request.method).toBe(expectedMethod);
      expect(new URL(request.url).pathname).toBe(expectedPath);
      expect(result).toMatchObject({ requestId });
    },
  );

  it("envia autorização e idempotência somente em headers", async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
      Response.json(
        { id: stayId, status: "draft", version: 1 },
        {
          status: 201,
          headers: {
            ...responseHeaders,
            ETag: '"1"',
            Location: `/api/v1/stays/${stayId}`,
            "Idempotency-Replayed": "false",
          },
        },
      ),
    );
    const client = createPhase2Client({ fetcher, getAccessToken: () => token });

    const result = await client.createStay(createBody, "idem-12345678");

    const request = fetcher.mock.calls[0]?.[0];
    expect(request).toBeInstanceOf(Request);
    expect((request as Request).headers.get("authorization")).toBe(
      `Bearer ${token}`,
    );
    expect((request as Request).headers.get("idempotency-key")).toBe(
      "idem-12345678",
    );
    expect((request as Request).url).not.toContain(token);
    expect(result.etag).toBe('"1"');
    expect(result.requestId).toBe(requestId);
    expect(result.replayed).toBe(false);
  });

  it("aceita cadastro concluído quando não existe pesquisa publicada", async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
      Response.json(
        { submission_id: stayId, status: "accepted", stay_status: "pre_registered" },
        {
          headers: {
            ...responseHeaders,
            ETag: '"1"',
            "Idempotency-Replayed": "false",
          },
        },
      ),
    );
    const client = createPhase2Client({ fetcher, getAccessToken: () => null });

    const result = await client.submitInviteGroup(
      stayId,
      groupBody,
      "idem-12345678",
    );

    expect(result.surveyCapability).toBeNull();
  });

  it("propaga Problem, status e Retry-After sem registrar dados", async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
      Response.json(
        {
          type: "https://cumuru.invalid/problems/conflict",
          title: "Conflito",
          status: 409,
        },
        {
          status: 409,
          headers: {
            ...responseHeaders,
            "Content-Type": "application/problem+json",
            "Retry-After": "3",
          },
        },
      ),
    );
    const client = createPhase2Client({ fetcher, getAccessToken: () => token });

    const failure = client.createStay(createBody, "idem-12345678");

    await expect(failure).rejects.toMatchObject({
      status: 409,
      requestId,
      retryAfterSeconds: 3,
      problem: expect.objectContaining({ title: "Conflito" }),
    } satisfies Partial<Phase2ApiError>);
  });

  it("envia If-Match em alteração concorrente", async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
      Response.json(
        {
          id: stayId,
          accommodation_id: stayId,
          status: "draft",
          planned_arrival_on: "2026-08-01",
          planned_departure_on: "2026-08-03",
          expected_guest_count: 3,
          visitor_count: 0,
          version: 2,
          created_at: "2026-07-28T12:00:00Z",
          updated_at: "2026-07-28T12:01:00Z",
        },
        { headers: { ...responseHeaders, ETag: '"2"' } },
      ),
    );
    const client = createPhase2Client({ fetcher, getAccessToken: () => token });

    await client.updateStay(stayId, { expected_guest_count: 3 }, '"1"');

    const request = fetcher.mock.calls[0]?.[0] as Request;
    expect(request.headers.get("if-match")).toBe('"1"');
  });

  it.each([
    ["status 2xx inesperado", 200, { ...responseHeaders, ETag: '"1"', Location: "/api/v1/stays/x", "Idempotency-Replayed": "false" }],
    ["ETag ausente", 201, { ...responseHeaders, Location: "/api/v1/stays/x", "Idempotency-Replayed": "false" }],
    ["ETag fraco ou não canônico", 201, { ...responseHeaders, ETag: 'W/"1"', Location: "/api/v1/stays/x", "Idempotency-Replayed": "false" }],
    ["Location ausente", 201, { ...responseHeaders, ETag: '"1"', "Idempotency-Replayed": "false" }],
    ["replay inválido", 201, { ...responseHeaders, ETag: '"1"', Location: "/api/v1/stays/x", "Idempotency-Replayed": "yes" }],
  ])("rejeita resposta de criação com %s", async (_label, status, headers) => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(Response.json({}, { status, headers }));
    const client = createPhase2Client({ fetcher, getAccessToken: () => token });

    await expect(
      client.createStay(createBody, "idem-12345678"),
    ).rejects.toMatchObject({
      problem: expect.objectContaining({
        type: "urn:cumuru:problem:invalid-api-response",
      }),
    });
  });

  it.each(invalidMetadataCases())(
    "rejeita sucesso com %s",
    async (_label, metadata, expectedRequestId) => {
      const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
        Response.json(
          { id: stayId, status: "draft", version: 1 },
          {
            status: 201,
            headers: {
              ...metadata,
              ETag: '"1"',
              Location: `/api/v1/stays/${stayId}`,
              "Idempotency-Replayed": "false",
            },
          },
        ),
      );
      const client = createPhase2Client({
        fetcher,
        getAccessToken: () => token,
      });

      await expect(
        client.createStay(createBody, "idem-12345678"),
      ).rejects.toMatchObject({
        requestId: expectedRequestId,
        status: 502,
        problem: expect.objectContaining({
          type: "urn:cumuru:problem:invalid-api-response",
        }),
      });
    },
  );

  it.each(invalidMetadataCases())(
    "rejeita Problem com %s",
    async (_label, metadata, expectedRequestId) => {
      const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
        Response.json(
          {
            type: "https://cumuru.invalid/problems/conflict",
            title: "Conflito",
            status: 409,
          },
          {
            status: 409,
            headers: {
              ...metadata,
              "Content-Type": "application/problem+json",
            },
          },
        ),
      );
      const client = createPhase2Client({
        fetcher,
        getAccessToken: () => token,
      });

      await expect(
        client.createStay(createBody, "idem-12345678"),
      ).rejects.toMatchObject({
        requestId: expectedRequestId,
        status: 502,
        problem: expect.objectContaining({
          type: "urn:cumuru:problem:invalid-api-response",
        }),
      });
    },
  );

  it("rejeita If-Match vazio, fraco ou não canônico antes da rede", async () => {
    const fetcher = vi.fn<typeof fetch>();
    const client = createPhase2Client({ fetcher, getAccessToken: () => token });

    await expect(
      client.updateStay(stayId, { expected_guest_count: 3 }, 'W/"1"'),
    ).rejects.toBeInstanceOf(Phase2ApiError);
    expect(fetcher).not.toHaveBeenCalled();
  });
});
