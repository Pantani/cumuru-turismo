import { useCallback, useId, useState, type FormEvent } from "react";

import { createUuidV7 } from "../../shared/identity/uuid-v7";
import {
  validateCreateAccommodation,
  type ValidationIssue,
} from "../../shared/validation/core-validation";
import {
  FieldError,
  invalidFlag,
  issueMessage,
  OperationStatus,
} from "../../shared/forms/FieldFeedback";
import { useAuthSession } from "../../shared/auth/AuthSession";
import {
  accommodationCategoryLabels,
  accommodationCategoryOptions,
  type Accommodation,
  type AccommodationInputCategory,
} from "./stay-lifecycle";
import { useOperation } from "./use-operation";

interface AccommodationOnboardingProps {
  onCancel: () => void;
  onCreated: (accommodation: Accommodation) => void;
}

interface DraftAccommodation {
  name: string;
  category: AccommodationInputCategory;
  capacity: string;
}

const emptyDraft: DraftAccommodation = {
  name: "",
  category: "seasonal_rental",
  capacity: "4",
};

export function AccommodationOnboarding({
  onCancel,
  onCreated,
}: AccommodationOnboardingProps) {
  const { coreClient: client } = useAuthSession();
  const operation = useOperation();
  const formId = useId();
  const [draft, setDraft] = useState(emptyDraft);
  const [issues, setIssues] = useState<readonly ValidationIssue[]>([]);

  const submit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      const body = {
        name: draft.name.trim(),
        category: draft.category,
        capacity: Number(draft.capacity),
        client_submission_id: createUuidV7(),
      };
      const found = validateCreateAccommodation(body);
      setIssues(found);
      if (found.length > 0) {
        return;
      }
      const result = await operation.run("Cadastrando a hospedagem", () =>
        client.createAccommodation(body, crypto.randomUUID()),
      );
      if (result !== null) {
        onCreated(result.data);
      }
    },
    [client, draft, onCreated, operation],
  );

  return (
    <form className="onboarding-card" onSubmit={(event) => void submit(event)} noValidate>
      <h3>Cadastrar hospedagem</h3>
      <p className="onboarding-intro">
        Vale para pousada, casa de temporada, quarto na sua casa, camping ou
        local em regularização. Não pedimos CPF, CNPJ, Cadastur nem chave da
        FNRH.
      </p>

      <label className="field-control" htmlFor={`${formId}-name`}>
        Como o local é conhecido
        <input
          id={`${formId}-name`}
          value={draft.name}
          maxLength={200}
          onChange={(event) =>
            setDraft((current) => ({ ...current, name: event.target.value }))
          }
          aria-invalid={invalidFlag(issueMessage(issues, "name"))}
          required
        />
        <FieldError message={issueMessage(issues, "name")} />
      </label>

      <div className="field-grid">
        <label className="field-control" htmlFor={`${formId}-category`}>
          Tipo de hospedagem
          <select
            id={`${formId}-category`}
            value={draft.category}
            onChange={(event) =>
              setDraft((current) => ({
                ...current,
                category: event.target.value as AccommodationInputCategory,
              }))
            }
          >
            {accommodationCategoryOptions.map((category) => (
              <option key={category} value={category}>
                {accommodationCategoryLabels[category]}
              </option>
            ))}
          </select>
        </label>

        <label className="field-control" htmlFor={`${formId}-capacity`}>
          Quantas pessoas cabem
          <input
            id={`${formId}-capacity`}
            type="number"
            inputMode="numeric"
            min={1}
            max={10000}
            value={draft.capacity}
            onChange={(event) =>
              setDraft((current) => ({ ...current, capacity: event.target.value }))
            }
            aria-invalid={invalidFlag(issueMessage(issues, "capacity"))}
            required
          />
          <FieldError message={issueMessage(issues, "capacity")} />
        </label>
      </div>

      <p className="disclaimer">
        O tipo informado é só uma classificação operacional do observatório. Ele
        não comprova regularização, não vale como parecer jurídico e não aciona
        nenhuma integração federal.
      </p>

      <OperationStatus operation={operation} />

      <div className="button-row">
        <button className="primary-action" type="submit" disabled={operation.busy}>
          Cadastrar
        </button>
        <button type="button" className="ghost-action" onClick={onCancel}>
          Cancelar
        </button>
      </div>
    </form>
  );
}
