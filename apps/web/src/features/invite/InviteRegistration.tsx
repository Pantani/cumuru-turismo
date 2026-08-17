import {
  type FormEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import type { components } from "../../generated/schema";
import {
  createVisitor,
  VisitorEditor,
  type VisitorEditorHandle,
} from "../visitors/VisitorEditor";
import { normalizedVisitor } from "../visitors/visitor-input";
import { ApiError } from "../../shared/api/http-client";
import { createCoreClient } from "../../shared/api/core-client";
import { guestCopyFor } from "../../shared/forms/guest-copy";
import {
  deleteDraft,
  loadDraft,
  saveDraft,
} from "../../shared/offline/encrypted-drafts";
import { draftDispositionFor } from "../../shared/offline/sync-policy";
import {
  clearInviteCapability,
  peekInviteCapability,
} from "../../shared/security/invite-capability";
import {
  peekSurveyCapability,
  setSurveyCapability,
} from "../../shared/security/survey-capability";
import { createUuidV7 } from "../../shared/identity/uuid-v7";
import {
  type ValidationIssue,
  validateSubmitGroup,
} from "../../shared/validation/core-validation";

type InviteContext = components["schemas"]["InviteContext"];
type SubmitGroupRequest = components["schemas"]["SubmitGroupRequest"];
type VisitorInput = components["schemas"]["VisitorInput"];

const guestClient = createCoreClient({ getAccessToken: () => null });

function draftIdFromHash() {
  const value = new URLSearchParams(window.location.hash.slice(1)).get("draft");
  return value !== null && /^[0-9a-f-]{36}$/i.test(value) ? value : null;
}

function updateDraftHash(id: string | null) {
  const hash = id === null ? "" : `#draft=${encodeURIComponent(id)}`;
  window.history.replaceState(
    null,
    "",
    `${window.location.pathname}${window.location.search}${hash}`,
  );
}

function requestFrom(
  visitors: VisitorInput[],
  privacyNoticeVersion: string,
  clientSubmissionId: string,
): SubmitGroupRequest {
  return {
    client_submission_id: clientSubmissionId,
    privacy_notice_version: privacyNoticeVersion,
    visitors: visitors.map(normalizedVisitor),
  };
}

/**
 * O convite é tela de hóspede, então `problem.title` não chega até ele: o
 * título do problema é escrito para quem opera o serviço e pode ser jargão sem
 * ação possível. Os status que a pessoa pode resolver têm cópia própria aqui;
 * o resto vira uma frase que ela consegue usar.
 */
const guestMessages: Readonly<Record<number, string>> = {
  404: "Convite expirado.",
  409: "O aviso de privacidade mudou desde que este convite foi criado. Peça outro à hospedagem.",
  422: "Alguns dados não foram aceitos. Revise e tente de novo.",
  429: "Já houve envios demais desta rede agora há pouco.",
};

function errorMessage(error: unknown) {
  if (!(error instanceof ApiError)) {
    return "Sem conexão. O rascunho foi preservado neste dispositivo.";
  }
  return guestCopyFor(error, guestMessages);
}

async function removeDraft(id: string | null) {
  if (id !== null) {
    await deleteDraft(id);
  }
  updateDraftHash(null);
}

async function handleSubmissionFailure(
  error: unknown,
  draftId: string | null,
  inviteWasValidated: boolean,
  preserve: () => Promise<void>,
) {
  const status = error instanceof ApiError ? error.status : 0;
  if (draftDispositionFor(status, inviteWasValidated) === "purge") {
    await removeDraft(draftId);
  } else {
    await preserve();
  }
}

function focusFieldFor(issue: ValidationIssue | undefined) {
  if (issue?.field === "visitors") {
    return "visitors.0.role";
  }
  return issue?.field ?? "";
}

function validateSubmission(
  payload: SubmitGroupRequest,
  report: (issues: ValidationIssue[], message: string, field: string) => void,
) {
  const validationIssues = validateSubmitGroup(payload);
  if (validationIssues.length === 0) {
    return true;
  }
  const first = validationIssues[0];
  report(
    validationIssues,
    first?.message ?? "Revise os dados informados.",
    focusFieldFor(first),
  );
  return false;
}

interface FailureActions {
  clearExpired: () => void;
  persistDraft: () => Promise<void>;
  setMessage: (message: string) => void;
}

function SurveyContinuation() {
  if (peekSurveyCapability() === null) {
    return (
      <p>
        A pesquisa voluntária não foi disponibilizada para esta conclusão.
      </p>
    );
  }
  return (
    <button
      className="primary-action"
      type="button"
      onClick={() => {
        window.history.pushState(null, "", "/pesquisa");
        window.dispatchEvent(new PopStateEvent("popstate"));
      }}
    >
      Responder pesquisa voluntária
    </button>
  );
}

export function RegistrationCompletion({
  message = "Grupo enviado com sucesso.",
}: {
  message?: string;
}) {
  return (
    <section className="form-card" aria-labelledby="invite-state-title">
      <h2 id="invite-state-title">Registro concluído</h2>
      <p role="status" aria-live="polite">
        {message}
      </p>
      <SurveyContinuation />
    </section>
  );
}

async function recoverSubmission(
  error: unknown,
  draftId: string | null,
  inviteWasValidated: boolean,
  actions: FailureActions,
) {
  await handleSubmissionFailure(
    error,
    draftId,
    inviteWasValidated,
    actions.persistDraft,
  );
  if (error instanceof ApiError && error.status === 404) {
    actions.clearExpired();
  }
  actions.setMessage(errorMessage(error));
}

async function purgeInitialInviteIfMissing(
  error: unknown,
  draftId: string | null,
) {
  if (!(error instanceof ApiError) || error.status !== 404) {
    return false;
  }
  await removeDraft(draftId);
  clearInviteCapability();
  return true;
}

export function InviteRegistration() {
  const [context, setContext] = useState<InviteContext | null>(null);
  const [visitors, setVisitors] = useState<VisitorInput[]>(() => [
    createVisitor("responsible"),
  ]);
  const [issues, setIssues] = useState<ValidationIssue[]>([]);
  const [draftId, setDraftId] = useState<string | null>(draftIdFromHash);
  const [message, setMessage] = useState("Validando convite…");
  const [completed, setCompleted] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [clientSubmissionId, setClientSubmissionId] = useState<string>(() =>
    createUuidV7(),
  );
  const initialDraftId = useRef(draftId);
  const visitorEditorRef = useRef<VisitorEditorHandle>(null);

  const payload = useMemo(
    () =>
      context === null
        ? null
        : requestFrom(
            visitors,
            context.privacy_notice_version,
            clientSubmissionId,
          ),
    [clientSubmissionId, context, visitors],
  );

  useEffect(() => {
    const capability = peekInviteCapability();
    if (capability === null) {
      setMessage("Convite ausente ou removido.");
      return;
    }
    let active = true;
    void guestClient
      .getInvite(capability)
      .then((result) => {
        if (active) {
          setContext(result.data);
          setMessage("");
        }
      })
      .catch(async (error: unknown) => {
        if (active) {
          const purged = await purgeInitialInviteIfMissing(
            error,
            initialDraftId.current,
          );
          if (!active) {
            return;
          }
          if (purged) {
            setDraftId(null);
            setContext(null);
          }
          setMessage(errorMessage(error));
        }
      });
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    const initialId = initialDraftId.current;
    if (initialId === null) {
      return;
    }
    void loadDraft<SubmitGroupRequest>(initialId).then((draft) => {
      if (draft !== null && draft.visitors.length > 0) {
        setClientSubmissionId(draft.client_submission_id);
        setVisitors(draft.visitors);
        setMessage("Rascunho recuperado neste dispositivo.");
      }
    });
  }, []);

  const persistDraft = useCallback(async () => {
    if (payload === null) {
      return;
    }
    const saved =
      draftId === null
        ? await saveDraft(payload)
        : await saveDraft(payload, { id: draftId });
    setDraftId(saved.id);
    updateDraftHash(saved.id);
    setMessage("Rascunho cifrado salvo por até 24 horas.");
  }, [draftId, payload]);

  const submit = useCallback(async () => {
    const capability = peekInviteCapability();
    if (payload === null || capability === null) {
      return;
    }
    const valid = validateSubmission(
      payload,
      (nextIssues, nextMessage, field) => {
        setIssues(nextIssues);
        setMessage(nextMessage);
        visitorEditorRef.current?.focus(field);
      },
    );
    if (!valid) {
      return;
    }
    setSubmitting(true);
    try {
      const validated = await guestClient.getInvite(capability);
      setContext(validated.data);
      const result = await guestClient.submitInviteGroup(
        capability,
        payload,
        payload.client_submission_id,
      );
      setSurveyCapability(result.surveyCapability);
      await removeDraft(draftId);
      setDraftId(null);
      clearInviteCapability();
      setMessage("Grupo enviado com sucesso.");
      setCompleted(true);
      setContext(null);
    } catch (error) {
      await recoverSubmission(
        error,
        draftId,
        context !== null,
        {
          persistDraft,
          setMessage,
          clearExpired: () => {
            clearInviteCapability();
            setContext(null);
            setDraftId(null);
          },
        },
      );
    } finally {
      setSubmitting(false);
    }
  }, [context, draftId, payload, persistDraft]);

  useEffect(() => {
    if (draftId === null) {
      return;
    }
    const synchronize = () => void submit();
    window.addEventListener("online", synchronize);
    return () => window.removeEventListener("online", synchronize);
  }, [draftId, submit]);

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void submit();
  }

  async function discard() {
    await removeDraft(draftId);
    setDraftId(null);
    clearInviteCapability();
    setContext(null);
    setMessage("Rascunho e convite descartados neste dispositivo.");
  }

  if (context === null) {
    if (completed) {
      return <RegistrationCompletion message={message} />;
    }
    return (
      <section className="form-card" aria-labelledby="invite-state-title">
        <h2 id="invite-state-title">Convite em validação</h2>
        <p role="status" aria-live="polite">
          {message}
        </p>
      </section>
    );
  }

  return (
    <section className="form-card" aria-labelledby="group-form-title">
      <h2 id="group-form-title">Confirme o grupo da estadia</h2>
      <dl className="context-summary">
        <div>
          <dt>Acomodação</dt>
          <dd>{context.accommodation_name}</dd>
        </div>
        <div>
          <dt>Período previsto</dt>
          <dd>
            {context.planned_arrival_on} a {context.planned_departure_on}
          </dd>
        </div>
      </dl>
      <p className="privacy-warning" role="note">
        Informe somente dados generalizados. Não inclua nome, documento,
        contato, endereço ou observações pessoais.
      </p>
      <form noValidate onSubmit={handleSubmit}>
        <VisitorEditor
          ref={visitorEditorRef}
          disabled={submitting}
          issues={issues}
          visitors={visitors}
          onChange={(nextVisitors) => {
            setVisitors(nextVisitors);
            setIssues([]);
          }}
        />
        <div className="form-actions">
          <button type="button" onClick={() => void persistDraft()}>
            Salvar rascunho cifrado
          </button>
          <button type="button" onClick={() => void discard()}>
            Descartar rascunho
          </button>
          <button className="primary-action" type="submit" disabled={submitting}>
            {submitting ? "Enviando…" : "Enviar grupo"}
          </button>
        </div>
      </form>
      {message.length > 0 ? (
        <p role="status" aria-live="polite">
          {message}
        </p>
      ) : null}
    </section>
  );
}
