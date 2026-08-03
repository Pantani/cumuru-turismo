import { describe, expect, it, vi } from "vitest";

import {
  CAPABILITY_CHANGE_EVENT,
  notifyCapabilityChange,
} from "./capability-events";

describe("contrato de evento de capability", () => {
  it("publica o nome canônico compartilhado", () => {
    const listener = vi.fn();
    window.addEventListener(CAPABILITY_CHANGE_EVENT, listener);

    notifyCapabilityChange();

    expect(CAPABILITY_CHANGE_EVENT).toBe("cumuru:capability-change");
    expect(listener).toHaveBeenCalledOnce();
    window.removeEventListener(CAPABILITY_CHANGE_EVENT, listener);
  });
});
