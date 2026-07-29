import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  captureInviteCapability,
  clearInviteCapability,
  peekInviteCapability,
} from "./invite-capability";

const token = "a".repeat(64);

describe("capability de convite", () => {
  beforeEach(() => clearInviteCapability());

  it.each([
    [`https://registro.invalid/convites/${token}`, "/registro"],
    [`https://registro.invalid/registro?convite=${token}`, "/registro"],
    [`https://registro.invalid/registro?token=${token}&utm=x`, "/registro?utm=x"],
  ])("extrai e remove a capability antes do render", (source, expectedPath) => {
    const replace = vi.fn();

    captureInviteCapability(new URL(source), replace);

    expect(peekInviteCapability()).toBe(token);
    expect(replace).toHaveBeenCalledWith(expectedPath);
  });

  it("remove token inválido sem mantê-lo em memória", () => {
    const replace = vi.fn();

    captureInviteCapability(
      new URL("https://registro.invalid/convites/curto"),
      replace,
    );

    expect(peekInviteCapability()).toBeNull();
    expect(replace).toHaveBeenCalledWith("/registro");
  });

  it("decodifica path válido e elimina todas as queries sensíveis", () => {
    const replace = vi.fn();
    const encoded = encodeURIComponent(token);

    captureInviteCapability(
      new URL(
        `https://registro.invalid/convites/${encoded}?TOKEN=curto&Token=outro&safe=1`,
      ),
      replace,
    );

    expect(peekInviteCapability()).toBe(token);
    expect(replace).toHaveBeenCalledWith("/registro?safe=1");
  });

  it("encontra valor válido entre chaves múltiplas sem preservar inválidos", () => {
    const replace = vi.fn();

    captureInviteCapability(
      new URL(
        `https://registro.invalid/registro?token=curto&CONVITE=${token}&invite=inválido`,
      ),
      replace,
    );

    expect(peekInviteCapability()).toBe(token);
    expect(replace).toHaveBeenCalledWith("/registro");
  });
});
