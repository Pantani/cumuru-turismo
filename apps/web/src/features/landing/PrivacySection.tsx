import { useLocale } from "../../shared/i18n/LocaleProvider";
import { PRIVACY_ITEMS } from "./landing-content";
import { SectionIndex } from "./SectionIndex";

export function PrivacySection() {
  const { t } = useLocale();
  return (
    <section className="lp-section lp-section-sand" id="privacidade">
      <div className="lp-shell">
        <SectionIndex
          index="landing.privacy.index"
          kicker="landing.privacy.kicker"
        />
        <h2 className="lp-display">
          {t("landing.privacy.titleLead")}{" "}
          <span className="lp-accent">{t("landing.privacy.titleAccent")}</span>
        </h2>
        <p className="lp-lead">{t("landing.privacy.lead")}</p>
        <ul className="lp-rule-grid lp-rule-grid-list">
          {PRIVACY_ITEMS.map((item) => (
            <li key={item.title}>
              <strong>{t(item.title)}</strong>
              <span>{t(item.body)}</span>
            </li>
          ))}
        </ul>
        <aside className="lp-prototype-note">
          <h3>{t("landing.privacy.prototypeTitle")}</h3>
          <p>{t("landing.privacy.prototypeBody")}</p>
        </aside>
      </div>
    </section>
  );
}
