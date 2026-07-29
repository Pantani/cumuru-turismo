import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AuthSessionProvider } from "../../shared/auth/AuthSession";
import { OperatorWorkspace } from "./OperatorWorkspace";

const uuid = "018f4e59-7a2a-7b12-8fd7-5d2e8dc99b80";

function phase2Response(body: unknown, init: ResponseInit = {}) {
  const headers = new Headers(init.headers);
  headers.set("Cache-Control", "no-store");
  headers.set("X-Request-ID", "request-operator-workspace-test");
  return Response.json(body, { ...init, headers });
}

function renderWorkspace() {
  return render(
    <AuthSessionProvider accessToken="opaque-test-token">
      <OperatorWorkspace />
    </AuthSessionProvider>,
  );
}

function stayResponse() {
  return {
    id: uuid,
    status: "draft",
    version: 1,
  };
}

describe("workspace operacional", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("renderiza projeção mínima e captura o ETag retornado", async () => {
    const user = userEvent.setup();
    const fetcher = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        phase2Response({ items: [], next_cursor: null }),
      )
      .mockResolvedValueOnce(
        phase2Response(
          {
            id: uuid,
            organization_id: uuid,
            name: "Pousada",
            category: "pousada",
            status: "active",
            version: 7,
            created_at: "2026-07-28T12:00:00Z",
            updated_at: "2026-07-28T12:00:00Z",
          },
          { headers: { ETag: '"7"' } },
        ),
      );
    renderWorkspace();
    const section = screen.getByRole("region", {
      name: "Acomodações e vínculos",
    });

    await user.click(within(section).getByRole("button", {
      name: "Listar acomodações",
    }));
    expect(
      await within(section).findByText("0 acomodação(ões) disponível(is)."),
    ).toBeInTheDocument();

    await user.type(within(section).getByLabelText("ID da acomodação"), uuid);
    await user.click(within(section).getByRole("button", {
      name: "Consultar acomodação",
    }));

    expect(await within(section).findByText(/Acomodação .*active/)).toHaveTextContent(
      `Acomodação ${uuid}: active, versão 7.`,
    );
    expect(within(section).getByLabelText("ETag da acomodação")).toHaveValue(
      '"7"',
    );
    expect(within(section).getByLabelText("ETag do vínculo")).toHaveValue("");
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("anuncia validação e foca o primeiro campo inválido", () => {
    renderWorkspace();
    const section = screen.getByRole("region", { name: "Estadias" });
    const button = within(section).getByRole("button", {
      name: "Criar estadia",
    });

    fireEvent.submit(button.closest("form") as HTMLFormElement);

    expect(
      within(section).getByText("Informe um identificador válido."),
    ).toBeInTheDocument();
    expect(within(section).getByLabelText("ID da acomodação")).toHaveFocus();
  });

  it("mantém a chave no retry e rotaciona somente após sucesso", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        phase2Response(
          {
            type: "urn:cumuru:problem:conflict",
            title: "Conflito",
            status: 409,
          },
          {
            status: 409,
            headers: {
              "Content-Type": "application/problem+json",
              "Retry-After": "1",
            },
          },
        ),
      )
      .mockResolvedValue(
        phase2Response(stayResponse(), {
          status: 201,
          headers: {
            ETag: '"1"',
            Location: `/api/v1/stays/${uuid}`,
            "Idempotency-Replayed": "false",
          },
        }),
      );
    renderWorkspace();
    const section = screen.getByRole("region", { name: "Estadias" });
    await user.type(within(section).getByLabelText("ID da acomodação"), uuid);
    await user.type(
      within(section).getByLabelText("Chegada prevista"),
      "2026-08-01",
    );
    await user.type(
      within(section).getByLabelText("Saída prevista"),
      "2026-08-03",
    );
    const create = within(section).getByRole("button", {
      name: "Criar estadia",
    });

    await user.click(create);
    await within(section).findByText(/Conflito/);
    await user.click(create);
    await within(section).findByText("Criar estadia: concluído.");
    await user.click(create);

    const calls = vi.mocked(globalThis.fetch).mock.calls;
    const first = calls[0]?.[0] as Request;
    const second = calls[1]?.[0] as Request;
    const third = calls[2]?.[0] as Request;
    const firstKey = first.headers.get("Idempotency-Key");
    expect(second.headers.get("Idempotency-Key")).toBe(firstKey);
    expect(third.headers.get("Idempotency-Key")).not.toBe(firstKey);
    const firstSubmission = (await first.clone().json()).client_submission_id;
    expect(firstSubmission).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
    );
    expect((await second.clone().json()).client_submission_id).toBe(
      firstSubmission,
    );
    expect((await third.clone().json()).client_submission_id).not.toBe(
      firstSubmission,
    );
  });

  it("mantém alterações concorrentes desabilitadas até receber ETag forte", () => {
    renderWorkspace();
    const accommodation = screen.getByRole("region", {
      name: "Acomodações e vínculos",
    });
    const stays = screen.getByRole("region", { name: "Estadias" });

    expect(
      within(accommodation).getByLabelText("ETag da acomodação"),
    ).toHaveAttribute("aria-invalid", "true");
    expect(
      within(accommodation).getByRole("button", {
        name: "Atualizar acomodação",
      }),
    ).toBeDisabled();
    expect(within(stays).getByLabelText("ETag da estadia")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(
      within(stays).getByRole("button", { name: "Atualizar estadia" }),
    ).toBeDisabled();
  });

  it("envia no fluxo assistido somente os valores generalizados editados", async () => {
    const user = userEvent.setup();
    const fetcher = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      phase2Response(
        {
          submission_id: uuid,
          status: "accepted",
        },
        {
          headers: {
            ETag: '"4"',
            "Idempotency-Replayed": "false",
            "Survey-Capability": "payload.signature",
          },
        },
      ),
    );
    renderWorkspace();
    const section = screen.getByRole("region", {
      name: "Grupo, convite e ciclo da estadia",
    });
    await user.type(within(section).getByLabelText("ID da estadia"), uuid);
    await user.type(
      within(section).getByLabelText("ETag da estadia"),
      '"3"',
    );
    await user.type(
      within(section).getByLabelText("Versão do aviso de privacidade"),
      "2026-07",
    );
    await user.selectOptions(
      within(section).getByLabelText("Faixa etária do visitante 1"),
      "60_plus",
    );
    await user.clear(
      within(section).getByLabelText("País de residência do visitante 1"),
    );
    await user.type(
      within(section).getByLabelText("País de residência do visitante 1"),
      "AR",
    );

    await user.click(
      within(section).getByRole("button", {
        name: "Enviar grupo assistido",
      }),
    );
    await within(section).findByText("Enviar grupo assistido: concluído.");

    const request = fetcher.mock.calls[0]?.[0] as Request;
    const body = await request.json();
    expect(body.client_submission_id).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
    );
    expect(body.visitors).toEqual([
      expect.objectContaining({
        client_id: expect.stringMatching(
          /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i,
        ),
        role: "responsible",
        age_band: "60_plus",
        residence_country: "AR",
      }),
    ]);
    expect(body.visitors[0]).not.toHaveProperty("residence_state");
    expect(body.visitors[0]).not.toHaveProperty("residence_city_code");
  });

  it("não apresenta violações axe no workspace autenticado", async () => {
    const { container } = renderWorkspace();

    const report = await axe.run(container, {
      rules: { "color-contrast": { enabled: false } },
    });

    expect(report.violations).toEqual([]);
  });
});
