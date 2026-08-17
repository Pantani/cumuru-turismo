import { describe, expect, it } from "vitest";

import { LOCALES, preferredLocale } from "./locale";
import { messagesPt } from "./messages-pt";
import {
  createTranslate,
  interpolate,
  messagesFor,
  placeholdersOf,
} from "./translate";

describe("dicionário do Observatório", () => {
  it("publica exatamente as mesmas chaves em todos os idiomas", () => {
    const expected = Object.keys(messagesPt).sort();

    for (const locale of LOCALES) {
      expect(Object.keys(messagesFor(locale)).sort()).toEqual(expected);
    }
  });

  it("nunca deixa uma mensagem vazia", () => {
    for (const locale of LOCALES) {
      const empty = Object.entries(messagesFor(locale))
        .filter(([, value]) => value.trim() === "")
        .map(([key]) => key);
      expect(empty).toEqual([]);
    }
  });

  /**
   * Um placeholder perdido na tradução vira número faltando na frase, e um
   * placeholder inventado vira exceção em produção. As duas falhas só aparecem
   * comparando os conjuntos chave a chave.
   */
  it("mantém os mesmos parâmetros de cada mensagem em todos os idiomas", () => {
    for (const locale of LOCALES) {
      const messages = messagesFor(locale);
      const divergent = Object.entries(messagesPt)
        .filter(
          ([key, source]) =>
            placeholdersOf(messages[key as keyof typeof messagesPt]).join() !==
            placeholdersOf(source).join(),
        )
        .map(([key]) => key);
      expect(divergent).toEqual([]);
    }
  });
});

describe("interpolação", () => {
  it("substitui todas as ocorrências do mesmo parâmetro", () => {
    expect(interpolate("{n} ante os {n} anteriores", { n: 7 })).toBe(
      "7 ante os 7 anteriores",
    );
  });

  it("recusa parâmetro ausente em desenvolvimento e em teste", () => {
    expect(() => interpolate("Média de {count} dias")).toThrowError(/count/);
  });

  it("nomeia a chave no erro, não a frase traduzida", () => {
    expect(() =>
      createTranslate("pt")("analytics.tile.averageHint"),
    ).toThrowError(/analytics\.tile\.averageHint/);
  });

  it("traduz a mesma chave em cada idioma", () => {
    expect(createTranslate("pt")("analytics.title")).toBe(
      "Indicadores públicos",
    );
    expect(createTranslate("en")("analytics.title")).toBe("Public indicators");
    expect(createTranslate("es")("analytics.title")).toBe(
      "Indicadores públicos",
    );
  });
});

describe("negociação de idioma", () => {
  it("aceita a primeira preferência publicada pelo Observatório", () => {
    expect(preferredLocale(["fr-FR", "es-AR", "pt-BR"])).toBe("es");
    expect(preferredLocale(["en-GB"])).toBe("en");
  });

  it("mantém o português quando o navegador não pede nenhum idioma servido", () => {
    expect(preferredLocale(["fr-FR", "de"])).toBe("pt");
    expect(preferredLocale([])).toBe("pt");
  });
});
