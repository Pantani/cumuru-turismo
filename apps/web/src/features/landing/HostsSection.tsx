import { useLocale } from "../../shared/i18n/LocaleProvider";
import { ImageSlot } from "./ImageSlot";
import { CONTACT_EMAIL, HOST_BENEFITS } from "./landing-content";
import { SectionIndex } from "./SectionIndex";

/**
 * O cartão abre conversa com a equipe em vez de mandar para `/acesso`: hoje não
 * existe autocadastro de hospedagem — a conta nasce de um link de ativação que
 * a equipe emite —, então um botão para a tela de login deixaria quem ainda não
 * tem conta num beco sem saída. O link de entrar fica embaixo, para quem já tem.
 */
function RegisterCard() {
  const { t } = useLocale();
  return (
    <div className="lp-register" id="cadastro">
      <h3>{t("landing.register.title")}</h3>
      <p>{t("landing.register.body")}</p>
      <a className="lp-button-dark" href={`mailto:${CONTACT_EMAIL}`}>
        {t("landing.register.action")}
      </a>
      <p className="lp-register-footnote">
        <a href="/acesso">{t("landing.register.signIn")}</a>
      </p>
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
