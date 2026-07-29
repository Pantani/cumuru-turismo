import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, describe, expect, it, vi } from "vitest";

import QuestionnaireAdminPage from "../../pages/QuestionnaireAdminPage";
import { AuthSessionProvider } from "../../shared/auth/AuthSession";

const responseHeaders = {
  "Cache-Control": "no-store",
  "X-Request-ID": "request-questionnaire-admin-test",
};
const questionnaireId = "019f0000-0000-7000-8000-000000000062";
const versionId = "019f0000-0000-7000-8000-000000000061";
const secondQuestionnaireId = "019f0000-0000-7000-8000-000000000072";
const secondVersionId = "019f0000-0000-7000-8000-000000000071";

function renderPage(token: string | null = null) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <AuthSessionProvider accessToken={token}>
        <QuestionnaireAdminPage />
      </AuthSessionProvider>
    </QueryClientProvider>,
  );
}

function questionnairePage() {
  return {
    items: [
      {
        id: questionnaireId,
        stable_key: "tourism_profile",
        name: "Perfil turístico",
        created_at: "2026-07-28T12:00:00Z",
      },
    ],
    next_cursor: null,
  };
}

function versionPage() {
  return {
    items: [
      {
        id: versionId,
        questionnaire_id: questionnaireId,
        version_number: 2,
        revision: 4,
        status: "draft",
        title: "Pesquisa retomada",
        privacy_notice_version: "survey-v2",
        created_at: "2026-07-28T12:00:00Z",
        updated_at: "2026-07-28T13:00:00Z",
      },
    ],
    next_cursor: null,
  };
}

function versionAdmin() {
  return {
    ...versionPage().items[0],
    introduction: "Introdução existente.",
    questions: [],
    consent_requirements: [],
    submitted_for_review_at: null,
    privacy_reviewed_at: null,
    published_at: null,
    retired_at: null,
  };
}

function secondQuestionnaire() {
  return {
    id: secondQuestionnaireId,
    stable_key: "visitor_experience",
    name: "Experiência do visitante",
    created_at: "2026-07-28T14:00:00Z",
  };
}

function secondVersion() {
  return {
    ...versionPage().items[0],
    id: secondVersionId,
    questionnaire_id: secondQuestionnaireId,
    version_number: 3,
    revision: 6,
    title: "Experiência selecionada",
  };
}

function adminFromSummary(
  summary: ReturnType<typeof secondVersion>,
  introduction: string,
) {
  return {
    ...summary,
    introduction,
    questions: [],
    consent_requirements: [],
    submitted_for_review_at: null,
    privacy_reviewed_at: null,
    published_at: null,
    retired_at: null,
  };
}

describe("administração de questionários", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("falha fechada sem sessão OIDC", () => {
    const fetcher = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetcher);

    renderPage();

    expect(
      screen.getByRole("heading", { name: "Acesso institucional necessário" }),
    ).toBeInTheDocument();
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("cria rascunho tipado e mantém o token fora do conteúdo", async () => {
    const user = userEvent.setup();
    const fetcher = vi.fn<typeof fetch>(async (input) => {
      const request = input as Request;
      if (request.method === "GET") {
        return Response.json(
          { items: [], next_cursor: null },
          { headers: responseHeaders },
        );
      }
      return Response.json(
        {
          id: versionId,
          questionnaire_id: questionnaireId,
          version_number: 1,
          revision: 1,
          status: "draft",
        },
        {
          status: 201,
          headers: {
            ...responseHeaders,
            ETag: '"1"',
            "Idempotency-Replayed": "false",
          },
        },
      );
    });
    vi.stubGlobal("fetch", fetcher);

    renderPage("opaque-editor-token");
    await screen.findByText("Nenhum questionário criado.");
    await user.click(screen.getByRole("button", { name: "Criar rascunho" }));

    expect(
      await screen.findByText("Questionário criado em rascunho."),
    ).toBeInTheDocument();
    const post = fetcher.mock.calls
      .map((call) => call[0] as Request)
      .find((request) => request.method === "POST");
    expect(post?.headers.get("Authorization")).toBe(
      "Bearer opaque-editor-token",
    );
    expect(document.body.textContent).not.toContain("opaque-editor-token");
  });

  it("lista e retoma a versão escolhida com seu ETag", async () => {
    const user = userEvent.setup();
    const fetcher = vi.fn<typeof fetch>(async (input) => {
      const request = input as Request;
      const url = new URL(request.url);
      if (url.pathname === "/api/v1/questionnaires") {
        return Response.json(questionnairePage(), { headers: responseHeaders });
      }
      if (url.pathname.endsWith("/versions")) {
        return Response.json(versionPage(), { headers: responseHeaders });
      }
      return Response.json(versionAdmin(), {
        headers: { ...responseHeaders, ETag: '"4"' },
      });
    });
    vi.stubGlobal("fetch", fetcher);

    const { container } = renderPage("opaque-editor-token");
    const questionnaireButton = await screen.findByRole("button", {
      name: "Perfil turístico",
    });
    questionnaireButton.focus();
    await user.keyboard("{Enter}");
    const versionButton = await screen.findByRole("button", {
      name: "Retomar versão 2 — rascunho",
    });
    versionButton.focus();
    await user.keyboard("{Enter}");

    expect(await screen.findByDisplayValue(versionId)).toBeInTheDocument();
    expect(screen.getByDisplayValue('"4"')).toBeInTheDocument();
    expect(screen.getByDisplayValue(/Pesquisa retomada/)).toBeInTheDocument();
    expect(
      screen.getByText("Versão 2 carregada: rascunho."),
    ).toBeInTheDocument();

    const requests = fetcher.mock.calls.map((call) => call[0] as Request);
    expect(
      requests.some(
        (request) =>
          new URL(request.url).pathname ===
          `/api/v1/questionnaires/${questionnaireId}/versions`,
      ),
    ).toBe(true);
    expect(
      requests.some(
        (request) =>
          new URL(request.url).pathname ===
          `/api/v1/questionnaire-versions/${versionId}`,
      ),
    ).toBe(true);

    const accessibility = await axe.run(container, {
      rules: { "color-contrast": { enabled: false } },
    });
    expect(accessibility.violations).toEqual([]);
  });

  it("pagina e remove duplicatas do catálogo e das versões", async () => {
    const user = userEvent.setup();
    const fetcher = vi.fn<typeof fetch>(async (input) => {
      const url = new URL((input as Request).url);
      const cursor = url.searchParams.get("cursor");
      if (url.pathname === "/api/v1/questionnaires") {
        const data =
          cursor === "questionnaires-next"
            ? {
                items: [questionnairePage().items[0], secondQuestionnaire()],
                next_cursor: null,
              }
            : { ...questionnairePage(), next_cursor: "questionnaires-next" };
        return Response.json(data, { headers: responseHeaders });
      }
      const data =
        cursor === "versions-next"
          ? {
              items: [versionPage().items[0], secondVersion()],
              next_cursor: null,
            }
          : { ...versionPage(), next_cursor: "versions-next" };
      return Response.json(data, { headers: responseHeaders });
    });
    vi.stubGlobal("fetch", fetcher);

    renderPage("opaque-editor-token");
    await user.click(
      await screen.findByRole("button", {
        name: "Carregar mais questionários",
      }),
    );
    expect(
      await screen.findByRole("button", { name: "Experiência do visitante" }),
    ).toBeInTheDocument();
    expect(
      screen.getAllByRole("button", { name: "Perfil turístico" }),
    ).toHaveLength(1);

    await user.click(screen.getByRole("button", { name: "Perfil turístico" }));
    await user.click(
      await screen.findByRole("button", { name: "Carregar mais versões" }),
    );
    expect(
      await screen.findByRole("button", {
        name: "Retomar versão 3 — rascunho",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getAllByRole("button", {
        name: "Retomar versão 2 — rascunho",
      }),
    ).toHaveLength(1);

    const urls = fetcher.mock.calls.map(
      (call) => new URL((call[0] as Request).url),
    );
    expect(
      urls.some(
        (url) =>
          url.pathname === "/api/v1/questionnaires" &&
          url.searchParams.get("cursor") === "questionnaires-next",
      ),
    ).toBe(true);
    expect(
      urls.some(
        (url) =>
          url.pathname.endsWith(`/${questionnaireId}/versions`) &&
          url.searchParams.get("cursor") === "versions-next",
      ),
    ).toBe(true);
  });

  it("preserva páginas carregadas e permite tentar novamente", async () => {
    const user = userEvent.setup();
    let questionnaireAttempts = 0;
    let versionAttempts = 0;
    const laterVersion = {
      ...versionPage().items[0],
      id: secondVersionId,
      version_number: 3,
      revision: 5,
      title: "Segunda página",
    };
    const fetcher = vi.fn<typeof fetch>(async (input) => {
      const url = new URL((input as Request).url);
      const cursor = url.searchParams.get("cursor");
      if (url.pathname === "/api/v1/questionnaires") {
        if (cursor === "questionnaires-next") {
          questionnaireAttempts += 1;
          if (questionnaireAttempts === 1) {
            return Response.json(
              { title: "Página indisponível.", status: 503 },
              { status: 503, headers: responseHeaders },
            );
          }
          return Response.json(
            { items: [secondQuestionnaire()], next_cursor: null },
            { headers: responseHeaders },
          );
        }
        return Response.json(
          { ...questionnairePage(), next_cursor: "questionnaires-next" },
          { headers: responseHeaders },
        );
      }
      if (cursor === "versions-next") {
        versionAttempts += 1;
        if (versionAttempts === 1) {
          return Response.json(
            { title: "Página indisponível.", status: 503 },
            { status: 503, headers: responseHeaders },
          );
        }
        return Response.json(
          { items: [laterVersion], next_cursor: null },
          { headers: responseHeaders },
        );
      }
      return Response.json(
        { ...versionPage(), next_cursor: "versions-next" },
        { headers: responseHeaders },
      );
    });
    vi.stubGlobal("fetch", fetcher);

    renderPage("opaque-editor-token");
    await user.click(
      await screen.findByRole("button", {
        name: "Carregar mais questionários",
      }),
    );
    expect(
      await screen.findByText("Não foi possível carregar mais questionários."),
    ).toHaveAttribute("role", "alert");
    expect(
      screen.getByRole("button", { name: "Perfil turístico" }),
    ).toBeInTheDocument();
    await user.click(
      screen.getByRole("button", {
        name: "Tentar carregar mais questionários",
      }),
    );
    expect(
      await screen.findByRole("button", { name: "Experiência do visitante" }),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Perfil turístico" }));
    await user.click(
      await screen.findByRole("button", { name: "Carregar mais versões" }),
    );
    expect(
      await screen.findByText("Não foi possível carregar mais versões."),
    ).toHaveAttribute("role", "alert");
    expect(
      screen.getByRole("button", { name: "Retomar versão 2 — rascunho" }),
    ).toBeInTheDocument();
    await user.click(
      screen.getByRole("button", { name: "Tentar carregar mais versões" }),
    );
    expect(
      await screen.findByRole("button", {
        name: "Retomar versão 3 — rascunho",
      }),
    ).toBeInTheDocument();
  });

  it("encerra a paginação quando o cursor retornado está vazio", async () => {
    const user = userEvent.setup();
    const fetcher = vi.fn<typeof fetch>(async (input) => {
      const url = new URL((input as Request).url);
      if (url.pathname === "/api/v1/questionnaires") {
        return Response.json(
          { ...questionnairePage(), next_cursor: "" },
          { headers: responseHeaders },
        );
      }
      return Response.json(
        { ...versionPage(), next_cursor: "" },
        { headers: responseHeaders },
      );
    });
    vi.stubGlobal("fetch", fetcher);

    renderPage("opaque-editor-token");
    await screen.findByRole("button", { name: "Perfil turístico" });
    expect(
      screen.queryByRole("button", { name: "Carregar mais questionários" }),
    ).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Perfil turístico" }));
    await screen.findByRole("button", {
      name: "Retomar versão 2 — rascunho",
    });
    expect(
      screen.queryByRole("button", { name: "Carregar mais versões" }),
    ).not.toBeInTheDocument();
  });

  it("descarta uma versão atrasada após trocar de questionário", async () => {
    const user = userEvent.setup();
    let resolveFirstVersion: ((response: Response) => void) | undefined;
    const firstVersionResponse = new Promise<Response>((resolve) => {
      resolveFirstVersion = resolve;
    });
    const fetcher = vi.fn<typeof fetch>(async (input) => {
      const url = new URL((input as Request).url);
      if (url.pathname === "/api/v1/questionnaires") {
        return Response.json(
          {
            items: [questionnairePage().items[0], secondQuestionnaire()],
            next_cursor: null,
          },
          { headers: responseHeaders },
        );
      }
      if (url.pathname === `/api/v1/questionnaires/${questionnaireId}/versions`) {
        return Response.json(versionPage(), { headers: responseHeaders });
      }
      if (
        url.pathname ===
        `/api/v1/questionnaires/${secondQuestionnaireId}/versions`
      ) {
        return Response.json(
          { items: [secondVersion()], next_cursor: null },
          { headers: responseHeaders },
        );
      }
      if (url.pathname.endsWith(`/${versionId}`)) {
        return firstVersionResponse;
      }
      return Response.json(
        adminFromSummary(secondVersion(), "Segunda introdução."),
        { headers: { ...responseHeaders, ETag: '"6"' } },
      );
    });
    vi.stubGlobal("fetch", fetcher);

    renderPage("opaque-editor-token");
    await user.click(
      await screen.findByRole("button", { name: "Perfil turístico" }),
    );
    await user.click(
      await screen.findByRole("button", {
        name: "Retomar versão 2 — rascunho",
      }),
    );
    await user.click(
      screen.getByRole("button", { name: "Experiência do visitante" }),
    );
    await user.click(
      await screen.findByRole("button", {
        name: "Retomar versão 3 — rascunho",
      }),
    );

    expect(await screen.findByDisplayValue(secondVersionId)).toBeInTheDocument();
    expect(screen.getByDisplayValue('"6"')).toBeInTheDocument();
    await act(async () => {
      resolveFirstVersion?.(
        Response.json(versionAdmin(), {
          headers: { ...responseHeaders, ETag: '"4"' },
        }),
      );
      await firstVersionResponse;
    });
    expect(screen.getByDisplayValue(secondVersionId)).toBeInTheDocument();
    expect(screen.getByDisplayValue('"6"')).toBeInTheDocument();
    expect(screen.queryByDisplayValue(versionId)).not.toBeInTheDocument();
  });

  it("descarta o erro atrasado de uma versão após trocar de questionário", async () => {
    const user = userEvent.setup();
    let rejectFirstVersion: ((reason?: unknown) => void) | undefined;
    const firstVersionResponse = new Promise<Response>((_, reject) => {
      rejectFirstVersion = reject;
    });
    const fetcher = vi.fn<typeof fetch>(async (input) => {
      const url = new URL((input as Request).url);
      if (url.pathname === "/api/v1/questionnaires") {
        return Response.json(
          {
            items: [questionnairePage().items[0], secondQuestionnaire()],
            next_cursor: null,
          },
          { headers: responseHeaders },
        );
      }
      if (url.pathname === `/api/v1/questionnaires/${questionnaireId}/versions`) {
        return Response.json(versionPage(), { headers: responseHeaders });
      }
      if (
        url.pathname ===
        `/api/v1/questionnaires/${secondQuestionnaireId}/versions`
      ) {
        return Response.json(
          { items: [secondVersion()], next_cursor: null },
          { headers: responseHeaders },
        );
      }
      if (url.pathname.endsWith(`/${versionId}`)) {
        return firstVersionResponse;
      }
      return Response.json(
        adminFromSummary(secondVersion(), "Segunda introdução."),
        { headers: { ...responseHeaders, ETag: '"6"' } },
      );
    });
    vi.stubGlobal("fetch", fetcher);

    renderPage("opaque-editor-token");
    await user.click(
      await screen.findByRole("button", { name: "Perfil turístico" }),
    );
    await user.click(
      await screen.findByRole("button", {
        name: "Retomar versão 2 — rascunho",
      }),
    );
    await user.click(
      screen.getByRole("button", { name: "Experiência do visitante" }),
    );
    await user.click(
      await screen.findByRole("button", {
        name: "Retomar versão 3 — rascunho",
      }),
    );
    expect(
      await screen.findByText("Versão 3 carregada: rascunho."),
    ).toBeInTheDocument();

    await act(async () => {
      rejectFirstVersion?.(new Error("stale failure"));
      await firstVersionResponse.catch(() => undefined);
    });
    expect(screen.getByDisplayValue(secondVersionId)).toBeInTheDocument();
    expect(
      screen.getByText("Versão 3 carregada: rascunho."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Não foi possível concluir a operação."),
    ).not.toBeInTheDocument();
  });

  it("descarta a leitura manual quando o ID muda durante a requisição", async () => {
    const user = userEvent.setup();
    let resolveDelayedLoad: ((response: Response) => void) | undefined;
    const delayedLoad = new Promise<Response>((resolve) => {
      resolveDelayedLoad = resolve;
    });
    let detailCalls = 0;
    const fetcher = vi.fn<typeof fetch>(async (input) => {
      const url = new URL((input as Request).url);
      if (url.pathname === "/api/v1/questionnaires") {
        return Response.json(questionnairePage(), { headers: responseHeaders });
      }
      if (url.pathname.endsWith("/versions")) {
        return Response.json(versionPage(), { headers: responseHeaders });
      }
      detailCalls += 1;
      if (detailCalls === 1) {
        return Response.json(versionAdmin(), {
          headers: { ...responseHeaders, ETag: '"4"' },
        });
      }
      return delayedLoad;
    });
    vi.stubGlobal("fetch", fetcher);

    renderPage("opaque-editor-token");
    await user.click(
      await screen.findByRole("button", { name: "Perfil turístico" }),
    );
    await user.click(
      await screen.findByRole("button", {
        name: "Retomar versão 2 — rascunho",
      }),
    );
    await screen.findByDisplayValue('"4"');
    await user.click(screen.getByRole("button", { name: "Carregar versão" }));
    const idInput = screen.getByLabelText("ID da versão");
    await user.clear(idInput);
    await user.type(idInput, secondVersionId);

    await act(async () => {
      resolveDelayedLoad?.(
        Response.json(
          { ...versionAdmin(), title: "Resposta atrasada" },
          { headers: { ...responseHeaders, ETag: '"5"' } },
        ),
      );
      await delayedLoad;
    });
    expect(idInput).toHaveValue(secondVersionId);
    expect(screen.getByLabelText("ETag")).toHaveValue("");
    expect(screen.getByLabelText("Definição estruturada em JSON")).not.toHaveValue(
      expect.stringContaining("Resposta atrasada"),
    );
  });

  it("descarta o resultado de escrita quando o ID muda durante a requisição", async () => {
    const user = userEvent.setup();
    let resolveSave: ((response: Response) => void) | undefined;
    const saveResponse = new Promise<Response>((resolve) => {
      resolveSave = resolve;
    });
    const fetcher = vi.fn<typeof fetch>(async (input) => {
      const request = input as Request;
      const url = new URL(request.url);
      if (url.pathname === "/api/v1/questionnaires") {
        return Response.json(questionnairePage(), { headers: responseHeaders });
      }
      if (url.pathname.endsWith("/versions")) {
        return Response.json(versionPage(), { headers: responseHeaders });
      }
      if (request.method === "PUT") {
        return saveResponse;
      }
      return Response.json(versionAdmin(), {
        headers: { ...responseHeaders, ETag: '"4"' },
      });
    });
    vi.stubGlobal("fetch", fetcher);

    renderPage("opaque-editor-token");
    await user.click(
      await screen.findByRole("button", { name: "Perfil turístico" }),
    );
    await user.click(
      await screen.findByRole("button", {
        name: "Retomar versão 2 — rascunho",
      }),
    );
    await screen.findByDisplayValue('"4"');
    await user.click(screen.getByRole("button", { name: "Salvar definição" }));
    const idInput = screen.getByLabelText("ID da versão");
    await user.clear(idInput);
    await user.type(idInput, secondVersionId);

    await act(async () => {
      resolveSave?.(
        Response.json(versionAdmin(), {
          headers: { ...responseHeaders, ETag: '"5"' },
        }),
      );
      await saveResponse;
    });
    expect(idInput).toHaveValue(secondVersionId);
    expect(screen.getByLabelText("ETag")).toHaveValue("");
    expect(screen.queryByText("Definição salva.")).not.toBeInTheDocument();
  });

  it("descarta a transição de workflow quando o ID muda durante a requisição", async () => {
    const user = userEvent.setup();
    let resolveApproval: ((response: Response) => void) | undefined;
    const approvalResponse = new Promise<Response>((resolve) => {
      resolveApproval = resolve;
    });
    const fetcher = vi.fn<typeof fetch>(async (input) => {
      const request = input as Request;
      const url = new URL(request.url);
      if (url.pathname === "/api/v1/questionnaires") {
        return Response.json(questionnairePage(), { headers: responseHeaders });
      }
      if (url.pathname.endsWith("/versions")) {
        return Response.json(versionPage(), { headers: responseHeaders });
      }
      if (url.pathname.endsWith("/approve")) {
        return approvalResponse;
      }
      return Response.json(versionAdmin(), {
        headers: { ...responseHeaders, ETag: '"4"' },
      });
    });
    vi.stubGlobal("fetch", fetcher);

    renderPage("opaque-editor-token");
    await user.click(
      await screen.findByRole("button", { name: "Perfil turístico" }),
    );
    await user.click(
      await screen.findByRole("button", {
        name: "Retomar versão 2 — rascunho",
      }),
    );
    await screen.findByDisplayValue('"4"');
    await user.click(screen.getByRole("button", { name: "Aprovar" }));
    const idInput = screen.getByLabelText("ID da versão");
    await user.clear(idInput);
    await user.type(idInput, secondVersionId);

    await act(async () => {
      resolveApproval?.(
        Response.json(
          {
            id: versionId,
            questionnaire_id: questionnaireId,
            version_number: 2,
            revision: 5,
            status: "approved",
          },
          { headers: responseHeaders },
        ),
      );
      await approvalResponse;
    });
    expect(idInput).toHaveValue(secondVersionId);
    expect(screen.getByLabelText("ETag")).toHaveValue("");
    expect(screen.queryByText("Aprovar: concluído.")).not.toBeInTheDocument();
  });

  it("expõe loading e lista vazia ao consultar versões", async () => {
    const user = userEvent.setup();
    let resolveVersions: ((response: Response) => void) | undefined;
    const versionsResponse = new Promise<Response>((resolve) => {
      resolveVersions = resolve;
    });
    const fetcher = vi.fn<typeof fetch>(async (input) => {
      const request = input as Request;
      const url = new URL(request.url);
      if (url.pathname === "/api/v1/questionnaires") {
        return Response.json(questionnairePage(), { headers: responseHeaders });
      }
      return versionsResponse;
    });
    vi.stubGlobal("fetch", fetcher);

    renderPage("opaque-editor-token");
    await user.click(
      await screen.findByRole("button", { name: "Perfil turístico" }),
    );
    expect(screen.getByText("Carregando versões…")).toHaveAttribute(
      "role",
      "status",
    );

    resolveVersions?.(
      Response.json(
        { items: [], next_cursor: null },
        { headers: responseHeaders },
      ),
    );
    expect(
      await screen.findByText("Nenhuma versão encontrada para este questionário."),
    ).toBeInTheDocument();
  });

  it("expõe erro ao consultar versões", async () => {
    const user = userEvent.setup();
    const fetcher = vi.fn<typeof fetch>(async (input) => {
      const request = input as Request;
      const url = new URL(request.url);
      if (url.pathname === "/api/v1/questionnaires") {
        return Response.json(questionnairePage(), { headers: responseHeaders });
      }
      return Response.json(
        {
          type: "urn:cumuru:problem:unavailable",
          title: "Catálogo de versões indisponível.",
          status: 503,
        },
        { status: 503, headers: responseHeaders },
      );
    });
    vi.stubGlobal("fetch", fetcher);

    renderPage("opaque-editor-token");
    await user.click(
      await screen.findByRole("button", { name: "Perfil turístico" }),
    );

    expect(
      await screen.findByText("Catálogo de versões indisponível."),
    ).toHaveAttribute("role", "alert");
  });
});
