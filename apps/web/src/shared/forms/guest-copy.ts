/**
 * Cópia de erro para telas de hóspede.
 *
 * `problem.title` é escrito para quem **opera** o serviço. Numa tela de hóspede
 * ele vira jargão sem ação possível — "O serviço respondeu fora do contrato.",
 * "Requisição em processamento" — e por isso nunca é repassado daqui.
 *
 * A ordem importa e foi um defeito real (D-08): a versão anterior testava
 * `retryAfterSeconds` **antes** da tabela de status e devolvia o título do
 * servidor nesse ramo. Como `httpapi.go` responde `429` e `409` **com**
 * `Retry-After`, os dois desfechos mais comuns contornavam a tabela inteira e
 * entregavam texto de engenharia ao hóspede.
 *
 * Agora o texto sai sempre da tabela, e o `Retry-After` só acrescenta a parte
 * acionável — "tente novamente em 60 segundos" é informação útil e continua
 * aparecendo.
 */

export const GUEST_UNEXPECTED_FAILURE =
  "Não conseguimos falar com o serviço agora. Tente de novo em alguns instantes.";

export interface GuestFailure {
  retryAfterSeconds: number | null;
  status: number;
}

export function guestCopyFor(
  failure: GuestFailure,
  messages: Readonly<Record<number, string>>,
) {
  const message = messages[failure.status] ?? GUEST_UNEXPECTED_FAILURE;
  if (failure.retryAfterSeconds === null) {
    return message;
  }
  return `${message} Tente novamente em ${failure.retryAfterSeconds} segundos.`;
}
