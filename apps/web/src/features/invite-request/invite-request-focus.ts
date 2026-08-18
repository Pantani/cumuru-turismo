import { useEffect, useState } from "react";

import {
  fieldElementId,
  type InviteRequestIssue,
} from "./invite-request-form";
import { isDuplicateRequest } from "./invite-request-messages";

/** Região viva do formulário; existe desde o primeiro render para poder falar. */
export const INVITE_REQUEST_STATUS_ID = "invite-request-status";

/** WCAG 2.2: a recusa precisa levar o cursor até o campo que a causou. */
export function issueFocusTarget(issue: InviteRequestIssue | undefined) {
  return issue === undefined ? null : fieldElementId(issue.field);
}

/**
 * O 409 fala do e-mail — já existe pedido pendente para aquele endereço —, então
 * o foco vai para o campo que a pessoa pode mudar. Qualquer outra falha é do
 * envio como um todo e o foco vai para o aviso, que é onde está a explicação.
 */
export function failureFocusTarget(error: unknown) {
  return isDuplicateRequest(error)
    ? fieldElementId("contact_email")
    : INVITE_REQUEST_STATUS_ID;
}

/**
 * O foco só é movido depois que o render que reabilita os campos foi confirmado.
 * Chamado direto do `catch`, ele tentaria focar um `input` ainda `disabled` pelo
 * estado de envio — o navegador ignora, e o cursor fica onde estava, sem chegar
 * à explicação da recusa.
 */
export function useDeferredFocus() {
  const [target, requestFocus] = useState<string | null>(null);

  useEffect(() => {
    if (target === null) {
      return;
    }
    document.getElementById(target)?.focus();
    requestFocus(null);
  }, [target]);

  return requestFocus;
}
