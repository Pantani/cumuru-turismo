import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import type { components } from "../../generated/schema";
import {
  phase4PublicClient,
  type Phase4Client,
} from "../../shared/api/phase4-client";

type Schemas = components["schemas"];
type PresenceWindow = components["parameters"]["PresenceWindow"];
type PresencePoint =
  | Schemas["ObservedPresencePoint"]
  | Schemas["ForecastPresencePoint"];
type ForecastPeak = Schemas["ForecastPeak"];
type Metadata = Schemas["PublicMetadata"];
type PreferenceCategory = Schemas["PreferenceCategory"];

interface AnalyticsDashboardProps {
  client?: Phase4Client;
}

const dateFormatter = new Intl.DateTimeFormat("pt-BR", {
  dateStyle: "medium",
  timeZone: "America/Bahia",
});
const dateTimeFormatter = new Intl.DateTimeFormat("pt-BR", {
  dateStyle: "long",
  timeStyle: "short",
  timeZone: "America/Bahia",
});

function formatDate(value: string) {
  return dateFormatter.format(new Date(`${value}T12:00:00-03:00`));
}

function formatDateTime(value: string) {
  return dateTimeFormatter.format(new Date(value));
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
      <article className="summary-card">
        <p className="metric-label">Presença de hoje</p>
        <p className="metric-kind">{kindLabel(summary.presence_today.kind)}</p>
        <PointValue point={summary.presence_today} />
      </article>
      <article className="summary-card">
        <p className="metric-label">Pico previsto nos próximos 30 dias</p>
        <p className="metric-kind">◇ Previsto</p>
        {"date" in peak ? (
          <time dateTime={peak.date}>{formatDate(peak.date)}</time>
        ) : null}
        <PointValue point={peak} />
      </article>
    </section>
  );
}

function PresenceTable({
  presence,
}: {
  presence: Schemas["PublicPresence"];
}) {
  return (
    <div className="table-scroll">
      <table aria-label="Presença observada e prevista">
        <thead>
          <tr>
            <th scope="col">Data</th>
            <th scope="col">Tipo</th>
            <th scope="col">Resultado</th>
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
            <li key={category.category_code}>
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
  const pending = queries.some((query) => query.isPending);
  const fetching = queries.some((query) => query.isFetching);
  const failed = queries.some((query) => query.isError);

  function retry() {
    void Promise.all(queries.map((query) => query.refetch()));
  }

  if (pending || fetching) {
    return (
      <section className="analytics-dashboard" aria-labelledby="analytics-title">
        <h2 id="analytics-title">Indicadores públicos</h2>
        <p role="status" aria-live="polite">
          Atualizando indicadores públicos…
        </p>
      </section>
    );
  }
  if (failed) {
    return (
      <section className="analytics-dashboard" aria-labelledby="analytics-title">
        <h2 id="analytics-title">Indicadores públicos</h2>
        <div className="analytics-error" role="alert">
          <p>Não foi possível carregar os indicadores públicos.</p>
          <button type="button" onClick={retry}>
            Tentar novamente
          </button>
        </div>
      </section>
    );
  }
  if (
    summary.data === undefined ||
    presence.data === undefined ||
    preferences.data === undefined ||
    methodology.data === undefined
  ) {
    return (
      <section className="analytics-dashboard" aria-labelledby="analytics-title">
        <h2 id="analytics-title">Indicadores públicos</h2>
        <p role="status" aria-live="polite">
          Atualizando indicadores públicos…
        </p>
      </section>
    );
  }

  return (
    <section className="analytics-dashboard" aria-labelledby="analytics-title">
      <div className="dashboard-heading">
        <div>
          <p className="section-kicker">Publicação protegida · Fase 4</p>
          <h2 id="analytics-title">Indicadores públicos</h2>
          <p>
            Tendências agregadas para planejamento, sem microdados, IDs ou
            recortes por estabelecimento.
          </p>
        </div>
        <span className="prototype-badge">Dados fictícios de protótipo</span>
      </div>
      <MetadataPanel metadata={summary.data.data.metadata} />
      <SummaryCards summary={summary.data.data} />
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
          <span>● Observado</span>
          <span>◇ Previsto, com faixa provável</span>
          <span>▨ Protegido ou indisponível, sem valor substituto</span>
        </p>
        <PresenceTable presence={presence.data.data} />
      </section>
      <Preferences preferences={preferences.data.data} />
      <Methodology methodology={methodology.data.data} />
    </section>
  );
}
