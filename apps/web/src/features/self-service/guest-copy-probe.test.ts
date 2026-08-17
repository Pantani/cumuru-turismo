import { describe, expect, it } from "vitest";

import { Phase7ApiError } from "../../shared/api/phase7-client";
import { describeSelfServiceFailure } from "./self-service-messages";

/**
 * QA probe, fourth gate. ae109ad claims `problem.title` no longer reaches the
 * guest on the three guest screens. On the open channel the guard is the status
 * table, but `describeSelfServiceFailure` tests `retryAfterSeconds !== null`
 * FIRST and returns the server title verbatim in that branch — so both
 * retryable outcomes the API can produce still hand engineering copy to the
 * guest, and the 409 entry of the status table becomes unreachable for them.
 *
 * httpapi.go:558-575 is the producer: rate-limited answers 429 with title
 * "Muitas tentativas" and Retry-After: 60; idempotency-in-progress answers 409
 * with title "Requisição em processamento" and Retry-After: N.
 */
function retryable(status: number, title: string, seconds: number) {
  return new Phase7ApiError(
    status,
    { type: "urn:cumuru:problem:x", title, status },
    seconds,
  );
}

describe("QA probe — cópia de hóspede nos status com Retry-After", () => {
  it("não repassa o título do 429 do servidor", () => {
    const message = describeSelfServiceFailure(
      retryable(429, "Muitas tentativas", 60),
    );
    expect(message).not.toContain("Muitas tentativas");
  });

  it("não repassa o título do 409 em processamento e usa a cópia do 409", () => {
    const message = describeSelfServiceFailure(
      retryable(409, "Requisição em processamento", 3),
    );
    expect(message).not.toContain("Requisição em processamento");
  });
});

/**
 * Extensão da sonda adotada: o ramo com `Retry-After` era o único quebrado, e o
 * teste anterior mockava um `503` **sem** `Retry-After` — exercitava o caminho
 * que já estava certo. Aqui os dois ramos são cobertos, e a parte acionável do
 * `Retry-After` é exigida em vez de tolerada.
 */
describe("cópia de hóspede nos dois ramos", () => {
  it.each([
    [429, "Muitas tentativas", 60],
    [409, "Requisição em processamento", 3],
  ])(
    "status %s com Retry-After: usa a cópia do cliente e mantém o prazo",
    (status, serverTitle, seconds) => {
      const message = describeSelfServiceFailure(
        retryable(status, serverTitle, seconds),
      );

      expect(message).not.toContain(serverTitle);
      expect(message).toContain(`${seconds} segundos`);
    },
  );

  it.each([409, 429, 503])(
    "status %s sem Retry-After: não repassa o título do servidor",
    (status) => {
      const serverTitle = "Upstream dependency returned an unexpected shape.";
      const message = describeSelfServiceFailure(
        new Phase7ApiError(status, { type: "urn:x", title: serverTitle, status }, null),
      );

      expect(message).not.toContain(serverTitle);
      expect(message).not.toContain("segundos");
    },
  );
});
