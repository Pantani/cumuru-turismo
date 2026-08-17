import { type Locale } from "./locale";
import { messagesEn } from "./messages-en";
import { messagesEs } from "./messages-es";
import { messagesPt, type MessageKey, type Messages } from "./messages-pt";

export type { MessageKey, Messages };

export type MessageParams = Readonly<Record<string, string | number>>;

export type Translate = (key: MessageKey, params?: MessageParams) => string;

const CATALOG: Record<Locale, Messages> = {
  pt: messagesPt,
  en: messagesEn,
  es: messagesEs,
};

/**
 * Gramática de placeholder. Exportada porque o teste de paridade compara os
 * parâmetros das três traduções: se ela ficasse duplicada lá, mudar o parser
 * aqui deixaria o teste validando uma gramática que não existe mais.
 */
export const PLACEHOLDER = /\{(\w+)\}/g;

export function placeholdersOf(template: string): string[] {
  return [...template.matchAll(PLACEHOLDER)]
    .map((match) => match[1] ?? "")
    .sort();
}

function reportMissing(name: string, key: string): void {
  const message = `Parâmetro "${name}" ausente na mensagem "${key}".`;
  // Em desenvolvimento e em teste o defeito precisa parar a execução. Em
  // produção `t` roda dentro do render: lançar derrubaria a subárvore inteira e
  // o leitor veria página em branco no lugar de uma frase malformada.
  if (import.meta.env.DEV) {
    throw new Error(message);
  }
  console.error(message);
}

/** Substitui `{nome}` pelos parâmetros informados. */
export function interpolate(
  template: string,
  params?: MessageParams,
  key: string = template,
): string {
  return template.replace(PLACEHOLDER, (match, name: string) => {
    const value = params?.[name];
    if (value === undefined) {
      reportMissing(name, key);
      return match;
    }
    return String(value);
  });
}

export function messagesFor(locale: Locale): Messages {
  return CATALOG[locale];
}

export function createTranslate(locale: Locale): Translate {
  const messages = messagesFor(locale);
  return (key, params) => interpolate(messages[key], params, key);
}
