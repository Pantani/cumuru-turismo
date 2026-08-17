import type { AnalyticsClient } from "../../shared/api/analytics-client";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import type { Translate } from "../../shared/i18n/translate";
import { coverageText } from "../analytics/coverage";
import { usePresenceFormat, type PresenceFormat } from "../analytics/presence-format";
import { usePublicSummary } from "../analytics/public-summary";
import { peakHeadline, todayHeadline } from "./summary-values";

type Summary = ReturnType<typeof usePublicSummary>["data"];

function tickerItems(
  t: Translate,
  format: PresenceFormat,
  summary: Summary,
): string[] {
  if (summary === undefined) {
    return [t("landing.ticker.pending")];
  }
  const { forecast_peak_next_30_days: peak, metadata, presence_today: today } =
    summary.data;
  return [
    t("landing.ticker.today", { value: todayHeadline(t, format, today) }),
    t("landing.ticker.peak", { value: peakHeadline(t, format, peak) }),
    t("landing.ticker.coverage", {
      coverage: coverageText(t, metadata.coverage),
    }),
    t("landing.ticker.prototype"),
  ];
}

function TickerTrack({
  items,
  mirrored,
}: {
  items: readonly string[];
  mirrored: boolean;
}) {
  return (
    <ul
      aria-hidden={mirrored ? "true" : undefined}
      className="lp-ticker-track"
      role={mirrored ? "presentation" : undefined}
    >
      {items.map((item) => (
        <li key={item}>
          {item}
          <span className="lp-ticker-dot" aria-hidden="true">
            ◆
          </span>
        </li>
      ))}
    </ul>
  );
}

/**
 * Faixa em movimento. A segunda cópia é puramente visual — ela existe para o
 * laço da animação fechar sem salto — e sai da árvore de acessibilidade para
 * que ninguém ouça os mesmos números duas vezes.
 */
export function TickerBar({ client }: { client?: AnalyticsClient }) {
  const { t } = useLocale();
  const format = usePresenceFormat();
  const summary = usePublicSummary(client);
  const items = tickerItems(t, format, summary.data);

  return (
    <div className="lp-ticker" aria-label={t("landing.ticker.aria")}>
      <div className="lp-ticker-rail">
        <TickerTrack items={items} mirrored={false} />
        <TickerTrack items={items} mirrored />
      </div>
    </div>
  );
}
