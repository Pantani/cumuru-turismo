import { useLocale } from "../../shared/i18n/LocaleProvider";
import { coverageText } from "../analytics/coverage";
import type { Accommodation } from "../operator/stay-lifecycle";
import { PerformanceChart } from "./PerformanceChart";
import {
  comparativeTone,
  type PerformancePayload,
  type PerformanceSummary,
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

/**
 * `comparison.status` governa toda saída da vila, não só a curva. O servidor já
 * retira ocupação e índice quando fecha o comparativo; a tela não deriva nem
 * renderiza nenhum deles mesmo que a resposta os traga. É defesa em
 * profundidade sobre um controle de privacidade, não redundância.
 */
type ComparativeTone = ReturnType<typeof comparativeTone>;

interface VillageReadings {
  change: number | null;
  occupancy: number | undefined;
  tone: ComparativeTone;
}

/**
 * Uma porta só para todo valor da vila. Fechado o comparativo, nada é derivado
 * nem renderizado — nem que a resposta traga os campos.
 */
function villageReadings(
  performance: PerformancePayload,
  summary: PerformanceSummary,
): VillageReadings {
  if (performance.comparison.status !== "available") {
    return { change: null, occupancy: undefined, tone: "unavailable" };
  }
  return {
    change: summary.villageChangePercent,
    occupancy: performance.occupancy.village_percent,
    tone: comparativeTone(summary),
  };
}

const TONE_SENTENCES: Record<Exclude<ComparativeTone, "unavailable">, string> = {
  ahead: "Você cresceu mais que a vila nesta janela.",
  behind: "A vila cresceu mais que você nesta janela.",
  even: "Você e a vila se moveram igual nesta janela.",
};

function ToneSentence({ tone }: { tone: ComparativeTone }) {
  if (tone === "unavailable") {
    return null;
  }
  return <p className="performance-tone">{TONE_SENTENCES[tone]}</p>;
}

function Readings({ performance }: { performance: PerformancePayload }) {
  const summary = summarize(performance.series);
  const village = villageReadings(performance, summary);
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
          <dd>{occupancyLabel(village.occupancy)}</dd>
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
          <dd>{changeLabel(village.change)}</dd>
        </div>
      </dl>
      <PerformanceChart
        label={`Sua curva${comparable ? " e a curva da vila" : ""} na janela selecionada`}
        points={performance.series}
        showVillage={comparable}
      />
      <ToneSentence tone={village.tone} />
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
