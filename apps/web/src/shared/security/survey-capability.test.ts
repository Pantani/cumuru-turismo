import { afterEach, describe, expect, it } from "vitest";

import {
  clearSurveyCapability,
  peekSurveyCapability,
  setSurveyCapability,
} from "./survey-capability";

describe("survey capability em memória", () => {
  afterEach(clearSurveyCapability);

  it("aceita somente o formato canônico na variável efêmera", () => {
    setSurveyCapability("payload.signature");
    expect(peekSurveyCapability()).toBe("payload.signature");

    setSurveyCapability("não-canônica");
    expect(peekSurveyCapability()).toBeNull();
  });
});
