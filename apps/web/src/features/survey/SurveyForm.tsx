import { useQuery } from "@tanstack/react-query";
import {
  type FormEvent,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import {
  createPhase3Client,
  Phase3ApiError,
} from "../../shared/api/phase3-client";
import { createUuidV7 } from "../../shared/identity/uuid-v7";
import {
  deleteDraft,
  loadDraft,
  saveDraft,
} from "../../shared/offline/encrypted-drafts";
import {
  clearSurveyCapability,
  peekSurveyCapability,
} from "../../shared/security/survey-capability";
import { SurveyQuestion } from "./SurveyQuestion";
import {
  type AnswerState,
  applicableQuestions,
  answerInputs,
  consentInputs,
  missingRequired,
  missingRequiredConsent,
  visibleQuestions,
} from "./survey-logic";

const surveyClient = createPhase3Client({ getAccessToken: () => null });

function errorMessage(error: unknown) {
  if (error instanceof Phase3ApiError) {
    if (error.status === 429 && error.retryAfterSeconds !== null) {
      return `Muitas tentativas. Aguarde ${error.retryAfterSeconds} segundos.`;
    }
    return error.problem.title;
  }
  return "Sem conexão. Você pode preservar um rascunho cifrado neste dispositivo.";
}

interface SurveyDraft {
  answers: AnswerState;
  clientSubmissionId: string;
  consents: Record<string, boolean>;
  questionnaireVersionId: string;
}

function surveyDraftIdFromHash() {
  const value = new URLSearchParams(window.location.hash.slice(1)).get(
    "survey-draft",
  );
  return value !== null && /^[0-9a-f-]{36}$/i.test(value) ? value : null;
}

function updateSurveyDraftHash(id: string | null) {
  const hash = id === null ? "" : `#survey-draft=${encodeURIComponent(id)}`;
  window.history.replaceState(
    null,
    "",
    `${window.location.pathname}${window.location.search}${hash}`,
  );
}

function recordValue(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function validSurveyDraft(value: unknown): value is SurveyDraft {
  const record = recordValue(value);
  return (
    record !== null &&
    typeof record.clientSubmissionId === "string" &&
    typeof record.questionnaireVersionId === "string" &&
    recordValue(record.answers) !== null &&
    recordValue(record.consents) !== null &&
    !("capability" in record)
  );
}

export function SurveyForm() {
  const capability = peekSurveyCapability();
  const [answers, setAnswers] = useState<AnswerState>({});
  const [consents, setConsents] = useState<Record<string, boolean>>({});
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [completed, setCompleted] = useState(false);
  const [draftId, setDraftId] = useState<string | null>(surveyDraftIdFromHash);
  const clientSubmissionId = useRef(createUuidV7());
  const idempotencyKey = useRef(crypto.randomUUID());
  const restoredDraftId = useRef<string | null>(null);
  const active = useQuery({
    queryKey: ["active-questionnaire", "tourism_profile"],
    queryFn: () => surveyClient.getActiveQuestionnaire(),
    enabled: capability !== null,
    retry: false,
  });

  const questionnaire = active.data?.data;
  const applicable = useMemo(
    () =>
      questionnaire === undefined
        ? []
        : applicableQuestions(questionnaire.questions, consents),
    [consents, questionnaire],
  );
  const shownQuestions = useMemo(
    () => visibleQuestions(applicable, answers),
    [answers, applicable],
  );

  useEffect(() => {
    if (
      draftId === null ||
      questionnaire === undefined ||
      restoredDraftId.current === draftId
    ) {
      return;
    }
    restoredDraftId.current = draftId;
    let mounted = true;
    void loadDraft<unknown>(draftId).then((value) => {
      if (!mounted) {
        return;
      }
      if (validSurveyDraft(value) && value.questionnaireVersionId === questionnaire.id) {
        setAnswers(value.answers);
        setConsents(value.consents);
        clientSubmissionId.current = value.clientSubmissionId;
        setMessage("Rascunho cifrado recuperado neste dispositivo.");
        return;
      }
      setDraftId(null);
      updateSurveyDraftHash(null);
    });
    return () => {
      mounted = false;
    };
  }, [draftId, questionnaire]);

  if (capability === null && !completed) {
    return (
      <section className="boundary-note" aria-labelledby="survey-authority">
        <h2 id="survey-authority">Pesquisa disponível após o registro</h2>
        <p>
          A pesquisa voluntária é liberada somente na sessão que concluiu um
          cadastro de grupo. A capability não é salva no navegador.
        </p>
      </section>
    );
  }
  if (completed) {
    return (
      <section className="form-card" aria-labelledby="survey-completed">
        <h2 id="survey-completed">Participação registrada</h2>
        <p>Obrigado. A capability desta pesquisa foi removida da sessão.</p>
      </section>
    );
  }
  if (active.isPending) {
    return <p role="status">Carregando pesquisa…</p>;
  }
  if (active.isError || questionnaire === undefined) {
    return <p role="alert">{errorMessage(active.error)}</p>;
  }
  const published = questionnaire;

  const draft: SurveyDraft = {
    answers,
    clientSubmissionId: clientSubmissionId.current,
    consents,
    questionnaireVersionId: published.id,
  };

  async function persistDraft() {
    const saved =
      draftId === null
        ? await saveDraft(draft)
        : await saveDraft(draft, { id: draftId });
    setDraftId(saved.id);
    updateSurveyDraftHash(saved.id);
    setMessage("Rascunho cifrado salvo por até 24 horas, sem a capability.");
  }

  async function finish() {
    if (draftId !== null) {
      await deleteDraft(draftId);
    }
    clearSurveyCapability();
    setDraftId(null);
    updateSurveyDraftHash(null);
    setCompleted(true);
  }

  async function submit(participation: "submitted" | "declined") {
    if (capability === null) {
      return;
    }
    if (participation === "submitted") {
      if (missingRequired(applicable, answers).length > 0) {
        setMessage("Responda todas as perguntas obrigatórias visíveis.");
        return;
      }
      if (missingRequiredConsent(published.consent_requirements, consents)) {
        setMessage("O consentimento indicado é necessário para enviar respostas.");
        return;
      }
    }
    setBusy(true);
    setMessage("Enviando participação…");
    try {
      await surveyClient.submitSurveyResponse(
        {
          questionnaire_version_id: published.id,
          client_submission_id: clientSubmissionId.current,
          participation,
          answers:
            participation === "declined"
              ? []
              : answerInputs(applicable, answers),
          consent_decisions:
            participation === "declined"
              ? []
              : consentInputs(published.consent_requirements, consents),
        },
        capability,
        idempotencyKey.current,
      );
      await finish();
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void submit("submitted");
  }

  return (
    <section className="form-card survey-card" aria-labelledby="survey-title">
      <h2 id="survey-title">{published.title}</h2>
      {published.introduction ? <p>{published.introduction}</p> : null}
      <p className="privacy-warning" role="note">
        Participar é voluntário. Você pode recusar sem responder e sem impacto
        sobre a estadia.
      </p>
      <form noValidate onSubmit={handleSubmit}>
        {published.consent_requirements.map((requirement) => (
          <label className="consent-control" key={requirement.purpose_code}>
            <input
              type="checkbox"
              disabled={busy}
              checked={consents[requirement.purpose_code] === true}
              onChange={(event) =>
                setConsents((current) => ({
                  ...current,
                  [requirement.purpose_code]: event.target.checked,
                }))
              }
            />
            {requirement.prompt} Aviso {requirement.notice_version}.
          </label>
        ))}
        {shownQuestions.map((question) => (
          <SurveyQuestion
            key={question.id}
            disabled={busy}
            question={question}
            value={answers[question.id]}
            onChange={(value) =>
              setAnswers((current) => ({ ...current, [question.id]: value }))
            }
          />
        ))}
        <div className="form-actions">
          <button type="button" disabled={busy} onClick={() => void persistDraft()}>
            Salvar rascunho cifrado
          </button>
          <button type="button" disabled={busy} onClick={() => void submit("declined")}>
            Não quero participar
          </button>
          <button className="primary-action" type="submit" disabled={busy}>
            Enviar respostas
          </button>
        </div>
      </form>
      <p role="status" aria-live="polite">
        {message}
      </p>
    </section>
  );
}
