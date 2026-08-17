// Solucionador do desafio de proof-of-work da Fase 7, para o gate integrado.
//
// A convenção de fio está fixada no cabeçalho de
// `apps/web/src/shared/security/proof-of-work.ts` e no verificador Go
// (`proofofwork.go`), e é fronteira produtor-consumidor entre os dois:
//
//   solution = base64url sem padding de um contador de 8 bytes big-endian
//   digest   = SHA-256( utf8(challenge) ‖ utf8(solution) )
//   válida  ⇔ zeros à esquerda do digest, MSB first, >= difficulty_bits
//
// Esta é uma reimplementação da convenção, não o módulo do cliente: bundlar o
// TypeScript exigiria criar arquivo em `apps/web/**`, que é lane alheia. O que o
// gate prova é a cadeia real emissor → transporte → verificador → livro de
// nonces; se o cliente divergir desta convenção, quem acusa é a suíte Vitest do
// próprio módulo, que fixa os mesmos vetores.

import { createHash } from "node:crypto";

const MAX_ATTEMPTS = 1 << 24;

function base64UrlEncode(bytes) {
  return Buffer.from(bytes).toString("base64url");
}

function countLeadingZeroBits(digest) {
  let bits = 0;
  for (const byte of digest) {
    if (byte !== 0) {
      return bits + Math.clz32(byte) - 24;
    }
    bits += 8;
  }
  return bits;
}

function solve(challenge, difficultyBits) {
  const counter = Buffer.alloc(8);
  for (let attempt = 0; attempt < MAX_ATTEMPTS; attempt += 1) {
    counter.writeBigUInt64BE(BigInt(attempt));
    const solution = base64UrlEncode(counter);
    const digest = createHash("sha256")
      .update(challenge, "utf8")
      .update(solution, "utf8")
      .digest();
    if (countLeadingZeroBits(digest) >= difficultyBits) {
      return solution;
    }
  }
  throw new Error(`no solution within ${MAX_ATTEMPTS} attempts`);
}

const [challenge, difficulty] = process.argv.slice(2);
if (!challenge || !difficulty) {
  console.error("usage: phase7-solve-pow.mjs <challenge> <difficulty_bits>");
  process.exit(2);
}

const bits = Number.parseInt(difficulty, 10);
if (!Number.isInteger(bits) || bits < 1 || bits > 32) {
  console.error(`difficulty out of range: ${difficulty}`);
  process.exit(2);
}

process.stdout.write(solve(challenge, bits));
