import { useLocale } from "./LocaleProvider";
import { LOCALES, type Locale } from "./locale";
import type { MessageKey } from "./translate";

/**
 * Sigla no botão e nome por extenso no rótulo acessível. O nome acessível
 * precisa conter o rótulo visível (WCAG 2.2 SC 2.5.3 Label in Name): quem usa
 * comando de voz diz "clicar PT", então a sigla não pode ser substituída pelo
 * nome do idioma — ela vem junto.
 */
const SHORT_KEYS: Record<Locale, MessageKey> = {
  pt: "app.locale.pt",
  en: "app.locale.en",
  es: "app.locale.es",
};

const NAME_KEYS: Record<Locale, MessageKey> = {
  pt: "app.locale.ptName",
  en: "app.locale.enName",
  es: "app.locale.esName",
};

export function LocaleSwitcher() {
  const { locale, setLocale, t } = useLocale();
  return (
    <div className="locale-switcher" role="group" aria-label={t("app.locale.aria")}>
      {LOCALES.map((code) => (
        <button
          aria-current={code === locale ? "true" : undefined}
          aria-label={`${t(SHORT_KEYS[code])} · ${t(NAME_KEYS[code])}`}
          key={code}
          lang={code}
          onClick={() => setLocale(code)}
          type="button"
        >
          {t(SHORT_KEYS[code])}
        </button>
      ))}
    </div>
  );
}
