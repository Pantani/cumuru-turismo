import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type PropsWithChildren,
} from "react";

import { localeTag, preferredLocale, type Locale } from "./locale";
import { createTranslate, type Translate } from "./translate";

interface LocaleContextValue {
  locale: Locale;
  setLocale: (next: Locale) => void;
  t: Translate;
  tag: string;
}

const LocaleContext = createContext<LocaleContextValue | null>(null);

/**
 * O idioma vive só na memória da aba, como o resto do estado do cliente: o
 * front do Observatório não escreve em `localStorage` nem em `sessionStorage`
 * (ADR-019). Cada carga negocia de novo pelo `Accept-Language` do navegador, e
 * a troca manual vale enquanto a aba estiver aberta.
 */
function initialLocale(): Locale {
  return preferredLocale(navigator.languages ?? []);
}

export function LocaleProvider({
  children,
  initial,
}: PropsWithChildren<{ initial?: Locale }>) {
  const [locale, setLocale] = useState<Locale>(() => initial ?? initialLocale());

  useEffect(() => {
    document.documentElement.lang = localeTag(locale);
  }, [locale]);

  const change = useCallback((next: Locale) => setLocale(next), []);

  const value = useMemo<LocaleContextValue>(
    () => ({
      locale,
      setLocale: change,
      t: createTranslate(locale),
      tag: localeTag(locale),
    }),
    [change, locale],
  );

  return (
    <LocaleContext.Provider value={value}>{children}</LocaleContext.Provider>
  );
}

export function useLocale(): LocaleContextValue {
  const value = useContext(LocaleContext);
  if (value === null) {
    throw new Error("useLocale exige que a árvore esteja sob LocaleProvider.");
  }
  return value;
}
