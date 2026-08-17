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

const PROBLEM_BASE = "https://turismo.prado.ba.gov.br/problems/";

/** `httpapi.go:558-577`, o único `409` que não é conflito de estado. */
export const IDEMPOTENCY_IN_PROGRESS = `${PROBLEM_BASE}idempotency-in-progress`;

/**
 * Causa compartilhada pelas três telas: a mesma submissão já está sendo
 * processada e vai concluir. A frase mora aqui, e não em cada tabela, porque a
 * condição é do transporte e não do domínio de cada tela.
 */
const sharedTypeMessages: Readonly<Record<string, string>> = {
  [IDEMPOTENCY_IN_PROGRESS]: "Já recebemos este envio e estamos concluindo.",
};

export const GUEST_UNEXPECTED_FAILURE =
  "Não conseguimos falar com o serviço agora. Tente de novo em alguns instantes.";

export interface GuestFailure {
  problem: { type: string };
  retryAfterSeconds: number | null;
  status: number;
}

function messageFor(
  failure: GuestFailure,
  messages: Readonly<Record<number, string>>,
) {
  const byType = sharedTypeMessages[failure.problem.type];
  if (byType !== undefined) {
    return byType;
  }
  return messages[failure.status] ?? GUEST_UNEXPECTED_FAILURE;
}

/** `NewProcessingError` tem piso de um segundo, então o singular acontece. */
function retryPhrase(seconds: number) {
  return seconds === 1
    ? "Tente novamente em 1 segundo."
    : `Tente novamente em ${seconds} segundos.`;
}

export function guestCopyFor(
  failure: GuestFailure,
  messages: Readonly<Record<number, string>>,
) {
  const message = messageFor(failure, messages);
  if (failure.retryAfterSeconds === null) {
    return message;
  }
  return `${message} ${retryPhrase(failure.retryAfterSeconds)}`;
}
