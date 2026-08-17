import { describe, expect, it } from "vitest";

import { ApiError } from "../../shared/api/http-client";
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
  return new ApiError(
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
  // D-13: afirmar só a ausência do título deixa passar o mutante do D-08, que
  // devolve a frase genérica — ela também não contém o título. O nome do teste
  // promete a cópia do cliente, então ele precisa exigi-la.
  it.each([
    [429, "Muitas tentativas", 60, "Já houve envios demais desta rede"],
    [409, "Requisição em processamento", 3, "O aviso de privacidade mudou"],
  ])(
    "status %s com Retry-After: usa a cópia do cliente e mantém o prazo",
    (status, serverTitle, seconds, expectedCopy) => {
      const message = describeSelfServiceFailure(
        retryable(status, serverTitle, seconds),
      );

      expect(message).toContain(expectedCopy);
      expect(message).not.toContain(serverTitle);
      expect(message).toContain(`${seconds} segundos`);
    },
  );

  it.each([409, 429, 503])(
    "status %s sem Retry-After: não repassa o título do servidor",
    (status) => {
      const serverTitle = "Upstream dependency returned an unexpected shape.";
      const message = describeSelfServiceFailure(
        new ApiError(status, { type: "urn:x", title: serverTitle, status }, null),
      );

      expect(message).not.toContain(serverTitle);
      expect(message).not.toContain("segundos");
    },
  );
});

/**
 * D-16. `httpapi.go:558-577` produz dois `409` semanticamente distintos:
 * aviso de privacidade desatualizado e submissão idempotente **em andamento**.
 * O canal aberto alcança o segundo de verdade — duplo envio em rede instável —
 * e mandá-lo pedir cartaz novo é instrução errada com aparência de frase limpa.
 */
const IN_PROGRESS = "https://turismo.prado.ba.gov.br/problems/idempotency-in-progress";

function inProgress(seconds: number) {
  return new ApiError(
    409,
    { type: IN_PROGRESS, title: "Requisição em processamento", status: 409 },
    seconds,
  );
}

describe("os dois 409 do canal aberto", () => {
  it("não manda pedir cartaz novo quando a submissão está em andamento", () => {
    const message = describeSelfServiceFailure(inProgress(3));

    expect(message).toContain("Já recebemos este envio");
    expect(message).not.toContain("cartaz atualizado");
    expect(message).not.toContain("aviso de privacidade mudou");
    expect(message).toContain("3 segundos");
  });

  it("mantém a cópia do aviso desatualizado para o outro 409", () => {
    const message = describeSelfServiceFailure(
      new ApiError(
        409,
        {
          type: "https://turismo.prado.ba.gov.br/problems/privacy-notice-version",
          title: "Versão do aviso divergente",
          status: 409,
        },
        null,
      ),
    );

    expect(message).toContain("cartaz atualizado");
  });

  it("concorda com o singular quando o prazo é de um segundo", () => {
    expect(describeSelfServiceFailure(inProgress(1))).toContain("1 segundo.");
  });
});
