import { useLocale } from "../../shared/i18n/LocaleProvider";
import { SECTION_ANCHORS } from "./landing-content";

/**
 * Índice de seções da capa. Fica separado da navegação de rotas do topo: uma
 * âncora dentro da página e uma troca de rota não são a mesma promessa, e
 * misturá-las num mesmo `nav` faria o leitor de tela anunciar as duas iguais.
 */
export function SectionNav() {
  const { t } = useLocale();
  return (
    <nav className="lp-section-nav" aria-label={t("landing.sections.aria")}>
      <ul>
        {SECTION_ANCHORS.map((anchor) => (
          <li key={anchor.id}>
            <a href={`#${anchor.id}`}>{t(anchor.label)}</a>
          </li>
        ))}
      </ul>
      {/* "Cadastrar hospedagem" é promessa de cadastro; entrar fica no menu. */}
      <a className="lp-section-nav-cta" href="/convite">
        {t("landing.nav.register")}
      </a>
    </nav>
  );
}
