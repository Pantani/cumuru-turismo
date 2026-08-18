import { useLocale } from "../../shared/i18n/LocaleProvider";
import type { MessageKey } from "../../shared/i18n/translate";
import { ImageSlot } from "./ImageSlot";
import { BEACH_PHOTO } from "./landing-content";
import { SectionIndex } from "./SectionIndex";

/** Fuso civil de toda a contagem de presença; nunca traduzido, é um identificador IANA. */
const TIME_ZONE = "America/Bahia";

const PLACE_FACTS: readonly [MessageKey, MessageKey | null][] = [
  ["landing.place.municipality", "landing.place.municipalityValue"],
  ["landing.place.timezone", null],
  ["landing.place.beach", "landing.place.beachValue"],
  ["landing.place.season", "landing.place.seasonValue"],
];

export function PlaceSection() {
  const { t } = useLocale();
  return (
    <section className="lp-section lp-section-deep" id="mapa">
      <div className="lp-shell lp-place">
        <div>
          <SectionIndex
            index="landing.place.index"
            kicker="landing.place.kicker"
          />
          <h2 className="lp-display lp-display-sm">
            {t("landing.place.title")}
          </h2>
          <p className="lp-lead">{t("landing.place.lead")}</p>
          <dl className="lp-fact-grid">
            {PLACE_FACTS.map(([label, value]) => (
              <div key={label}>
                <dt>{t(label)}</dt>
                <dd>{value === null ? TIME_ZONE : t(value)}</dd>
              </div>
            ))}
          </dl>
        </div>
        <div className="lp-place-map">
          <ImageSlot caption="landing.place.image" photo={BEACH_PHOTO} />
        </div>
      </div>
    </section>
  );
}
