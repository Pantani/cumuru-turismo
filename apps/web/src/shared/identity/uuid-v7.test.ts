import { describe, expect, it } from "vitest";

import { createUuidV7, uuidV7Pattern } from "./uuid-v7";

describe("UUIDv7 de domínio", () => {
  it("codifica o timestamp Unix em milissegundos, versão 7 e variante RFC", () => {
    const now = 0x018f4e597a2a;
    const uuid = createUuidV7(now, new Uint8Array(10).fill(0xff));

    expect(uuid).toMatch(uuidV7Pattern);
    expect(uuid.slice(0, 13).replace("-", "")).toBe("018f4e597a2a");
    expect(uuid[14]).toBe("7");
    expect(uuid[19]).toBe("b");
  });

  it("produz identificadores únicos mesmo no mesmo milissegundo", () => {
    const values = new Set(
      Array.from({ length: 1_000 }, () => createUuidV7(1_722_160_000_000)),
    );

    expect(values.size).toBe(1_000);
  });
});
