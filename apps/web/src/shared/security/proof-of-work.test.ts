import { describe, expect, it, vi } from "vitest";

import {
  base64UrlEncode,
  countLeadingZeroBits,
  digestInputFor,
  ProofOfWorkAbortedError,
  ProofOfWorkExhaustedError,
  solveProofOfWork,
} from "./proof-of-work";

const challenge = "Y3VtdXJ1LXBvdy10ZXN0LWNoYWxsZW5nZS0wMDAwMDAwMQ";

async function digestBits(solution: string) {
  const digest = await crypto.subtle.digest(
    "SHA-256",
    digestInputFor(challenge, solution),
  );
  return countLeadingZeroBits(new Uint8Array(digest));
}

describe("countLeadingZeroBits", () => {
  it("conta byte a byte e para no primeiro bit em um", () => {
    expect(countLeadingZeroBits(new Uint8Array([0xff]))).toBe(0);
    expect(countLeadingZeroBits(new Uint8Array([0x7f]))).toBe(1);
    expect(countLeadingZeroBits(new Uint8Array([0x01]))).toBe(7);
    expect(countLeadingZeroBits(new Uint8Array([0x00, 0x80]))).toBe(8);
    expect(countLeadingZeroBits(new Uint8Array([0x00, 0x0f]))).toBe(12);
    expect(countLeadingZeroBits(new Uint8Array([0x00, 0x00]))).toBe(16);
  });
});

describe("base64UrlEncode", () => {
  it("emite alfabeto seguro para URL e sem preenchimento", () => {
    const encoded = base64UrlEncode(
      new Uint8Array([0xff, 0xef, 0xfe, 0x00, 0x01, 0x02, 0x03, 0x04]),
    );

    expect(encoded).toMatch(/^[A-Za-z0-9_-]+$/u);
    expect(encoded).not.toContain("=");
  });
});

describe("digestInputFor", () => {
  it("concatena desafio e solução em UTF-8, nessa ordem", () => {
    const input = digestInputFor("ab", "cd");

    expect(Array.from(input)).toEqual([0x61, 0x62, 0x63, 0x64]);
  });
});

describe("solveProofOfWork", () => {
  it("devolve solução cujo digest satisfaz a dificuldade exigida", async () => {
    const solution = await solveProofOfWork({
      challenge,
      difficultyBits: 10,
    });

    expect(solution).toMatch(/^[A-Za-z0-9_-]{1,64}$/u);
    await expect(digestBits(solution)).resolves.toBeGreaterThanOrEqual(10);
  });

  it("relata progresso para a região viva sem expor o desafio", async () => {
    const onAttempt = vi.fn();

    await solveProofOfWork({
      challenge,
      difficultyBits: 8,
      batchSize: 4,
      onAttempt,
    });

    expect(onAttempt).toHaveBeenCalled();
    for (const call of onAttempt.mock.calls) {
      expect(typeof call[0]).toBe("number");
    }
  });

  it("interrompe quando o sinal é abortado", async () => {
    const controller = new AbortController();
    controller.abort();

    await expect(
      solveProofOfWork({
        challenge,
        difficultyBits: 32,
        signal: controller.signal,
      }),
    ).rejects.toBeInstanceOf(ProofOfWorkAbortedError);
  });

  it("desiste com erro próprio quando esgota o orçamento de tentativas", async () => {
    await expect(
      solveProofOfWork({
        challenge,
        difficultyBits: 32,
        maxAttempts: 64,
        batchSize: 16,
      }),
    ).rejects.toBeInstanceOf(ProofOfWorkExhaustedError);
  });

  it("recusa dificuldade fora da faixa do contrato", async () => {
    await expect(
      solveProofOfWork({ challenge, difficultyBits: 0 }),
    ).rejects.toThrow(RangeError);
    await expect(
      solveProofOfWork({ challenge, difficultyBits: 33 }),
    ).rejects.toThrow(RangeError);
  });
});
