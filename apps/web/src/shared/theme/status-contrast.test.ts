import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * Vigia o contraste das cores de estado que a autoatendimento trocou.
 *
 * A regra `color-contrast` do axe **não serve aqui**, e isso foi medido, não
 * suposto: sob jsdom ela devolve `violations: 0, incomplete: 1, passes: 0` para
 * a tela carregada, porque não há layout nem pintura para resolver a cor de
 * fundo através do contexto de empilhamento. Ligá-la daria aparência de
 * vigilância com cobertura zero — nunca reprovaria uma regressão real e nunca
 * confirmaria nada.
 *
 * Este teste lê os tokens do próprio `styles.css` e calcula a razão de
 * contraste. Não depende de renderização, e falha de verdade se alguém baixar o
 * contraste de `--positive` ou `--danger` — que é o risco que a troca por
 * turquesa e coral introduziu.
 */

const MINIMUM_RATIO = 4.5;

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/styles.css"),
  "utf8",
);

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

/** O wash é translúcido: o contraste real é contra o fundo já composto. */
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
  return ratio <= 0.03928
    ? ratio / 12.92
    : ((ratio + 0.055) / 1.055) ** 2.4;
}

function luminance({ r, g, b }: Rgb) {
  return (
    0.2126 * toLinear(r) + 0.7152 * toLinear(g) + 0.0722 * toLinear(b)
  );
}

function contrastRatio(foreground: Rgb, background: Rgb) {
  const [lighter, darker] = [
    luminance(foreground),
    luminance(background),
  ].sort((left, right) => right - left);
  return (lighter! + 0.05) / (darker! + 0.05);
}

function statusRatio(colorToken: string, washToken: string) {
  const page = parseHex(tokenValue("--bg"));
  const wash = parseRgba(tokenValue(washToken));
  return contrastRatio(
    parseHex(tokenValue(colorToken)),
    compose(wash.color, wash.alpha, page),
  );
}

describe("contraste das cores de estado", () => {
  it.each([
    ["confirmação", "--positive", "--positive-wash"],
    ["alerta", "--danger", "--danger-wash"],
  ])("%s atinge 4.5:1 sobre o próprio wash", (_label, color, wash) => {
    expect(statusRatio(color, wash)).toBeGreaterThanOrEqual(MINIMUM_RATIO);
  });

  it("não reintroduz verde nem vermelho de status genéricos", () => {
    // O brandkit fixa cinco cores. Confirmação é turquesa e alerta é coral;
    // qualquer outro valor aqui é uma sexta cor entrando pela porta dos fundos.
    expect(tokenValue("--positive")).toBe("#2fc2af");
    expect(tokenValue("--danger")).toBe("#ff5a36");
  });
});
