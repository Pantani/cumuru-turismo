import { describe, expect, it, vi } from "vitest";

import { createFragmentCapability } from "./fragment-capability";

const token = "a".repeat(96);

/** Reads whatever web storage this environment exposes, without requiring it. */
function storageSnapshot() {
  const stores = [window.localStorage, window.sessionStorage].filter(
    (store): store is Storage => store !== undefined && store !== null,
  );
  return stores.map((store) => JSON.stringify({ ...store })).join("|");
}

describe("capability transportada no fragmento", () => {
  it("captura o token do fragmento e apaga o fragmento da barra de endereço", () => {
    const capability = createFragmentCapability("/i");
    const replace = vi.fn();

    const captured = capability.capture(
      new URL(`https://cumuru.invalid/i#${token}`),
      replace,
    );

    expect(captured).toBe(true);
    expect(capability.peek()).toBe(token);
    expect(replace).toHaveBeenCalledWith("/i");
  });

  it("não persiste o token em localStorage nem em sessionStorage", () => {
    const capability = createFragmentCapability("/i");

    capability.capture(new URL(`https://cumuru.invalid/i#${token}`), vi.fn());

    expect(storageSnapshot()).not.toContain(token);
    expect(document.cookie).not.toContain(token);
  });

  it("ignora rota diferente e preserva a URL", () => {
    const capability = createFragmentCapability("/i");
    const replace = vi.fn();

    const captured = capability.capture(
      new URL(`https://cumuru.invalid/registro#${token}`),
      replace,
    );

    expect(captured).toBe(false);
    expect(capability.peek()).toBeNull();
    expect(replace).not.toHaveBeenCalled();
  });

  it("preserva fragmento que não é capability, como o de rascunho", () => {
    const capability = createFragmentCapability("/i");
    const replace = vi.fn();

    const captured = capability.capture(
      new URL("https://cumuru.invalid/i#draft=018f4e59-7a2a-7b12-8fd7-5d2e8dc99b80"),
      replace,
    );

    expect(captured).toBe(false);
    expect(capability.peek()).toBeNull();
    expect(replace).not.toHaveBeenCalled();
  });

  it("aceita barra final na rota e recusa token fora do formato", () => {
    const capability = createFragmentCapability("/i");

    expect(
      capability.capture(new URL(`https://cumuru.invalid/i/#${token}`), vi.fn()),
    ).toBe(true);

    capability.clear();
    expect(
      capability.capture(new URL("https://cumuru.invalid/i#curto"), vi.fn()),
    ).toBe(false);
    expect(capability.peek()).toBeNull();
  });

  it("esquece o token quando a capability é descartada", () => {
    const capability = createFragmentCapability("/i");
    capability.capture(new URL(`https://cumuru.invalid/i#${token}`), vi.fn());

    capability.clear();

    expect(capability.peek()).toBeNull();
  });

  it("mantém capabilities de rotas distintas isoladas", () => {
    const poster = createFragmentCapability("/i");
    const activation = createFragmentCapability("/ativacao");

    poster.capture(new URL(`https://cumuru.invalid/i#${token}`), vi.fn());

    expect(activation.peek()).toBeNull();
    expect(poster.peek()).toBe(token);
  });
});
