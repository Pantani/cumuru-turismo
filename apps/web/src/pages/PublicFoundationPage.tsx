import { AnalyticsDashboard } from "../features/analytics/AnalyticsDashboard";
import {
  phase4PublicClient,
  type Phase4Client,
} from "../shared/api/phase4-client";

export default function PublicFoundationPage({
  client = phase4PublicClient,
}: {
  client?: Phase4Client;
}) {
  return (
    <article className="page page-public">
      <div className="eyebrow">Analytics e dashboard · Fase 4</div>
      <div className="hero-grid">
        <div>
          <h1 data-route-heading tabIndex={-1}>
            Observatório Turístico de Cumuruxatiba
          </h1>
          <p className="lead">
            Presença, previsão e preferências agregadas para apoiar o
            planejamento local com privacidade por construção.
          </p>
        </div>
        <aside className="prototype-note" aria-labelledby="prototype-title">
          <h2 id="prototype-title">Ambiente de protótipo</h2>
          <p>
            Esta demonstração usa somente dados fictícios. Não substitui
            estatística oficial nem censo e não autoriza operação real.
          </p>
        </aside>
      </div>
      <AnalyticsDashboard client={client} />
    </article>
  );
}
