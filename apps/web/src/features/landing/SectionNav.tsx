import { useLocale } from "../../shared/i18n/LocaleProvider";
import { SECTION_ANCHORS } from "./landing-content";
import { useStickyNavHeight } from "./sticky-nav-height";

/**
 * Índice de seções da capa. Fica separado da navegação de rotas do topo: uma
 * âncora dentro da página e uma troca de rota não são a mesma promessa, e
 * misturá-las num mesmo `nav` faria o leitor de tela anunciar as duas iguais.
 */
export function SectionNav() {
  const { t } = useLocale();
  const navRef = useStickyNavHeight();
  return (
    <nav
      className="lp-section-nav"
      ref={navRef}
      aria-label={t("landing.sections.aria")}
    >
      <ul>
        {SECTION_ANCHORS.map((anchor) => (
          <li key={anchor.id}>
            <a href={`#${anchor.id}`}>{t(anchor.label)}</a>
          </li>
        ))}
      </ul>
      {/* Mesma âncora da capa: o cartão de cadastro é a porta para `/convite`. */}
      <a className="lp-section-nav-cta" href="#cadastro">
        {t("landing.nav.register")}
      </a>
    </nav>
  );
}
