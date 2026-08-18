import { useLocale } from "../../shared/i18n/LocaleProvider";
import { ImageSlot } from "./ImageSlot";
import { GATE_PHOTO, HOW_STEPS, SQUARE_PHOTO, STREET_PHOTO } from "./landing-content";
import { SectionIndex } from "./SectionIndex";

/** Número do passo com dois dígitos, como no material impresso da vila. */
function stepNumber(index: number) {
  return String(index + 1).padStart(2, "0");
}

export function HowItWorksSection() {
  const { t } = useLocale();
  return (
    <section className="lp-section lp-section-sand" id="como">
      <div className="lp-shell">
        <SectionIndex index="landing.how.index" kicker="landing.how.kicker" />
        <h2 className="lp-display">{t("landing.how.title")}</h2>
        <p className="lp-lead">{t("landing.how.lead")}</p>
        <ol className="lp-steps">
          {HOW_STEPS.map((step, index) => (
            <li key={step.title}>
              <p className="lp-step-number" aria-hidden="true">
                {stepNumber(index)}
              </p>
              <h3>{t(step.title)}</h3>
              <p>{t(step.body)}</p>
            </li>
          ))}
        </ol>
        <div className="lp-how-gallery">
          <ImageSlot caption="landing.how.image.square" photo={SQUARE_PHOTO} />
          <ImageSlot caption="landing.how.image.gate" photo={GATE_PHOTO} />
          <ImageSlot caption="landing.how.image.street" photo={STREET_PHOTO} />
        </div>
      </div>
    </section>
  );
}
