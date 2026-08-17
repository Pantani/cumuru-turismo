import { describe, expect, it } from "vitest";

import type { components } from "../../generated/schema";
import {
  validateActivationPassword,
  validateSelfRegistration,
} from "./phase7-validation";

type SelfRegistrationRequest = components["schemas"]["SelfRegistrationRequest"];

const identifier = "018f4e59-7a2a-7b12-8fd7-5d2e8dc99b80";
const companionId = "018f4e59-7a2a-7c12-8fd7-5d2e8dc99b80";

function request(
  overrides: Partial<SelfRegistrationRequest> = {},
): SelfRegistrationRequest {
  return {
    client_submission_id: identifier,
    privacy_notice_version: "2026-07",
    planned_arrival_on: "2026-08-01",
    planned_departure_on: "2026-08-03",
    visitors: [
      {
        client_id: identifier,
        role: "responsible",
        age_band: "25_34",
        residence_country: "BR",
        residence_state: "BA",
        residence_city_code: "2925509",
      },
    ],
    proof_of_work: { challenge: "c".repeat(40), solution: "AAAAAAAAAAA" },
    ...overrides,
  };
}

function fields(issues: ReturnType<typeof validateSelfRegistration>) {
  return issues.map((issue) => issue.field);
}

describe("validação do autocadastro pelo cartaz", () => {
  it("aceita a submissão generalizada mínima", () => {
    expect(validateSelfRegistration(request())).toEqual([]);
  });

  it("recusa papel de menor no canal aberto", () => {
    const issues = validateSelfRegistration(
      request({
        visitors: [
          {
            client_id: identifier,
            role: "minor" as never,
            age_band: "12_17",
            residence_country: "BR",
            residence_state: "BA",
            residence_city_code: "2925509",
          },
        ],
      }),
    );

    expect(fields(issues)).toContain("visitors.0.role");
    expect(issues[0]?.message).toMatch(/canal assistido/iu);
  });

  it("exige exatamente uma pessoa responsável", () => {
    const issues = validateSelfRegistration(
      request({
        visitors: [
          {
            client_id: identifier,
            role: "companion",
            age_band: "25_34",
            residence_country: "AR",
          },
          {
            client_id: companionId,
            role: "companion",
            age_band: "35_44",
            residence_country: "AR",
          },
        ],
      }),
    );

    expect(fields(issues)).toContain("visitors");
  });

  it("valida a janela de datas planejada", () => {
    const issues = validateSelfRegistration(
      request({ planned_arrival_on: "2026-08-05", planned_departure_on: "2026-08-01" }),
    );

    expect(fields(issues)).toContain("planned_departure_on");
  });

  it("exige residência brasileira completa", () => {
    const issues = validateSelfRegistration(
      request({
        visitors: [
          {
            client_id: identifier,
            role: "responsible",
            age_band: "25_34",
            residence_country: "BR",
            residence_state: "",
            residence_city_code: "",
          },
        ],
      }),
    );

    expect(fields(issues)).toEqual([
      "visitors.0.residence_state",
      "visitors.0.residence_city_code",
    ]);
  });

  it("exige a prova de trabalho resolvida", () => {
    const issues = validateSelfRegistration(
      request({ proof_of_work: { challenge: "", solution: "" } }),
    );

    expect(fields(issues)).toContain("proof_of_work");
  });
});

describe("validação da senha de ativação", () => {
  it("aceita senha dentro da política do contrato", () => {
    expect(
      validateActivationPassword("senha-bem-longa", "senha-bem-longa"),
    ).toEqual({});
  });

  it("recusa senha curta e confirmação divergente", () => {
    const issues = validateActivationPassword("curta", "outra");

    expect(issues.password).toMatch(/12/u);
    expect(issues.confirmation).toBeDefined();
  });

  it("recusa senha acima do limite do contrato", () => {
    const issues = validateActivationPassword("a".repeat(257), "a".repeat(257));

    expect(issues.password).toMatch(/256/u);
  });
});
