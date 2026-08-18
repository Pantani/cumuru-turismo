import { defineConfig } from "@playwright/test";

const baseURL = process.env.LOCAL_E2E_BASE_URL;
if (baseURL === undefined || baseURL.length === 0) {
  throw new Error("LOCAL_E2E_BASE_URL is required");
}

export default defineConfig({
  testDir: "./e2e",
  testMatch: "local-demo.spec.ts",
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: "line",
  outputDir: "../_workspace/playwright/local-demo",
  use: {
    baseURL,
    browserName: "chromium",
    // A jornada afirma texto em português. O site negocia o idioma pelo
    // Accept-Language, então sem fixar o locale o Chromium do CI abriria a
    // página em inglês e as asserções falhariam por tradução, não por defeito.
    locale: "pt-BR",
    // A auditoria de contraste mede a cor computada, e `.stay-list > li` entra
    // por `rise`, que anima `opacity` de 0 a 1 com atraso escalonado. Medida no
    // meio do fade, a cor lida é a combinação com o fundo, não a do token: um
    // texto legível reprova por estar a meio caminho de aparecer. Reduzir o
    // movimento faz o CSS colapsar a animação (styles.css, bloco
    // prefers-reduced-motion) e a auditoria passa a ver o estado assentado, que
    // é o que ela pretende afirmar. Não afrouxa nada: WCAG cobre o estado
    // final, não o quadro intermediário de uma transição.
    reducedMotion: "reduce",
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
});
