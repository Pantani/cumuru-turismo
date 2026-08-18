import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
} from "react";

import {
  createInviteRequestClient,
  type AccessRequestContext,
} from "../../shared/api/invite-request-client";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import type { Translate } from "../../shared/i18n/translate";
import { solveProofOfWork } from "../../shared/security/proof-of-work";
import { ContactFields, LodgingFields } from "./InviteRequestFields";
import { InviteRequestCompletion } from "./InviteRequestCompletion";
import {
  failureFocusTarget,
  INVITE_REQUEST_STATUS_ID,
  issueFocusTarget,
  useDeferredFocus,
} from "./invite-request-focus";
import {
  initialInviteRequestForm,
  inviteRequestBodyFrom,
  validateInviteRequest,
  type InviteRequestField,
  type InviteRequestFormState,
  type InviteRequestIssue,
} from "./invite-request-form";
import { describeInviteRequestFailure } from "./invite-request-messages";

/** Canal público: sem sessão, sem token, sem cookie — só o desafio de trabalho. */
const inviteRequestClient = createInviteRequestClient({
  getAccessToken: () => null,
});

function useAccessRequestContext(t: Translate) {
  const [context, setContext] = useState<AccessRequestContext | null>(null);
  const [message, setMessage] = useState(t("inviteRequest.loading"));

  const load = useCallback(async () => {
    setMessage(t("inviteRequest.loading"));
    try {
      const result = await inviteRequestClient.getAccessRequestContext();
      setContext(result.data);
      setMessage("");
    } catch (error) {
      setMessage(describeInviteRequestFailure(t, error));
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  return { context, load, message };
}

async function sendAccessRequest(
  form: InviteRequestFormState,
  context: AccessRequestContext,
  onProgress: () => void,
) {
  const solution = await solveProofOfWork({
    challenge: context.proof_of_work.challenge,
    difficultyBits: context.proof_of_work.difficulty_bits,
    onAttempt: onProgress,
  });
  const body = inviteRequestBodyFrom(form, context, solution);
  return inviteRequestClient.createAccessRequest(
    body,
    body.client_submission_id,
  );
}

/**
 * O teste de envio em voo mora numa `ref`, não no `busy`: um segundo clique
 * enquanto a prova de trabalho roda gastaria de novo a CPU do aparelho e
 * chegaria ao servidor com a mesma `Idempotency-Key`, sem nada a ganhar.
 */
function useInviteRequestSubmission(context: AccessRequestContext, t: Translate) {
  const [issues, setIssues] = useState<readonly InviteRequestIssue[]>([]);
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [completed, setCompleted] = useState(false);
  const inFlightRef = useRef(false);
  const requestFocus = useDeferredFocus();

  const send = useCallback(
    async (form: InviteRequestFormState) => {
      inFlightRef.current = true;
      setBusy(true);
      try {
        await sendAccessRequest(form, context, () =>
          setMessage(t("inviteRequest.verifyingDevice")),
        );
        setMessage("");
        setCompleted(true);
      } catch (error) {
        setMessage(describeInviteRequestFailure(t, error));
        requestFocus(failureFocusTarget(error));
      } finally {
        inFlightRef.current = false;
        setBusy(false);
      }
    },
    [context, requestFocus, t],
  );

  const submit = useCallback(
    async (form: InviteRequestFormState) => {
      if (inFlightRef.current) {
        return;
      }
      const found = validateInviteRequest(form);
      setIssues(found);
      if (found.length > 0) {
        setMessage(t("inviteRequest.error.summary"));
        requestFocus(issueFocusTarget(found[0]));
        return;
      }
      setMessage("");
      await send(form);
    },
    [requestFocus, send, t],
  );

  return { busy, completed, issues, message, submit };
}

function issueMessageOf(
  t: Translate,
  issues: readonly InviteRequestIssue[],
  field: InviteRequestField,
) {
  const found = issues.find((issue) => issue.field === field);
  return found === undefined ? undefined : t(found.message);
}

/**
 * A região existe vazia desde o primeiro render. Criada só quando há texto, a
 * mensagem chegaria junto com o elemento e leitores de tela costumam perder o
 * primeiro anúncio de uma região viva recém-inserida.
 */
function InviteRequestStatus({ message }: { message: string }) {
  return (
    <p
      className="operation-status"
      id={INVITE_REQUEST_STATUS_ID}
      tabIndex={-1}
      role="status"
      aria-live="polite"
    >
      {message}
    </p>
  );
}

function LoadedInviteRequestForm({ context }: { context: AccessRequestContext }) {
  const { t } = useLocale();
  const [form, setForm] = useState<InviteRequestFormState>(
    initialInviteRequestForm,
  );
  const { busy, completed, issues, message, submit } =
    useInviteRequestSubmission(context, t);

  const change = useCallback((field: InviteRequestField, value: string) => {
    setForm((current) => ({ ...current, [field]: value }));
  }, []);

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void submit(form);
  }

  if (completed) {
    return <InviteRequestCompletion email={form.contact_email.trim()} />;
  }

  const fieldProps = {
    disabled: busy,
    form,
    issueOf: (field: InviteRequestField) => issueMessageOf(t, issues, field),
    onChange: change,
    t,
  };

  return (
    <section className="form-card" aria-labelledby="invite-request-title">
      <h2 id="invite-request-title">{t("inviteRequest.formTitle")}</h2>
      <p className="disclaimer">
        {t("inviteRequest.privacyNote", {
          version: context.privacy_notice_version,
        })}
      </p>
      <form noValidate onSubmit={handleSubmit}>
        <LodgingFields {...fieldProps} />
        <ContactFields {...fieldProps} />
        <div className="form-actions">
          <button className="primary-action" type="submit" disabled={busy}>
            {busy ? t("inviteRequest.submitting") : t("inviteRequest.submit")}
          </button>
        </div>
      </form>
      <InviteRequestStatus message={message} />
    </section>
  );
}

function InviteRequestPending({
  message,
  onRetry,
}: {
  message: string;
  onRetry: () => void;
}) {
  const { t } = useLocale();
  return (
    <section className="form-card" aria-labelledby="invite-request-pending">
      <h2 id="invite-request-pending">{t("inviteRequest.pending.title")}</h2>
      <p role="status" aria-live="polite">
        {message}
      </p>
      <button type="button" onClick={onRetry}>
        {t("inviteRequest.retry")}
      </button>
    </section>
  );
}

export function InviteRequestForm() {
  const { t } = useLocale();
  const { context, load, message } = useAccessRequestContext(t);

  if (context === null) {
    return (
      <InviteRequestPending message={message} onRetry={() => void load()} />
    );
  }
  return <LoadedInviteRequestForm context={context} />;
}
