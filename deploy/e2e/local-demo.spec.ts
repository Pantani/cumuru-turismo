import { expect, test, type Locator, type Page } from "@playwright/test";

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
  // A Fase 7 responde 404 uniforme quando a capability não existe, para não
  // revelar se ela existe (ADR-039). O painel consulta o convite de cada
  // acomodação a cada carga, então "sem convite ativo" é um 404 esperado e o
  // navegador o registra no console. Só esse par rota+status é tolerado; o
  // resto continua exigindo zero, inclusive outro 404 em outra rota.
  const ABSENT_CAPABILITY_ROUTE =
    /^\/api\/v1\/accommodations\/[0-9a-f-]+\/invite$/;
  // location().url vem vazio em erros de console sem recurso associado.
  const pathnameOf = (url: string) => {
    try {
      return new URL(url).pathname;
    } catch {
      return "";
    }
  };
  const isAbsentCapabilityRoute = (url: string) =>
    ABSENT_CAPABILITY_ROUTE.test(pathnameOf(url));
  const consoleErrors: string[] = [];
  const failedAPIResponses: string[] = [];
  page.on("response", (response) => {
    if (!response.url().includes("/api/") || response.status() < 400) {
      return;
    }
    if (response.status() === 404 && isAbsentCapabilityRoute(response.url())) {
      return;
    }
    failedAPIResponses.push(
      `${response.status()} ${new URL(response.url()).pathname}`,
    );
  });
  page.on("console", (message) => {
    if (message.type() !== "error") {
      return;
    }
    // location().url é a URL do recurso que falhou, então o descarte é por
    // pedido e não por categoria: um erro em qualquer outra rota, inclusive
    // num asset, continua reprovando. Um status diferente de 404 nessa mesma
    // rota ainda reprova pelo failedAPIResponses acima.
    if (isAbsentCapabilityRoute(message.location().url)) {
      return;
    }
    consoleErrors.push(message.text());
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
});
