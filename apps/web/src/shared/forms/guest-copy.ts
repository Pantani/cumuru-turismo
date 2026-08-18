/**
 * Cópia de erro para telas de hóspede.
 *
 * `problem.title` é escrito para quem **opera** o serviço. Numa tela de hóspede
 * ele vira jargão sem ação possível — "O serviço respondeu fora do contrato.",
 * "Requisição em processamento" — e por isso nunca é repassado daqui.
 *
 * Duas correções anteriores moram nesta função, e a segunda existiu porque a
 * primeira foi longe demais:
 *
 * **D-08.** A versão original testava `retryAfterSeconds` antes da tabela e
 * devolvia o título do servidor nesse ramo. Como `httpapi.go` responde `429` e
 * `409` **com** `Retry-After`, os dois desfechos mais comuns contornavam a
 * tabela inteira. Hoje o texto sai sempre da tabela e o prazo só se acrescenta.
 *
 * **D-16.** Corrigido aquilo, o status virou o único critério — e `409` tem
 * **duas** causas distintas: aviso de privacidade desatualizado e submissão
 * idempotente em andamento. As duas colapsaram na primeira frase, então um
 * envio que ia concluir sozinho mandava o hóspede pedir um cartaz novo. Era
 * jargão verdadeiro antes e português limpo e falso depois; desinformação é
 * pior que jargão. Por isso `problem.type` decide **antes** do status.
 */

import type { Translate } from "../i18n/translate";

const PROBLEM_BASE = "https://turismo.prado.ba.gov.br/problems/";

/** `httpapi.go:558-577`, o único `409` que não é conflito de estado. */
export const IDEMPOTENCY_IN_PROGRESS = `${PROBLEM_BASE}idempotency-in-progress`;

export interface GuestFailure {
  problem: { type: string };
  retryAfterSeconds: number | null;
  status: number;
}

function messageFor(
  t: Translate,
  failure: GuestFailure,
  messages: Readonly<Record<number, string>>,
) {
  // Causa compartilhada pelas três telas: a mesma submissão já está sendo
  // processada e vai concluir. A frase mora aqui, e não em cada tabela, porque
  // a condição é do transporte e não do domínio de cada tela.
  if (failure.problem.type === IDEMPOTENCY_IN_PROGRESS) {
    return t("guestCopy.idempotencyInProgress");
  }
  return messages[failure.status] ?? t("guestCopy.unexpectedFailure");
}

/** `NewProcessingError` tem piso de um segundo, então o singular acontece. */
function retryPhrase(t: Translate, seconds: number) {
  return seconds === 1
    ? t("guestCopy.retrySeconds.one")
    : t("guestCopy.retrySeconds.other", { seconds });
}

export function guestCopyFor(
  t: Translate,
  failure: GuestFailure,
  messages: Readonly<Record<number, string>>,
) {
  const message = messageFor(t, failure, messages);
  if (failure.retryAfterSeconds === null) {
    return message;
  }
  return `${message} ${retryPhrase(t, failure.retryAfterSeconds)}`;
}
