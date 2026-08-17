import { useLocale } from "../../shared/i18n/LocaleProvider";
import { COMMERCE_ITEMS } from "./landing-content";
import { SectionIndex } from "./SectionIndex";

export function CommerceSection() {
  const { t } = useLocale();
  return (
    <section className="lp-section lp-section-coral" id="comercio">
      <div className="lp-shell">
        <SectionIndex
          index="landing.commerce.index"
          kicker="landing.commerce.kicker"
        />
        <h2 className="lp-display">{t("landing.commerce.title")}</h2>
        <p className="lp-lead">{t("landing.commerce.lead")}</p>
        <div className="lp-rule-grid">
          {COMMERCE_ITEMS.map((item) => (
            <article key={item.title}>
              <h3>{t(item.title)}</h3>
              <p>{t(item.body)}</p>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}
