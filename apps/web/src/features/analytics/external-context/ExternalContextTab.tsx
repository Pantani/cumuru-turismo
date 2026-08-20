/**
 * Aba de contexto externo.
 *
 * Aba, e não um bloco a mais no painel de presença, porque dois gráficos lado
 * a lado produzem leitura causal na cabeça do leitor mesmo sem nenhuma frase
 * de correlação (ADR-045 §2 e §4). A separação aqui é dupla: a aba não importa
 * nada da camada medida — nem gráfico, nem previsão, nem cobertura, nem o
 * cliente de analytics — e as duas camadas nunca ocupam a tela ao mesmo tempo.
 * Sem os dois números na mesma árvore de componentes não existe onde escrever
 * a razão, a diferença ou o percentual que a ADR proíbe.
 *
 * Não há eixo, escala nem legenda: a série é tabela, e a caixa indisponível é
 * traço com motivo. Curva suave sem rótulo ainda comunica horário de extremo
 * ao olho, e o rótulo não desfaz isso (ADR-045 §8).
 */

import { useQuery } from "@tanstack/react-query";
import { useMemo, type ReactNode } from "react";

import type { components } from "../../../generated/schema";
import { useLocale } from "../../../shared/i18n/LocaleProvider";
import { contextCopyFor, type ContextCopy } from "./context-copy";
import {
  publicContextClient,
  type ContextClient,
} from "./context-client";
import { freshnessOf } from "./context-freshness";
import "./external-context.css";

type Schemas = components["schemas"];
type Card = Schemas["PublicContextCard"];
type PublishedCard = Schemas["PublishedContextCard"];
type UnavailableCard = Schemas["UnavailableContextCard"];
type Provenance = Schemas["ExternalProvenance"];
type SeriesPoint = Schemas["ExternalSeriesPoint"];
type CreditedSource = Schemas["ExternalCreditedSource"];

/**
 * Cinco minutos, como as demais leituras públicas: a rota é cacheável por
 * cache compartilhado e o documento não tem seletor.
 */
const CONTEXT_STALE_TIME = 5 * 60 * 1000;

interface ContextFormat {
  dateTime: (iso: string) => string;
  day: (iso: string) => string;
  number: (value: number) => string;
}

function useContextFormat(tag: string): ContextFormat {
  return useMemo(() => {
    const dateTime = new Intl.DateTimeFormat(tag, {
      dateStyle: "long",
      timeStyle: "short",
      timeZone: "America/Bahia",
    });
    const day = new Intl.DateTimeFormat(tag, {
      dateStyle: "long",
      timeZone: "America/Bahia",
    });
    const number = new Intl.NumberFormat(tag, { maximumFractionDigits: 2 });
    return {
      dateTime: (iso) => dateTime.format(new Date(iso)),
      day: (iso) => day.format(new Date(iso)),
      number: (value) => number.format(value),
    };
  }, [tag]);
}

/**
 * O ponto mais recente da própria série externa. Escolher um elemento não é
 * combinar camadas: nada aqui conhece cobertura, amostra ou célula protegida.
 */
function latestPoint(series: readonly SeriesPoint[]): SeriesPoint {
  return series.reduce((latest, point) =>
    Date.parse(point.period_start) > Date.parse(latest.period_start)
      ? point
      : latest,
  );
}

function ProvenanceRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <dt>{label}</dt>
      <dd>{children}</dd>
    </div>
  );
}

function OriginMoment({
  copy,
  format,
  observedAt,
}: {
  copy: ContextCopy;
  format: ContextFormat;
  observedAt: string | undefined;
}) {
  if (observedAt === undefined) {
    return <>{copy.observedAtMissing}</>;
  }
  return <time dateTime={observedAt}>{format.dateTime(observedAt)}</time>;
}

/**
 * Proveniência inline, no render inicial: nem tooltip, nem "saiba mais". A
 * licença CC-BY se cumpre onde a obra aparece, e o `data_mode` é por card
 * porque uma página que mistura clima real com presença fictícia sob um rótulo
 * global mente nas duas direções (ADR-045 §7).
 */
function CardProvenance({
  copy,
  dataMode,
  format,
  provenance,
}: {
  copy: ContextCopy;
  dataMode: Schemas["ExternalCardDataMode"];
  format: ContextFormat;
  provenance: Provenance;
}) {
  return (
    <dl className="external-card-provenance" data-card-provenance>
      <ProvenanceRow label={copy.sourceLabel}>{provenance.publisher}</ProvenanceRow>
      <ProvenanceRow label={copy.licenseLabel}>
        <a href={provenance.license_url} rel="noreferrer noopener" target="_blank">
          {provenance.license_code}
        </a>
      </ProvenanceRow>
      <ProvenanceRow label={copy.observedAtLabel}>
        <OriginMoment
          copy={copy}
          format={format}
          observedAt={provenance.observed_at}
        />
      </ProvenanceRow>
      <ProvenanceRow label={copy.retrievedAtLabel}>
        <time dateTime={provenance.retrieved_at}>
          {format.dateTime(provenance.retrieved_at)}
        </time>
      </ProvenanceRow>
      <ProvenanceRow label={copy.coveredPeriod}>
        {format.day(provenance.covered_period.start)} —{" "}
        {format.day(provenance.covered_period.end)}
      </ProvenanceRow>
      <ProvenanceRow label={copy.dataModeLabel}>
        <span data-card-data-mode={dataMode}>{copy.dataModes[dataMode]}</span>
      </ProvenanceRow>
      <ProvenanceRow label={copy.termsLabel}>
        <a href={provenance.terms_url} rel="noreferrer noopener" target="_blank">
          {provenance.terms_url}
        </a>
      </ProvenanceRow>
      <ProvenanceRow label={copy.attribution}>
        {provenance.attribution_text}
      </ProvenanceRow>
    </dl>
  );
}

function SeriesTable({
  copy,
  format,
  series,
  unit,
}: {
  copy: ContextCopy;
  format: ContextFormat;
  series: readonly SeriesPoint[];
  unit: string;
}) {
  return (
    <table className="external-series-table">
      <thead>
        <tr>
          <th scope="col">{copy.seriesPeriodColumn}</th>
          <th scope="col">{copy.seriesValueColumn}</th>
        </tr>
      </thead>
      <tbody>
        {series.map((point) => (
          <tr key={point.period_start}>
            <th scope="row">{format.day(point.period_start)}</th>
            <td>
              {format.number(point.value)} {unit}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function PublishedCardBody({
  card,
  copy,
  format,
  now,
}: {
  card: PublishedCard;
  copy: ContextCopy;
  format: ContextFormat;
  now: number;
}) {
  const point = latestPoint(card.series);
  const unit = copy.units[card.unit_code];
  const freshness = freshnessOf(
    card.provenance.retrieved_at,
    card.provenance.declared_lag_seconds,
    now,
  );
  return (
    <>
      <p className="external-card-freshness" data-card-freshness={freshness}>
        {copy.freshness[freshness]}
      </p>
      <p className="external-card-value" data-card-value>
        <strong>{format.number(point.value)}</strong>
        <span className="external-card-unit">{unit}</span>
      </p>
      <details className="external-series-details">
        <summary>{copy.seriesDetails}</summary>
        <div className="table-scroll">
          <SeriesTable
            copy={copy}
            format={format}
            series={card.series}
            unit={unit}
          />
        </div>
      </details>
    </>
  );
}

/**
 * Sem numeral, sem traço no eixo, sem barra de altura zero, sem linha: zero
 * desenhado é um valor afirmado, e não há valor nenhum a afirmar aqui. Só o
 * traço e o motivo, em linguagem leiga, a partir da lista fechada de
 * `reason_code`.
 */
function UnavailableCardBody({
  card,
  copy,
}: {
  card: UnavailableCard;
  copy: ContextCopy;
}) {
  return (
    <p className="external-card-value" data-card-value>
      <strong className="external-card-absent">—</strong>
      <span className="external-card-reason">
        {copy.unavailableLabel}. {copy.reasons[card.reason_code]}
      </span>
    </p>
  );
}

function CardBody({
  card,
  copy,
  format,
  now,
}: {
  card: Card;
  copy: ContextCopy;
  format: ContextFormat;
  now: number;
}) {
  if (card.status === "published") {
    return <PublishedCardBody card={card} copy={copy} format={format} now={now} />;
  }
  return <UnavailableCardBody card={card} copy={copy} />;
}

function ContextCard({
  card,
  copy,
  format,
  now,
}: {
  card: Card;
  copy: ContextCopy;
  format: ContextFormat;
  now: number;
}) {
  const titleId = `external-card-${card.card_code}`;
  return (
    <article
      aria-labelledby={titleId}
      className="external-card"
      data-card-code={card.card_code}
      data-card-status={card.status}
    >
      <h4 id={titleId}>{copy.cardTitles[card.card_code]}</h4>
      <CardBody card={card} copy={copy} format={format} now={now} />
      {card.card_code === "tide" ? (
        <p className="external-card-note">{copy.tideNote}</p>
      ) : null}
      <CardProvenance
        copy={copy}
        dataMode={card.data_mode}
        format={format}
        provenance={card.provenance}
      />
    </article>
  );
}

/**
 * Área de fontes creditadas. O Cadastur vive aqui e só aqui: atribuição e
 * link, sem contagem, sem card com valor e sem série de universo publicada
 * (ADR-045 §5, decisão U-7).
 */
function CreditedSources({
  copy,
  sources,
}: {
  copy: ContextCopy;
  sources: readonly CreditedSource[];
}) {
  return (
    <section
      aria-labelledby="external-sources-title"
      className="external-sources"
    >
      <h4 id="external-sources-title">{copy.sourcesTitle}</h4>
      <p>{copy.sourcesNote}</p>
      <ul>
        {sources.map((source) => (
          <li key={source.source_code} data-source-code={source.source_code}>
            <p className="external-source-name">{source.publisher}</p>
            <p className="external-source-attribution">
              {source.attribution_text}
            </p>
            <p className="external-source-links">
              <a href={source.license_url} rel="noreferrer noopener" target="_blank">
                {source.license_code}
              </a>
              <a href={source.terms_url} rel="noreferrer noopener" target="_blank">
                {copy.termsLabel}
              </a>
            </p>
          </li>
        ))}
      </ul>
    </section>
  );
}

function ContextPlaceholder({
  copy,
  failed,
  onRetry,
}: {
  copy: ContextCopy;
  failed: boolean;
  onRetry: () => void;
}) {
  if (!failed) {
    return (
      <p className="external-context-status" role="status">
        {copy.loading}
      </p>
    );
  }
  return (
    <div className="external-context-status">
      <p role="alert">{copy.errorTitle}</p>
      <button className="ghost" onClick={onRetry} type="button">
        {copy.errorRetry}
      </button>
    </div>
  );
}

export interface ExternalContextTabProps {
  client?: ContextClient;
  /** Injetável só para teste: o frescor precisa de um relógio determinístico. */
  now?: number;
}

export function ExternalContextTab({
  client = publicContextClient,
  now,
}: ExternalContextTabProps) {
  const { locale, tag } = useLocale();
  const copy = contextCopyFor(locale);
  const format = useContextFormat(tag);
  const context = useQuery({
    queryKey: ["external-context", "public"],
    queryFn: () => client.getContext(),
    staleTime: CONTEXT_STALE_TIME,
  });
  const contextDocument = context.data?.data;
  return (
    <section
      aria-labelledby="external-context-title"
      className="external-context"
    >
      <div className="section-heading">
        <div>
          <p className="section-kicker">{copy.kicker}</p>
          <h3 id="external-context-title">{copy.title}</h3>
          <p>{copy.lead}</p>
        </div>
      </div>
      <p className="external-context-disclaimer">{copy.noCausalClaim}</p>
      {contextDocument === undefined ? (
        <ContextPlaceholder
          copy={copy}
          failed={context.isError}
          onRetry={() => void context.refetch()}
        />
      ) : (
        <>
          <div className="external-cards">
            {contextDocument.cards.map((card) => (
              <ContextCard
                card={card}
                copy={copy}
                format={format}
                key={card.card_code}
                now={now ?? Date.now()}
              />
            ))}
          </div>
          <CreditedSources copy={copy} sources={contextDocument.sources} />
        </>
      )}
    </section>
  );
}
