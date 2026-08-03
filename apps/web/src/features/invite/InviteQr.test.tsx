import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { InviteQr } from "./InviteQr";

const { toCanvas } = vi.hoisted(() => ({ toCanvas: vi.fn() }));

vi.mock("qrcode", () => ({ default: { toCanvas } }));

describe("QR local do convite", () => {
  it("desenha no canvas sem inserir a URL sensível no DOM", () => {
    const sensitiveUrl = `https://registro.invalid/convites/${"a".repeat(64)}`;

    render(<InviteQr url={sensitiveUrl} onDiscard={vi.fn()} />);

    expect(toCanvas).toHaveBeenCalledWith(
      expect.any(HTMLCanvasElement),
      sensitiveUrl,
      expect.any(Object),
    );
    expect(document.body.innerHTML).not.toContain(sensitiveUrl);
    expect(screen.getByLabelText("Código QR do convite")).toBeInTheDocument();
  });

  it("oferece abertura no mesmo navegador sem renderizar a URL", async () => {
    const user = userEvent.setup();
    const openHere = vi.fn();
    const sensitiveUrl = `https://registro.invalid/convites/${"a".repeat(64)}`;

    render(
      <InviteQr
        url={sensitiveUrl}
        onDiscard={vi.fn()}
        onOpenHere={openHere}
      />,
    );
    await user.click(screen.getByRole("button", {
      name: "Abrir registro neste navegador",
    }));

    expect(openHere).toHaveBeenCalledOnce();
    expect(document.body.innerHTML).not.toContain(sensitiveUrl);
  });
});
