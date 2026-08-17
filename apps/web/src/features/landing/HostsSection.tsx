import { useLocale } from "../../shared/i18n/LocaleProvider";
import { ImageSlot } from "./ImageSlot";
import { HOST_BENEFITS } from "./landing-content";
import { SectionIndex } from "./SectionIndex";

/**
 * O cartão de cadastro leva à área da hospedagem em vez de repetir um
 * formulário próprio: o cadastro real acontece lá, com validação e trilha de
 * auditoria. Um formulário de capa que não escreve em lugar nenhum prometeria
 * um envio que não existe.
 */
function RegisterCard() {
  const { t } = useLocale();
  return (
    <div className="lp-register" id="cadastro">
      <h3>{t("landing.register.title")}</h3>
      <p>{t("landing.register.body")}</p>
      <a className="lp-button-dark" href="/acesso">
        {t("landing.register.action")}
      </a>
      <p className="lp-register-footnote">{t("landing.register.footnote")}</p>
    </div>
  );
}

export function HostsSection() {
  const { t } = useLocale();
  return (
    <section className="lp-section lp-section-clay" id="anfitrioes">
      <div className="lp-shell lp-hosts">
        <div>
          <SectionIndex
            index="landing.hosts.index"
            kicker="landing.hosts.kicker"
          />
          <h2 className="lp-display">{t("landing.hosts.title")}</h2>
          <p className="lp-lead">{t("landing.hosts.lead")}</p>
          <ul className="lp-benefits">
            {HOST_BENEFITS.map((benefit) => (
              <li key={benefit.title}>
                <span className="lp-benefit-arrow" aria-hidden="true">
                  →
                </span>
                <span>
                  <strong>{t(benefit.title)}</strong>
                  <span>{t(benefit.body)}</span>
                </span>
              </li>
            ))}
          </ul>
          <figure className="lp-quote">
            <blockquote>{t("landing.hosts.quote")}</blockquote>
            <figcaption>{t("landing.hosts.quoteCaption")}</figcaption>
          </figure>
        </div>
        <div className="lp-hosts-aside">
          <div className="lp-hosts-portrait">
            <ImageSlot caption="landing.hosts.image" />
          </div>
          <RegisterCard />
        </div>
      </div>
    </section>
  );
}
