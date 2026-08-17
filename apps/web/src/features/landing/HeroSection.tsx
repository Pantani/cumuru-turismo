import type { AnalyticsClient } from "../../shared/api/analytics-client";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import { usePresenceFormat } from "../analytics/presence-format";
import { usePublicSummary } from "../analytics/public-summary";
import { ImageSlot } from "./ImageSlot";
import { HERO_PHOTO } from "./landing-content";
import { todayHeadline } from "./summary-values";

/**
 * Capa. O número grande é o mesmo `presence_today` que o painel publica logo
 * abaixo — vem do contrato público, nunca de constante no front, e some com a
 * mesma supressão estatística quando a célula não é publicável.
 */
export function HeroSection({ client }: { client?: AnalyticsClient }) {
  const { t } = useLocale();
  const format = usePresenceFormat();
  const summary = usePublicSummary(client);
  const today = todayHeadline(t, format, summary.data?.data.presence_today);

  return (
    <section className="lp-hero" id="topo">
      <div className="lp-hero-art">
        <ImageSlot caption="landing.hero.image" photo={HERO_PHOTO} />
      </div>
      <div className="lp-hero-veil" aria-hidden="true" />
      <div className="lp-hero-body">
        <p className="lp-kicker">
          <span className="lp-kicker-rule" aria-hidden="true" />
          {t("landing.hero.kicker")}
        </p>
        <h1 className="lp-hero-title" data-route-heading tabIndex={-1}>
          {t("landing.hero.titleLead")}{" "}
          <span className="lp-accent">{t("landing.hero.titleAccent")}</span>
        </h1>
        <p className="lp-hero-lead">{t("landing.hero.lead")}</p>
        <div className="lp-hero-actions">
          <a className="lp-button-primary" href="/acesso">
            {t("landing.hero.primary")}
          </a>
          <a className="lp-button-ghost" href="#numeros">
            {t("landing.hero.secondary")}
          </a>
        </div>
        <div className="lp-hero-metric">
          <div>
            <p className="lp-hero-metric-label">
              {t("landing.hero.todayLabel")}
            </p>
            <p className="lp-hero-metric-value">
              {today}
            </p>
          </div>
          <p className="lp-hero-metric-hint">
            <strong>{t("landing.hero.todayUnit")}</strong> ·{" "}
            {t("landing.hero.todayHint")}
          </p>
        </div>
      </div>
    </section>
  );
}
