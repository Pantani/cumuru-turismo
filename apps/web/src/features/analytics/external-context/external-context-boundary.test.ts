/**
 * Asserções negativas da fronteira.
 *
 * O que estes testes provam não se prova renderizando: são ausências. Não há
 * como afirmar "nenhum componente calcula razão entre número externo e célula
 * protegida" observando uma tela — só olhando o que o módulo tem permissão de
 * conhecer. Se a camada externa não importa a camada medida, e nenhum dos dois
 * números entra na mesma função, a aritmética proibida não tem onde ser
 * escrita (ADR-045 §4).
 */

import { readdirSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const here = dirname(fileURLToPath(import.meta.url));
const webRoot = resolve(here, "../../../..");
const repoRoot = resolve(webRoot, "../..");

/**
 * Os módulos que vão para o bundle, lidos do diretório e não de uma lista.
 *
 * Lista fixa é a falha silenciosa desta suíte: um módulo novo aqui deixaria de
 * ser varrido sem que nenhum teste ficasse vermelho, e uma garantia de
 * fronteira que para de cobrir código sem avisar é pior que garantia nenhuma.
 * Fixture e teste ficam de fora porque não entram no bundle.
 */
const PRODUCTION_MODULES = readdirSync(here).filter(
  (name) =>
    /\.tsx?$/u.test(name) &&
    !name.includes(".test.") &&
    !name.includes("-fixtures"),
);

function moduleSource(name: string): string {
  return readFileSync(resolve(here, name), "utf8");
}

/**
 * Comentário é onde a regra é explicada, e explicar "não cruza com cobertura"
 * escreve a palavra proibida. A varredura precisa olhar código.
 */
function withoutComments(source: string): string {
  return source
    .replaceAll(/\/\*[\s\S]*?\*\//gu, " ")
    .replaceAll(/(^|[^:\\])\/\/.*$/gmu, "$1");
}

/**
 * Reduz o código a uma sequência de palavras separadas por espaço, quebrando
 * também nas maiúsculas internas.
 *
 * Substring pura acusa `ratio` dentro de `duration`, `operation` e
 * `declaration` — falso positivo com mensagem enganosa esperando o primeiro
 * `requestDuration`. Fronteira de palavra (`\b`) resolve isso e abre outro
 * buraco: deixaria passar `coverageRatio`, que é exatamente o nome que a
 * proibição existe para pegar. Quebrar a maiúscula interna fecha os dois:
 * `duration` continua uma palavra só, e `coverageRatio` vira duas.
 */
function tokenized(source: string): string {
  const words = source
    .replaceAll(/([a-z0-9])([A-Z])/gu, "$1 $2")
    .replaceAll(/[^A-Za-z0-9]+/gu, " ")
    .toLowerCase()
    .trim();
  return ` ${words} `;
}

const FORBIDDEN_IMPORTS = [
  "./PresenceChart",
  "../PresenceChart",
  "../presence-stats",
  "../presence-format",
  "../presence-months",
  "../coverage",
  "../public-summary",
  "../AnalyticsDashboard",
  "../AnalyticsQuality",
  "shared/api/analytics-client",
  "shared/api/analytics-payload",
] as const;

/**
 * Nomes da camada medida. Nenhum deles pode estar ao alcance deste módulo:
 * é o que fecha a brecha da API pura com a interface reconstituindo a
 * quantidade suprimida.
 */
const FORBIDDEN_IDENTIFIERS = [
  "coverage",
  "cobertura",
  "ratio",
  "sample_size",
  "accommodation_count",
  "PublishedCoverage",
  "PublicPresence",
  "PublicSummary",
  "PresencePoint",
  "ForecastPeak",
  "forecastTotals",
  "seriesStats",
] as const;

describe("fronteira entre a camada externa e a camada medida", () => {
  it("não importa gráfico, previsão, cobertura nem o cliente de analytics", () => {
    for (const name of PRODUCTION_MODULES) {
      const code = withoutComments(moduleSource(name));
      for (const specifier of FORBIDDEN_IMPORTS) {
        expect(`${name}:${code.includes(specifier)}`).toBe(`${name}:false`);
      }
    }
  });

  it("varre todo módulo de produção do diretório, e não uma lista fixa", () => {
    // Piso, não teto: a derivação impede cobertura silenciosamente parcial, e
    // esta asserção impede que um caminho errado faça a suíte passar varrendo
    // zero arquivo.
    expect(PRODUCTION_MODULES.length).toBeGreaterThan(0);
    expect(PRODUCTION_MODULES).toContain("ExternalContextTab.tsx");
    expect(PRODUCTION_MODULES).not.toContain("context-fixtures.ts");
  });

  it("não nomeia cobertura, razão, amostra nem série protegida no código", () => {
    for (const name of PRODUCTION_MODULES) {
      const code = tokenized(withoutComments(moduleSource(name)));
      for (const identifier of FORBIDDEN_IDENTIFIERS) {
        const found = code.includes(tokenized(identifier));
        expect(`${name}:${identifier}:${found}`).toBe(
          `${name}:${identifier}:false`,
        );
      }
    }
  });

  it("não publica string de correlação em nenhum dos três idiomas", () => {
    expect(moduleSource("context-copy.ts")).not.toMatch(
      /correla[a-zç]*|causa[a-zç]+ de|explica a presença/iu,
    );
  });

  it("não embute texto de licença no componente: ele vem do payload", () => {
    const component = moduleSource("ExternalContextTab.tsx");

    expect(component).toContain("attribution_text");
    const code = withoutComments(component);
    expect(code).not.toMatch(/CC[- ]?BY/u);
    expect(code).not.toContain("Open-Meteo");
    expect(code).not.toContain("Creative Commons");
  });

  it("não carrega nenhum ativo remoto", () => {
    const css = readFileSync(resolve(here, "external-context.css"), "utf8");

    expect(css).not.toMatch(/url\(\s*["']?https?:/iu);
    for (const name of PRODUCTION_MODULES) {
      expect(moduleSource(name)).not.toMatch(/src=["']https?:/iu);
    }
  });
});

describe("CSP publicada pelo nginx", () => {
  const conf = readFileSync(
    resolve(repoRoot, "deploy/nginx/default.conf"),
    "utf8",
  );

  it("mantém connect-src e img-src fechados em self", () => {
    expect(conf).toContain("connect-src 'self';");
    expect(conf).toContain("img-src 'self' data:;");
  });

  it("não acrescenta host externo a nenhuma diretiva de origem", () => {
    const policy = /Content-Security-Policy "([^"]+)"/u.exec(conf)?.[1] ?? "";

    expect(policy).not.toBe("");
    expect(policy).not.toMatch(/https?:\/\//u);
    expect(policy).not.toContain("*");
  });
});

describe("service worker", () => {
  const source = readFileSync(
    resolve(webRoot, "public/service-worker.js"),
    "utf8",
  );
  const scope = {
    addEventListener: () => {},
    clients: { claim: () => {} },
    location: { origin: "https://observatorio.invalid" },
    skipWaiting: () => {},
  };
  const isShellRequest = new Function(
    "self",
    `${source}\nreturn isShellRequest;`,
  )(scope) as (request: unknown, url: URL) => boolean;

  function shellDecision(path: string) {
    const url = new URL(path, scope.location.origin);
    return isShellRequest({ headers: new Headers(), method: "GET", mode: "cors" }, url);
  }

  it("não intercepta a rota de contexto externo", () => {
    expect(shellDecision("/api/v1/public/context")).toBe(false);
  });

  it("continua sem interceptar as rotas públicas de analytics", () => {
    expect(shellDecision("/api/v1/public/summary")).toBe(false);
  });
});
