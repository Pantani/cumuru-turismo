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
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
});
