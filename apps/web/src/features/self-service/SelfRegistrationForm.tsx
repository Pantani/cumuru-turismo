import {
  type FormEvent,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";

import type { components } from "../../generated/schema";
import { createPhase7Client } from "../../shared/api/phase7-client";
import { createUuidV7 } from "../../shared/identity/uuid-v7";
import { deleteDraft, saveDraft } from "../../shared/offline/encrypted-drafts";
import { solveProofOfWork } from "../../shared/security/proof-of-work";
import {
  clearSelfServiceCapability,
  markSelfRegistrationCompleted,
  peekSelfServiceCapability,
} from "../../shared/security/self-service-capability";
import { setSurveyCapability } from "../../shared/security/survey-capability";
import type { ValidationIssue } from "../../shared/validation/phase2-validation";
import {
  validateSelfRegistrationDraft,
  type SelfRegistrationDraft,
} from "../../shared/validation/phase7-validation";
import { defaultStayDates } from "../operator/stay-lifecycle";
import {
  createVisitor,
  VisitorEditor,
  type VisitorEditorHandle,
} from "../visitors/VisitorEditor";
import { PlannedWindowFields } from "./PlannedWindowFields";
import { PrivacyNotice } from "./PrivacyNotice";
import { focusFirstIssue } from "./self-registration-focus";
import {
  selfRegistrationDraftFrom,
  type SelfRegistrationFormState,
} from "./self-registration-request";
import {
  describeSelfServiceFailure,
  shouldPurgeDraft,
} from "./self-service-messages";
import { SelfRegistrationCompletion } from "./SelfRegistrationCompletion";

type AccommodationInviteContext =
  components["schemas"]["AccommodationInviteContext"];
type VisitorInput = components["schemas"]["VisitorInput"];

const guestClient = createPhase7Client({ getAccessToken: () => null });

/** ADR-040: `minor` is not offered, so it cannot be chosen and then refused. */
const openChannelRoles = [
  "responsible",
  "companion",
] as const satisfies readonly VisitorInput["role"][];

function initialFormState(): SelfRegistrationFormState {
  const dates = defaultStayDates();
  return {
    clientSubmissionId: createUuidV7(),
    plannedArrivalOn: dates.arrival,
    plannedDepartureOn: dates.departure,
    visitors: [createVisitor("responsible")],
  };
}

function useInviteContext() {
  const [context, setContext] = useState<AccommodationInviteContext | null>(null);
  const [message, setMessage] = useState("Validando o cartaz…");

  const load = useCallback(async () => {
    const capability = peekSelfServiceCapability();
    if (capability === null) {
      return;
    }
    try {
      const result = await guestClient.getAccommodationInviteContext(capability);
      setContext(result.data);
      setMessage("");
    } catch (error) {
      setMessage(describeSelfServiceFailure(error));
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return { context, load, message, setMessage };
}

interface SubmissionInput {
  capability: string;
  context: AccommodationInviteContext;
  draft: SelfRegistrationDraft;
  onProgress: () => void;
}

async function sendSubmission({
  capability,
  context,
  draft,
  onProgress,
}: SubmissionInput) {
  const solution = await solveProofOfWork({
    challenge: context.proof_of_work.challenge,
    difficultyBits: context.proof_of_work.difficulty_bits,
    onAttempt: onProgress,
  });
  return guestClient.submitAccommodationSelfRegistration(
    capability,
    {
      ...draft,
      proof_of_work: {
        challenge: context.proof_of_work.challenge,
        solution,
      },
    },
    draft.client_submission_id,
  );
}

/**
 * Decides in one place whether a submission may start, and returns the two
 * values it needs. The in-flight test lives here, not in `busy`: the `online`
 * listener holds a captured closure, so a state value read there can be stale
 * by a render, while a ref is read at call time. Without this, a flapping
 * connection starts a second submission that solves the proof of work again and
 * then spends the same challenge, losing to replay — punishing exactly the
 * weak device on the unstable network this channel exists to serve.
 */
function submissionTarget(
  context: AccommodationInviteContext | null,
  inFlight: boolean,
) {
  const capability = peekSelfServiceCapability();
  if (context === null || capability === null || inFlight) {
    return null;
  }
  return { capability, context };
}

export function SelfRegistrationForm() {
  const { context, load, message, setMessage } = useInviteContext();
  const [form, setForm] = useState<SelfRegistrationFormState>(initialFormState);
  const [issues, setIssues] = useState<readonly ValidationIssue[]>([]);
  const [busy, setBusy] = useState(false);
  const [completed, setCompleted] = useState(false);
  const draftIdRef = useRef<string | null>(null);
  const inFlightRef = useRef(false);
  const editorRef = useRef<VisitorEditorHandle>(null);

  const reportIssues = useCallback(
    (found: readonly ValidationIssue[]) => {
      setIssues(found);
      const first = found[0];
      setMessage(first === undefined ? "" : first.message);
      focusFirstIssue(editorRef.current, first);
    },
    [setMessage],
  );

  const preserveDraft = useCallback(
    async (draft: SelfRegistrationDraft, error: unknown) => {
      if (shouldPurgeDraft(error)) {
        await discardDraft(draftIdRef);
        return;
      }
      const saved = await saveDraft(draft, currentDraftOptions(draftIdRef));
      draftIdRef.current = saved.id;
    },
    [],
  );

  const submit = useCallback(async () => {
    const target = submissionTarget(context, inFlightRef.current);
    if (target === null) {
      return;
    }
    const draft = selfRegistrationDraftFrom(form, target.context);
    const found = validateSelfRegistrationDraft(draft);
    if (found.length > 0) {
      reportIssues(found);
      return;
    }
    inFlightRef.current = true;
    setIssues([]);
    setBusy(true);
    try {
      const result = await sendSubmission({
        capability: target.capability,
        context: target.context,
        draft,
        onProgress: () => setMessage("Verificando este dispositivo…"),
      });
      setSurveyCapability(result.surveyCapability);
      await discardDraft(draftIdRef);
      // O sinal de conclusão é registrado antes de o token ser descartado, e
      // sobrevive a ele. `setCompleted` continua valendo para o caso em que
      // este subtree não é desmontado; os dois caminhos renderizam a mesma
      // confirmação, de modo que a tela não depende de quem redesenha primeiro.
      markSelfRegistrationCompleted();
      clearSelfServiceCapability();
      setCompleted(true);
    } catch (error) {
      await preserveDraft(draft, error);
      setMessage(describeSelfServiceFailure(error));
    } finally {
      inFlightRef.current = false;
      setBusy(false);
    }
  }, [context, form, preserveDraft, reportIssues, setMessage]);

  useEffect(() => {
    const retry = () => void submit();
    window.addEventListener("online", retry);
    return () => window.removeEventListener("online", retry);
  }, [submit]);

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void submit();
  }

  if (completed) {
    return <SelfRegistrationCompletion />;
  }

  if (context === null) {
    return (
      <section className="form-card" aria-labelledby="poster-state-title">
        <h2 id="poster-state-title">Cartaz em validação</h2>
        <p role="status" aria-live="polite">
          {message}
        </p>
        <button type="button" onClick={() => void load()}>
          Tentar de novo
        </button>
      </section>
    );
  }

  return (
    <>
      <PrivacyNotice
        accommodationName={context.accommodation_name}
        version={context.privacy_notice_version}
      />
      <section className="form-card" aria-labelledby="self-registration-title">
        <h2 id="self-registration-title">Confirme os dados da estadia</h2>
        <form noValidate onSubmit={handleSubmit}>
          <PlannedWindowFields
            arrival={form.plannedArrivalOn}
            departure={form.plannedDepartureOn}
            disabled={busy}
            issues={issues}
            onChange={(field, value) =>
              setForm((current) => withDate(current, field, value))
            }
          />
          <VisitorEditor
            ref={editorRef}
            disabled={busy}
            issues={[...issues]}
            roles={openChannelRoles}
            visitors={form.visitors}
            onChange={(visitors) => {
              setForm((current) => ({ ...current, visitors }));
              setIssues([]);
            }}
          />
          <div className="form-actions">
            <button className="primary-action" type="submit" disabled={busy}>
              {busy ? "Enviando…" : "Enviar autocadastro"}
            </button>
          </div>
        </form>
        {message.length > 0 ? (
          <p role="status" aria-live="polite">
            {message}
          </p>
        ) : null}
      </section>
    </>
  );
}

function withDate(
  current: SelfRegistrationFormState,
  field: "arrival" | "departure",
  value: string,
): SelfRegistrationFormState {
  if (field === "arrival") {
    return { ...current, plannedArrivalOn: value };
  }
  return { ...current, plannedDepartureOn: value };
}

function currentDraftOptions(draftIdRef: { current: string | null }) {
  const id = draftIdRef.current;
  return id === null ? {} : { id };
}

async function discardDraft(draftIdRef: { current: string | null }) {
  const id = draftIdRef.current;
  if (id !== null) {
    await deleteDraft(id);
    draftIdRef.current = null;
  }
}
