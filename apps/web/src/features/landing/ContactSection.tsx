import { useLocale } from "../../shared/i18n/LocaleProvider";
import { CONTACT_EMAIL, DPO_EMAIL } from "./landing-content";

export function ContactSection() {
  const { t } = useLocale();
  const mailto = `mailto:${CONTACT_EMAIL}`;
  return (
    <section className="lp-section lp-section-contact" id="contato">
      <div className="lp-shell lp-contact">
        <div>
          <h2 className="lp-display lp-display-sm">
            {t("landing.contact.title")}
          </h2>
          <p className="lp-lead">{t("landing.contact.lead")}</p>
          <div className="lp-hero-actions">
            <a className="lp-button-primary" href={mailto}>
              {t("landing.contact.write")}
            </a>
            <a className="lp-button-ghost" href={mailto}>
              {t("landing.contact.visit")}
            </a>
          </div>
        </div>
        <dl className="lp-contact-list">
          <div>
            <dt>{t("landing.contact.email")}</dt>
            <dd>
              <a href={mailto}>{CONTACT_EMAIL}</a>
            </dd>
          </div>
          <div>
            <dt>{t("landing.contact.inPerson")}</dt>
            <dd>{t("landing.contact.inPersonValue")}</dd>
          </div>
          <div>
            <dt>{t("landing.contact.dpo")}</dt>
            <dd>
              <a href={`mailto:${DPO_EMAIL}`}>{DPO_EMAIL}</a>
            </dd>
          </div>
        </dl>
      </div>
    </section>
  );
}
