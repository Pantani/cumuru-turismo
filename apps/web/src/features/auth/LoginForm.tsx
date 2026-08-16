import { useCallback, useId, useRef, useState, type FormEvent } from "react";

import { AuthError } from "../../shared/api/auth-client";
import { useAuthSession } from "../../shared/auth/AuthSession";
import {
  describedBy,
  FieldError,
  invalidFlag,
} from "../../shared/forms/FieldFeedback";

const MIN_PASSWORD_LENGTH = 12;

interface FieldIssues {
  email?: string;
  password?: string;
}

/**
 * validateCredentials mirrors the contract bounds so an obviously malformed
 * attempt never costs a round trip. It never judges whether the account exists.
 */
export function validateCredentials(
  email: string,
  password: string,
): FieldIssues {
  const issues: FieldIssues = {};
  if (!/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(email.trim())) {
    issues.email = "Informe um e-mail válido, no formato nome@dominio.com.";
  }
  if (password.length < MIN_PASSWORD_LENGTH) {
    issues.password = `A senha tem no mínimo ${MIN_PASSWORD_LENGTH} caracteres.`;
  }
  return issues;
}

function submissionMessage(error: unknown) {
  if (error instanceof AuthError) {
    return error.message;
  }
  return "Não foi possível entrar agora. Verifique sua conexão e tente de novo.";
}

function FormAlert({
  alertRef,
  failure,
}: {
  alertRef: React.RefObject<HTMLDivElement | null>;
  failure: string | null;
}) {
  if (failure === null) {
    return null;
  }
  return (
    <div className="form-alert" role="alert" tabIndex={-1} ref={alertRef}>
      {failure}
    </div>
  );
}

interface CredentialFieldProps {
  autoComplete: string;
  id: string;
  issue: string | undefined;
  label: string;
  onChange: (value: string) => void;
  type: "email" | "password";
  value: string;
}

function CredentialField({
  autoComplete,
  id,
  issue,
  label,
  onChange,
  type,
  value,
}: CredentialFieldProps) {
  return (
    <label className="field-control" htmlFor={id}>
      {label}
      <input
        id={id}
        type={type}
        autoComplete={autoComplete}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        aria-invalid={invalidFlag(issue)}
        aria-describedby={describedBy(`${id}-error`, issue)}
        required
      />
      <FieldError id={`${id}-error`} message={issue} />
    </label>
  );
}

export function LoginForm() {
  const session = useAuthSession();
  const formId = useId();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [issues, setIssues] = useState<FieldIssues>({});
  const [failure, setFailure] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const alertRef = useRef<HTMLDivElement>(null);

  const fail = useCallback((message: string) => {
    setFailure(message);
    window.requestAnimationFrame(() => alertRef.current?.focus());
  }, []);

  const submit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      setFailure(null);
      const found = validateCredentials(email, password);
      setIssues(found);
      if (found.email !== undefined || found.password !== undefined) {
        return;
      }
      setPending(true);
      try {
        await session.signIn(email.trim().toLowerCase(), password);
      } catch (error) {
        fail(submissionMessage(error));
      } finally {
        setPending(false);
      }
    },
    [email, fail, password, session],
  );

  return (
    <form
      className="login-card"
      onSubmit={(event) => void submit(event)}
      noValidate
    >
      <h2>Entrar na área da hospedagem</h2>
      <p className="login-intro">
        Use o e-mail e a senha cadastrados. Não é preciso CNPJ, Cadastur nem
        chave federal para participar do observatório local.
      </p>

      <FormAlert alertRef={alertRef} failure={failure} />

      <CredentialField
        autoComplete="username"
        id={`${formId}-email`}
        issue={issues.email}
        label="E-mail"
        onChange={setEmail}
        type="email"
        value={email}
      />
      <CredentialField
        autoComplete="current-password"
        id={`${formId}-password`}
        issue={issues.password}
        label="Senha"
        onChange={setPassword}
        type="password"
        value={password}
      />

      <button className="primary-action" type="submit" disabled={pending}>
        {pending ? "Entrando…" : "Entrar"}
      </button>

      <p className="login-footnote">
        A sessão fica só na memória desta aba. Ao fechar ou recarregar a página,
        você entra de novo — nada é guardado no navegador.
      </p>
    </form>
  );
}
