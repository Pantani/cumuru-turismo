import { expect, test, type Locator, type Page } from "@playwright/test";

// axe's `color-contrast` rule needs real layout and paint to resolve a
// stacking context's composed background, which jsdom never provides — the
// unit suites measured `violations: 0, incomplete: 1, passes: 0` under jsdom
// and disabled the rule there for that reason (see
// shared/theme/status-contrast.test.ts). Chromium here has real layout, so
// this is the one place the rule can actually reprove a regression instead of
// rubber-stamping one.
const AXE_CORE_PATH = require.resolve("axe-core/axe.min.js");

interface AxeViolationNode {
  failureSummary: string | null;
  html: string;
}

interface AxeViolation {
  id: string;
  nodes: AxeViolationNode[];
}

interface AxeResult {
  violations: AxeViolation[];
}

interface AxeRunner {
  run(
    context: Document,
    options: { runOnly: { type: "rule"; values: string[] } },
  ): Promise<AxeResult>;
}

declare global {
  interface Window {
    axe?: AxeRunner;
  }
}

/**
 * Runs axe's `color-contrast` rule against whatever is on screen right now
 * and fails the test with the violated nodes' HTML and failure summary if any
 * text fails WCAG contrast. Scoped to this one rule: the rest of the axe
 * ruleset is exercised in the jsdom-based component suites, and duplicating
 * it here would just slow this journey down without adding coverage.
 */
const AXE_CORE_ROUTE = "/__axe-core__.js";

async function expectNoColorContrastViolations(page: Page, label: string) {
  // The site's CSP is `script-src 'self'` with no 'unsafe-inline' (proven
  // earlier in this same spec), and Playwright's addScriptTag always injects
  // an inline <script> when given a local `path`— the browser blocks that
  // outright. Routing axe-core through a same-origin URL keeps the script
  // request inside 'self' instead of asking the app to relax its policy for
  // a test harness.
  await page.route(AXE_CORE_ROUTE, async (route) => {
    await route.fulfill({
      contentType: "application/javascript",
      path: AXE_CORE_PATH,
    });
  });
  await page.addScriptTag({ url: AXE_CORE_ROUTE });
  await page.unroute(AXE_CORE_ROUTE);
  const violations = await page.evaluate(async () => {
    const axe = window.axe;
    if (axe === undefined) {
      throw new Error("axe-core did not attach to window");
    }
    const result = await axe.run(document, {
      runOnly: { type: "rule", values: ["color-contrast"] },
    });
    return result.violations.map((violation) => ({
      id: violation.id,
      nodes: violation.nodes.map((node) => ({
        html: node.html,
        failureSummary: node.failureSummary,
      })),
    }));
  });
  expect(violations, `color-contrast violations at "${label}"`).toEqual([]);
}

const accommodationRequestFields = [
  "capacity",
  "category",
  "client_submission_id",
  "name",
];

const forbiddenAccommodationFields = [
  "cadastur_id",
  "cnpj",
  "cpf",
  "document",
  "document_hmac",
  "fnrh",
  "fnrh_eligible",
  "fnrh_key",
  "oidc_issuer",
  "oidc_subject",
  "organization_id",
  "tenant_id",
];

interface AccommodationFixture {
  capacity: number;
  category: "family_hosting" | "formal_lodging";
  name: string;
}

const DEMO_ACCOUNT_EMAIL = "operador@cumuru.local";
const DEMO_ACCOUNT_PASSWORD = process.env.LOCAL_DEMO_ACCOUNT_PASSWORD ??
  "demonstracao-local-2026";

/** The session lives in tab memory only, so every run signs in from scratch. */
async function signIn(page: Page) {
  await page.getByLabel("E-mail", { exact: true }).fill(DEMO_ACCOUNT_EMAIL);
  await page.getByLabel("Senha", { exact: true }).fill(DEMO_ACCOUNT_PASSWORD);
  await page.getByRole("button", { name: "Entrar", exact: true }).click();
  await expect(
    page.getByRole("region", { name: "Suas hospedagens" }),
  ).toBeVisible();
}

/**
 * A sessão vive só na memória da aba, então uma navegação completa pode ou não
 * preservá-la. Voltar à área do operador precisa funcionar nos dois casos.
 */
async function ensureWorkspace(page: Page) {
  await page.goto("/acesso");
  const accommodations = page.getByRole("region", { name: "Suas hospedagens" });
  if (await accommodations.isVisible().catch(() => false)) {
    return accommodations;
  }
  await signIn(page);
  return accommodations;
}

/** The board is titled after the selected accommodation, never after an id. */
function stayBoardFor(page: Page, accommodationName: string) {
  return page.getByRole("region", {
    name: `Estadias de ${accommodationName}`,
  });
}

async function onboardAccommodation(
  page: Page,
  accommodations: Locator,
  fixture: AccommodationFixture,
) {
  await accommodations.getByRole("button", {
    name: "Cadastrar outra hospedagem",
    exact: true,
  }).click();
  // The labels wrap their controls, so the accessible name carries the current
  // value as well: match by prefix instead of exact text.
  await page.getByLabel(/^Como o local é conhecido/u).fill(fixture.name);
  await page.getByLabel(/^Tipo de hospedagem/u).selectOption(fixture.category);
  await page.getByLabel(/^Quantas pessoas cabem/u).fill(
    String(fixture.capacity),
  );

  const responsePromise = page.waitForResponse((response) =>
    response.request().method() === "POST" &&
    new URL(response.url()).pathname === "/api/v1/accommodations"
  );
  await page.getByRole("button", { name: "Cadastrar", exact: true }).click();
  const response = await responsePromise;
  expect(response.status()).toBe(201);

  const requestBody = response.request().postDataJSON() as Record<
    string,
    unknown
  >;
  expect(Object.keys(requestBody).sort()).toEqual(accommodationRequestFields);
  expect(requestBody).toMatchObject(fixture);
  expect(requestBody.client_submission_id).toEqual(
    expect.stringMatching(
      /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/iu,
    ),
  );
  for (const field of forbiddenAccommodationFields) {
    expect(requestBody).not.toHaveProperty(field);
  }

  const created = await response.json() as {
    cadastur_id?: string | null;
    category: string;
    id: string;
    name: string;
    status: string;
  };
  expect(created).toMatchObject({
    category: fixture.category,
    name: fixture.name,
    status: "active",
  });
  expect(created.cadastur_id ?? null).toBeNull();
  // The freshly created accommodation becomes the selection, and the board
  // header is the only place its identity is surfaced.
  await expect(stayBoardFor(page, fixture.name)).toBeVisible();
  return created.id;
}

async function createStayForAccommodation(
  page: Page,
  accommodationName: string,
  accommodationId: string,
) {
  const stays = stayBoardFor(page, accommodationName);
  await expect(stays.getByRole("button", { name: "Criar estadia" }))
    .toBeEnabled();
  const responsePromise = page.waitForResponse((response) =>
    response.request().method() === "POST" &&
    new URL(response.url()).pathname === "/api/v1/stays"
  );
  await stays.getByRole("button", { name: "Criar estadia" }).click();
  const response = await responsePromise;
  expect(response.status()).toBe(201);
  expect(response.request().postDataJSON()).toMatchObject({
    accommodation_id: accommodationId,
  });
  const created = await response.json() as { id: string };
  await expect(
    stays.getByText("Criando a estadia: pronto.", { exact: true }),
  ).toBeVisible();
  return created.id;
}

test("percorre a jornada local sem persistir authorities", async ({
  context,
  page,
}) => {
  const consoleErrors: string[] = [];
  const failedAPIResponses: string[] = [];
  // A política que o NAVEGADOR recebeu, não a que o nginx pretende enviar. O
  // gate integrado já afirma o cabeçalho servido; aqui o que se prova é a
  // entrega ponta a ponta, com o documento carregado pelo próprio Chromium.
  const documentPolicies: string[] = [];
  page.on("response", (response) => {
    if (response.request().resourceType() !== "document") {
      return;
    }
    const policy = response.headers()["content-security-policy"];
    if (policy !== undefined) {
      documentPolicies.push(policy);
    }
  });
  // Exceção nomeada, deliberadamente estreita. O painel do cartaz consulta
  // GET /accommodations/{id}/invite assim que monta, e "não há cartaz ativo" é
  // 404 declarado no contrato, não falha. Tolerar 404 em geral cegaria o gate
  // inteiro; aqui só este caminho, só com este status, e só antes de o cartaz
  // existir. Depois de emitido, um 404 nesta mesma rota volta a ser falha.
  const posterAbsenceRoute = /^\/api\/v1\/accommodations\/[0-9a-fA-F-]+\/invite$/u;
  const declaredPosterAbsences: string[] = [];
  const declaredPosterConsoleErrors: string[] = [];
  let posterIssued = false;

  // O navegador registra o mesmo 404 duas vezes: uma na rede e uma no console.
  // Aceitá-lo na rede e proibi-lo no console faria o spec se contradizer sobre
  // o mesmo fato. A exceção é a mesma, com os mesmos três limites — aquela
  // rota, aquele status, e só antes da emissão —, e o casamento é pela URL de
  // origem da mensagem, não pelo texto, que é do navegador e não do contrato.
  page.on("console", (message) => {
    if (message.type() !== "error") {
      return;
    }
    const location = message.location().url;
    const pathname = location === "" ? "" : new URL(location).pathname;
    if (
      !posterIssued &&
      posterAbsenceRoute.test(pathname) &&
      message.text().includes("404")
    ) {
      declaredPosterConsoleErrors.push(`${pathname} ${message.text()}`);
      return;
    }
    consoleErrors.push(message.text());
  });
  page.on("response", (response) => {
    if (!response.url().includes("/api/") || response.status() < 400) {
      return;
    }
    const pathname = new URL(response.url()).pathname;
    const entry = `${response.status()} ${pathname}`;
    if (
      !posterIssued &&
      response.status() === 404 &&
      posterAbsenceRoute.test(pathname)
    ) {
      declaredPosterAbsences.push(entry);
      return;
    }
    failedAPIResponses.push(entry);
  });

  await page.goto("/");
  await expect(
    page.getByRole("heading", {
      level: 1,
      name: "O turismo da nossa praia, finalmente em números.",
    }),
  ).toBeVisible();
  await expect(page.getByText(/Cobertura estimada: \d+%/u).first()).toBeVisible();
  await expect(page.getByText(/não representa um censo/iu)).toBeVisible();
  await expect(
    page.getByText("Não foi possível carregar os indicadores públicos."),
  ).toHaveCount(0);
  // Not checked here: the landing page has a pre-existing, unrelated
  // color-contrast defect that axe caught the moment this rule was turned
  // on. The `sand`/`clay` light sections (`.lp-accent`, `.lp-index-number`,
  // `.lp-step-number` in landing.css) were fixed by reusing the `--lp-ink`
  // token the section's own heading already uses — see
  // shared/theme/landing-accent-contrast.test.ts. The `coral` section has
  // its own separate, still-open instance of the same class of defect
  // (`.lp-section-coral .lp-index-number` resolves to `--ink` on `--coral`,
  // ~2.6:1) that CommerceSection renders; it predates this change, is
  // outside Fase 7 scope, and fixing it needs the same design sign-off.

  await page.goto("/acesso");
  await signIn(page);

  const accommodations = page.getByRole("region", {
    name: "Suas hospedagens",
  });
  // The seeded accommodation is preselected, so the board is already titled
  // after it before anything is created.
  await expect(
    accommodations.getByRole("button", { pressed: true }),
  ).toHaveCount(1);
  await expect(
    page.getByRole("region", { name: /^Estadias de / }),
  ).toBeVisible();
  // Not checked here: this operator workspace screen predates Fase 7 and has
  // its own pre-existing, unrelated color-contrast defect (`.property-capacity`
  // muted text at ~2.67:1 against the panel background in styles.css) that axe
  // caught the moment this rule was turned on. Flagged separately; the four
  // Fase 7 screens below are what this debt item asked for.

  const familyName = "Casa Horizonte Fictícia E2E";
  const formalName = "Pousada Mar Azul Fictícia E2E";

  const familyAccommodationId = await onboardAccommodation(
    page,
    accommodations,
    {
      capacity: 7,
      category: "family_hosting",
      name: familyName,
    },
  );
  const familyStayId = await createStayForAccommodation(
    page,
    familyName,
    familyAccommodationId,
  );

  const formalAccommodationId = await onboardAccommodation(
    page,
    accommodations,
    {
      capacity: 12,
      category: "formal_lodging",
      name: formalName,
    },
  );
  const formalStayId = await createStayForAccommodation(
    page,
    formalName,
    formalAccommodationId,
  );
  expect(formalAccommodationId).not.toBe(familyAccommodationId);
  expect(formalStayId).not.toBe(familyStayId);

  // The card offers only the transitions the server accepts for the current
  // state; a draft stay can be invited, never checked out.
  const board = stayBoardFor(page, formalName);
  const draftCard = board.getByRole("listitem").first();
  await expect(draftCard.getByRole("button", { name: "Gerar convite" }))
    .toBeVisible();
  await expect(draftCard.getByRole("button", { name: "Fazer check-out" }))
    .toHaveCount(0);
  await draftCard.getByRole("button", { name: "Gerar convite" }).click();
  // The board refreshes right after the invite, so the transient status line is
  // replaced. The QR panel and the new stay state are the durable outcomes.
  await expect(
    page.getByRole("group", { name: "Convite pronto para leitura local" }),
  ).toBeVisible();
  await expect(board.getByText("Convite enviado")).toBeVisible();
  await page.getByRole("button", {
    name: "Abrir registro neste navegador",
  }).click();
  // Scrubbing the token rewrites the address; the invite nav entry is what
  // carries the operator into the registration view.
  await page.getByRole("link", { name: "Registro", exact: true }).click();

  await expect(page).toHaveURL(/\/registro$/u);
  expect(new URL(page.url()).search).toBe("");
  await expect(
    page.getByRole("heading", { name: "Confirme o grupo da estadia" }),
  ).toBeVisible();
  await page.getByLabel("UF de residência do visitante 1").fill("BA");
  await page.getByLabel("Município IBGE do visitante 1").fill("2925509");
  await page.getByRole("button", { name: "Enviar grupo" }).click();
  await expect(
    page.getByRole("heading", { name: "Registro concluído" }),
  ).toBeVisible();
  await page.getByRole("button", {
    name: "Responder pesquisa voluntária",
  }).click();

  await expect(page).toHaveURL(/\/pesquisa$/u);
  await expect(
    page.getByRole("heading", {
      level: 2,
      name: "Pesquisa turística de demonstração",
    }),
  ).toBeVisible();
  await page.getByRole("checkbox", {
    name: /Aceito o uso agregado/iu,
  }).check();
  await page.getByRole("combobox", {
    name: "Esta é sua primeira visita a Cumuruxatiba?",
  }).selectOption("first_visit");
  await page.getByRole("button", { name: "Enviar respostas" }).click();
  await expect(
    page.getByRole("heading", { name: "Participação registrada" }),
  ).toBeVisible();

  // --- autoatendimento: cartaz, autocadastro pelo link aberto e aprovação -----------
  //
  // Fecha N-45: o QR é gerado no navegador, sob a CSP existente, sem nenhuma
  // requisição a host externo, e a prova de trabalho é resolvida pelo módulo
  // real do cliente — não por uma reimplementação.
  // A jornada terminou na pesquisa; a área do operador precisa ser reaberta e a
  // hospedagem reselecionada antes de o painel do cartaz existir.
  const workspace = await ensureWorkspace(page);
  await workspace.getByRole("button", { name: formalName }).click();

  const posterPanel = page.getByRole("region", {
    name: "Cartaz de autocadastro",
  });
  await expect(posterPanel).toBeVisible();

  const posterResponsePromise = page.waitForResponse((response) =>
    response.request().method() === "POST" &&
    /\/api\/v1\/accommodations\/[0-9a-fA-F-]+\/invite$/u.test(
      new URL(response.url()).pathname,
    )
  );
  await posterPanel.getByRole("button", { name: "Emitir cartaz" }).click();
  const posterResponse = await posterResponsePromise;
  expect(posterResponse.status()).toBe(201);
  posterIssued = true;

  const poster = await posterResponse.json() as { url: string };
  const posterURL = new URL(poster.url);
  // O token vive no fragmento e nunca no caminho: é isso que o mantém fora de
  // linha de requisição, access log, WAF e CDN (ADR-039).
  expect(posterURL.pathname).toBe("/i");
  expect(posterURL.search).toBe("");
  const posterToken = posterURL.hash.replace(/^#/u, "");
  expect(posterToken.length).toBeGreaterThanOrEqual(64);

  // O QR é desenhado no próprio navegador; nenhuma imagem vem de fora.
  await expect(
    page.getByRole("group", { name: "Cartaz pronto para impressão" }),
  ).toBeVisible();
  await expectNoColorContrastViolations(page, "poster panel");

  await page.goto(`/i#${posterToken}`);
  await expect(
    page.getByRole("heading", { name: "Aviso de privacidade" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Confirme os dados da estadia" }),
  ).toBeVisible();
  // /i is the one Fase 7 screen a guest — not an operator — ever sees, so its
  // contrast matters more than any of the operator-only ones.
  await expectNoColorContrastViolations(page, "self-service form (/i)");

  await page.getByLabel("Data de chegada").fill("2026-11-03");
  await page.getByLabel("Data de saída").fill("2026-11-07");
  await page.getByLabel("UF de residência do visitante 1").fill("BA");
  await page.getByLabel("Município IBGE do visitante 1").fill("2925509");

  const submitResponsePromise = page.waitForResponse((response) =>
    response.request().method() === "POST" &&
    new URL(response.url()).pathname === "/api/v1/accommodation-invite/submit"
  );
  await page.getByRole("button", { name: "Enviar autocadastro" }).click();
  const submitResponse = await submitResponsePromise;
  expect(submitResponse.status()).toBe(200);
  expect(await submitResponse.json()).toMatchObject({
    approval_state: "pending",
    stay_status: "pre_registered",
    status: "accepted",
  });
  // O corpo enviado carrega a solução da prova de trabalho, resolvida pelo
  // módulo do cliente sob WebCrypto, e nenhum campo de identidade.
  const submitBody = submitResponse.request().postDataJSON() as Record<
    string,
    unknown
  >;
  expect(submitBody).toHaveProperty("proof_of_work");
  for (const forbidden of ["contact", "document_number", "email", "full_name"]) {
    expect(JSON.stringify(submitBody)).not.toContain(forbidden);
  }
  await expect(
    page.getByRole("heading", { name: "Autocadastro enviado" }),
  ).toBeVisible();
  await expectNoColorContrastViolations(page, "self-service submission receipt");

  // A sessão vive só na memória da aba, então voltar exige entrar de novo.
  const workspaceAgain = await ensureWorkspace(page);
  await workspaceAgain.getByRole("button", { name: formalName }).click();

  const queue = page.getByRole("region", { name: "Aguardando aprovação" });
  await expect(queue).toBeVisible();
  const pending = queue.getByRole("listitem").first();
  await expect(pending).toBeVisible();
  await expectNoColorContrastViolations(page, "approval queue");

  const approveResponsePromise = page.waitForResponse((response) =>
    response.request().method() === "POST" &&
    new URL(response.url()).pathname.endsWith("/approve")
  );
  await pending.getByRole("button", { name: "Aprovar" }).click();
  const approveResponse = await approveResponsePromise;
  expect(approveResponse.status()).toBe(200);
  await expect(
    page.getByRole("group", { name: "Autocadastro aprovado" }),
  ).toBeVisible();
  await expectNoColorContrastViolations(page, "approval confirmation");

  await page.evaluate(async () => {
    if ("serviceWorker" in navigator) {
      await navigator.serviceWorker.ready;
    }
  });
  const storage = await page.evaluate(async () => {
    const cacheNames = "caches" in window
      ? await window.caches.keys()
      : [];
    const cachedRequests = (
      await Promise.all(
        cacheNames.map(async (name) => {
          const cache = await window.caches.open(name);
          return (await cache.keys()).map((request) => request.url);
        }),
      )
    ).flat();
    const serviceWorkers = "serviceWorker" in navigator
      ? await navigator.serviceWorker.getRegistrations()
      : [];
    const databaseNames = (await indexedDB.databases())
      .map((database) => database.name)
      .filter((name): name is string => name !== undefined)
      .sort();
    const indexedDatabases = await Promise.all(
      databaseNames.map(async (name) => {
        const database = await new Promise<IDBDatabase>((resolve, reject) => {
          const request = indexedDB.open(name);
          request.addEventListener("success", () => resolve(request.result));
          request.addEventListener("error", () => reject(request.error));
        });
        const storeNames = Array.from(database.objectStoreNames).sort();
        const stores = await Promise.all(
          storeNames.map(async (storeName) => {
            const transaction = database.transaction(storeName, "readonly");
            const count = await new Promise<number>((resolve, reject) => {
              const request = transaction.objectStore(storeName).count();
              request.addEventListener("success", () => resolve(request.result));
              request.addEventListener("error", () => reject(request.error));
            });
            return { count, name: storeName };
          }),
        );
        database.close();
        return { name, stores };
      }),
    );
    return {
      body: document.body.textContent ?? "",
      cachedRequests,
      caches: cacheNames,
      cookies: document.cookie,
      indexedDatabases,
      local: Object.values(localStorage),
      serviceWorkerCount: serviceWorkers.length,
      session: Object.values(sessionStorage),
      url: window.location.href,
    };
  });
  const serialized = JSON.stringify(storage);
  expect(serialized).not.toContain("cumuru-local-platform-read");
  expect(storage.url).not.toContain("/convites/");
  expect(storage.local).toEqual([]);
  expect(storage.session).toEqual([]);
  expect(storage.cookies).toBe("");
  expect(storage.indexedDatabases).toEqual([
    {
      name: "cumuru-encrypted-drafts",
      stores: [
        { count: 0, name: "drafts" },
        { count: 0, name: "keys" },
      ],
    },
  ]);
  expect(storage.serviceWorkerCount).toBe(1);
  expect(storage.caches).toContain("cumuru-shell-v1");
  expect(storage.caches.every((name) => name === "cumuru-shell-v1")).toBe(true);
  expect(
    storage.cachedRequests.every((url) => {
      const parsed = new URL(url);
      return (
        !parsed.pathname.startsWith("/api/") &&
        !parsed.pathname.startsWith("/convites/") &&
        !parsed.searchParams.has("convite") &&
        !parsed.searchParams.has("invite") &&
        !parsed.searchParams.has("invite_token") &&
        !parsed.searchParams.has("token")
      );
    }),
  ).toBe(true);
  expect(await context.cookies()).toEqual([]);
  expect(consoleErrors).toEqual([]);
  expect(failedAPIResponses).toEqual([]);
  // A exceção só se justifica enquanto o estado que ela descreve acontecer. Se
  // nunca ocorrer, ela virou tolerância morta e precisa sair daqui — por isso a
  // asserção exige que tenha ocorrido, e que cada ocorrência seja exatamente o
  // 404 do painel do cartaz antes da emissão.
  expect(declaredPosterAbsences.length).toBeGreaterThan(0);
  for (const absence of declaredPosterAbsences) {
    expect(absence).toMatch(
      /^404 \/api\/v1\/accommodations\/[0-9a-fA-F-]+\/invite$/u,
    );
  }
  // A tolerância do console também não pode sobreviver à sua causa: se nunca
  // ocorrer, ela virou cegueira e sai daqui. E cada ocorrência precisa vir da
  // rota do cartaz, não de qualquer 404 que o navegador resolva registrar.
  // N-20: sem esta política o formulário público poderia abrir conexão para
  // qualquer host, e o comentário "sem requisição externa" viraria afirmação sem
  // lastro. Nenhum documento pode chegar ao navegador sem ela.
  expect(documentPolicies.length).toBeGreaterThan(0);
  for (const policy of documentPolicies) {
    expect(policy).toContain("default-src 'self'");
    expect(policy).toContain("connect-src 'self'");
    expect(policy).toContain("script-src 'self'");
    // data: é o que permite o QR desenhado no navegador; blob: seria bloqueado.
    expect(policy).toContain("img-src 'self' data:");
    expect(policy).toContain("frame-ancestors 'none'");
    // Nenhuma origem externa em diretiva de busca: o toContain acima passaria se
    // alguém apenas acrescentasse um host ao lado de 'self'.
    for (const directive of ["connect-src", "script-src", "img-src"]) {
      const value = policy
        .split(";")
        .map((part) => part.trim())
        .find((part) => part.startsWith(`${directive} `));
      expect(value, `${directive} is missing from the policy`).toBeDefined();
      const sources = (value ?? "").slice(directive.length + 1).split(/\s+/u);
      expect(
        sources.filter((source) =>
          !["'self'", "'none'", "data:"].includes(source)
        ),
        `${directive} admits an external source`,
      ).toEqual([]);
    }
  }
  expect(declaredPosterConsoleErrors.length).toBeGreaterThan(0);
  for (const logged of declaredPosterConsoleErrors) {
    expect(logged).toMatch(
      /^\/api\/v1\/accommodations\/[0-9a-fA-F-]+\/invite /u,
    );
  }
});
