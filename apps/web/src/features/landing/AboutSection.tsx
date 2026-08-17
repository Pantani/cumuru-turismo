import { useLocale } from "../../shared/i18n/LocaleProvider";
import { FAQ_ENTRIES, GUIDE_LINKS } from "./landing-content";
import { SectionIndex } from "./SectionIndex";

function GuideList() {
  const { t } = useLocale();
  return (
    <div className="lp-guides">
      {GUIDE_LINKS.map((guide) => (
        <a download href={guide.href} key={guide.href}>
          <span>
            <strong>{t(guide.title)}</strong>
            <span>{t(guide.meta)}</span>
          </span>
          <span className="lp-guide-arrow" aria-hidden="true">
            ↓
          </span>
        </a>
      ))}
    </div>
  );
}

function Faq() {
  const { t } = useLocale();
  return (
    <div>
      <h3 className="lp-display lp-display-sm">{t("landing.faq.title")}</h3>
      <div className="lp-faq">
        {FAQ_ENTRIES.map((entry) => (
          <details key={entry.question}>
            <summary>{t(entry.question)}</summary>
            <p>{t(entry.answer)}</p>
          </details>
        ))}
      </div>
    </div>
  );
}

export function AboutSection() {
  const { t } = useLocale();
  return (
    <section className="lp-section lp-section-deep" id="sobre">
      <div className="lp-shell lp-about">
        <div>
          <SectionIndex
            index="landing.about.index"
            kicker="landing.about.kicker"
          />
          <h2 className="lp-display lp-display-sm">
            {t("landing.about.title")}
          </h2>
          <p className="lp-lead">{t("landing.about.body1")}</p>
          <p className="lp-lead">{t("landing.about.body2")}</p>
          <GuideList />
        </div>
        <Faq />
      </div>
    </section>
  );
}
