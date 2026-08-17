import { Phase7ApiError } from "../../shared/api/phase7-client";
import {
  ProofOfWorkAbortedError,
  ProofOfWorkExhaustedError,
} from "../../shared/security/proof-of-work";

/**
 * The uniform 404 is deliberate: a bad token, a revoked poster and an expired
 * one are indistinguishable by design, so the message must not speculate about
 * which one happened.
 */
const statusMessages: Readonly<Record<number, string>> = {
  403: "Este cartaz não está aceitando cadastros agora.",
  404: "Este cartaz não é mais válido. Peça um novo à hospedagem.",
  409: "O aviso de privacidade mudou desde que o cartaz foi impresso. Peça um cartaz atualizado à hospedagem.",
  422: "Alguns dados não são aceitos neste formulário aberto. Revise e tente de novo.",
};

function localFailureMessage(error: unknown) {
  if (error instanceof ProofOfWorkExhaustedError) {
    return error.message;
  }
  if (error instanceof ProofOfWorkAbortedError) {
    return "A verificação foi interrompida. Envie novamente quando quiser.";
  }
  return "Sem conexão agora. O que você preencheu ficou guardado, cifrado, neste dispositivo.";
}

export function describeSelfServiceFailure(error: unknown) {
  if (!(error instanceof Phase7ApiError)) {
    return localFailureMessage(error);
  }
  if (error.retryAfterSeconds !== null) {
    return `${error.problem.title} Tente novamente em ${error.retryAfterSeconds} segundos.`;
  }
  return statusMessages[error.status] ?? error.problem.title;
}

/** A refusal that will never succeed on retry must not keep a local copy. */
export function shouldPurgeDraft(error: unknown) {
  if (!(error instanceof Phase7ApiError)) {
    return false;
  }
  return error.status === 404 || error.status === 403 || error.status === 422;
}
