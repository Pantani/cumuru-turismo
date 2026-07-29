import type { components } from "../../generated/schema";
import { describe, expect, it } from "vitest";

import {
  answerInputs,
  applicableQuestions,
  consentInputs,
  visibleQuestions,
} from "./survey-logic";

const questions: components["schemas"]["PublicQuestion"][] = [
  {
    id: "019f0000-0000-7000-8000-000000000041",
    stable_key: "first_visit",
    prompt: "Primeira visita?",
    answer_type: "boolean" as const,
    required: false,
    purpose_code: "tourism_planning",
    display_order: 1,
    options: [],
  },
  {
    id: "019f0000-0000-7000-8000-000000000042",
    stable_key: "follow_up",
    prompt: "Conte mais",
    answer_type: "short_text" as const,
    required: false,
    purpose_code: "tourism_planning",
    display_order: 2,
    options: [],
    visibility_rule: {
      all: [{ question: "first_visit", operator: "equals" as const, value: true }],
    },
  },
];

describe("projeção local da pesquisa", () => {
  it("usa somente resposta anterior para revelar pergunta condicional", () => {
    expect(visibleQuestions(questions, {})).toHaveLength(1);
    expect(
      visibleQuestions(questions, {
        "019f0000-0000-7000-8000-000000000041": true,
      }),
    ).toHaveLength(2);
  });

  it("não envia resposta de pergunta oculta", () => {
    expect(
      answerInputs(questions, {
        "019f0000-0000-7000-8000-000000000041": false,
        "019f0000-0000-7000-8000-000000000042": "não deve sair",
      }),
    ).toEqual([
      {
        question_id: "019f0000-0000-7000-8000-000000000041",
        value: false,
      },
    ]);
  });

  it("produz uma decisão exata por finalidade e versão", () => {
    expect(
      consentInputs(
        [
          {
            purpose_code: "tourism_planning",
            notice_version: "notice-v1",
            prompt: "Aceita?",
            required_for_answers: true,
            display_order: 1,
          },
        ],
        { tourism_planning: true },
      ),
    ).toEqual([
      {
        purpose_code: "tourism_planning",
        notice_version: "notice-v1",
        granted: true,
      },
    ]);
  });

  it("torna perguntas required e optional não aplicáveis sem consentimento", () => {
    const required = { ...questions[0]!, required: true };
    expect(applicableQuestions([required, questions[1]!], {})).toEqual([]);
    expect(
      applicableQuestions([required, questions[1]!], {
        tourism_planning: true,
      }),
    ).toHaveLength(2);
  });
});
