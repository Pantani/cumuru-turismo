import type { Phase4Client } from "../../shared/api/phase4-client";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import type { Translate } from "../../shared/i18n/translate";
import { coverageText } from "../analytics/coverage";
import { usePublicSummary } from "../analytics/public-summary";
import { peakHeadline, todayHeadline } from "./summary-values";

type Summary = ReturnType<typeof usePublicSummary>["data"];

function tickerItems(t: Translate, summary: Summary): string[] {
  if (summary === undefined) {
    return [t("landing.ticker.pending")];
  }
  const { forecast_peak_next_30_days: peak, metadata, presence_today: today } =
    summary.data;
  return [
    t("landing.ticker.today", { value: todayHeadline(t, today) }),
    t("landing.ticker.peak", { value: peakHeadline(t, peak) }),
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
export function TickerBar({ client }: { client?: Phase4Client }) {
  const { t } = useLocale();
  const summary = usePublicSummary(client);
  const items = tickerItems(t, summary.data);

  return (
    <div className="lp-ticker" aria-label={t("landing.ticker.aria")}>
      <div className="lp-ticker-rail">
        <TickerTrack items={items} mirrored={false} />
        <TickerTrack items={items} mirrored />
      </div>
    </div>
  );
}
