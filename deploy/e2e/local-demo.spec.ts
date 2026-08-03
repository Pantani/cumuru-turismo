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

async function onboardAccommodation(
  page: Page,
  accommodations: Locator,
  stays: Locator,
  fixture: AccommodationFixture,
) {
  await accommodations.getByRole("button", {
    name: "Cadastrar outro local",
    exact: true,
  }).click();
  await accommodations.getByLabel("Nome do local", { exact: true }).fill(
    fixture.name,
  );
  await accommodations.getByLabel("Tipo", { exact: true }).selectOption(
    fixture.category,
  );
  await accommodations.getByLabel("Capacidade aproximada", {
    exact: true,
  }).fill(String(fixture.capacity));

  const responsePromise = page.waitForResponse((response) =>
    response.request().method() === "POST" &&
    new URL(response.url()).pathname === "/api/v1/accommodations"
  );
  await accommodations.getByRole("button", {
    name: "Cadastrar local",
    exact: true,
  }).click();
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
  await expect(
    accommodations.getByRole("listitem").filter({ hasText: fixture.name }),
  ).toContainText("Cadastur: Não informado");
  await expect(
    accommodations.getByText("Cadastrar local: concluído.", { exact: true }),
  ).toBeVisible();
  await expect(accommodations.getByLabel("ID da acomodação")).toHaveValue(
    created.id,
  );
  await expect(stays.getByLabel("ID da acomodação")).toHaveValue(created.id);
  return created.id;
}

async function createStayForAccommodation(
  page: Page,
  stays: Locator,
  accommodationId: string,
) {
  await expect(stays.getByLabel("ID da acomodação")).toHaveValue(
    accommodationId,
  );
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
    stays.getByText("Criar estadia: concluído.", { exact: true }),
  ).toBeVisible();
  return created.id;
}

test("percorre a jornada local sem persistir authorities", async ({
  context,
  page,
}) => {
  const consoleErrors: string[] = [];
  const failedAPIResponses: string[] = [];
  page.on("console", (message) => {
    if (message.type() === "error") {
      consoleErrors.push(message.text());
    }
  });
  page.on("response", (response) => {
    if (
      response.url().includes("/api/") &&
      response.status() >= 400
    ) {
      failedAPIResponses.push(
        `${response.status()} ${new URL(response.url()).pathname}`,
      );
    }
  });

  await page.goto("/");
  await expect(
    page.getByRole("heading", {
      level: 1,
      name: "Observatório Turístico de Cumuruxatiba",
    }),
  ).toBeVisible();
  await expect(page.getByText(/Cobertura estimada: \d+%/u).first()).toBeVisible();
  await expect(page.getByText(/não representa um censo/iu)).toBeVisible();
  await expect(
    page.getByText("Não foi possível carregar os indicadores públicos."),
  ).toHaveCount(0);

  await page.goto("/acesso");
  await expect(
    page.getByLabel("Sessão fictícia local"),
  ).toContainText("PROTOTYPE_ONLY");
  await expect(
    page.getByRole("heading", { name: "Operação de estadias" }),
  ).toBeVisible();

  const accommodations = page.getByRole("region", {
    name: "Acomodações e vínculos",
  });
  await accommodations.getByRole("button", {
    name: "Listar acomodações",
  }).click();
  await expect(
    accommodations.getByText(/A primeira foi selecionada\./u),
  ).toBeVisible();

  const stays = page.getByRole("region", { name: "Estadias", exact: true });
  const selectedAccommodationId = await accommodations
    .getByLabel("ID da acomodação")
    .inputValue();
  expect(selectedAccommodationId).not.toBe("");
  await expect(stays.getByLabel("ID da acomodação")).toHaveValue(
    selectedAccommodationId,
  );
  await expect(stays.getByLabel("Chegada prevista")).not.toHaveValue("");
  await expect(stays.getByLabel("Saída prevista")).not.toHaveValue("");

  const familyAccommodationId = await onboardAccommodation(
    page,
    accommodations,
    stays,
    {
      capacity: 7,
      category: "family_hosting",
      name: "Casa Horizonte Fictícia E2E",
    },
  );
  const familyStayId = await createStayForAccommodation(
    page,
    stays,
    familyAccommodationId,
  );

  const formalAccommodationId = await onboardAccommodation(
    page,
    accommodations,
    stays,
    {
      capacity: 12,
      category: "formal_lodging",
      name: "Pousada Mar Azul Fictícia E2E",
    },
  );
  const formalStayId = await createStayForAccommodation(
    page,
    stays,
    formalAccommodationId,
  );
  expect(formalAccommodationId).not.toBe(familyAccommodationId);
  expect(formalStayId).not.toBe(familyStayId);

  const lifecycle = page.getByRole("region", {
    name: "Grupo, convite e ciclo da estadia",
  });
  await expect(
    lifecycle.getByLabel("Versão do aviso de privacidade"),
  ).toHaveValue("prototype-v1");
  await lifecycle.getByRole("button", {
    name: "Criar QR de convite",
  }).click();
  await expect(
    lifecycle.getByText("Criar convite: concluído."),
  ).toBeVisible();
  await lifecycle.getByRole("button", {
    name: "Abrir registro neste navegador",
  }).click();

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
    return {
      body: document.body.textContent ?? "",
      cachedRequests,
      caches: cacheNames,
      cookies: document.cookie,
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
