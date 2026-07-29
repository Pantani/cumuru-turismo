import type { components, operations } from "../../generated/schema";
import { describe, expect, expectTypeOf, it, vi } from "vitest";

import {
  Phase4ApiError,
  createPhase4Client,
  type Phase4Client,
  phase4OperationNames,
} from "./phase4-client";

type Schemas = components["schemas"];

const requestId = "request-phase4-client-test";
const publicHeaders = {
  "Cache-Control": "public, max-age=300, stale-if-error=86400",
  ETag: `"sha256-${"a".repeat(64)}"`,
  "X-Request-ID": requestId,
} as const;
const privateHeaders = {
  "Cache-Control": "no-store",
  "X-Request-ID": requestId,
} as const;
const metadata: Schemas["PublicMetadata"] = {
  period: {
    start: "2026-07-01",
    end: "2026-07-31",
    end_exclusive: true,
    time_zone: "America/Bahia",
  },
  unit: "person_day",
  data_mode: "prototype_fixtures",
  updated_at: "2026-07-28T12:00:00Z",
  privacy_policy_version: "prototype-v1",
  methodology_version: "explainable-baseline-v1",
  coverage: { status: "published", ratio: 65 },
};
const summary: Schemas["PublicSummary"] = {
  metadata,
  presence_today: {
    date: "2026-07-28",
    kind: "observed",
    status: "published",
    value: 120,
  },
  forecast_peak_next_30_days: {
    date: "2026-08-02",
    kind: "forecast",
    status: "published",
    lower: 130,
    central: 150,
    upper: 180,
  },
};
const presence: Schemas["PublicPresence"] = {
  metadata,
  window: "recent_30_days",
  series: [
    {
      date: "2026-07-28",
      kind: "observed",
      status: "published",
      value: 120,
    },
  ],
};
const preferences: Schemas["PublicPreferences"] = {
  metadata: { ...metadata, unit: "survey_response" },
  period: "last_complete_month",
  metrics: [
    {
      metric_code: "first_visit_share",
      dimension_code: "visit_profile",
      categories: [
        {
          category_code: "first_visit",
          status: "published",
          share_percent: 60,
        },
        {
          category_code: "returning",
          status: "protected",
        },
      ],
    },
  ],
};
const methodology: Schemas["PublicMethodology"] = {
  metadata,
  presence_interval: "[arrival,departure)",
  time_zone: "America/Bahia",
  observed_definition_code: "checked_presence_through_as_of",
  forecast_definition_code: "explainable-baseline-v1",
  forecast_bounds_percent: [85, 115],
  forecast_fallback_bounds_percent: [70, 130],
  primary_threshold: 10,
  minimum_reporting_accommodations: 3,
  complementary_suppression: true,
  rounding_base: 10,
  rounding_mode: "stable-half-up",
  allowed_presence_windows: ["recent_30_days", "next_30_days"],
  allowed_preference_periods: ["last_complete_month"],
};
const quality: Schemas["QualitySnapshot"] = {
  window: "last_30_days",
  updated_at: "2026-07-28T12:00:00Z",
  incomplete_stays: { status: "available", value: 3 },
  overdue_planned_departures: { status: "available", value: 2 },
  silent_accommodations: { status: "available", value: 1 },
  aggregation_failures: { status: "available", value: 0 },
  suspected_duplicates: {
    status: "not_available",
    reason_code: "pseudonym_not_approved",
  },
  fnrh_failures: {
    status: "not_available",
    reason_code: "phase_not_implemented",
  },
  coverage_by_category: [
    { category_code: "pousada", status: "available", ratio: 0.75 },
  ],
};

interface InvalidPayloadCase {
  authenticated?: boolean;
  invoke: (client: Phase4Client) => Promise<unknown>;
  name: string;
  payload: unknown;
}

function clientReturning(payload: unknown, authenticated = false) {
  return createPhase4Client({
    baseUrl: "https://example.test",
    fetcher: vi.fn<typeof fetch>().mockResolvedValue(
      Response.json(payload, {
        headers: authenticated ? privateHeaders : publicHeaders,
      }),
    ),
    getAccessToken: () => (authenticated ? "opaque-session-token" : null),
  });
}

const invalidPayloads: InvalidPayloadCase[] = [
  {
    name: "metadata.email extra",
    payload: {
      ...summary,
      metadata: { ...metadata, email: "pii@example.invalid" },
    },
    invoke: (client) => client.getSummary(),
  },
  {
    name: "tipo de valor observado",
    payload: {
      ...summary,
      presence_today: { ...summary.presence_today, value: "120" },
    },
    invoke: (client) => client.getSummary(),
  },
  {
    name: "presença de hoje prevista",
    payload: {
      ...summary,
      presence_today: {
        date: "2026-07-28",
        kind: "forecast",
        status: "published",
        lower: 100,
        central: 120,
        upper: 140,
      },
    },
    invoke: (client) => client.getSummary(),
  },
  {
    name: "name extra no envelope",
    payload: { ...presence, name: "canary-name" },
    invoke: (client) => client.getPresence("recent_30_days"),
  },
  {
    name: "phone extra no ponto aninhado",
    payload: {
      ...presence,
      series: [{ ...presence.series[0], phone: "canary-phone" }],
    },
    invoke: (client) => client.getPresence("recent_30_days"),
  },
  {
    name: "enum de janela",
    payload: { ...presence, window: "all_time" },
    invoke: (client) => client.getPresence("recent_30_days"),
  },
  {
    name: "ponto previsto na janela observada",
    payload: {
      ...presence,
      series: [
        {
          date: "2026-07-28",
          kind: "forecast",
          status: "published",
          lower: 100,
          central: 120,
          upper: 140,
        },
      ],
    },
    invoke: (client) => client.getPresence("recent_30_days"),
  },
  {
    name: "document extra na categoria aninhada",
    payload: {
      ...preferences,
      metrics: [
        {
          ...preferences.metrics[0],
          categories: [
            {
              ...preferences.metrics[0]?.categories[0],
              document: "canary-document",
            },
            preferences.metrics[0]?.categories[1],
          ],
        },
      ],
    },
    invoke: (client) => client.getPreferences(),
  },
  {
    name: "percentual fora do múltiplo catalogado",
    payload: {
      ...preferences,
      metrics: [
        {
          ...preferences.metrics[0],
          categories: [
            {
              ...preferences.metrics[0]?.categories[0],
              share_percent: 61,
            },
            preferences.metrics[0]?.categories[1],
          ],
        },
      ],
    },
    invoke: (client) => client.getPreferences(),
  },
  {
    name: "secret extra na metodologia",
    payload: { ...methodology, secret: "canary-secret" },
    invoke: (client) => client.getMethodology(),
  },
  {
    name: "ciphertext extra no metadata da metodologia",
    payload: {
      ...methodology,
      metadata: { ...metadata, ciphertext: "canary-ciphertext" },
    },
    invoke: (client) => client.getMethodology(),
  },
  {
    name: "constantes do intervalo metodológico",
    payload: { ...methodology, forecast_bounds_percent: [80, 120] },
    invoke: (client) => client.getMethodology(),
  },
  {
    name: "campo obrigatório do fallback metodológico",
    payload: Object.fromEntries(
      Object.entries(methodology).filter(
        ([key]) => key !== "forecast_fallback_bounds_percent",
      ),
    ),
    invoke: (client) => client.getMethodology(),
  },
  {
    name: "constantes do fallback metodológico",
    payload: {
      ...methodology,
      forecast_fallback_bounds_percent: [60, 140],
    },
    invoke: (client) => client.getMethodology(),
  },
  {
    authenticated: true,
    name: "cpf extra na contagem interna",
    payload: {
      ...quality,
      incomplete_stays: {
        ...quality.incomplete_stays,
        cpf: "canary-cpf",
      },
    },
    invoke: (client) => client.getQuality(),
  },
  {
    authenticated: true,
    name: "structured_value extra na cobertura interna",
    payload: {
      ...quality,
      coverage_by_category: [
        {
          ...quality.coverage_by_category[0],
          structured_value: "canary-structured-value",
        },
      ],
    },
    invoke: (client) => client.getQuality(),
  },
  {
    authenticated: true,
    name: "ratio interno fora do intervalo",
    payload: {
      ...quality,
      coverage_by_category: [
        { ...quality.coverage_by_category[0], ratio: 1.5 },
      ],
    },
    invoke: (client) => client.getQuality(),
  },
  {
    authenticated: true,
    name: "cobertura disponível sem ratio",
    payload: {
      ...quality,
      coverage_by_category: [
        { category_code: "pousada", status: "available" },
      ],
    },
    invoke: (client) => client.getQuality(),
  },
  {
    authenticated: true,
    name: "cobertura indisponível com ratio",
    payload: {
      ...quality,
      coverage_by_category: [
        { category_code: "pousada", status: "not_available", ratio: 0.5 },
      ],
    },
    invoke: (client) => client.getQuality(),
  },
  {
    authenticated: true,
    name: "enum de motivo interno",
    payload: {
      ...quality,
      suspected_duplicates: {
        status: "not_available",
        reason_code: "contains_pii",
      },
    },
    invoke: (client) => client.getQuality(),
  },
];

describe("cliente tipado da Fase 4", () => {
  it("declara exatamente as cinco operações do contrato congelado", () => {
    expectTypeOf(phase4OperationNames).toMatchTypeOf<
      ReadonlyArray<keyof operations>
    >();
    expect(phase4OperationNames).toEqual([
      "getPublicSummary",
      "getPublicPresence",
      "getPublicPreferences",
      "getPublicMethodology",
      "getAnalyticsQuality",
    ]);
  });

  it("consulta a superfície pública sem cookie, token ou persistência privada", async () => {
    const fetcher = vi.fn<typeof fetch>().mockImplementation(async (input) => {
      const request = input as Request;
      expect(request.url).toBe("https://example.test/api/v1/public/summary");
      expect(request.credentials).toBe("omit");
      expect(request.headers.has("Authorization")).toBe(false);
      expect(request.cache).toBe("default");
      return Response.json(summary, { headers: publicHeaders });
    });
    const client = createPhase4Client({
      baseUrl: "https://example.test",
      fetcher,
      getAccessToken: () => "não-deve-ser-usado",
    });

    await expect(client.getSummary()).resolves.toMatchObject({
      data: summary,
      etag: publicHeaders.ETag,
      requestId,
    });
  });

  it("envia somente seletores catalogados na query", async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
      Response.json(
        {
          metadata,
          window: "next_30_days",
          series: [],
        } satisfies Schemas["PublicPresence"],
        { headers: publicHeaders },
      ),
    );
    const client = createPhase4Client({
      baseUrl: "https://example.test",
      fetcher,
      getAccessToken: () => null,
    });

    await client.getPresence("next_30_days");

    const request = fetcher.mock.calls[0]?.[0] as Request;
    expect(new URL(request.url).searchParams.toString()).toBe(
      "window=next_30_days",
    );
  });

  it.each([
    {
      name: "presença",
      payload: presence,
      invoke: (client: Phase4Client) =>
        client.getPresence("recent_30_days"),
    },
    {
      name: "preferências",
      payload: preferences,
      invoke: (client: Phase4Client) => client.getPreferences(),
    },
    {
      name: "metodologia",
      payload: methodology,
      invoke: (client: Phase4Client) => client.getMethodology(),
    },
  ])("aceita o shape exato de $name", async ({ invoke, payload }) => {
    await expect(invoke(clientReturning(payload))).resolves.toMatchObject({
      data: payload,
      requestId,
    });
  });

  it("rejeita sucesso público sem headers fortes do contrato", async () => {
    const client = createPhase4Client({
      baseUrl: "https://example.test",
      fetcher: vi.fn<typeof fetch>().mockResolvedValue(Response.json(summary)),
      getAccessToken: () => null,
    });

    await expect(client.getSummary()).rejects.toMatchObject({
      status: 502,
      problem: expect.objectContaining({
        type: "urn:cumuru:problem:invalid-api-response",
      }),
    });
  });

  it("falha fechado se um payload público introduzir chave de ID", async () => {
    const unsafe = {
      ...summary,
      metadata: { ...metadata, stay_id: "canary-internal-id" },
    };
    const client = createPhase4Client({
      baseUrl: "https://example.test",
      fetcher: vi
        .fn<typeof fetch>()
        .mockResolvedValue(Response.json(unsafe, { headers: publicHeaders })),
      getAccessToken: () => null,
    });

    const response = client.getSummary();
    await expect(response).rejects.toBeInstanceOf(Phase4ApiError);
    await expect(response).rejects.toMatchObject({ status: 502 });
  });

  it.each(invalidPayloads)(
    "rejeita $name antes de entregar o payload",
    async ({ authenticated = false, invoke, payload }) => {
      const response = invoke(clientReturning(payload, authenticated));

      await expect(response).rejects.toMatchObject({
        status: 502,
        problem: expect.objectContaining({
          type: "urn:cumuru:problem:invalid-api-response",
        }),
      });
    },
  );

  it("falha fechado antes da rede sem sessão na qualidade interna", async () => {
    const fetcher = vi.fn<typeof fetch>();
    const client = createPhase4Client({
      baseUrl: "https://example.test",
      fetcher,
      getAccessToken: () => null,
    });

    await expect(client.getQuality()).rejects.toMatchObject({ status: 401 });
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("usa o token somente em memória e exige no-store na qualidade", async () => {
    const fetcher = vi.fn<typeof fetch>().mockImplementation(async (input) => {
      const request = input as Request;
      expect(request.headers.get("Authorization")).toBe(
        "Bearer opaque-session-token",
      );
      expect(request.credentials).toBe("omit");
      expect(request.cache).toBe("no-store");
      expect(request.url).not.toContain("opaque-session-token");
      return Response.json(quality, { headers: privateHeaders });
    });
    const client = createPhase4Client({
      baseUrl: "https://example.test",
      fetcher,
      getAccessToken: () => "opaque-session-token",
    });

    await expect(client.getQuality()).resolves.toMatchObject({
      data: quality,
      etag: null,
      requestId,
    });
  });
});
