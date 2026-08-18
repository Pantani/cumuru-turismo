import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * Vigia o contraste de `.property-capacity` (`AccommodationPicker.tsx`), que
 * axe apontou em `deploy/e2e/local-demo.spec.ts` como um defeito pré-existente
 * fora do escopo do autocadastro: texto `--ink-3` a ~2.67:1 contra o painel do
 * operador. A mesma razão jsdom-vs-Chromium de `status-contrast.test.ts` vale
 * aqui — este teste lê os tokens do próprio `styles.css` em vez de depender de
 * layout renderizado.
 *
 * `.property-capacity` não declara cor própria: herda `.property-card span {
 * color: var(--ink-3); }`. O fundo muda por estado:
 *   - em repouso, `.property-card` declara `background: var(--surface)`
 *     diretamente — a cascata anterior não importa;
 *   - selecionado (`aria-pressed="true"`), o fundo passa a ser o translúcido
 *     `--brand-wash`, composto sobre o que está atrás do botão. Não há
 *     `background` em nenhum ancestral entre `.property-card` e `<body>`
 *     (`.page`, `.workspace`, `.workspace-section`, `.property-grid` — nenhum
 *     define `background` em `styles.css`), então o fundo efetivo é o
 *     `--brand-wash` composto sobre `--bg` do `<body>`.
 */

const MINIMUM_RATIO = 4.5;

const rawStylesheet = readFileSync(
  resolve(process.cwd(), "src/styles.css"),
  "utf8",
);
// Comments can contain commas (prose, examples), which would corrupt the
// naive comma-split selector-list parsing in assertDeclares/assertDoesNotDeclare.
const stylesheet = rawStylesheet.replace(/\/\*[\s\S]*?\*\//gu, "");

function tokenValue(name: string) {
  const matched = new RegExp(`${name}:\\s*([^;]+);`, "u").exec(stylesheet);
  if (matched === null) {
    throw new Error(`token ${name} não existe em styles.css`);
  }
  return matched[1]!.trim();
}

interface Rgb {
  b: number;
  g: number;
  r: number;
}

function parseHex(value: string): Rgb {
  const matched = /^#([0-9a-f]{6})$/iu.exec(value);
  if (matched === null) {
    throw new Error(`esperava hexadecimal de 6 dígitos, veio "${value}"`);
  }
  const digits = matched[1]!;
  return {
    r: Number.parseInt(digits.slice(0, 2), 16),
    g: Number.parseInt(digits.slice(2, 4), 16),
    b: Number.parseInt(digits.slice(4, 6), 16),
  };
}

function parseRgba(value: string) {
  const matched =
    /^rgba\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*([\d.]+)\s*\)$/u.exec(value);
  if (matched === null) {
    throw new Error(`esperava rgba(), veio "${value}"`);
  }
  const [, r, g, b, alpha] = matched;
  return {
    color: { r: Number(r), g: Number(g), b: Number(b) },
    alpha: Number(alpha),
  };
}

function compose(over: Rgb, alpha: number, under: Rgb): Rgb {
  const channel = (top: number, bottom: number) =>
    Math.round(top * alpha + bottom * (1 - alpha));
  return {
    r: channel(over.r, under.r),
    g: channel(over.g, under.g),
    b: channel(over.b, under.b),
  };
}

function toLinear(channel: number) {
  const ratio = channel / 255;
  return ratio <= 0.03928 ? ratio / 12.92 : ((ratio + 0.055) / 1.055) ** 2.4;
}

function luminance({ r, g, b }: Rgb) {
  return 0.2126 * toLinear(r) + 0.7152 * toLinear(g) + 0.0722 * toLinear(b);
}

function contrastRatio(foreground: Rgb, background: Rgb) {
  const [lighter, darker] = [luminance(foreground), luminance(background)].sort(
    (left, right) => right - left,
  );
  return (lighter! + 0.05) / (darker! + 0.05);
}

/**
 * Confirma que a regra existe e declara exatamente a propriedade esperada,
 * em vez de só recalcular a razão sobre os tokens. Sem isso, trocar o
 * seletor que herda `--ink-3` para `.property-capacity`, ou trocar o fundo
 * declarado, manteria os testes de razão verdes enquanto a cor renderizada
 * já teria mudado.
 */
function assertDeclares(selector: string, property: string, value: string) {
  const rules = stylesheet.split("}");
  const matchingRule = rules.find((rule) => {
    const braceIndex = rule.indexOf("{");
    if (braceIndex === -1) {
      return false;
    }
    const selectorList = rule
      .slice(0, braceIndex)
      .split(",")
      .map((entry) => entry.trim());
    return selectorList.includes(selector);
  });
  if (matchingRule === undefined) {
    throw new Error(`${selector} não existe como regra em styles.css`);
  }
  const declarations = matchingRule.slice(matchingRule.indexOf("{") + 1);
  const pattern = new RegExp(`${property}:\\s*${value.replace(/[.*+?^${}()|[\]\\]/gu, "\\$&")}\\s*;`, "u");
  if (!pattern.test(declarations)) {
    throw new Error(`${selector} não declara ${property}: ${value} em styles.css`);
  }
}

function assertDoesNotDeclare(selector: string, property: string) {
  const rules = stylesheet.split("}");
  const matchingRule = rules.find((rule) => {
    const braceIndex = rule.indexOf("{");
    if (braceIndex === -1) {
      return false;
    }
    return rule
      .slice(0, braceIndex)
      .split(",")
      .map((entry) => entry.trim())
      .includes(selector);
  });
  if (matchingRule === undefined) {
    return;
  }
  const declarations = matchingRule.slice(matchingRule.indexOf("{") + 1);
  if (new RegExp(`${property}:`, "u").test(declarations)) {
    throw new Error(`${selector} declara ${property} em styles.css; a cadeia que este teste assume mudou`);
  }
}

describe("contraste de .property-capacity no painel do operador", () => {
  it("herda a cor de --ink-3 via .property-card span, não de uma regra própria", () => {
    assertDoesNotDeclare(".property-capacity", "color");
    assertDeclares(".property-card span", "color", "var(--ink-3)");
  });

  it("nenhum ancestral entre .property-card e <body> declara background", () => {
    for (const ancestor of [".page", ".workspace", ".workspace-section", ".property-grid"]) {
      assertDoesNotDeclare(ancestor, "background");
    }
  });

  it("em repouso, .property-card declara background: var(--surface), e atinge 4.5:1", () => {
    assertDeclares(".property-card", "background", "var(--surface)");
    const ratio = contrastRatio(parseHex(tokenValue("--ink-3")), parseHex(tokenValue("--surface")));
    expect(ratio).toBeGreaterThanOrEqual(MINIMUM_RATIO);
  });

  it('selecionado, .property-card[aria-pressed="true"] declara background: var(--brand-wash) sobre --bg, e atinge 4.5:1', () => {
    assertDeclares('.property-card[aria-pressed="true"]', "background", "var(--brand-wash)");
    const wash = parseRgba(tokenValue("--brand-wash"));
    const composedBackground = compose(wash.color, wash.alpha, parseHex(tokenValue("--bg")));
    const ratio = contrastRatio(parseHex(tokenValue("--ink-3")), composedBackground);
    expect(ratio).toBeGreaterThanOrEqual(MINIMUM_RATIO);
  });
});
