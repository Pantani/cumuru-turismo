import {
  useCallback,
  useId,
  useState,
  type FormEvent,
  type ReactNode,
} from "react";

import { useAuthSession } from "../../shared/auth/AuthSession";
import { OperationStatus } from "../../shared/forms/FieldFeedback";
import { InviteQr, type QrPresentation } from "../invite/InviteQr";
import { entityTagFor } from "../operator/stay-lifecycle";
import { useOperation } from "../operator/use-operation";

interface ActivationIssuePanelProps {
  accommodationId: string;
  /**
   * Preenchimento inicial vindo de um pedido já aprovado. Reemitir força o
   * formulário a voltar vazio, porque a segunda emissão pode ser para outra
   * pessoa e herdar o endereço anterior seria a decisão errada por omissão.
   */
  initialDraft?: ActivationDraft | undefined;
  /** Ressalva de quem chamou, mostrada junto do formulário. */
  note?: ReactNode;
  onVersionChange: (version: number) => void;
  version: number;
}

interface ActivationDraft {
  displayName: string;
  email: string;
}

const emptyDraft: ActivationDraft = { displayName: "", email: "" };

const activationPresentation: QrPresentation = {
  title: "Acesso pronto para entrega",
  label: "Código QR do acesso da hospedagem",
  discardLabel: "Ocultar acesso",
  description: (
    <p>
      O link vale uma única vez e aparece só agora. Se ele se perder, emita
      outro — o anterior deixa de valer.
    </p>
  ),
};

/**
 * ADR-041: no e-mail is sent by this feature, and none will be. The capability is
 * shown here, as a link and as a locally drawn QR, and delivering it to whoever
 * runs the property is a human act outside the system. That is a declared
 * limitation, not a security control.
 */
export function ActivationIssuePanel({
  accommodationId,
  initialDraft,
  note,
  onVersionChange,
  version,
}: ActivationIssuePanelProps) {
  const { selfServiceClient } = useAuthSession();
  const operation = useOperation();
  const { run } = operation;
  const formId = useId();
  const [draft, setDraft] = useState(initialDraft ?? emptyDraft);
  const [activationUrl, setActivationUrl] = useState<string | null>(null);

  const submit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      const result = await run("Emitindo o acesso da hospedagem", () =>
        selfServiceClient.createAccommodationActivation(
          accommodationId,
          { email: draft.email.trim(), display_name: draft.displayName.trim() },
          entityTagFor(version),
          crypto.randomUUID(),
        ),
      );
      if (result !== null) {
        onVersionChange(result.data.version);
        setActivationUrl(result.data.url);
        setDraft(emptyDraft);
      }
    },
    [accommodationId, draft, onVersionChange, run, selfServiceClient, version],
  );

  return (
    <section
      className="activation-panel"
      aria-labelledby={`${formId}-title`}
    >
      <h2 id={`${formId}-title`}>Acesso de quem opera a hospedagem</h2>
      <p className="queue-hint">
        Cria uma conta sem senha e um link de uso único para que a pessoa defina
        a própria senha. Nada é enviado por e-mail: entregue o link ou o código
        diretamente a ela.
      </p>
      {note}

      <form onSubmit={(event) => void submit(event)} noValidate>
        <div className="field-grid">
          <label className="field-control" htmlFor={`${formId}-name`}>
            Como chamar essa pessoa
            <input
              id={`${formId}-name`}
              value={draft.displayName}
              maxLength={120}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  displayName: event.target.value,
                }))
              }
              required
            />
          </label>
          <label className="field-control" htmlFor={`${formId}-email`}>
            E-mail de acesso
            <input
              id={`${formId}-email`}
              type="email"
              autoComplete="off"
              value={draft.email}
              maxLength={254}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  email: event.target.value,
                }))
              }
              required
            />
          </label>
        </div>
        <OperationStatus operation={operation} />
        <div className="button-row">
          <button
            type="submit"
            className="primary-action"
            disabled={operation.busy}
          >
            Emitir acesso
          </button>
        </div>
      </form>

      {activationUrl === null ? null : (
        <InviteQr
          url={activationUrl}
          presentation={activationPresentation}
          onDiscard={() => setActivationUrl(null)}
        />
      )}
    </section>
  );
}
