import { useCallback, useState, type FormEvent } from "react";

import type { components } from "../../generated/schema";
import { createUuidV7 } from "../../shared/identity/uuid-v7";
import {
  validateSubmitGroup,
  type ValidationIssue,
} from "../../shared/validation/phase2-validation";
import { useAuthSession } from "../../shared/auth/AuthSession";
import { createVisitor, VisitorEditor } from "../visitors/VisitorEditor";
import { entityTagFor, formatCivilDate, type Stay } from "./stay-lifecycle";
import { PRIVACY_NOTICE_VERSION } from "./stay-commands";
import { OperationStatus } from "../../shared/forms/FieldFeedback";
import { useOperation } from "./use-operation";

type VisitorInput = components["schemas"]["VisitorInput"];

interface VisitorGroupPanelProps {
  onClose: () => void;
  onSubmitted: () => void;
  stay: Stay;
}

function initialVisitors(): VisitorInput[] {
  return [createVisitor("responsible")];
}

export function VisitorGroupPanel({
  onClose,
  onSubmitted,
  stay,
}: VisitorGroupPanelProps) {
  const { client } = useAuthSession();
  const operation = useOperation();
  const [visitors, setVisitors] = useState<VisitorInput[]>(initialVisitors);
  const [issues, setIssues] = useState<ValidationIssue[]>([]);

  const submit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      const body = {
        client_submission_id: createUuidV7(),
        privacy_notice_version: PRIVACY_NOTICE_VERSION,
        visitors,
      };
      const found = validateSubmitGroup(body);
      setIssues(found);
      if (found.length > 0) {
        return;
      }
      const result = await operation.run("Registrando os visitantes", () =>
        client.submitAssistedStayGroup(
          stay.id,
          body,
          entityTagFor(stay.version),
          crypto.randomUUID(),
        ),
      );
      if (result !== null) {
        onSubmitted();
      }
    },
    [client, onSubmitted, operation, stay, visitors],
  );

  return (
    <form className="panel-card" onSubmit={(event) => void submit(event)} noValidate>
      <h3>
        Visitantes da estadia de {formatCivilDate(stay.planned_arrival_on)}
      </h3>
      <p className="disclaimer">
        Registre só faixa etária e residência generalizada. Não escreva nome,
        documento, telefone ou qualquer dado que identifique a pessoa.
      </p>

      <VisitorEditor
        disabled={operation.busy}
        issues={issues}
        onChange={setVisitors}
        visitors={visitors}
      />

      <div className="button-row">
        <button
          type="button"
          className="ghost-action"
          disabled={operation.busy}
          onClick={() => setVisitors((current) => [...current, createVisitor()])}
        >
          Adicionar visitante
        </button>
      </div>

      <OperationStatus operation={operation} />

      <div className="button-row">
        <button className="primary-action" type="submit" disabled={operation.busy}>
          Salvar visitantes
        </button>
        <button type="button" className="ghost-action" onClick={onClose}>
          Fechar
        </button>
      </div>
    </form>
  );
}
