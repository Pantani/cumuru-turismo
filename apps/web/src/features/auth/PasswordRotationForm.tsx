import { useCallback, useId, useRef, useState, type FormEvent } from "react";

import { AuthError } from "../../shared/api/auth-client";
import { useAuthSession } from "../../shared/auth/AuthSession";
import {
  describedBy,
  FieldError,
  invalidFlag,
} from "../../shared/forms/FieldFeedback";

const MIN_PASSWORD_LENGTH = 12;

interface RotationIssues {
  current?: string;
  next?: string;
  confirmation?: string;
}

/**
 * validateRotation mirrors the contract bounds so an obviously invalid attempt
 * never costs a round trip. The confirmation field exists only here: a typed
 * secret that cannot be read back is easy to mistype.
 */
export function validateRotation(
  current: string,
  next: string,
  confirmation: string,
): RotationIssues {
  const issues: RotationIssues = {};
  if (current.length < MIN_PASSWORD_LENGTH) {
    issues.current = `A senha atual tem no mínimo ${MIN_PASSWORD_LENGTH} caracteres.`;
  }
  if (next.length < MIN_PASSWORD_LENGTH) {
    issues.next = `A nova senha tem no mínimo ${MIN_PASSWORD_LENGTH} caracteres.`;
  } else if (next === current) {
    issues.next = "A nova senha precisa ser diferente da atual.";
  }
  if (confirmation !== next) {
    issues.confirmation = "A confirmação não confere com a nova senha.";
  }
  return issues;
}

function rotationMessage(error: unknown) {
  if (error instanceof AuthError) {
    return error.message;
  }
  return "Não foi possível trocar a senha agora. Verifique sua conexão e tente de novo.";
}

interface SecretFieldProps {
  autoComplete: "current-password" | "new-password";
  id: string;
  issue: string | undefined;
  label: string;
  onChange: (value: string) => void;
  value: string;
}

function SecretField({
  autoComplete,
  id,
  issue,
  label,
  onChange,
  value,
}: SecretFieldProps) {
  return (
    <label className="field-control" htmlFor={id}>
      {label}
      <input
        id={id}
        type="password"
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

interface RotationState {
  current: string;
  next: string;
  confirmation: string;
}

const emptyRotation: RotationState = { current: "", next: "", confirmation: "" };

export function PasswordRotationForm() {
  const session = useAuthSession();
  const formId = useId();
  const [secrets, setSecrets] = useState<RotationState>(emptyRotation);
  const [issues, setIssues] = useState<RotationIssues>({});
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
      const found = validateRotation(
        secrets.current,
        secrets.next,
        secrets.confirmation,
      );
      setIssues(found);
      if (Object.keys(found).length > 0) {
        return;
      }
      setPending(true);
      try {
        await session.rotatePassword(secrets.current, secrets.next);
        setSecrets(emptyRotation);
      } catch (error) {
        fail(rotationMessage(error));
      } finally {
        setPending(false);
      }
    },
    [fail, secrets, session],
  );

  return (
    <form
      className="login-card"
      onSubmit={(event) => void submit(event)}
      noValidate
    >
      <h2>Defina uma senha própria</h2>
      <p className="login-intro">
        A senha atual foi criada durante a instalação e vale apenas para este
        primeiro acesso. Escolha uma senha só sua para continuar.
      </p>

      {failure !== null && (
        <div className="form-alert" role="alert" tabIndex={-1} ref={alertRef}>
          {failure}
        </div>
      )}

      <SecretField
        autoComplete="current-password"
        id={`${formId}-current`}
        issue={issues.current}
        label="Senha atual"
        onChange={(current) => setSecrets((state) => ({ ...state, current }))}
        value={secrets.current}
      />
      <SecretField
        autoComplete="new-password"
        id={`${formId}-next`}
        issue={issues.next}
        label="Nova senha"
        onChange={(next) => setSecrets((state) => ({ ...state, next }))}
        value={secrets.next}
      />
      <SecretField
        autoComplete="new-password"
        id={`${formId}-confirmation`}
        issue={issues.confirmation}
        label="Confirme a nova senha"
        onChange={(confirmation) =>
          setSecrets((state) => ({ ...state, confirmation }))
        }
        value={secrets.confirmation}
      />

      <button type="submit" className="primary-action" disabled={pending}>
        {pending ? "Trocando…" : "Trocar senha e entrar"}
      </button>
    </form>
  );
}
