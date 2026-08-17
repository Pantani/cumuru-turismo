/**
 * Second proof for the open channel, solved in the browser with WebCrypto only.
 * No CAPTCHA, no third party, no cookie: the public form must not depend on an
 * external network and must not ship the guest's IP or behaviour to anyone
 * (ADR-039, `study/privacy.md` §4).
 *
 * The challenge is opaque and MAC-authenticated by the API; the client cannot
 * lower the difficulty, because the difficulty travels inside the MAC. The work
 * is a toll, not a bot defence: what protects data subjects here is
 * human approval, purge on rejection and expiry, and the presence filter.
 *
 * Wire convention, pinned by the tests in `proof-of-work.test.ts`:
 * `solution` is the base64url (unpadded) encoding of an 8-byte big-endian
 * counter, and the digest is `SHA-256(utf8(challenge) ‖ utf8(solution))` with
 * the difficulty measured in leading zero bits, most significant bit first.
 * The contract fixes only the alphabets, so this encoding is a producer and
 * consumer boundary shared with the backend verifier.
 */

const SOLUTION_BYTES = 8;
const DEFAULT_BATCH_SIZE = 256;
const DEFAULT_MAX_ATTEMPTS = 1 << 24;
const MIN_DIFFICULTY_BITS = 1;
const MAX_DIFFICULTY_BITS = 32;

export class ProofOfWorkAbortedError extends Error {
  constructor() {
    super("A prova de trabalho foi interrompida.");
    this.name = "ProofOfWorkAbortedError";
  }
}

export class ProofOfWorkExhaustedError extends Error {
  constructor() {
    super("Não foi possível concluir a prova de trabalho neste dispositivo.");
    this.name = "ProofOfWorkExhaustedError";
  }
}

export interface SolveProofOfWorkInput {
  batchSize?: number;
  challenge: string;
  difficultyBits: number;
  maxAttempts?: number;
  onAttempt?: (attempts: number) => void;
  signal?: AbortSignal;
}

export function countLeadingZeroBits(digest: Uint8Array) {
  let bits = 0;
  for (const byte of digest) {
    if (byte !== 0) {
      return bits + Math.clz32(byte) - 24;
    }
    bits += 8;
  }
  return bits;
}

export function base64UrlEncode(bytes: Uint8Array) {
  let binary = "";
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary)
    .replace(/\+/gu, "-")
    .replace(/\//gu, "_")
    .replace(/=+$/u, "");
}

export function digestInputFor(challenge: string, solution: string) {
  return new TextEncoder().encode(`${challenge}${solution}`);
}

function counterBytes(counter: number) {
  const bytes = new Uint8Array(SOLUTION_BYTES);
  new DataView(bytes.buffer).setBigUint64(0, BigInt(counter));
  return bytes;
}

async function attemptSolution(challenge: string, counter: number) {
  const solution = base64UrlEncode(counterBytes(counter));
  const digest = await crypto.subtle.digest(
    "SHA-256",
    digestInputFor(challenge, solution),
  );
  return { bits: countLeadingZeroBits(new Uint8Array(digest)), solution };
}

async function runBatch(
  challenge: string,
  difficultyBits: number,
  from: number,
  size: number,
) {
  for (let counter = from; counter < from + size; counter += 1) {
    const attempt = await attemptSolution(challenge, counter);
    if (attempt.bits >= difficultyBits) {
      return attempt.solution;
    }
  }
  return null;
}

/** A macrotask, not a microtask: awaiting digests alone still starves paint. */
function yieldToEventLoop() {
  return new Promise<void>((resolve) => {
    setTimeout(resolve, 0);
  });
}

function requireDifficulty(bits: number) {
  const supported =
    Number.isInteger(bits) &&
    bits >= MIN_DIFFICULTY_BITS &&
    bits <= MAX_DIFFICULTY_BITS;
  if (!supported) {
    throw new RangeError(
      "A dificuldade da prova de trabalho está fora do contrato.",
    );
  }
}

function requireNotAborted(signal: AbortSignal | undefined) {
  if (signal?.aborted === true) {
    throw new ProofOfWorkAbortedError();
  }
}

function reportAttempts(
  onAttempt: ((attempts: number) => void) | undefined,
  attempts: number,
) {
  if (onAttempt !== undefined) {
    onAttempt(attempts);
  }
}

export async function solveProofOfWork(input: SolveProofOfWorkInput) {
  requireDifficulty(input.difficultyBits);
  const batchSize = input.batchSize ?? DEFAULT_BATCH_SIZE;
  const maxAttempts = input.maxAttempts ?? DEFAULT_MAX_ATTEMPTS;
  for (let attempts = 0; attempts < maxAttempts; attempts += batchSize) {
    requireNotAborted(input.signal);
    const solution = await runBatch(
      input.challenge,
      input.difficultyBits,
      attempts,
      batchSize,
    );
    if (solution !== null) {
      return solution;
    }
    reportAttempts(input.onAttempt, attempts + batchSize);
    await yieldToEventLoop();
  }
  throw new ProofOfWorkExhaustedError();
}
