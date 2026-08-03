import { describe, expect, it } from "vitest";

import {
  validateCreateAccommodation,
  validateCreateStay,
  validateSubmitGroup,
} from "./phase2-validation";

const uuid = "018f4e59-7a2a-7b12-8fd7-5d2e8dc99b80";
const secondUuid = "018f4e59-7a2a-7c12-8fd7-5d2e8dc99b80";

describe("validação compartilhável da Fase 2", () => {
  it("aceita o cadastro local formal ou familiar sem documento", () => {
    expect(validateCreateAccommodation({
      name: "Pousada Fictícia",
      category: "formal_lodging",
      capacity: 20,
      client_submission_id: uuid,
    })).toEqual([]);
    expect(validateCreateAccommodation({
      name: "Casa de família fictícia",
      category: "family_hosting",
      capacity: 4,
      client_submission_id: uuid,
    })).toEqual([]);
  });

  it("rejeita unclassified, nome vazio, capacidade inválida e UUID não v7", () => {
    const result = validateCreateAccommodation({
      name: "   ",
      category: "unclassified" as never,
      capacity: 0,
      client_submission_id: "018f4e59-7a2a-4b12-8fd7-5d2e8dc99b80",
    });

    expect(result).toEqual(expect.arrayContaining([
      expect.objectContaining({ field: "name" }),
      expect.objectContaining({ field: "category" }),
      expect.objectContaining({ field: "capacity" }),
      expect.objectContaining({ field: "client_submission_id" }),
    ]));
  });

  it("rejeita datas invertidas e quantidade fora do contrato", () => {
    const result = validateCreateStay({
      accommodation_id: uuid,
      planned_arrival_on: "2026-08-10",
      planned_departure_on: "2026-08-09",
      expected_guest_count: 101,
      client_submission_id: uuid,
    });

    expect(result).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ field: "planned_departure_on" }),
        expect.objectContaining({ field: "expected_guest_count" }),
      ]),
    );
  });

  it("rejeita estadia sem intervalo positivo", () => {
    const result = validateCreateStay({
      accommodation_id: uuid,
      planned_arrival_on: "2026-08-10",
      planned_departure_on: "2026-08-10",
      expected_guest_count: 1,
      client_submission_id: uuid,
    });

    expect(result).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ field: "planned_departure_on" }),
      ]),
    );
  });

  it("aceita ISO alpha-2 e exige UF e IBGE para residência BR", () => {
    const result = validateSubmitGroup({
      client_submission_id: uuid,
      privacy_notice_version: "2026-07",
      visitors: [
        {
          client_id: uuid,
          role: "responsible",
          age_band: "25_34",
          residence_country: "BR",
        },
      ],
    });

    expect(result).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ field: "visitors.0.residence_state" }),
        expect.objectContaining({ field: "visitors.0.residence_city_code" }),
      ]),
    );
  });

  it("proíbe UF e município residuais fora do Brasil", () => {
    const result = validateSubmitGroup({
      client_submission_id: uuid,
      privacy_notice_version: "2026-07",
      visitors: [
        {
          client_id: uuid,
          role: "responsible",
          age_band: "25_34",
          residence_country: "AR",
          residence_state: "BA",
          residence_city_code: "2925509",
        },
      ],
    });

    expect(result).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ field: "visitors.0.residence_state" }),
        expect.objectContaining({ field: "visitors.0.residence_city_code" }),
      ]),
    );
  });

  it("rejeita UUID que não seja v7 e client_id duplicado", () => {
    const versionFour = "018f4e59-7a2a-4b12-8fd7-5d2e8dc99b80";
    const result = validateSubmitGroup({
      client_submission_id: versionFour,
      privacy_notice_version: "2026-07",
      visitors: [
        {
          client_id: secondUuid,
          role: "responsible",
          age_band: "25_34",
          residence_country: "BR",
          residence_state: "BA",
          residence_city_code: "2925509",
        },
        {
          client_id: secondUuid,
          role: "companion",
          age_band: "0_5",
          residence_country: "AR",
        },
      ],
    });

    expect(result).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ field: "client_submission_id" }),
        expect.objectContaining({ field: "visitors.1.client_id" }),
      ]),
    );
  });

  it("exige exatamente uma pessoa responsável", () => {
    const result = validateSubmitGroup({
      client_submission_id: uuid,
      privacy_notice_version: "2026-07",
      visitors: [
        {
          client_id: uuid,
          role: "responsible",
          age_band: "25_34",
          residence_country: "AR",
        },
        {
          client_id: secondUuid,
          role: "responsible",
          age_band: "35_44",
          residence_country: "UY",
        },
      ],
    });

    expect(result).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ field: "visitors" }),
      ]),
    );
  });
});
