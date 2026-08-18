import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * Vigia o contraste de `.lp-accent` e `.lp-index-number` dentro da seção
 * `.lp-section-coral` da landing. Essas classes usam `--coral` como cor de
 * texto por padrão; como o fundo de `.lp-section-coral` já É `--coral`, a
 * seção precisa de uma sobrescrita, e essa sobrescrita tem que resolver para
 * uma cor de contraste alto sobre o próprio fundo — não para `--ink` (creme),
 * que dá 2.58:1 e falha o WCAG AA.
 */

const MINIMUM_RATIO = 4.5;

const landing = readFileSync(
  resolve(process.cwd(), "src/landing.css"),
  "utf8",
);
const styles = readFileSync(resolve(process.cwd(), "src/styles.css"), "utf8");

function tokenValue(name: string) {
  const matched = new RegExp(`${name}:\\s*([^;]+);`, "u").exec(styles);
  if (matched === null) {
    throw new Error(`token ${name} não existe em styles.css`);
  }
  return matched[1]!.trim();
}

function declaredColor(selector: string) {
  const escaped = selector.replace(/[.]/gu, "\\$&");
  const matched = new RegExp(
    `${escaped}\\s*\\{[^}]*color:\\s*([^;]+);`,
    "u",
  ).exec(landing);
  if (matched === null) {
    throw new Error(`seletor "${selector}" não define color em landing.css`);
  }
  return matched[1]!.trim();
}

/** Resolve uma cadeia de var(--x) até chegar num token final de styles.css. */
function resolveColor(value: string): string {
  const varMatch = /^var\((--[\w-]+)\)$/u.exec(value);
  if (varMatch === null) {
    return value;
  }
  const name = varMatch[1]!;
  const sectionOverride = new RegExp(
    `\\.lp-section-coral\\s*\\{[^}]*${name}:\\s*([^;]+);`,
    "u",
  ).exec(landing);
  return resolveColor(
    sectionOverride === null ? tokenValue(name) : sectionOverride[1]!.trim(),
  );
}

function parseHex(value: string) {
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

function luminance({ r, g, b }: { r: number; g: number; b: number }) {
  return 0.2126 * toLinear(r) + 0.7152 * toLinear(g) + 0.0722 * toLinear(b);
}

function contrastRatio(foregroundHex: string, backgroundHex: string) {
  const foreground = luminance(parseHex(foregroundHex));
  const background = luminance(parseHex(backgroundHex));
  const [lighter, darker] = [foreground, background].sort(
    (left, right) => right - left,
  );
  return (lighter! + 0.05) / (darker! + 0.05);
}

describe("contraste de .lp-accent e .lp-index-number sobre .lp-section-coral", () => {
  it.each([
    ["lp-accent", ".lp-section-coral .lp-accent"],
    ["lp-index-number", ".lp-section-coral .lp-index-number"],
  ])("%s atinge 4.5:1 sobre --coral", (_label, selector) => {
    const foreground = resolveColor(declaredColor(selector));
    const background = resolveColor("var(--coral)");
    expect(contrastRatio(foreground, background)).toBeGreaterThanOrEqual(
      MINIMUM_RATIO,
    );
  });
});
