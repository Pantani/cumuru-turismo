import { notifyCapabilityChange } from "./capability-events";

/**
 * ADR-039 moves the poster token from the path to the URL fragment: a fragment
 * never leaves the browser, so it cannot reach a request line, an access log, a
 * WAF or a CDN. The token is read once at boot, kept in a module closure and
 * erased from the address bar. It is never written to `localStorage`,
 * `sessionStorage`, IndexedDB or the query string, and never logged.
 */

const capabilityPattern = /^[A-Za-z0-9_-]{64,128}$/;

export interface FragmentCapability {
  capture: (url: URL, replace: (path: string) => void) => boolean;
  clear: () => void;
  /**
   * Esquece o token **sem** notificar, para quem precisa compor o descarte com
   * outra mudança de estado e emitir uma notificação só. Duas notificações em
   * sequência fazem a tela passar por um estado intermediário que nunca foi
   * verdadeiro.
   */
  forget: () => void;
  peek: () => string | null;
}

function canonicalPath(pathname: string) {
  return pathname.replace(/\/+$/u, "") || "/";
}

/**
 * A fragment that is not a capability — the encrypted draft pointer, for one —
 * is left untouched, so recovering a draft does not cost the URL it lives in.
 */
function capabilityFromHash(hash: string) {
  if (hash.length < 2) {
    return null;
  }
  try {
    const candidate = decodeURIComponent(hash.slice(1));
    return capabilityPattern.test(candidate) ? candidate : null;
  } catch {
    return null;
  }
}

export function createFragmentCapability(
  routePath: string,
): FragmentCapability {
  let capability: string | null = null;

  return {
    capture(url, replace) {
      if (canonicalPath(url.pathname) !== routePath) {
        return false;
      }
      const candidate = capabilityFromHash(url.hash);
      if (candidate === null) {
        return false;
      }
      capability = candidate;
      replace(routePath);
      notifyCapabilityChange();
      return true;
    },
    clear() {
      capability = null;
      notifyCapabilityChange();
    },
    forget() {
      capability = null;
    },
    peek() {
      return capability;
    },
  };
}
