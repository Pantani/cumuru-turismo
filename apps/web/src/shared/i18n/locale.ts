/**
 * Idiomas publicados pelo Observatório. O painel público e a landing falam a
 * mesma língua ao mesmo tempo: quem chega pelo QR de uma pousada pode ser um
 * visitante estrangeiro lendo o mesmo indicador que a associação de moradores.
 */
export const LOCALES = ["pt", "en", "es"] as const;

export type Locale = (typeof LOCALES)[number];

/** Tag BCP 47 usada em `Intl` e no atributo `lang` do documento. */
const LOCALE_TAGS: Record<Locale, string> = {
  pt: "pt-BR",
  en: "en-US",
  es: "es-ES",
};

export const DEFAULT_LOCALE: Locale = "pt";

export function localeTag(locale: Locale): string {
  return LOCALE_TAGS[locale];
}

export function isLocale(value: unknown): value is Locale {
  return LOCALES.includes(value as Locale);
}

/**
 * Primeiro idioma anunciado pelo navegador que o Observatório publica. A
 * ausência de correspondência mantém o português: a vila é o público primário.
 */
export function preferredLocale(languages: readonly string[]): Locale {
  for (const language of languages) {
    const base = language.toLowerCase().split("-")[0];
    if (isLocale(base)) {
      return base;
    }
  }
  return DEFAULT_LOCALE;
}
