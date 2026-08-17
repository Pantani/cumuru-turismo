import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

import { countLeadingZeroBits, solveProofOfWork } from "./proof-of-work";

/**
 * Fecha o triângulo da convenção de fio do proof-of-work.
 *
 * Existem três implementações da mesma regra: este módulo, o verificador Go
 * (`proofofwork.go:262`) e o solucionador do gate integrado
 * (`deploy/scripts/self-service-solve-pow.mjs`), que é uma reimplementação porque
 * bundlar o TypeScript exigiria criar arquivo nesta lane.
 *
 * O gate do platform prova solucionador ↔ verificador Go pelo fio real. A suíte
 * do módulo prova módulo ↔ seus próprios vetores. Faltava o terceiro lado: se
 * este módulo divergir, seus testes divergem junto, o gate segue verde com a
 * própria reimplementação, e o canal aberto quebra em produção sem sinal algum.
 *
 * Este teste executa o **artefato real** do gate, não uma cópia dele.
 */

/**
 * Ancorado no cwd do Vitest (`apps/web`, a raiz do workspace web) e não em
 * `import.meta.url`, que sob o ambiente jsdom não é uma URL `file:`. A primeira
 * asserção da suíte falha com o caminho resolvido impresso, para que uma
 * eventual mudança de raiz apareça como mensagem clara em vez de erro obscuro.
 */
const gateSolverPath = resolve(
  process.cwd(),
  "../../deploy/scripts/self-service-solve-pow.mjs",
);

/** Desafios com a forma real: base64url opaco emitido pelo cunhador Go. */
const wireCases = [
  { challenge: "vi6y1p2vboae4CwELwF6TQAAAABqgnieCmyl5nFXr0X7iY2b", bits: 8 },
  { challenge: "Y3VtdXJ1LXBvdy1zZWxmLXNlcnZpY2UtY2hhbGxlbmdlLTAx", bits: 10 },
  { challenge: "aG9zcGVkYWdlbS1jYXJ0YXotZGUtcGFyZWRlLTAwMDAwMDAy", bits: 12 },
] as const;

function solveWithGateScript(challenge: string, bits: number) {
  return execFileSync(process.execPath, [gateSolverPath, challenge, String(bits)], {
    encoding: "utf8",
  });
}

/**
 * Terceiro caminho, independente dos dois sob teste: confirma que a solução em
 * que ambos concordam de fato satisfaz o contrato. Sem isto, duas implementações
 * igualmente erradas passariam de mãos dadas.
 */
function leadingZeroBitsOf(challenge: string, solution: string) {
  const digest = createHash("sha256")
    .update(challenge, "utf8")
    .update(solution, "utf8")
    .digest();
  return countLeadingZeroBits(new Uint8Array(digest));
}

describe("convenção de fio do proof-of-work: módulo ↔ solucionador do gate", () => {
  it("o solucionador do gate existe onde o par presume", () => {
    expect(
      existsSync(gateSolverPath),
      `${gateSolverPath} sumiu: o par módulo ↔ gate deixou de ser demonstrável`,
    ).toBe(true);
  });

  it.each(wireCases)(
    "produz a mesma solução para o mesmo desafio (dificuldade $bits)",
    async ({ challenge, bits }) => {
      const fromModule = await solveProofOfWork({
        challenge,
        difficultyBits: bits,
      });
      const fromGate = solveWithGateScript(challenge, bits);

      expect(fromModule).toBe(fromGate);
      expect(fromModule).toMatch(/^[A-Za-z0-9_-]{1,64}$/u);
      expect(leadingZeroBitsOf(challenge, fromModule)).toBeGreaterThanOrEqual(
        bits,
      );
    },
  );

  it("percorre os contadores na mesma ordem, então a primeira solução coincide", async () => {
    const { challenge, bits } = wireCases[1];

    const first = await solveProofOfWork({
      challenge,
      difficultyBits: bits,
      batchSize: 1,
    });
    const second = await solveProofOfWork({
      challenge,
      difficultyBits: bits,
      batchSize: 4096,
    });

    // O tamanho do lote é detalhe de agendamento, nunca da convenção de fio: se
    // ele mudasse a solução, o módulo deixaria de casar com o gate a cada
    // ajuste de responsividade.
    expect(first).toBe(second);
    expect(first).toBe(solveWithGateScript(challenge, bits));
  });
});
