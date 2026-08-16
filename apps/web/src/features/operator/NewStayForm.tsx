import { useCallback, useId, useState, type FormEvent } from "react";

import { createUuidV7 } from "../../shared/identity/uuid-v7";
import {
  validateCreateStay,
  type ValidationIssue,
} from "../../shared/validation/phase2-validation";
import {
  FieldError,
  invalidFlag,
  issueMessage,
  OperationStatus,
} from "../../shared/forms/FieldFeedback";
import { useAuthSession } from "../../shared/auth/AuthSession";
import { defaultStayDates } from "./stay-lifecycle";
import { useOperation } from "./use-operation";

interface NewStayFormProps {
  accommodationId: string;
  onCreated: () => void;
}

function emptyDraft() {
  const dates = defaultStayDates();
  return { arrival: dates.arrival, departure: dates.departure, guests: "2" };
}

export function NewStayForm({ accommodationId, onCreated }: NewStayFormProps) {
  const { client } = useAuthSession();
  const operation = useOperation();
  const formId = useId();
  const [draft, setDraft] = useState(emptyDraft);
  const [issues, setIssues] = useState<readonly ValidationIssue[]>([]);

  const submit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      const body = {
        accommodation_id: accommodationId,
        planned_arrival_on: draft.arrival,
        planned_departure_on: draft.departure,
        expected_guest_count: Number(draft.guests),
        client_submission_id: createUuidV7(),
      };
      const found = validateCreateStay(body);
      setIssues(found);
      if (found.length > 0) {
        return;
      }
      const result = await operation.run("Criando a estadia", () =>
        client.createStay(body, crypto.randomUUID()),
      );
      if (result !== null) {
        setDraft(emptyDraft());
        onCreated();
      }
    },
    [accommodationId, client, draft, onCreated, operation],
  );

  return (
    <form className="new-stay-form" onSubmit={(event) => void submit(event)} noValidate>
      <h3>Nova estadia</h3>
      <div className="field-grid">
        <label className="field-control" htmlFor={`${formId}-arrival`}>
          Chegada
          <input
            id={`${formId}-arrival`}
            type="date"
            value={draft.arrival}
            onChange={(event) =>
              setDraft((current) => ({ ...current, arrival: event.target.value }))
            }
            required
          />
        </label>
        <label className="field-control" htmlFor={`${formId}-departure`}>
          Saída
          <input
            id={`${formId}-departure`}
            type="date"
            value={draft.departure}
            onChange={(event) =>
              setDraft((current) => ({ ...current, departure: event.target.value }))
            }
            aria-invalid={invalidFlag(issueMessage(issues, "planned_departure_on"))}
            required
          />
          <FieldError message={issueMessage(issues, "planned_departure_on")} />
        </label>
        <label className="field-control" htmlFor={`${formId}-guests`}>
          Quantas pessoas
          <input
            id={`${formId}-guests`}
            type="number"
            inputMode="numeric"
            min={1}
            max={100}
            value={draft.guests}
            onChange={(event) =>
              setDraft((current) => ({ ...current, guests: event.target.value }))
            }
            aria-invalid={invalidFlag(issueMessage(issues, "expected_guest_count"))}
            required
          />
          <FieldError message={issueMessage(issues, "expected_guest_count")} />
        </label>
      </div>

      <OperationStatus operation={operation} />

      <button className="primary-action" type="submit" disabled={operation.busy}>
        Criar estadia
      </button>
    </form>
  );
}
