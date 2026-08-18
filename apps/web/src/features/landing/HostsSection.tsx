import { useLocale } from "../../shared/i18n/LocaleProvider";
import { ImageSlot } from "./ImageSlot";
import { HAMMOCK_PHOTO, HOST_BENEFITS } from "./landing-content";
import { SectionIndex } from "./SectionIndex";

/**
 * O cartão leva a `/convite`, que é onde o pedido de acesso passa a ser escrito
 * de fato. Ele já apontou para `/acesso`, a tela de login, e depois para um
 * `mailto:` da equipe: os dois eram contorno para a mesma ausência, porque não
 * existia autocadastro de hospedagem e quem chega pela capa ainda não tem conta.
 * Agora existe — `/convite` grava o pedido na fila de aprovação (ADR-042), e a
 * conta continua nascendo do link de ativação que a administração emite depois
 * da análise. O link de entrar fica embaixo, para quem já tem conta.
 */
function RegisterCard() {
  const { t } = useLocale();
  return (
    <div className="lp-register" id="cadastro">
      <h3>{t("landing.register.title")}</h3>
      <p>{t("landing.register.body")}</p>
      <a className="lp-button-dark" href="/convite">
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
            <ImageSlot caption="landing.hosts.image" photo={HAMMOCK_PHOTO} />
          </div>
          <RegisterCard />
        </div>
      </div>
    </section>
  );
}
