import "@testing-library/jest-dom/vitest";
import "fake-indexeddb/auto";

/**
 * jsdom não implementa `ResizeObserver`, e a barra de seções da capa depende
 * dele para publicar a própria altura. O duplo entra só quando o ambiente não
 * traz o original: no dia em que jsdom implementar o observador de verdade, o
 * teste passa a exercitá-lo em vez de continuar preso a um substituto que
 * esconderia a diferença. Ele nunca dispara porque em jsdom nada tem altura —
 * medir aqui só produziria zero. A medida de verdade é reprovada no navegador,
 * em `deploy/e2e/local-demo.spec.ts`.
 */
class NoopResizeObserver implements ResizeObserver {
  disconnect(): void {}
  observe(_target: Element, _options?: ResizeObserverOptions): void {}
  unobserve(_target: Element): void {}
}

if (typeof globalThis.ResizeObserver === "undefined") {
  globalThis.ResizeObserver = NoopResizeObserver;
}
