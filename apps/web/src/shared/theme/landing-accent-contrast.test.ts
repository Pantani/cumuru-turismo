import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * Vigia o contraste dos acentos coral (`.lp-accent`, `.lp-index-number`,
 * `.lp-step-number`) contra os fundos claros da landing — o defeito que axe
 * apontou em `deploy/e2e/local-demo.spec.ts` como pré-existente e fora do
 * escopo do autocadastro: coral `#ff5a36` sobre creme, a ~2.6:1 e ~2.3:1.
 *
 * `.lp-section-coral` já resolve o mesmo tipo de conflito (acento sobre fundo
 * da própria cor) trocando a cor do acento para `var(--lp-ink)` — o token de
 * tinta que a própria seção já usa para o título. Este teste prova a mesma
 * troca para as duas seções claras que a herdam sem override: `sand`
 * (`HowItWorksSection`, `PrivacySection`) e `clay` (`HostsSection`).
 *
 * Cálculo de razão de contraste feito sobre os tokens, sem depender de layout
 * renderizado, pela mesma razão de `status-contrast.test.ts`: jsdom não
 * resolve a cor de fundo composta através do contexto de empilhamento.
 */

const MINIMUM_RATIO = 4.5;

const stylesheet = readFileSync(resolve(process.cwd(), "src/landing.css"), "utf8").replace(
  /\/\*[\s\S]*?\*\//gu,
  "",
);
const rootStylesheet = readFileSync(resolve(process.cwd(), "src/styles.css"), "utf8");

function tokenValue(name: string) {
  const matched = new RegExp(`${name}:\\s*([^;]+);`, "u").exec(rootStylesheet);
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
 * Confirma que a seção sobrescreve o acento para `var(--lp-ink)`, e não
 * apenas que a cor de marca em si atinge 4.5:1 — sem essa checagem, o teste
 * numérico abaixo passaria mesmo se a seleção de seletor estivesse errada
 * (ex.: mirando uma classe que o JSX não usa). Faz o próprio parsing de
 * regras (seletor -> declarações) em vez de casar regex direto no texto,
 * porque os seletores relevantes ficam agrupados por vírgula com outras
 * seções (`.lp-section-sand X,\n.lp-section-clay X { ... }`).
 */
function assertOverridesToLpInk(section: string, accentClass: string) {
  const selector = `.lp-section-${section} .${accentClass}`;
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
    throw new Error(`${selector} não existe como regra em landing.css`);
  }
  const declarations = matchingRule.slice(matchingRule.indexOf("{") + 1);
  if (!/color:\s*var\(--lp-ink\)/u.test(declarations)) {
    throw new Error(`${selector} não sobrescreve color para var(--lp-ink) em landing.css`);
  }
}

describe("contraste dos acentos coral sobre fundos claros da landing", () => {
  const onAccent = parseHex(tokenValue("--on-accent"));
  const cream = parseHex(tokenValue("--ink"));
  const sand2 = parseHex(tokenValue("--sand-2"));

  it.each([
    ["sand", "lp-accent", cream],
    ["sand", "lp-index-number", cream],
    ["sand", "lp-step-number", cream],
    ["clay", "lp-index-number", sand2],
  ])("%s .%s atinge 4.5:1 contra o fundo da seção", (section, accentClass, background) => {
    assertOverridesToLpInk(section as string, accentClass as string);
    expect(contrastRatio(onAccent, background as Rgb)).toBeGreaterThanOrEqual(MINIMUM_RATIO);
  });
});
