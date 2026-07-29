import { readFile } from "node:fs/promises";
import vm from "node:vm";

import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

interface RequestLike {
  headers: Headers;
  method: string;
  mode: string;
  url: string;
}

type FetchListener = (event: {
  request: RequestLike;
  respondWith: (response: Promise<Response>) => void;
}) => void;

let fetchListener: FetchListener;
let source = "";
const cacheMatch = vi.fn().mockResolvedValue(undefined);
const cachePut = vi.fn();
const cacheOpen = vi.fn().mockResolvedValue({
  add: vi.fn(),
  put: cachePut,
});
const networkFetch = vi
  .fn()
  .mockResolvedValue(new Response("asset", { status: 200 }));

beforeAll(async () => {
  const path = new URL(
    "public/service-worker.js",
    `file://${process.cwd().replaceAll("\\", "/")}/`,
  );
  source = await readFile(path, "utf8");
  const listeners = new Map<string, FetchListener>();
  const sandbox = {
    URL,
    Request,
    Response,
    caches: {
      delete: vi.fn(),
      keys: vi.fn().mockResolvedValue([]),
      match: cacheMatch,
      open: cacheOpen,
    },
    fetch: networkFetch,
    self: {
      addEventListener: (type: string, listener: FetchListener) =>
        listeners.set(type, listener),
      clients: { claim: vi.fn() },
      location: { origin: "https://registro.invalid" },
      skipWaiting: vi.fn(),
    },
  };
  vm.runInNewContext(source, sandbox);
  fetchListener = listeners.get("fetch") as FetchListener;
});

beforeEach(() => {
  cacheMatch.mockClear();
  cacheOpen.mockClear();
  cachePut.mockClear();
  networkFetch.mockClear();
});

function isIntercepted(url: string, init: Partial<RequestLike> = {}) {
  const respondWith = vi.fn();
  fetchListener({
    request: {
      headers: new Headers(),
      method: "GET",
      mode: "cors",
      url,
      ...init,
    },
    respondWith,
  });
  return respondWith.mock.calls.length > 0;
}

describe("service worker versionado e fail-closed", () => {
  it("mantém um cache de shell explicitamente versionado", () => {
    expect(source).toContain('cumuru-shell-"');
    expect(source).toContain("`${SHELL_CACHE_PREFIX}v1`");
  });

  it("aceita asset same-origin somente sem query", () => {
    expect(isIntercepted("https://registro.invalid/assets/app.js")).toBe(true);
    expect(isIntercepted("https://registro.invalid/assets/app.js?v=1")).toBe(
      false,
    );
  });

  it.each([
    "https://registro.invalid/api/v1/health",
    "https://registro.invalid/convites/capability",
    "https://registro.invalid/registro?token=capability",
    "https://outro.invalid/assets/app.js",
  ])("não intercepta %s", (url) => {
    expect(isIntercepted(url)).toBe(false);
  });

  it("não intercepta request autenticado nem não-GET", () => {
    expect(
      isIntercepted("https://registro.invalid/assets/app.js", {
        headers: new Headers({ Authorization: "Bearer opaque" }),
      }),
    ).toBe(false);
    expect(
      isIntercepted("https://registro.invalid/assets/app.js", {
        method: "POST",
      }),
    ).toBe(false);
  });

  it.each([
    "https://registro.invalid/registro?TOKEN=capability",
    "https://registro.invalid/registro?InViTe_ToKeN=capability",
    "https://registro.invalid/registro?origem=qr&TOKEN=one&invite_token=two",
  ])(
    "não intercepta nem consulta cache em navegação com capability: %s",
    (url) => {
      expect(isIntercepted(url, { mode: "navigate" })).toBe(false);
      expect(cacheMatch).not.toHaveBeenCalled();
      expect(cacheOpen).not.toHaveBeenCalled();
      expect(cachePut).not.toHaveBeenCalled();
      expect(networkFetch).not.toHaveBeenCalled();
    },
  );

  it("não registra sincronização em segundo plano", () => {
    expect(source).not.toMatch(/addEventListener\(\s*["']sync["']/);
  });
});
