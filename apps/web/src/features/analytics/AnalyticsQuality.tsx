import { useQuery } from "@tanstack/react-query";

import type { components } from "../../generated/schema";
import {
  Phase4ApiError,
  type Phase4Client,
} from "../../shared/api/phase4-client";

type Schemas = components["schemas"];
type QualityCount = Schemas["QualityCount"];
type QualityCoverage = Schemas["QualityCoverage"];

interface AnalyticsQualityProps {
  client: Phase4Client;
}

const dateTimeFormatter = new Intl.DateTimeFormat("pt-BR", {
  dateStyle: "long",
  timeStyle: "short",
  timeZone: "America/Bahia",
});

const reasonMessages: Record<
  Schemas["UnavailableQualityCount"]["reason_code"],
  string
> = {
  phase_not_implemented: "A integração FNRH pertence à Fase 5.",
  pseudonym_not_approved:
    "Não há pseudônimo transversal aprovado para esta análise.",
  insufficient_source: "A fonte agregada ainda é insuficiente.",
};

function qualityError(error: unknown) {
  if (error instanceof Phase4ApiError && error.status === 403) {
    return "Sua sessão não possui o escopo analytics:read:internal.";
  }
  if (error instanceof Phase4ApiError && error.status === 401) {
    return "A sessão institucional não está disponível.";
  }
  return "Não foi possível carregar a qualidade agregada.";
}

function CountValue({ count }: { count: QualityCount }) {
  if (count.status === "available") {
    return <strong>{count.value}</strong>;
  }
  return (
    <span className="quality-na">
      <strong>N/A</strong>
      <small>{reasonMessages[count.reason_code]}</small>
    </span>
  );
}

function CoverageValue({ coverage }: { coverage: QualityCoverage }) {
  if (
    coverage.status === "available" &&
    typeof coverage.ratio === "number"
  ) {
    return <strong>{Math.round(coverage.ratio * 100)}%</strong>;
  }
  return <strong>N/A</strong>;
}

function countItems(snapshot: Schemas["QualitySnapshot"]) {
  return [
    ["Cadastros incompletos", snapshot.incomplete_stays],
    ["Saídas previstas vencidas", snapshot.overdue_planned_departures],
    ["Acomodações silenciosas", snapshot.silent_accommodations],
    ["Falhas de agregação", snapshot.aggregation_failures],
    ["Duplicatas suspeitas", snapshot.suspected_duplicates],
    ["Falhas FNRH", snapshot.fnrh_failures],
  ] as const;
}

function QualityContent({ snapshot }: { snapshot: Schemas["QualitySnapshot"] }) {
  return (
    <>
      <div className="quality-grid">
        {countItems(snapshot).map(([label, count]) => (
          <article className="quality-card" key={label}>
            <h3>{label}</h3>
            <CountValue count={count} />
          </article>
        ))}
      </div>
      <section
        className="analytics-section"
        aria-labelledby="quality-coverage-title"
      >
        <h3 id="quality-coverage-title">Cobertura agregada por categoria</h3>
        {snapshot.coverage_by_category.length === 0 ? (
          <p>Nenhuma categoria com fonte suficiente.</p>
        ) : (
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th scope="col">Categoria</th>
                  <th scope="col">Cobertura</th>
                </tr>
              </thead>
              <tbody>
                {snapshot.coverage_by_category.map((coverage) => (
                  <tr key={coverage.category_code}>
                    <th scope="row">{coverage.category_code}</th>
                    <td>
                      <CoverageValue coverage={coverage} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </>
  );
}

export function AnalyticsQuality({ client }: AnalyticsQualityProps) {
  const quality = useQuery({
    queryKey: ["analytics", "quality", "last_30_days"],
    queryFn: () => client.getQuality(),
    gcTime: 0,
    retry: false,
    staleTime: 0,
  });

  if (quality.isPending) {
    return <p role="status">Carregando qualidade agregada…</p>;
  }
  if (quality.isError) {
    return (
      <div className="analytics-error" role="alert">
        <p>{qualityError(quality.error)}</p>
        <button type="button" onClick={() => void quality.refetch()}>
          Tentar novamente
        </button>
      </div>
    );
  }

  return (
    <section aria-labelledby="quality-summary-title">
      <div className="dashboard-heading">
        <div>
          <p className="section-kicker">Somente agregados · sem IDs</p>
          <h2 id="quality-summary-title">Resumo dos últimos 30 dias</h2>
          <p>
            Atualizado em{" "}
            {dateTimeFormatter.format(new Date(quality.data.data.updated_at))}.
          </p>
        </div>
        <span className="internal-badge">Acesso interno</span>
      </div>
      <QualityContent snapshot={quality.data.data} />
    </section>
  );
}
