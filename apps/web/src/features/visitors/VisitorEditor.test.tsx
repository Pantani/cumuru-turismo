import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { type ReactElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { LocaleProvider } from "../../shared/i18n/LocaleProvider";
import {
  createVisitor,
  VisitorEditor,
  type VisitorEditorHandle,
} from "./VisitorEditor";

function renderVisitorEditor(element: ReactElement) {
  return render(<LocaleProvider initial="pt">{element}</LocaleProvider>);
}

describe("editor compartilhado de visitantes", () => {
  afterEach(cleanup);

  it("oferece todas as faixas etárias e adiciona visitante com UUIDv7", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const initial = [createVisitor("responsible")];

    renderVisitorEditor(<VisitorEditor visitors={initial} onChange={onChange} />);

    const ageBand = screen.getByLabelText("Faixa etária do visitante 1");
    expect(
      within(ageBand).getAllByRole("option").map((option) => option.getAttribute("value")),
    ).toEqual([
      "0_5",
      "6_11",
      "12_17",
      "18_24",
      "25_34",
      "35_44",
      "45_59",
      "60_plus",
    ]);

    await user.click(screen.getByRole("button", { name: "Adicionar visitante" }));

    expect(onChange).toHaveBeenCalledWith([
      initial[0],
      expect.objectContaining({
        client_id: expect.stringMatching(
          /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
        ),
        role: "companion",
      }),
    ]);
  });

  it("impede remover o último visitante e adicionar além de 100", () => {
    const initial = [createVisitor("responsible")];
    const visitors = Array.from({ length: 100 }, (_, index) =>
      createVisitor(index === 0 ? "responsible" : "companion"),
    );
    const { rerender } = renderVisitorEditor(
      <VisitorEditor visitors={initial} onChange={vi.fn()} />,
    );

    expect(
      screen.getByRole("button", { name: "Remover visitante 1" }),
    ).toBeDisabled();

    rerender(
      <LocaleProvider initial="pt">
        <VisitorEditor visitors={visitors} onChange={vi.fn()} />
      </LocaleProvider>,
    );

    expect(
      screen.getByRole("button", { name: "Adicionar visitante" }),
    ).toBeDisabled();
  }, 15_000);

  it("marca e foca o primeiro campo inválido", () => {
    const handle = { current: null as VisitorEditorHandle | null };
    renderVisitorEditor(
      <VisitorEditor
        ref={handle}
        visitors={[createVisitor("responsible")]}
        onChange={vi.fn()}
        issues={[
          {
            field: "visitors.0.residence_country",
            message: "Use o código ISO alpha-2.",
          },
        ]}
      />,
    );

    handle.current?.focus("visitors.0.residence_country");

    expect(
      screen.getByLabelText("País de residência do visitante 1"),
    ).toHaveAttribute("aria-invalid", "true");
    expect(
      screen.getByLabelText("País de residência do visitante 1"),
    ).toHaveFocus();
  });
});
