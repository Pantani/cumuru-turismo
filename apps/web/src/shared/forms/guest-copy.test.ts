import { describe, expect, it } from "vitest";

import { GUEST_UNEXPECTED_FAILURE, guestCopyFor } from "./guest-copy";

const messages: Readonly<Record<number, string>> = {
  409: "O aviso mudou. Peça outro à hospedagem.",
  429: "Já houve envios demais desta rede agora há pouco.",
};

describe("cópia de hóspede", () => {
  it("usa a tabela quando o status é conhecido", () => {
    expect(guestCopyFor({ status: 409, retryAfterSeconds: null }, messages)).toBe(
      messages[409],
    );
  });

  it("cai na frase genérica quando o status não é explicável", () => {
    expect(guestCopyFor({ status: 503, retryAfterSeconds: null }, messages)).toBe(
      GUEST_UNEXPECTED_FAILURE,
    );
  });

  // D-08: o prazo é informação acionável e não pode sumir junto com o título
  // técnico. O que muda é de onde vem a frase, não se o prazo aparece.
  it.each([
    [409, 3],
    [429, 60],
    [503, 12],
  ])("acrescenta o prazo do Retry-After ao status %s", (status, seconds) => {
    const message = guestCopyFor(
      { status, retryAfterSeconds: seconds },
      messages,
    );

    expect(message).toContain(`${seconds} segundos`);
    expect(message.startsWith(messages[status] ?? GUEST_UNEXPECTED_FAILURE)).toBe(
      true,
    );
  });

  it("não menciona prazo quando o servidor não mandou Retry-After", () => {
    expect(
      guestCopyFor({ status: 429, retryAfterSeconds: null }, messages),
    ).not.toContain("segundos");
  });
});
