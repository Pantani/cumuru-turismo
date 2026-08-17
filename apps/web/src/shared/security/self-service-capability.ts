import { notifyCapabilityChange } from "./capability-events";
import { createFragmentCapability } from "./fragment-capability";

/** Poster token of the reusable accommodation invite (ADR-039), read from `/i#<token>`. */
const capability = createFragmentCapability("/i");

/**
 * "Tenho um token" e "concluí com este token" são fatos diferentes, e confundir
 * os dois custou uma regressão: a ADR-019 exige que o cartaz não sobreviva ao
 * sucesso, mas `App` se redesenha quando uma capability muda, e a página, ao
 * reavaliar só o token, trocava o formulário pela tela "Cartaz necessário" —
 * destruindo a confirmação junto.
 *
 * Este sinal sobrevive ao descarte do token e **não é um token**: é um booleano
 * de aba, sem segredo, sem identificador e sem nada que viaje para lugar algum.
 */
let selfRegistrationCompleted = false;

export function captureSelfServiceCapability(
  url: URL,
  replace: (path: string) => void,
) {
  const captured = capability.capture(url, replace);
  if (captured) {
    // Um cartaz novo começa uma submissão nova; a conclusão anterior não pode
    // esconder o formulário de quem acabou de ler outro QR.
    selfRegistrationCompleted = false;
  }
  return captured;
}

/**
 * O sinal morre com a capability que o originou. Sem isto, descartar o token
 * deixava a aba afirmando uma conclusão que ela já não podia comprovar, e um
 * `/i` aberto sem fragmento mostrava confirmação obsoleta.
 */
export function clearSelfServiceCapability() {
  selfRegistrationCompleted = false;
  capability.forget();
  notifyCapabilityChange();
}

/**
 * Transição atômica do sucesso: o cartaz é consumido — a ADR-019 exige que ele
 * não sobreviva — e a conclusão passa a valer, com **uma** notificação. Fazer
 * as duas coisas em chamadas separadas obrigaria a confiar no lote de render do
 * React para que a tela não piscasse "Cartaz necessário" entre elas.
 */
export function completeSelfRegistration() {
  capability.forget();
  selfRegistrationCompleted = true;
  notifyCapabilityChange();
}

export const peekSelfServiceCapability = capability.peek;

export function peekSelfRegistrationCompleted() {
  return selfRegistrationCompleted;
}
