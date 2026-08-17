import { useId, useState } from "react";

import type { components } from "../../generated/schema";
import {
  rejectionReasonLabels,
  rejectionReasonOptions,
} from "./approval-vocabulary";

type RejectionReasonCode =
  components["schemas"]["RejectStayRequest"]["reason_code"];

interface RejectionReasonFormProps {
  busy: boolean;
  onCancel: () => void;
  onConfirm: (reasonCode: RejectionReasonCode) => void;
}

/**
 * A select, never a text field. The operator cannot type anything about the
 * person being refused, which is the whole point of the closed list.
 */
export function RejectionReasonForm({
  busy,
  onCancel,
  onConfirm,
}: RejectionReasonFormProps) {
  const fieldId = useId();
  const [reasonCode, setReasonCode] = useState<RejectionReasonCode>(
    rejectionReasonOptions[0],
  );

  return (
    <div className="rejection-reason" role="group" aria-labelledby={`${fieldId}-title`}>
      <p id={`${fieldId}-title`} className="rejection-title">
        Motivo da recusa
      </p>
      <label className="field-control" htmlFor={fieldId}>
        Escolha o motivo
        <select
          id={fieldId}
          value={reasonCode}
          disabled={busy}
          onChange={(event) =>
            setReasonCode(event.target.value as RejectionReasonCode)
          }
        >
          {rejectionReasonOptions.map((option) => (
            <option key={option} value={option}>
              {rejectionReasonLabels[option]}
            </option>
          ))}
        </select>
      </label>
      <p className="rejection-hint">
        A recusa elimina os dados enviados e guarda apenas o registro auditável
        da decisão. Não há campo livre: nada que você escrevesse poderia ser
        corrigido depois.
      </p>
      <div className="button-row">
        <button
          type="button"
          className="quiet-action"
          disabled={busy}
          onClick={() => onConfirm(reasonCode)}
        >
          Confirmar recusa
        </button>
        <button
          type="button"
          className="ghost-action"
          disabled={busy}
          onClick={onCancel}
        >
          Voltar
        </button>
      </div>
    </div>
  );
}
