import { describe, expect, it } from "vitest";

import { isPublicContext } from "./context-payload";
import { contextDocument } from "./context-fixtures";

function mutated(change: (draft: Record<string, unknown>) => void) {
  const draft = structuredClone(contextDocument) as unknown as Record<
    string,
    unknown
  >;
  change(draft);
  return draft;
}

describe("allowlist do documento de contexto externo", () => {
  it("aceita o documento do contrato", () => {
    expect(isPublicContext(contextDocument)).toBe(true);
  });

  it("recusa propriedade fora do contrato", () => {
    expect(isPublicContext(mutated((draft) => {
      draft.coverage_ratio_percent = 65;
    }))).toBe(false);
  });

  it("recusa card sem proveniência, inclusive no ramo indisponível", () => {
    expect(isPublicContext(mutated((draft) => {
      const cards = draft.cards as Record<string, unknown>[];
      delete cards[1]?.provenance;
    }))).toBe(false);
  });

  it("recusa o status protected, que pertence à série protegida", () => {
    expect(isPublicContext(mutated((draft) => {
      const cards = draft.cards as Record<string, unknown>[];
      if (cards[1] !== undefined) {
        cards[1].status = "protected";
      }
    }))).toBe(false);
  });

  it("recusa motivo fora da lista fechada", () => {
    expect(isPublicContext(mutated((draft) => {
      const cards = draft.cards as Record<string, unknown>[];
      if (cards[1] !== undefined) {
        cards[1].reason_code = "a fonte estava de folga";
      }
    }))).toBe(false);
  });

  it("recusa card indisponível que traga série ou valor", () => {
    expect(isPublicContext(mutated((draft) => {
      const cards = draft.cards as Record<string, unknown>[];
      if (cards[1] !== undefined) {
        cards[1].series = [];
      }
    }))).toBe(false);
  });

  it("recusa licença servida fora de https", () => {
    expect(isPublicContext(mutated((draft) => {
      const sources = draft.sources as Record<string, unknown>[];
      if (sources[0] !== undefined) {
        sources[0].license_url = "http://creativecommons.org/licenses/by/4.0/";
      }
    }))).toBe(false);
  });

  it("recusa data civil impossível, que Date.parse rolaria para o mês seguinte", () => {
    expect(isPublicContext(mutated((draft) => {
      draft.generated_at = "2026-02-31T00:00:00Z";
    }))).toBe(false);
    expect(isPublicContext(mutated((draft) => {
      draft.generated_at = "2027-02-29T00:00:00Z";
    }))).toBe(false);
  });

  it("aceita 29 de fevereiro em ano bissexto", () => {
    expect(isPublicContext(mutated((draft) => {
      draft.generated_at = "2028-02-29T00:00:00Z";
    }))).toBe(true);
  });

  it("recusa hora fora de faixa", () => {
    expect(isPublicContext(mutated((draft) => {
      draft.generated_at = "2026-08-18T99:00:00Z";
    }))).toBe(false);
  });

  it("recusa documento sem fonte creditada", () => {
    expect(isPublicContext(mutated((draft) => {
      draft.sources = [];
    }))).toBe(false);
  });
});
