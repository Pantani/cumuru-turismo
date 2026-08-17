import {
  useCallback,
  useEffect,
  useId,
  useState,
  type FormEvent,
} from "react";

import type { components } from "../../generated/schema";
import { createPhase7Client, Phase7ApiError } from "../../shared/api/phase7-client";
import {
  describedBy,
  FieldError,
  invalidFlag,
} from "../../shared/forms/FieldFeedback";
import {
  clearActivationCapability,
  peekActivationCapability,
} from "../../shared/security/activation-capability";
import {
  validateActivationPassword,
  type ActivationPasswordIssues,
} from "../../shared/validation/phase7-validation";

type AccommodationActivationContext =
  components["schemas"]["AccommodationActivationContext"];

const guestClient = createPhase7Client({ getAccessToken: () => null });

/** Consumed, revoked, expired and forged all answer the same 404, by design. */
/**
 * Mesma regra do canal aberto: `problem.title` é escrito para quem opera o
 * serviço, e esta tela é pública. Só o `422` continua vindo do servidor, porque
 * é a política de senha e a pessoa precisa saber o que corrigir.
 */
function describeActivationFailure(error: unknown) {
  if (!(error instanceof Phase7ApiError)) {
    return "Sem conexão agora. Tente de novo em instantes.";
  }
  if (error.status === 404) {
    return "Este link de acesso não é mais válido. Peça outro a quem administra a hospedagem.";
  }
  if (error.status === 422) {
    return error.problem.title;
  }
  return "Não conseguimos concluir a ativação agora. Tente de novo em alguns instantes.";
}

interface SecretFieldProps {
  id: string;
  issue: string | undefined;
  label: string;
  onChange: (value: string) => void;
  value: string;
}

function SecretField({ id, issue, label, onChange, value }: SecretFieldProps) {
  // The error sits outside the label on purpose: nested inside, it would be
  // appended to the field's accessible name and a screen reader would announce
  // the label and the failure as one run-on phrase.
  return (
    <div className="field-control">
      <label htmlFor={id}>{label}</label>
      <input
        id={id}
        type="password"
        autoComplete="new-password"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        aria-invalid={invalidFlag(issue)}
        aria-describedby={describedBy(`${id}-error`, issue)}
        required
      />
      <FieldError id={`${id}-error`} message={issue} />
    </div>
  );
}

function useActivationContext() {
  const [context, setContext] = useState<AccommodationActivationContext | null>(
    null,
  );
  const [message, setMessage] = useState("Validando o link de acesso…");

  useEffect(() => {
    const capability = peekActivationCapability();
    if (capability === null) {
      return;
    }
    let active = true;
    void guestClient
      .getAccommodationActivation(capability)
      .then((result) => {
        if (active) {
          setContext(result.data);
          setMessage("");
        }
      })
      .catch((error: unknown) => {
        if (active) {
          setMessage(describeActivationFailure(error));
        }
      });
    return () => {
      active = false;
    };
  }, []);

  return { context, message, setMessage };
}

function ActivationDone() {
  return (
    <section className="form-card" aria-labelledby="activation-done">
      <h2 id="activation-done">Senha definida</h2>
      <p role="status" aria-live="polite">
        A conta está ativa. Agora é só entrar na área da hospedagem com o e-mail
        que você recebeu e a senha que acabou de escolher. Este link já não vale
        mais.
      </p>
      <button
        className="primary-action"
        type="button"
        onClick={() => {
          window.history.pushState(null, "", "/acesso");
          window.dispatchEvent(new PopStateEvent("popstate"));
        }}
      >
        Ir para a área da hospedagem
      </button>
    </section>
  );
}

export function AccountActivationForm() {
  const { context, message, setMessage } = useActivationContext();
  const formId = useId();
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [issues, setIssues] = useState<ActivationPasswordIssues>({});
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);

  const submit = useCallback(async () => {
    const capability = peekActivationCapability();
    if (capability === null) {
      return;
    }
    const found = validateActivationPassword(password, confirmation);
    setIssues(found);
    if (Object.keys(found).length > 0) {
      return;
    }
    setBusy(true);
    try {
      await guestClient.activateAccommodationAccount(capability, { password });
      clearActivationCapability();
      setPassword("");
      setConfirmation("");
      setDone(true);
    } catch (error) {
      setMessage(describeActivationFailure(error));
    } finally {
      setBusy(false);
    }
  }, [confirmation, password, setMessage]);

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void submit();
  }

  if (done) {
    return <ActivationDone />;
  }

  if (context === null) {
    return (
      <section className="form-card" aria-labelledby="activation-state">
        <h2 id="activation-state">Link em validação</h2>
        <p role="status" aria-live="polite">
          {message}
        </p>
      </section>
    );
  }

  return (
    <section className="form-card" aria-labelledby="activation-title">
      <h2 id="activation-title">Defina a senha de acesso</h2>
      <dl className="context-summary">
        <div>
          <dt>Hospedagem</dt>
          <dd>{context.accommodation_name}</dd>
        </div>
        <div>
          <dt>Conta</dt>
          <dd>{context.display_name}</dd>
        </div>
      </dl>
      <p className="disclaimer">
        Este link vale uma única vez e expira em {context.expires_at.slice(0, 10)}.
        Escolha uma senha de {context.password_policy.min_length} caracteres ou
        mais, que ninguém mais conheça.
      </p>
      <form noValidate onSubmit={handleSubmit}>
        <SecretField
          id={`${formId}-password`}
          issue={issues.password}
          label="Nova senha"
          onChange={setPassword}
          value={password}
        />
        <SecretField
          id={`${formId}-confirmation`}
          issue={issues.confirmation}
          label="Confirme a nova senha"
          onChange={setConfirmation}
          value={confirmation}
        />
        <div className="form-actions">
          <button className="primary-action" type="submit" disabled={busy}>
            {busy ? "Definindo…" : "Definir senha"}
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
