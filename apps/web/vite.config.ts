import react from "@vitejs/plugin-react";
import { createLogger, loadEnv } from "vite";
import type { LogErrorOptions, ProxyOptions } from "vite";
import { defineConfig } from "vitest/config";

import { validateLocalDemoBuild } from "./local-demo-build";

const PROXY_ERROR_MESSAGE = "[vite] proxy upstream unavailable";

function containsCapability(message: string): boolean {
  return message.includes("/api/v1/invites/");
}

function isProxyError(message: string): boolean {
  return message.toLowerCase().includes("http proxy error");
}

function redactCapability(message: string): string {
  return message.replace(/\/api\/v1\/invites\/[^\s?]+(?:\?[^\s]*)?/gu, "/api/v1/invites/[redacted]");
}

const viteLogger = createLogger();
const defaultError = viteLogger.error.bind(viteLogger);
viteLogger.error = (message: string, options?: LogErrorOptions) => {
  if (isProxyError(message)) {
    defaultError(PROXY_ERROR_MESSAGE);
    return;
  }
  if (containsCapability(message)) {
    defaultError(redactCapability(message));
    return;
  }
  defaultError(message, options);
};

const configureProxy: NonNullable<ProxyOptions["configure"]> = (proxy) => {
  proxy.on("proxyReq", (proxyRequest, request) => {
    const remoteAddress = request.socket.remoteAddress ?? "unknown";
    proxyRequest.removeHeader("referer");
    proxyRequest.removeHeader("forwarded");
    proxyRequest.setHeader("x-forwarded-for", remoteAddress);
    proxyRequest.setHeader("x-real-ip", remoteAddress);
  });
};

const apiProxy: Record<string, ProxyOptions> = {
  "/api/v1": {
    target: process.env.CUMURU_VITE_PROXY_TARGET ?? "http://127.0.0.1:8080",
    changeOrigin: false,
    configure: configureProxy,
  },
};

export default defineConfig(({ mode }) => {
  const environment = {
    ...loadEnv(mode, process.cwd(), ""),
    ...process.env,
  };
  validateLocalDemoBuild(environment);
  return {
    customLogger: viteLogger,
    plugins: [react()],
    server: {
      port: 5173,
      strictPort: true,
      proxy: apiProxy,
    },
    preview: {
      port: 4173,
      strictPort: true,
      proxy: apiProxy,
    },
    test: {
      environment: "jsdom",
      setupFiles: ["./src/test/setup.ts"],
      restoreMocks: true,
    },
  };
});
