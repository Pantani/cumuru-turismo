import { describe, expect, it } from "vitest";

import { draftDispositionFor } from "./sync-policy";

describe("matriz de preservação do rascunho", () => {
  it.each([0, 409, 412, 422, 429, 503])(
    "preserva em falha de rede ou HTTP %s",
    (status) => {
      expect(draftDispositionFor(status, true)).toBe("preserve");
    },
  );

  it.each([
    [200, true],
    [200, false],
    [404, true],
  ])("elimina para HTTP %s, contexto validado=%s", (status, validated) => {
    expect(draftDispositionFor(status, validated)).toBe("purge");
  });

  it("preserva 404 antes de validar o convite", () => {
    expect(draftDispositionFor(404, false)).toBe("preserve");
  });
});
