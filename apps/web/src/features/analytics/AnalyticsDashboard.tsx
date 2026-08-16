import { useQuery } from "@tanstack/react-query";
import { useState, type CSSProperties, type ReactNode } from "react";

import type { components } from "../../generated/schema";
import {
  phase4PublicClient,
  type Phase4Client,
} from "../../shared/api/phase4-client";
import {
  formatCount,
  formatDate,
  formatDateTime,
  formatDelta,
  formatTrend,
} from "./presence-format";
import { PresenceChart } from "./PresenceChart";
import {
  centralValue,
  percentFromAverage,
  seriesStats,
  type PresencePoint,
  type SeriesStats,
} from "./presence-stats";

type Schemas = components["schemas"];
type PresenceWindow = components["parameters"]["PresenceWindow"];
type ForecastPeak = Schemas["ForecastPeak"];
type Metadata = Schemas["PublicMetadata"];
type PreferenceCategory = Schemas["PreferenceCategory"];

interface AnalyticsDashboardProps {
  client?: Phase4Client;
}

function coverageText(coverage: Metadata["coverage"]) {
  if (coverage.status === "published") {
    return `Cobertura estimada: ${coverage.ratio}%`;
  }
  return coverage.status === "protected"
    ? "Cobertura protegida pela política de publicação"
    : "Cobertura indisponível";
}

function MetadataPanel({ metadata }: { metadata: Metadata }) {
  return (
    <dl className="analytics-metadata" aria-label="Contexto dos indicadores">
      <div>
        <dt>Atualização</dt>
        <dd>{formatDateTime(metadata.updated_at)}</dd>
      </div>
      <div>
        <dt>Cobertura</dt>
        <dd>{coverageText(metadata.coverage)}</dd>
      </div>
      <div>
        <dt>Unidade</dt>
        <dd>
          {metadata.unit === "person_day"
            ? "Pessoas-dia"
            : "Respostas de pesquisa"}
        </dd>
      </div>
      <div>
        <dt>Modo dos dados</dt>
        <dd>Dados fictícios de protótipo</dd>
      </div>
    </dl>
  );
}

function kindLabel(kind: PresencePoint["kind"]) {
  return kind === "observed" ? "● Observado" : "◇ Previsto";
}

function protectedLabel(status: "protected" | "unavailable") {
  return status === "protected" ? "Dado protegido" : "Dado indisponível";
}

function PointValue({ point }: { point: PresencePoint | ForecastPeak }) {
  if (point.status !== "published") {
    return (
      <span className={`data-state data-state-${point.status}`}>
        {protectedLabel(point.status)}
      </span>
    );
  }
  if (point.kind === "observed") {
    return <strong>{point.value} pessoas-dia</strong>;
  }
  return (
    <span className="forecast-value">
      <strong>Estimativa central: {point.central} pessoas-dia</strong>
      <span>Faixa provável: {point.lower} a {point.upper}</span>
    </span>
  );
}

function SummaryCards({ summary }: { summary: Schemas["PublicSummary"] }) {
  const peak = summary.forecast_peak_next_30_days;
  return (
    <section className="summary-grid" aria-labelledby="summary-title">
      <h3 id="summary-title" className="visually-hidden">
        Resumo da presença
      </h3>
      <article className="summary-card" data-kind={summary.presence_today.kind}>
        <p className="metric-label">Presença de hoje</p>
        <p className="metric-kind">{kindLabel(summary.presence_today.kind)}</p>
        <PointValue point={summary.presence_today} />
        <p className="metric-hint">
          Pessoas presentes hoje, contadas no intervalo [chegada, saída) em
          America/Bahia. Uma pessoa só conta uma vez por dia.
        </p>
      </article>
      <article className="summary-card" data-kind="forecast">
        <p className="metric-label">Pico previsto nos próximos 30 dias</p>
        <p className="metric-kind">◇ Previsto</p>
        {"date" in peak ? (
          <time dateTime={peak.date}>{formatDate(peak.date)}</time>
        ) : null}
        <PointValue point={peak} />
        <p className="metric-hint">
          Maior estimativa central do baseline explicável. Planeje pela faixa
          provável, não pelo número central isolado.
        </p>
      </article>
    </section>
  );
}

interface StatTile {
  hint: string;
  label: string;
  value: ReactNode;
}

const WINDOW_LABELS: Record<PresenceWindow, string> = {
  recent_30_days: "últimos 30 dias",
  next_30_days: "próximos 30 dias",
};

function DayValue({ day }: { day: { date: string; value: number } }) {
  return (
    <>
      <span>{day.value} pessoas-dia</span>
      <time className="stat-when" dateTime={day.date}>
        {formatDate(day.date)}
      </time>
    </>
  );
}

function averageTile(stats: SeriesStats): StatTile {
  return {
    label: "Média diária",
    value:
      stats.average === null
        ? "—"
        : `${Math.round(stats.average)} pessoas-dia`,
    hint: `Média dos ${stats.published.length} dias com valor publicado. A linha tracejada do gráfico marca esse nível.`,
  };
}

function totalTile(stats: SeriesStats): StatTile {
  return {
    label: "Total acumulado",
    value: `${formatCount(stats.total)} pessoas-dia`,
    hint: "Soma dos dias publicados. Dias protegidos ficam de fora, então o total é um piso, não um censo.",
  };
}

function peakTile(stats: SeriesStats): StatTile {
  return {
    label: "Dia mais cheio",
    value: stats.peak === null ? "—" : <DayValue day={stats.peak} />,
    hint: "Maior valor publicado da janela e o dia em que ocorreu.",
  };
}

function troughTile(stats: SeriesStats): StatTile {
  return {
    label: "Dia mais vazio",
    value: stats.trough === null ? "—" : <DayValue day={stats.trough} />,
    hint: "Menor valor publicado da janela. Compare com o dia mais cheio para dimensionar a variação.",
  };
}

function trendTile(stats: SeriesStats): StatTile {
  const size = stats.trendSize;
  return {
    label: "Tendência",
    value:
      stats.trendPercent === null
        ? "—"
        : formatTrend(stats.trendPercent, size),
    hint:
      stats.trendPercent === null
        ? "São necessários pelo menos dois dias publicados para comparar períodos."
        : `Média dos ${size} últimos dias publicados ante os ${size} anteriores. Dias protegidos são pulados, nunca contados como zero.`,
  };
}

function withheldHint(withheld: number) {
  if (withheld === 0) {
    return "Todos os dias da janela têm valor publicado.";
  }
  const noun = withheld === 1 ? "dia ficou" : "dias ficaram";
  return `${withheld} ${noun} sem valor por proteção estatística ou ausência de dado. Nenhum valor substituto é exibido.`;
}

function publishedTile(stats: SeriesStats): StatTile {
  return {
    label: "Dias publicados",
    value: `${stats.published.length} de ${stats.days}`,
    hint: withheldHint(stats.withheld),
  };
}

function statTiles(stats: SeriesStats): StatTile[] {
  return [
    averageTile(stats),
    peakTile(stats),
    troughTile(stats),
    trendTile(stats),
    totalTile(stats),
    publishedTile(stats),
  ];
}

function WindowStats({
  stats,
  window,
}: {
  stats: SeriesStats;
  window: PresenceWindow;
}) {
  return (
    <ul
      className="stat-grid"
      aria-label={`Estatísticas dos ${WINDOW_LABELS[window]}`}
    >
      {statTiles(stats).map((tile) => (
        <li className="stat-tile" key={tile.label}>
          <p className="metric-label">{tile.label}</p>
          <p className="stat-value">{tile.value}</p>
          <p className="metric-hint">{tile.hint}</p>
        </li>
      ))}
    </ul>
  );
}

/** Distance from the window average, so a row is read without doing the math. */
function ComparisonCell({
  average,
  point,
}: {
  average: number | null;
  point: PresencePoint;
}) {
  const value = centralValue(point);
  const percent =
    value === null ? null : percentFromAverage(value, average);
  if (percent === null) {
    return <span className="stat-when">—</span>;
  }
  return <span>{formatDelta(percent)}</span>;
}

function PresenceTable({
  presence,
  stats,
}: {
  presence: Schemas["PublicPresence"];
  stats: SeriesStats;
}) {
  return (
    <div className="table-scroll">
      <table aria-label="Presença observada e prevista">
        <thead>
          <tr>
            <th scope="col">Data</th>
            <th scope="col">Tipo</th>
            <th scope="col">Resultado</th>
            <th scope="col">Ante a média</th>
          </tr>
        </thead>
        <tbody>
          {presence.series.map((point) => (
            <tr key={`${point.date}-${point.kind}`}>
              <th scope="row">
                <time dateTime={point.date}>{formatDate(point.date)}</time>
              </th>
              <td>{kindLabel(point.kind)}</td>
              <td>
                <PointValue point={point} />
              </td>
              <td>
                <ComparisonCell average={stats.average} point={point} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function categoryLabel(category: PreferenceCategory["category_code"]) {
  return category === "first_visit" ? "Primeira visita" : "Visitante recorrente";
}

/**
 * The row draws its own proportional bar from --share. A protected category has
 * no share to draw, so it reports zero and the bar stays absent.
 */
function shareStyle(category: PreferenceCategory): CSSProperties {
  const share = category.status === "published" ? category.share_percent : 0;
  return { "--share": share } as CSSProperties;
}

function PreferenceValue({ category }: { category: PreferenceCategory }) {
  if (category.status === "published") {
    return <strong>{category.share_percent}% das respostas elegíveis</strong>;
  }
  return (
    <span className={`data-state data-state-${category.status}`}>
      {protectedLabel(category.status)}
    </span>
  );
}

function Preferences({
  preferences,
}: {
  preferences: Schemas["PublicPreferences"];
}) {
  const metric = preferences.metrics[0];
  return (
    <section className="analytics-section" aria-labelledby="preferences-title">
      <div className="section-heading">
        <div>
          <p className="section-kicker">Pesquisa voluntária</p>
          <h3 id="preferences-title">Perfil de visita agregado</h3>
        </div>
        <label>
          Período das preferências
          <select value={preferences.period} disabled>
            <option value="last_complete_month">Último mês completo</option>
          </select>
        </label>
      </div>
      <p>
        Unidade: respostas de pesquisa estruturadas e consentidas. Uma resposta
        de grupo não é multiplicada pela quantidade de visitantes.
      </p>
      {metric === undefined ? (
        <p className="data-state data-state-unavailable">Dado indisponível</p>
      ) : (
        <ul className="preference-list">
          {metric.categories.map((category) => (
            <li key={category.category_code} style={shareStyle(category)}>
              <span>{categoryLabel(category.category_code)}</span>
              <PreferenceValue category={category} />
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function Methodology({
  methodology,
}: {
  methodology: Schemas["PublicMethodology"];
}) {
  return (
    <section
      className="analytics-section methodology"
      aria-labelledby="methodology-title"
    >
      <p className="section-kicker">Como interpretar</p>
      <h3 id="methodology-title">Metodologia e limitações</h3>
      <div className="methodology-grid">
        <article>
          <h4>Presença observada</h4>
          <p>
            Cada pessoa contribui no intervalo civil [chegada, saída), em
            America/Bahia. A saída não conta como novo dia de presença.
          </p>
        </article>
        <article>
          <h4>Presença prevista</h4>
          <p>
            O baseline explicável combina reservas já conhecidas e histórico
            sazonal. A faixa normal usa limites de{" "}
            {methodology.forecast_bounds_percent[0]}% a{" "}
            {methodology.forecast_bounds_percent[1]}%. Quando não há histórico
            elegível suficiente, o baseline usa fallback mais amplo, de{" "}
            {methodology.forecast_fallback_bounds_percent[0]}% a{" "}
            {methodology.forecast_fallback_bounds_percent[1]}%. O contrato
            público não identifica qual faixa foi aplicada a cada ponto; por
            isso a interface não atribui esse estado a valores individuais.
          </p>
        </article>
        <article>
          <h4>Proteção estatística</h4>
          <p>
            Células abaixo de {methodology.primary_threshold} contribuições ou
            de {methodology.minimum_reporting_accommodations} acomodações são
            protegidas. Há supressão complementar e arredondamento em base{" "}
            {methodology.rounding_base}.
          </p>
        </article>
        <article>
          <h4>Limitações</h4>
          <p>
            A cobertura é parcial e não representa um censo. Os dados são
            fictícios de protótipo; acurácia operacional, política municipal e
            uso com dados reais permanecem não verificados.
          </p>
        </article>
      </div>
    </section>
  );
}

type DashboardStage = "loading" | "failed";

interface QueryStage {
  isPending: boolean;
  isFetching: boolean;
  isError: boolean;
}

function dashboardStage(queries: readonly QueryStage[]): DashboardStage {
  return queries.some((query) => query.isError) ? "failed" : "loading";
}

interface DashboardPayloads<S, P, F, M> {
  summary: S;
  presence: P;
  preferences: F;
  methodology: M;
}

/**
 * Narrows the four payloads together so the render below reads them without a
 * per-panel undefined check.
 */
function loadedPayloads<S, P, F, M>(
  summary: S | undefined,
  presence: P | undefined,
  preferences: F | undefined,
  methodology: M | undefined,
): DashboardPayloads<S, P, F, M> | null {
  if (
    summary === undefined ||
    presence === undefined ||
    preferences === undefined ||
    methodology === undefined
  ) {
    return null;
  }
  return { summary, presence, preferences, methodology };
}

function DashboardPlaceholder({
  onRetry,
  stage,
}: {
  onRetry: () => void;
  stage: DashboardStage;
}) {
  return (
    <section className="analytics-dashboard" aria-labelledby="analytics-title">
      <h2 id="analytics-title">Indicadores públicos</h2>
      {stage === "failed" ? (
        <div className="analytics-error" role="alert">
          <p>Não foi possível carregar os indicadores públicos.</p>
          <button type="button" onClick={onRetry}>
            Tentar novamente
          </button>
        </div>
      ) : (
        <p role="status" aria-live="polite">
          Atualizando indicadores públicos…
        </p>
      )}
    </section>
  );
}

export function AnalyticsDashboard({
  client = phase4PublicClient,
}: AnalyticsDashboardProps) {
  const [window, setWindow] =
    useState<PresenceWindow>("recent_30_days");
  const summary = useQuery({
    queryKey: ["analytics", "public", "summary"],
    queryFn: () => client.getSummary(),
    staleTime: 300_000,
  });
  const presence = useQuery({
    queryKey: ["analytics", "public", "presence", window],
    queryFn: () => client.getPresence(window),
    staleTime: 300_000,
  });
  const preferences = useQuery({
    queryKey: ["analytics", "public", "preferences", "last_complete_month"],
    queryFn: () => client.getPreferences(),
    staleTime: 300_000,
  });
  const methodology = useQuery({
    queryKey: ["analytics", "public", "methodology"],
    queryFn: () => client.getMethodology(),
    staleTime: 300_000,
  });
  const queries = [summary, presence, preferences, methodology];

  function retry() {
    void Promise.all(queries.map((query) => query.refetch()));
  }

  const loaded = loadedPayloads(
    summary.data,
    presence.data,
    preferences.data,
    methodology.data,
  );
  if (loaded === null) {
    return (
      <DashboardPlaceholder onRetry={retry} stage={dashboardStage(queries)} />
    );
  }
  const stats = seriesStats(loaded.presence.data.series);

  return (
    <section className="analytics-dashboard" aria-labelledby="analytics-title">
      <div className="dashboard-heading">
        <div>
          <p className="section-kicker">Publicação protegida</p>
          <h2 id="analytics-title">Indicadores públicos</h2>
          <p>
            Tendências agregadas para planejamento, sem microdados, IDs ou
            recortes por estabelecimento.
          </p>
        </div>
        <span className="prototype-badge">Dados fictícios de protótipo</span>
      </div>
      <MetadataPanel metadata={loaded.summary.data.metadata} />
      <SummaryCards summary={loaded.summary.data} />
      <section className="analytics-section" aria-labelledby="presence-title">
        <div className="section-heading">
          <div>
            <p className="section-kicker">Série em pessoas-dia</p>
            <h3 id="presence-title">Presença ao longo do tempo</h3>
          </div>
          <label>
            Janela da presença
            <select
              value={window}
              onChange={(event) =>
                setWindow(event.target.value as PresenceWindow)
              }
            >
              <option value="recent_30_days">Últimos 30 dias</option>
              <option value="next_30_days">Próximos 30 dias</option>
            </select>
          </label>
        </div>
        <p className="legend" aria-label="Legenda da série">
          <span className="legend-observed">Observado</span>
          <span className="legend-forecast">Previsto, com faixa provável</span>
          <span className="legend-gap">
            Protegido ou indisponível, sem valor substituto
          </span>
          <span className="legend-average">Média da janela</span>
          <span className="legend-weekend">Fim de semana</span>
        </p>
        <WindowStats stats={stats} window={window} />
        <PresenceChart series={loaded.presence.data.series} stats={stats} />
        <details className="series-details">
          <summary>Ver a série dia a dia</summary>
          <PresenceTable presence={loaded.presence.data} stats={stats} />
        </details>
      </section>
      <Preferences preferences={loaded.preferences.data} />
      <Methodology methodology={loaded.methodology.data} />
    </section>
  );
}
