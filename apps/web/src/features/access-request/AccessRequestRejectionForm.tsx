import { useId, useState } from "react";

import type { AccessRequestRejectionReason } from "../../shared/api/invite-request-client";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import {
  rejectionReasonKeys,
  rejectionReasonOptions,
} from "./access-request-vocabulary";

interface AccessRequestRejectionFormProps {
  busy: boolean;
  onCancel: () => void;
  onConfirm: (reasonCode: AccessRequestRejectionReason) => void;
}

/**
 * Uma lista, nunca um campo livre: o motivo vai para trilha append-only, e
 * qualquer frase digitada sobre quem pediu viraria dado pessoal permanente
 * dentro do caminho que existe justamente para apagar dado pessoal (ADR-042).
 *
 * O aviso de que a recusa apaga o contato fica aqui, colado no botão que a
 * executa, e não no cabeçalho da fila: é neste instante que a pessoa decide.
 */
export function AccessRequestRejectionForm({
  busy,
  onCancel,
  onConfirm,
}: AccessRequestRejectionFormProps) {
  const { t } = useLocale();
  const fieldId = useId();
  const [reasonCode, setReasonCode] = useState<AccessRequestRejectionReason>(
    rejectionReasonOptions[0],
  );

  return (
    <div
      className="rejection-reason"
      role="group"
      aria-labelledby={`${fieldId}-title`}
    >
      <p id={`${fieldId}-title`} className="rejection-title">
        {t("accessRequest.rejection.title")}
      </p>
      <label className="field-control" htmlFor={fieldId}>
        {t("accessRequest.rejection.field")}
        <select
          id={fieldId}
          value={reasonCode}
          disabled={busy}
          aria-describedby={`${fieldId}-warning`}
          onChange={(event) =>
            setReasonCode(event.target.value as AccessRequestRejectionReason)
          }
        >
          {rejectionReasonOptions.map((option) => (
            <option key={option} value={option}>
              {t(rejectionReasonKeys[option])}
            </option>
          ))}
        </select>
      </label>
      <p className="rejection-hint" id={`${fieldId}-warning`}>
        {t("accessRequest.rejection.warning")}
      </p>
      <div className="button-row">
        <button
          type="button"
          className="quiet-action"
          disabled={busy}
          aria-describedby={`${fieldId}-warning`}
          onClick={() => onConfirm(reasonCode)}
        >
          {t("accessRequest.rejection.confirm")}
        </button>
        <button
          type="button"
          className="ghost-action"
          disabled={busy}
          onClick={onCancel}
        >
          {t("accessRequest.rejection.cancel")}
        </button>
      </div>
    </div>
  );
}
