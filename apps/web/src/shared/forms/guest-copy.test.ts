import { describe, expect, it } from "vitest";

import {
  GUEST_UNEXPECTED_FAILURE,
  guestCopyFor,
  IDEMPOTENCY_IN_PROGRESS,
} from "./guest-copy";

const messages: Readonly<Record<number, string>> = {
  409: "O aviso mudou. Peça outro à hospedagem.",
  429: "Já houve envios demais desta rede agora há pouco.",
};

const OTHER_409 = "https://turismo.prado.ba.gov.br/problems/privacy-notice-version";

function failure(
  status: number,
  retryAfterSeconds: number | null,
  type = "https://turismo.prado.ba.gov.br/problems/generic",
) {
  return { problem: { type }, retryAfterSeconds, status };
}

describe("cópia de hóspede", () => {
  it("usa a tabela quando o status é conhecido", () => {
    expect(guestCopyFor(failure(409, null, OTHER_409), messages)).toBe(
      messages[409],
    );
  });

  it("cai na frase genérica quando o status não é explicável", () => {
    expect(guestCopyFor(failure(503, null), messages)).toBe(
      GUEST_UNEXPECTED_FAILURE,
    );
  });

  // D-08: o prazo é informação acionável e não pode sumir junto com o título
  // técnico. O que muda é de onde vem a frase, não se o prazo aparece.
  it.each([
    [409, 3, OTHER_409],
    [429, 60, "https://turismo.prado.ba.gov.br/problems/rate-limited"],
    [503, 12, "https://turismo.prado.ba.gov.br/problems/generic"],
  ])("acrescenta o prazo do Retry-After ao status %s", (status, seconds, type) => {
    const message = guestCopyFor(failure(status, seconds, type), messages);

    expect(message).toContain(`${seconds} segundos`);
    expect(message.startsWith(messages[status] ?? GUEST_UNEXPECTED_FAILURE)).toBe(
      true,
    );
  });

  it("não menciona prazo quando o servidor não mandou Retry-After", () => {
    expect(guestCopyFor(failure(429, null), messages)).not.toContain("segundos");
  });

  // D-16: o `409` tem duas causas e elas não podem virar a mesma frase.
  describe("os dois 409", () => {
    it("a submissão em andamento tem cópia própria, não a do aviso", () => {
      const message = guestCopyFor(
        failure(409, 3, IDEMPOTENCY_IN_PROGRESS),
        messages,
      );

      expect(message).toContain("Já recebemos este envio");
      expect(message).not.toContain(messages[409]!);
      expect(message).toContain("3 segundos");
    });

    it("o tipo decide antes do status, em qualquer tabela de tela", () => {
      const outra: Readonly<Record<number, string>> = {
        409: "A pesquisa mudou. Recarregue a página.",
      };

      expect(
        guestCopyFor(failure(409, null, IDEMPOTENCY_IN_PROGRESS), outra),
      ).toBe("Já recebemos este envio e estamos concluindo.");
    });
  });

  it("concorda o singular quando o prazo é de um segundo", () => {
    expect(
      guestCopyFor(failure(409, 1, IDEMPOTENCY_IN_PROGRESS), messages),
    ).toContain("1 segundo.");
  });
});
