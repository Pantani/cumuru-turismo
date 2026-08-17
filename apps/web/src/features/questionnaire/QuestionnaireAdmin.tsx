import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import {
  type FormEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import type { components } from "../../generated/schema";
import { ApiError } from "../../shared/api/http-client";
import { type QuestionnaireClient } from "../../shared/api/questionnaire-client";
import { createUuidV7 } from "../../shared/identity/uuid-v7";

type Schemas = components["schemas"];
type Definition = Schemas["UpdateQuestionnaireVersionRequest"];

function uniquePageItems<T extends { id: string }>(
  pages: ReadonlyArray<{ data: { items: T[] } }> | undefined,
): T[] {
  const items = new Map<string, T>();
  for (const page of pages ?? []) {
    for (const item of page.data.items) {
      items.set(item.id, item);
    }
  }
  return [...items.values()];
}

function nextCursor(page: { data: { next_cursor?: string | null } }) {
  return page.data.next_cursor || undefined;
}

interface PaginationControlsProps {
  errorLabel: string;
  failed: boolean;
  fetchNext: () => Promise<unknown>;
  hasNextPage: boolean;
  isFetching: boolean;
  loadLabel: string;
  loadingLabel: string;
  retryLabel: string;
}

function PaginationControls({
  errorLabel,
  failed,
  fetchNext,
  hasNextPage,
  isFetching,
  loadLabel,
  loadingLabel,
  retryLabel,
}: PaginationControlsProps) {
  const buttonLabel = isFetching
    ? loadingLabel
    : failed
      ? retryLabel
      : loadLabel;
  return (
    <>
      {failed ? <p role="alert">{errorLabel}</p> : null}
      {hasNextPage ? (
        <button
          type="button"
          disabled={isFetching}
          onClick={() => void fetchNext()}
        >
          {buttonLabel}
        </button>
      ) : null}
    </>
  );
}

function errorMessage(error: unknown) {
  if (error instanceof SyntaxError) {
    return "A definição JSON não é válida.";
  }
  if (error instanceof ApiError) {
    return error.problem.title;
  }
  return "Não foi possível concluir a operação.";
}

function starterDefinition(): Definition {
  const purpose = "tourism_planning";
  return {
    title: "Pesquisa turística",
    introduction: "Participação voluntária para apoiar o planejamento turístico.",
    privacy_notice_version: "survey-v1",
    consent_requirements: [
      {
        purpose_code: purpose,
        notice_version: "survey-v1",
        prompt: "Aceito responder para fins de planejamento turístico.",
        required_for_answers: true,
        display_order: 1,
      },
    ],
    questions: [
      {
        id: createUuidV7(),
        stable_key: "first_visit",
        prompt: "Esta é sua primeira visita a Cumuruxatiba?",
        answer_type: "boolean",
        required: false,
        data_classification: "personal",
        purpose_code: purpose,
        retention_policy_code: "survey_prototype_v1",
        analytics_key: "first_visit",
        public_aggregation_allowed: true,
        minimum_public_cell: 10,
        display_order: 1,
        options: [],
      },
    ],
  };
}

function definitionFrom(version: Schemas["QuestionnaireVersionAdmin"]): Definition {
  return {
    title: version.title,
    introduction: version.introduction,
    privacy_notice_version: version.privacy_notice_version,
    questions: version.questions,
    consent_requirements: version.consent_requirements,
  };
}

interface CreateFormProps {
  client: QuestionnaireClient;
  disabled: boolean;
  onCreated: (id: string, etag: string) => void;
  report: (message: string) => void;
}

function CreateForm({ client, disabled, onCreated, report }: CreateFormProps) {
  const [stableKey, setStableKey] = useState("tourism_profile");
  const [name, setName] = useState("Perfil turístico");
  const [title, setTitle] = useState("Pesquisa turística");
  const [notice, setNotice] = useState("survey-v1");

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    report("Criando questionário…");
    try {
      const result = await client.createQuestionnaire(
        {
          id: createUuidV7(),
          version_id: createUuidV7(),
          stable_key: stableKey,
          name,
          title,
          privacy_notice_version: notice,
        },
        crypto.randomUUID(),
      );
      onCreated(result.data.id, result.etag ?? "");
      report("Questionário criado em rascunho.");
    } catch (error) {
      report(errorMessage(error));
    }
  }

  return (
    <form onSubmit={(event) => void create(event)}>
      <h3>Novo questionário</h3>
      <div className="field-grid">
        <label>
          Chave estável
          <input
            value={stableKey}
            pattern="[a-z][a-z0-9_]{2,63}"
            required
            onChange={(event) => setStableKey(event.target.value)}
          />
        </label>
        <label>
          Nome administrativo
          <input
            value={name}
            maxLength={160}
            required
            onChange={(event) => setName(event.target.value)}
          />
        </label>
        <label>
          Título público
          <input
            value={title}
            maxLength={200}
            required
            onChange={(event) => setTitle(event.target.value)}
          />
        </label>
        <label>
          Versão do aviso
          <input
            value={notice}
            maxLength={100}
            required
            onChange={(event) => setNotice(event.target.value)}
          />
        </label>
      </div>
      <button type="submit" disabled={disabled}>
        Criar rascunho
      </button>
    </form>
  );
}

interface CatalogProps {
  client: QuestionnaireClient;
  selectedId: string;
  onSelect: (id: string) => void;
}

function Catalog({ client, onSelect, selectedId }: CatalogProps) {
  const catalog = useInfiniteQuery({
    queryKey: ["questionnaires"],
    initialPageParam: "",
    queryFn: ({ pageParam }) =>
      client.listQuestionnaires(pageParam || undefined, 100),
    getNextPageParam: nextCursor,
  });
  if (catalog.isPending) {
    return <p role="status">Carregando catálogo…</p>;
  }
  if (catalog.isError && !catalog.isFetchNextPageError) {
    return <p role="alert">{errorMessage(catalog.error)}</p>;
  }
  const items = uniquePageItems(catalog.data.pages);
  return (
    <section aria-labelledby="questionnaire-catalog-title">
      <h3 id="questionnaire-catalog-title">Catálogo global</h3>
      {items.length === 0 ? (
        <p>Nenhum questionário criado.</p>
      ) : (
        <ul className="catalog-list">
          {items.map((item) => (
            <li key={item.id}>
              <button
                type="button"
                aria-pressed={selectedId === item.id}
                onClick={() => onSelect(item.id)}
              >
                {item.name}
              </button>
              <span>{item.stable_key}</span>
            </li>
          ))}
        </ul>
      )}
      <PaginationControls
        errorLabel="Não foi possível carregar mais questionários."
        failed={catalog.isFetchNextPageError}
        fetchNext={catalog.fetchNextPage}
        hasNextPage={catalog.hasNextPage}
        isFetching={catalog.isFetchingNextPage}
        loadLabel="Carregar mais questionários"
        loadingLabel="Carregando mais questionários…"
        retryLabel="Tentar carregar mais questionários"
      />
    </section>
  );
}

type VersionResult = Awaited<
  ReturnType<QuestionnaireClient["getQuestionnaireVersion"]>
>;

const versionStatusLabels: Record<
  Schemas["QuestionnaireVersionStatus"],
  string
> = {
  draft: "rascunho",
  privacy_review: "em revisão de privacidade",
  approved: "aprovada",
  published: "publicada",
  retired: "retirada",
};

interface VersionCatalogProps {
  client: QuestionnaireClient;
  questionnaireId: string;
  selectedVersionId: string;
  onSelect: (
    version: Schemas["QuestionnaireVersionAdmin"],
    etag: string,
    questionnaireId: string,
  ) => boolean;
  report: (message: string) => void;
}

function VersionCatalog({
  client,
  questionnaireId,
  selectedVersionId,
  onSelect,
  report,
}: VersionCatalogProps) {
  const [loadingVersionId, setLoadingVersionId] = useState("");
  const loadGeneration = useRef(0);
  useEffect(
    () => () => {
      loadGeneration.current += 1;
    },
    [],
  );
  const versions = useInfiniteQuery({
    queryKey: ["questionnaires", questionnaireId, "versions"],
    initialPageParam: "",
    queryFn: ({ pageParam }) =>
      client.listQuestionnaireVersions(
        questionnaireId,
        pageParam || undefined,
        100,
      ),
    getNextPageParam: nextCursor,
  });

  function applyResumed(result: VersionResult, generation: number) {
    if (generation !== loadGeneration.current) {
      return;
    }
    if (!onSelect(result.data, result.etag ?? "", questionnaireId)) {
      return;
    }
    report(
      `Versão ${result.data.version_number} carregada: ${versionStatusLabels[result.data.status]}.`,
    );
  }

  async function resume(versionId: string) {
    const generation = ++loadGeneration.current;
    setLoadingVersionId(versionId);
    report("Carregando versão…");
    try {
      applyResumed(await client.getQuestionnaireVersion(versionId), generation);
    } catch (error) {
      if (generation === loadGeneration.current) {
        report(errorMessage(error));
      }
    } finally {
      if (generation === loadGeneration.current) {
        setLoadingVersionId("");
      }
    }
  }

  if (versions.isPending) {
    return <p role="status">Carregando versões…</p>;
  }
  if (versions.isError && !versions.isFetchNextPageError) {
    return <p role="alert">{errorMessage(versions.error)}</p>;
  }
  const items = uniquePageItems(versions.data.pages);

  return (
    <section aria-labelledby="questionnaire-versions-title">
      <h3 id="questionnaire-versions-title">Versões existentes</h3>
      {items.length === 0 ? (
        <p>Nenhuma versão encontrada para este questionário.</p>
      ) : (
        <ul className="catalog-list">
          {items.map((version) => (
            <li key={version.id}>
              <button
                type="button"
                aria-pressed={selectedVersionId === version.id}
                disabled={loadingVersionId === version.id}
                onClick={() => void resume(version.id)}
              >
                Retomar versão {version.version_number} —{" "}
                {versionStatusLabels[version.status]}
              </button>
              <span>Revisão {version.revision}</span>
            </li>
          ))}
        </ul>
      )}
      <PaginationControls
        errorLabel="Não foi possível carregar mais versões."
        failed={versions.isFetchNextPageError}
        fetchNext={versions.fetchNextPage}
        hasNextPage={versions.hasNextPage}
        isFetching={versions.isFetchingNextPage}
        loadLabel="Carregar mais versões"
        loadingLabel="Carregando mais versões…"
        retryLabel="Tentar carregar mais versões"
      />
    </section>
  );
}

interface VersionEditorProps {
  client: QuestionnaireClient;
  initialEtag: string;
  initialVersion: Schemas["QuestionnaireVersionAdmin"] | null;
  initialVersionId: string;
  report: (message: string) => void;
}

function VersionEditor({
  client,
  initialEtag,
  initialVersion,
  initialVersionId,
  report,
}: VersionEditorProps) {
  const [versionId, setVersionId] = useState(initialVersionId);
  const versionIdRef = useRef(initialVersionId);
  const [etag, setEtag] = useState(initialEtag);
  const [definition, setDefinition] = useState(() =>
    JSON.stringify(
      initialVersion === null ? starterDefinition() : definitionFrom(initialVersion),
      null,
      2,
    ),
  );
  const [reason, setReason] =
    useState<Schemas["RequestChangesRequest"]["reason_code"]>(
      "privacy_metadata_incomplete",
    );
  const actionKey = useRef(crypto.randomUUID());
  const operationGeneration = useRef(0);
  useEffect(
    () => () => {
      operationGeneration.current += 1;
    },
    [],
  );

  function operationIsCurrent(generation: number, expectedVersionId: string) {
    return (
      generation === operationGeneration.current &&
      expectedVersionId === versionIdRef.current
    );
  }

  function changeVersionId(nextVersionId: string) {
    operationGeneration.current += 1;
    versionIdRef.current = nextVersionId;
    setVersionId(nextVersionId);
    setEtag("");
  }

  function changeEtag(nextEtag: string) {
    operationGeneration.current += 1;
    setEtag(nextEtag);
  }

  const applyLoaded = useCallback(
    (result: VersionResult, generation: number, expectedVersionId: string) => {
      const stale =
        !operationIsCurrent(generation, expectedVersionId) ||
        result.data.id !== expectedVersionId;
      if (stale) {
        return;
      }
      setDefinition(JSON.stringify(definitionFrom(result.data), null, 2));
      setEtag(result.etag ?? "");
      report(
        `Versão ${result.data.version_number} carregada: ${result.data.status}.`,
      );
    },
    [report],
  );

  const load = useCallback(async () => {
    const expectedVersionId = versionIdRef.current;
    const generation = ++operationGeneration.current;
    report("Carregando versão…");
    try {
      const result = await client.getQuestionnaireVersion(expectedVersionId);
      applyLoaded(result, generation, expectedVersionId);
    } catch (error) {
      if (operationIsCurrent(generation, expectedVersionId)) {
        report(errorMessage(error));
      }
    }
  }, [applyLoaded, client, report]);

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const expectedVersionId = versionIdRef.current;
    const generation = ++operationGeneration.current;
    report("Salvando definição…");
    try {
      const body = JSON.parse(definition) as Definition;
      const result = await client.updateQuestionnaireVersion(
        expectedVersionId,
        body,
        etag,
      );
      if (!operationIsCurrent(generation, expectedVersionId)) {
        return;
      }
      setEtag(result.etag ?? "");
      report("Definição salva.");
    } catch (error) {
      if (operationIsCurrent(generation, expectedVersionId)) {
        report(errorMessage(error));
      }
    }
  }

  async function execute(
    label: string,
    action: (key: string) => ReturnType<QuestionnaireClient["approve"]>,
  ) {
    const expectedVersionId = versionIdRef.current;
    const generation = ++operationGeneration.current;
    report(`${label}…`);
    try {
      const result = await action(actionKey.current);
      actionKey.current = crypto.randomUUID();
      if (!operationIsCurrent(generation, expectedVersionId)) {
        return;
      }
      setEtag(`"${result.data.revision}"`);
      report(`${label}: concluído.`);
    } catch (error) {
      if (operationIsCurrent(generation, expectedVersionId)) {
        report(errorMessage(error));
      }
    }
  }

  const unavailable = versionId.length === 0 || !/^"[1-9][0-9]*"$/.test(etag);

  return (
    <section aria-labelledby="version-editor-title">
      <h3 id="version-editor-title">Editor e workflow</h3>
      <div className="field-grid">
        <label>
          ID da versão
          <input
            value={versionId}
            required
            onChange={(event) => changeVersionId(event.target.value)}
          />
        </label>
        <label>
          ETag
          <input
            value={etag}
            pattern={'^"[1-9][0-9]*"$'}
            required
            onChange={(event) => changeEtag(event.target.value)}
          />
        </label>
      </div>
      <button type="button" disabled={versionId.length === 0} onClick={() => void load()}>
        Carregar versão
      </button>
      <form onSubmit={(event) => void save(event)}>
        <label>
          Definição estruturada em JSON
          <textarea
            value={definition}
            rows={20}
            spellCheck={false}
            required
            onChange={(event) => setDefinition(event.target.value)}
          />
        </label>
        <p className="privacy-warning" role="note">
          Texto livre deve ser opcional e nunca deve solicitar nome, documento,
          contato, credencial ou dado sensível.
        </p>
        <button type="submit" disabled={unavailable}>
          Salvar definição
        </button>
      </form>
      <div className="workflow-panel" aria-label="Ações de workflow">
        <button
          type="button"
          disabled={unavailable}
          onClick={() =>
            void execute("Enviar para revisão", (key) =>
              client.submitReview(versionId, etag, key),
            )
          }
        >
          Enviar para revisão
        </button>
        <label>
          Motivo da devolução
          <select
            value={reason}
            onChange={(event) =>
              setReason(
                event.target.value as Schemas["RequestChangesRequest"]["reason_code"],
              )
            }
          >
            <option value="privacy_metadata_incomplete">Metadados incompletos</option>
            <option value="excessive_collection">Coleta excessiva</option>
            <option value="unsafe_condition">Condição insegura</option>
            <option value="consent_mismatch">Consentimento divergente</option>
          </select>
        </label>
        <button
          type="button"
          disabled={unavailable}
          onClick={() =>
            void execute("Solicitar mudanças", (key) =>
              client.requestChanges(versionId, { reason_code: reason }, etag, key),
            )
          }
        >
          Solicitar mudanças
        </button>
        <button
          type="button"
          disabled={unavailable}
          onClick={() =>
            void execute("Aprovar", (key) => client.approve(versionId, etag, key))
          }
        >
          Aprovar
        </button>
        <button
          type="button"
          disabled={unavailable}
          onClick={() =>
            void execute("Publicar", (key) => client.publish(versionId, etag, key))
          }
        >
          Publicar
        </button>
        <button
          type="button"
          disabled={unavailable}
          onClick={() =>
            void execute("Retirar", (key) => client.retire(versionId, etag, key))
          }
        >
          Retirar
        </button>
      </div>
    </section>
  );
}

export function QuestionnaireAdmin({ client }: { client: QuestionnaireClient }) {
  const queryClient = useQueryClient();
  const [message, setMessage] = useState("");
  const [questionnaireId, setQuestionnaireId] = useState("");
  const questionnaireIdRef = useRef("");
  const [selectedVersion, setSelectedVersion] =
    useState<Schemas["QuestionnaireVersionAdmin"] | null>(null);
  const [versionId, setVersionId] = useState("");
  const [etag, setEtag] = useState("");
  const selectedKey = useMemo(() => `${versionId}:${etag}`, [etag, versionId]);

  const created = useCallback(
    (id: string, nextEtag: string) => {
      setVersionId(id);
      setEtag(nextEtag);
      setSelectedVersion(null);
      void queryClient.invalidateQueries({ queryKey: ["questionnaires"] });
    },
    [queryClient],
  );

  const selectQuestionnaire = useCallback((id: string) => {
    questionnaireIdRef.current = id;
    setQuestionnaireId(id);
    setVersionId("");
    setEtag("");
    setSelectedVersion(null);
    setMessage("Selecione uma versão para retomar.");
  }, []);

  const selectVersion = useCallback(
    (
      version: Schemas["QuestionnaireVersionAdmin"],
      nextEtag: string,
      expectedQuestionnaireId: string,
    ) => {
      if (
        questionnaireIdRef.current !== expectedQuestionnaireId ||
        version.questionnaire_id !== expectedQuestionnaireId
      ) {
        return false;
      }
      setVersionId(version.id);
      setEtag(nextEtag);
      setSelectedVersion(version);
      return true;
    },
    [],
  );

  return (
    <div className="questionnaire-admin operation-grid">
      <section className="operation-card" aria-labelledby="questionnaire-create-title">
        <h2 id="questionnaire-create-title">Configuração versionada</h2>
        <p>
          Editor e revisor usam escopos OIDC separados; publicação exige revisão
          explícita de privacidade.
        </p>
        <CreateForm
          client={client}
          disabled={false}
          onCreated={created}
          report={setMessage}
        />
        <Catalog
          client={client}
          selectedId={questionnaireId}
          onSelect={selectQuestionnaire}
        />
        {questionnaireId.length > 0 ? (
          <VersionCatalog
            key={questionnaireId}
            client={client}
            questionnaireId={questionnaireId}
            selectedVersionId={versionId}
            onSelect={selectVersion}
            report={setMessage}
          />
        ) : null}
      </section>
      <section className="operation-card">
        <VersionEditor
          key={selectedKey}
          client={client}
          initialEtag={etag}
          initialVersion={selectedVersion}
          initialVersionId={versionId}
          report={setMessage}
        />
        <p role="status" aria-live="polite">
          {message}
        </p>
      </section>
    </div>
  );
}
