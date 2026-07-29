import { readFile } from "node:fs/promises";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

describe("ordem de bootstrap da capability", () => {
  it("remove a capability antes de criar a raiz React", async () => {
    const source = await readFile(join(process.cwd(), "src/main.tsx"), "utf8");

    expect(source.indexOf("captureInviteCapability(")).toBeGreaterThan(-1);
    expect(source.indexOf("captureInviteCapability(")).toBeLessThan(
      source.indexOf("createRoot(rootElement)"),
    );
    expect(source.indexOf("purgeExpiredDrafts()")).toBeGreaterThan(-1);
    expect(source.indexOf("purgeExpiredDrafts()")).toBeLessThan(
      source.indexOf("createRoot(rootElement)"),
    );
  });
});
