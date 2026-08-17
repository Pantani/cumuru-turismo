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

const PLACEHOLDER = /\{(\w+)\}/g;

/**
 * Substitui `{nome}` pelos parâmetros. Um placeholder sem valor é defeito de
 * chamada, não de conteúdo: lançar aqui o expõe no teste em vez de publicar
 * `{count}` cru para o leitor.
 */
export function interpolate(template: string, params?: MessageParams): string {
  return template.replace(PLACEHOLDER, (_match, name: string) => {
    const value = params?.[name];
    if (value === undefined) {
      throw new Error(`Parâmetro "${name}" ausente na mensagem "${template}".`);
    }
    return String(value);
  });
}

export function messagesFor(locale: Locale): Messages {
  return CATALOG[locale];
}

export function createTranslate(locale: Locale): Translate {
  const messages = messagesFor(locale);
  return (key, params) => interpolate(messages[key], params);
}
