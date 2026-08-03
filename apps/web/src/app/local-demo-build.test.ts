import { describe, expect, it } from "vitest";

import { validateLocalDemoBuild } from "../../local-demo-build";

describe("variante de build da demonstração local", () => {
  it("aceita o build padrão sem autoridade fictícia", () => {
    expect(() => validateLocalDemoBuild({})).not.toThrow();
    expect(() =>
      validateLocalDemoBuild({ VITE_LOCAL_DEMO_MODE: "false" }),
    ).not.toThrow();
  });

  it("exige sinal e identidade juntos", () => {
    expect(() =>
      validateLocalDemoBuild({ VITE_LOCAL_DEMO_MODE: "true" }),
    ).toThrow(/VITE_LOCAL_DEMO_IDENTITY/u);
    expect(() =>
      validateLocalDemoBuild({
        VITE_LOCAL_DEMO_IDENTITY: "fixture-token",
        VITE_LOCAL_DEMO_MODE: "false",
      }),
    ).toThrow(/requires local demo mode/u);
  });

  it("aceita somente a variante local completa", () => {
    expect(() =>
      validateLocalDemoBuild({
        VITE_LOCAL_DEMO_IDENTITY: "fixture-token",
        VITE_LOCAL_DEMO_MODE: "true",
      }),
    ).not.toThrow();
    expect(() =>
      validateLocalDemoBuild({ VITE_LOCAL_DEMO_MODE: "yes" }),
    ).toThrow(/must be true or false/u);
  });
});
