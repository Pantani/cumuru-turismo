import type { operations } from "../../generated/schema";
import { describe, expect, expectTypeOf, it, vi } from "vitest";

import {
  createPhase3Client,
  phase3OperationNames,
} from "./phase3-client";

const responseHeaders = {
  "Cache-Control": "no-store",
  "X-Request-ID": "request-phase3-client-test",
} as const;
const id = "019f0000-0000-7000-8000-000000000031";

describe("cliente tipado da Fase 3", () => {
  it("cobre as quatorze operações do contrato", () => {
    expectTypeOf(phase3OperationNames).toMatchTypeOf<
      ReadonlyArray<keyof operations>
    >();
    expect(phase3OperationNames).toHaveLength(14);
    expect(new Set(phase3OperationNames).size).toBe(14);
  });

  it("mantém capability apenas no header da submissão pública", async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
      Response.json(
        { submission_id: id, participation: "declined", status: "accepted" },
        {
          headers: {
            ...responseHeaders,
            "Idempotency-Replayed": "false",
          },
        },
      ),
    );
    const client = createPhase3Client({
      fetcher,
      getAccessToken: () => null,
    });

    await client.submitSurveyResponse(
      {
        questionnaire_version_id: id,
        client_submission_id: id,
        participation: "declined",
        answers: [],
        consent_decisions: [],
      },
      "payload.signature",
      "survey-submit-key-1234",
    );

    const request = fetcher.mock.calls[0]?.[0] as Request;
    expect(request.headers.get("Survey-Capability")).toBe("payload.signature");
    expect(request.url).not.toContain("payload.signature");
    expect(await request.clone().text()).not.toContain("payload.signature");
    expect(request.credentials).toBe("omit");
    expect(request.cache).toBe("no-store");
  });

  it("falha fechado sem sessão nas operações administrativas", async () => {
    const client = createPhase3Client({
      fetcher: vi.fn<typeof fetch>(),
      getAccessToken: () => null,
    });

    await expect(client.listQuestionnaires()).rejects.toMatchObject({
      status: 401,
    });
  });
});
