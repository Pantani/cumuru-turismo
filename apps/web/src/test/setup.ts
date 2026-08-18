import "@testing-library/jest-dom/vitest";
import "fake-indexeddb/auto";

/**
 * jsdom não implementa `ResizeObserver`, e a barra de seções da capa depende
 * dele para publicar a própria altura. O duplo abaixo nunca dispara porque em
 * jsdom nada tem altura: medir aqui só produziria zero. A medida de verdade é
 * reprovada no navegador, em `deploy/e2e/local-demo.spec.ts`.
 */
class NoopResizeObserver implements ResizeObserver {
  disconnect(): void {}
  observe(): void {}
  unobserve(): void {}
}

globalThis.ResizeObserver = NoopResizeObserver;
