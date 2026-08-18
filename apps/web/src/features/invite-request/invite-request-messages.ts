import { ApiError } from "../../shared/api/http-client";
import { guestCopyFor } from "../../shared/forms/guest-copy";
import type { Translate } from "../../shared/i18n/translate";
import {
  ProofOfWorkAbortedError,
  ProofOfWorkExhaustedError,
} from "../../shared/security/proof-of-work";

/**
 * Quem preenche este formulário não opera o serviço, então `problem.title` nunca
 * chega à tela: vale a mesma regra do canal aberto do hóspede.
 *
 * O 409 é o único desfecho que precisa de frase própria. Ele não é erro de
 * digitação nem falha de rede: já existe um pedido pendente para aquele e-mail, e
 * mandar "tente de novo" faria a pessoa reenviar um formulário que vai ser
 * recusado de novo. A ação certa é esperar a análise ou usar outro e-mail.
 */
function statusMessages(t: Translate): Readonly<Record<number, string>> {
  return {
    409: t("inviteRequest.error.duplicate"),
    422: t("inviteRequest.error.unprocessable"),
    429: t("inviteRequest.error.rateLimited"),
  };
}

function localFailureMessage(t: Translate, error: unknown) {
  if (error instanceof ProofOfWorkExhaustedError) {
    return error.message;
  }
  if (error instanceof ProofOfWorkAbortedError) {
    return t("inviteRequest.error.proofOfWorkAborted");
  }
  return t("inviteRequest.error.offline");
}

export function describeInviteRequestFailure(t: Translate, error: unknown) {
  if (!(error instanceof ApiError)) {
    return localFailureMessage(t, error);
  }
  return guestCopyFor(t, error, statusMessages(t));
}

/** O 409 fala do e-mail, então o foco tem de voltar para o campo de e-mail. */
export function isDuplicateRequest(error: unknown) {
  return error instanceof ApiError && error.status === 409;
}
