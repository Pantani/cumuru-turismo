import { useLocale } from "../../shared/i18n/LocaleProvider";
import { coverageText } from "../analytics/coverage";
import type { Accommodation } from "../operator/stay-lifecycle";
import { PerformanceChart } from "./PerformanceChart";
import {
  comparativeTone,
  type PerformancePayload,
  summarize,
} from "./performance-summary";
import {
  PERFORMANCE_WINDOW_LABELS,
  PERFORMANCE_WINDOWS,
  usePerformance,
} from "./use-performance";

/**
 * O comparativo da hospedagem. O lado próprio é exato; o da vila é o índice da
 * publicação corrente, e some inteiro quando a política do servidor fecha a
 * comparação. A tela nunca decide isso: ela lê `comparison.status`.
 */
const CLOSED_COMPARISON: Record<string, string> = {
  few_reporting_accommodations:
    "Poucas hospedagens reportaram nesta janela. Comparar agora diria demais sobre cada uma delas, então mostramos só o seu movimento.",
  own_capacity_share_too_high:
    "Sua capacidade responde por boa parte da vila nesta janela: o agregado seria quase o seu próprio número. A comparação fica fechada.",
  no_published_series: "Ainda não há série publicada para esta janela.",
};

function occupancyLabel(value: number | undefined): string {
  return value === undefined ? "—" : `${value}%`;
}

function changeLabel(value: number | null): string {
  if (value === null) {
    return "—";
  }
  return value > 0 ? `+${value}%` : `${value}%`;
}

function WindowPicker({
  onSelect,
  selected,
}: {
  onSelect: (window: (typeof PERFORMANCE_WINDOWS)[number]) => void;
  selected: string;
}) {
  return (
    <div className="button-row" role="group" aria-label="Janela do comparativo">
      {PERFORMANCE_WINDOWS.map((window) => (
        <button
          aria-pressed={window === selected}
          className={window === selected ? "primary-action" : "ghost-action"}
          key={window}
          onClick={() => onSelect(window)}
          type="button"
        >
          {PERFORMANCE_WINDOW_LABELS[window]}
        </button>
      ))}
    </div>
  );
}

function ComparisonNotice({ performance }: { performance: PerformancePayload }) {
  if (performance.comparison.status === "available") {
    return null;
  }
  const reason = performance.comparison.reason ?? "no_published_series";
  return (
    <p className="queue-hint" role="status">
      {CLOSED_COMPARISON[reason] ?? CLOSED_COMPARISON.no_published_series}
    </p>
  );
}

function Readings({ performance }: { performance: PerformancePayload }) {
  const summary = summarize(performance.series);
  const tone = comparativeTone(summary);
  const comparable = performance.comparison.status === "available";
  return (
    <>
      <dl className="performance-readings">
        <div>
          <dt>Sua ocupação na janela</dt>
          <dd>{occupancyLabel(performance.occupancy.own_percent)}</dd>
        </div>
        <div>
          {/* A banda de cinco pontos é do servidor; a tela só a nomeia, para
              que ninguém leia precisão que o número não tem. */}
          <dt>Ocupação da vila (faixa)</dt>
          <dd>{occupancyLabel(performance.occupancy.village_percent)}</dd>
        </div>
        <div>
          <dt>Suas pessoas-dia na janela</dt>
          <dd>{summary.ownPersonDays}</dd>
        </div>
        <div>
          <dt>Sua variação desde o início da janela</dt>
          <dd>{changeLabel(summary.ownChangePercent)}</dd>
        </div>
        <div>
          <dt>Variação da vila</dt>
          <dd>{changeLabel(summary.villageChangePercent)}</dd>
        </div>
      </dl>
      <PerformanceChart
        label={`Sua curva${comparable ? " e a curva da vila" : ""} na janela selecionada`}
        points={performance.series}
        showVillage={comparable}
      />
      {tone === "unavailable" ? null : (
        <p className="performance-tone">
          {tone === "ahead"
            ? "Você cresceu mais que a vila nesta janela."
            : tone === "behind"
              ? "A vila cresceu mais que você nesta janela."
              : "Você e a vila se moveram igual nesta janela."}
        </p>
      )}
    </>
  );
}

export function PerformancePanel({
  accommodation,
}: {
  accommodation: Accommodation;
}) {
  const { t } = useLocale();
  const { failure, loading, performance, setWindow, window } = usePerformance(
    accommodation.id,
  );

  return (
    <section className="performance-panel" aria-labelledby="performance-title">
      <div className="board-heading">
        <h3 id="performance-title">Seu movimento e o da vila</h3>
        <WindowPicker onSelect={setWindow} selected={window} />
      </div>
      {failure === null ? null : (
        <p className="operation-status tone-failed" role="alert">
          {failure}
        </p>
      )}
      {loading ? <p className="loading-note">Carregando…</p> : null}
      {performance === null ? null : (
        <>
          {/* A cobertura vem junto do número, sempre: metade da vila reportando
              é metade da vila comparada, e docs/07 proíbe apresentar parcial
              como censo. */}
          <p className="queue-hint">
            {coverageText(t, performance.metadata.coverage)}
          </p>
          <ComparisonNotice performance={performance} />
          <Readings performance={performance} />
        </>
      )}
    </section>
  );
}
