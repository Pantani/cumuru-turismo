import { useQuery } from "@tanstack/react-query";

import type { components } from "../../generated/schema";
import { ApiError } from "../../shared/api/http-client";
import { type AnalyticsClient } from "../../shared/api/analytics-client";
import { accommodationCategoryLabels } from "../operator/stay-lifecycle";

type Schemas = components["schemas"];
type QualityCount = Schemas["QualityCount"];
type QualityCoverage = Schemas["QualityCoverage"];

interface AnalyticsQualityProps {
  client: AnalyticsClient;
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
  not_implemented: "A integração FNRH ainda não foi implementada.",
  pseudonym_not_approved:
    "Não há pseudônimo transversal aprovado para esta análise.",
  insufficient_source: "A fonte agregada ainda é insuficiente.",
};

/**
 * The quality contract types category_code as a free string, so an unknown code
 * is shown as it came instead of being hidden behind a guessed label.
 */
function categoryLabel(code: string) {
  const labels: Record<string, string> = accommodationCategoryLabels;
  return labels[code] ?? code;
}

function qualityError(error: unknown) {
  if (error instanceof ApiError && error.status === 403) {
    return "Sua sessão não possui o escopo analytics:read:internal.";
  }
  if (error instanceof ApiError && error.status === 401) {
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

/**
 * O funil conta estados que o registro já guarda; nada aqui vem de telemetria.
 * A mediana some abaixo de dez submissões na janela — é decisão do servidor, e
 * a tela só respeita a ausência em vez de inventar zero.
 */
function funnelRows(funnel: Schemas["AdoptionFunnel"]) {
  return [
    {
      stage: "Convite nominal",
      offered: funnel.invite.issued,
      completed: funnel.invite.submitted,
      lost: funnel.invite.expired_unused + funnel.invite.revoked,
      median: funnel.invite.median_hours_to_submit,
    },
    {
      stage: "Pesquisa",
      offered: funnel.survey.issued,
      completed: funnel.survey.answered,
      lost: funnel.survey.expired_unanswered,
      median: funnel.survey.median_hours_to_answer,
    },
    {
      stage: "Autocadastro",
      offered: funnel.self_registration.started,
      completed: funnel.self_registration.approved,
      lost: funnel.self_registration.rejected + funnel.self_registration.expired,
      median: undefined,
    },
  ] as const;
}

function conversionLabel(offered: number, completed: number) {
  return offered === 0 ? "—" : `${Math.round((completed / offered) * 100)}%`;
}

function medianLabel(hours: number | undefined) {
  return hours === undefined ? "—" : `${hours} h`;
}

function FunnelTable({ funnel }: { funnel: Schemas["AdoptionFunnel"] }) {
  return (
    <div className="table-scroll">
      <table>
        <thead>
          <tr>
            <th scope="col">Etapa</th>
            <th scope="col">Ofertados</th>
            <th scope="col">Concluídos</th>
            <th scope="col">Conversão</th>
            <th scope="col">Perdidos</th>
            <th scope="col">Mediana até concluir</th>
          </tr>
        </thead>
        <tbody>
          {funnelRows(funnel).map((row) => (
            <tr key={row.stage}>
              <th scope="row">{row.stage}</th>
              <td>{row.offered}</td>
              <td>{row.completed}</td>
              <td>{conversionLabel(row.offered, row.completed)}</td>
              <td>{row.lost}</td>
              <td>{medianLabel(row.median)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function AdoptionFunnelSection({ client }: AnalyticsQualityProps) {
  const funnel = useQuery({
    queryKey: ["analytics", "funnel", "last_30_days"],
    queryFn: () => client.getFunnel(),
    gcTime: 0,
    retry: false,
    staleTime: 0,
  });
  if (funnel.isPending || funnel.isError) {
    return null;
  }
  return (
    <section className="analytics-section" aria-labelledby="funnel-title">
      <h3 id="funnel-title">Funil de adesão nos últimos 30 dias</h3>
      <p>
        Contagens de estados já registrados. A pesquisa recusada explicitamente
        conta como resposta, não como perda.
      </p>
      <FunnelTable funnel={funnel.data.data} />
    </section>
  );
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
                    <th scope="row">{categoryLabel(coverage.category_code)}</th>
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
      <AdoptionFunnelSection client={client} />
    </section>
  );
}
