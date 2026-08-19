/**
 * Texto da aba de contexto externo, nos três idiomas publicados.
 *
 * O catálogo mora na própria aba, e não no catálogo global, porque nenhuma
 * destas frases descreve a série medida: elas descrevem uma camada que a
 * ADR-045 mantém separada de propósito. O tipo `Record<Locale, ContextCopy>`
 * é o que impede tradução faltando — não há chave que exista em português e
 * não exista nos outros dois.
 *
 * O texto de atribuição de cada fonte **não** está aqui: ele vem do payload
 * (`attribution_text`), porque texto de licença concatenado em código é texto
 * de licença que ninguém revisa (ADR-045 §7).
 */

import type { components } from "../../../generated/schema";
import type { Locale } from "../../../shared/i18n/locale";
import type { Freshness } from "./context-freshness";

type Schemas = components["schemas"];
type CardCode = Schemas["ExternalCardCode"];
type UnitCode = Schemas["ExternalUnitCode"];
type DataMode = Schemas["ExternalCardDataMode"];
type ReasonCode = Schemas["UnavailableContextCard"]["reason_code"];

export interface ContextCopy {
  attribution: string;
  cardTitles: Record<CardCode, string>;
  coveredPeriod: string;
  dataModeLabel: string;
  dataModes: Record<DataMode, string>;
  errorRetry: string;
  errorTitle: string;
  freshness: Record<Freshness, string>;
  kicker: string;
  lead: string;
  licenseLabel: string;
  loading: string;
  noCausalClaim: string;
  observedAtLabel: string;
  observedAtMissing: string;
  reasons: Record<ReasonCode, string>;
  retrievedAtLabel: string;
  seriesDetails: string;
  seriesPeriodColumn: string;
  seriesValueColumn: string;
  sourceLabel: string;
  sourcesNote: string;
  sourcesTitle: string;
  tabLabel: string;
  tabsLabel: string;
  termsLabel: string;
  tideNote: string;
  title: string;
  unavailableLabel: string;
  units: Record<UnitCode, string>;
}

const pt: ContextCopy = {
  attribution: "Atribuição da fonte",
  cardTitles: {
    weather_daily: "Clima do dia",
    tide: "Maré",
  },
  coveredPeriod: "Período coberto",
  dataModeLabel: "Origem do dado",
  dataModes: {
    real_source: "fonte externa real",
    prototype_fixtures: "dado fictício de protótipo",
  },
  errorRetry: "Tentar de novo",
  errorTitle:
    "Não foi possível carregar o contexto externo. O painel de presença não depende desta camada e continua como está.",
  freshness: {
    fresh: "Coleta recente",
    stale: "Coleta defasada: o número abaixo é o último que a fonte nos deu.",
  },
  kicker: "Camada de contexto",
  lead: "Informação copiada de fontes de fora, exibida com o crédito de quem a publicou.",
  licenseLabel: "Licença",
  loading: "Carregando o contexto externo.",
  noCausalClaim:
    "Isto é contexto externo. Não é medição do Observatório, está fora da metodologia do painel de presença e não afirma nenhuma relação entre estes números e a presença de pessoas na vila.",
  observedAtLabel: "Momento a que o dado se refere na fonte",
  observedAtMissing: "A fonte não publicou momento de referência.",
  reasons: {
    source_unavailable: "A fonte não respondeu na última tentativa.",
    source_rate_limited: "A fonte limitou o número de consultas do dia.",
    source_not_licensed: "Ainda não temos permissão de uso para publicar este dado.",
    source_data_missing: "A fonte respondeu, mas não publicou este dado.",
    constants_not_imported:
      "A tábua oficial da Marinha do Brasil para esta parte da costa ainda não está disponível para nós. Sem ela, qualquer horário de maré aqui seria um palpite, e por isso não mostramos nenhum.",
    stale_beyond_declared_lag:
      "O dado mais recente da fonte é antigo demais para ser exibido.",
  },
  retrievedAtLabel: "Momento em que buscamos o dado",
  seriesDetails: "Ver todos os dias desta fonte",
  seriesPeriodColumn: "Dia",
  seriesValueColumn: "Valor na fonte",
  sourceLabel: "Fonte",
  sourcesNote:
    "Fontes creditadas desta aba. O Cadastur é o registro público de hospedagens do Ministério do Turismo: aparece aqui como crédito e link, sem nenhuma contagem calculada pelo Observatório.",
  sourcesTitle: "Fontes e licenças",
  tabLabel: "Contexto externo",
  tabsLabel: "Camadas do painel",
  termsLabel: "Termos de uso",
  tideNote:
    "Enquanto a tábua oficial não chegar, esta caixa não mostra curva, horário de preamar nem de baixa-mar.",
  title: "Contexto externo, fora da metodologia do Observatório",
  unavailableLabel: "Indisponível",
  units: {
    celsius: "°C",
    millimetre: "mm",
    metre: "m",
    metre_per_second: "m/s",
    pageview: "visitas de página",
    person: "pessoas",
    brl: "R$",
    count: "ocorrências",
    degree: "°",
  },
};

const en: ContextCopy = {
  attribution: "Source attribution",
  cardTitles: {
    weather_daily: "Weather of the day",
    tide: "Tide",
  },
  coveredPeriod: "Period covered",
  dataModeLabel: "Data origin",
  dataModes: {
    real_source: "real external source",
    prototype_fixtures: "prototype fixture data",
  },
  errorRetry: "Try again",
  errorTitle:
    "The external context could not be loaded. The presence panel does not depend on this layer and is unchanged.",
  freshness: {
    fresh: "Recently collected",
    stale: "Stale collection: the number below is the last one the source gave us.",
  },
  kicker: "Context layer",
  lead: "Information copied from outside sources, shown with credit to whoever published it.",
  licenseLabel: "License",
  loading: "Loading the external context.",
  noCausalClaim:
    "This is external context. It is not measured by the Observatory, it sits outside the methodology of the presence panel, and it claims no relationship between these numbers and how many people are in the village.",
  observedAtLabel: "Moment the data refers to at the source",
  observedAtMissing: "The source published no reference moment.",
  reasons: {
    source_unavailable: "The source did not answer on the last attempt.",
    source_rate_limited: "The source capped the number of queries for the day.",
    source_not_licensed: "We do not yet have permission to publish this data.",
    source_data_missing: "The source answered but published no such data.",
    constants_not_imported:
      "The official tide table from the Brazilian Navy for this stretch of coast is not available to us yet. Without it, any tide time here would be a guess, so we show none.",
    stale_beyond_declared_lag:
      "The newest data at the source is too old to be shown.",
  },
  retrievedAtLabel: "Moment we fetched the data",
  seriesDetails: "See every day from this source",
  seriesPeriodColumn: "Day",
  seriesValueColumn: "Value at the source",
  sourceLabel: "Source",
  sourcesNote:
    "Credited sources of this tab. Cadastur is the Brazilian Ministry of Tourism public register of lodgings: it appears here as credit and link, with no count calculated by the Observatory.",
  sourcesTitle: "Sources and licenses",
  tabLabel: "External context",
  tabsLabel: "Panel layers",
  termsLabel: "Terms of use",
  tideNote:
    "Until the official table arrives, this box shows no curve and no high or low tide time.",
  title: "External context, outside the Observatory methodology",
  unavailableLabel: "Unavailable",
  units: {
    celsius: "°C",
    millimetre: "mm",
    metre: "m",
    metre_per_second: "m/s",
    pageview: "page views",
    person: "people",
    brl: "BRL",
    count: "occurrences",
    degree: "°",
  },
};

const es: ContextCopy = {
  attribution: "Atribución de la fuente",
  cardTitles: {
    weather_daily: "Clima del día",
    tide: "Marea",
  },
  coveredPeriod: "Período cubierto",
  dataModeLabel: "Origen del dato",
  dataModes: {
    real_source: "fuente externa real",
    prototype_fixtures: "dato ficticio de prototipo",
  },
  errorRetry: "Intentar de nuevo",
  errorTitle:
    "No fue posible cargar el contexto externo. El panel de presencia no depende de esta capa y sigue igual.",
  freshness: {
    fresh: "Recolección reciente",
    stale:
      "Recolección desactualizada: el número de abajo es el último que la fuente nos dio.",
  },
  kicker: "Capa de contexto",
  lead: "Información copiada de fuentes externas, mostrada con el crédito de quien la publicó.",
  licenseLabel: "Licencia",
  loading: "Cargando el contexto externo.",
  noCausalClaim:
    "Esto es contexto externo. No lo mide el Observatorio, está fuera de la metodología del panel de presencia y no afirma ninguna relación entre estos números y la presencia de personas en el pueblo.",
  observedAtLabel: "Momento al que se refiere el dato en la fuente",
  observedAtMissing: "La fuente no publicó momento de referencia.",
  reasons: {
    source_unavailable: "La fuente no respondió en el último intento.",
    source_rate_limited: "La fuente limitó la cantidad de consultas del día.",
    source_not_licensed: "Todavía no tenemos permiso de uso para publicar este dato.",
    source_data_missing: "La fuente respondió, pero no publicó este dato.",
    constants_not_imported:
      "La tabla oficial de la Marina de Brasil para este tramo de costa aún no está disponible para nosotros. Sin ella, cualquier horario de marea aquí sería una suposición, y por eso no mostramos ninguno.",
    stale_beyond_declared_lag:
      "El dato más reciente de la fuente es demasiado antiguo para mostrarse.",
  },
  retrievedAtLabel: "Momento en que buscamos el dato",
  seriesDetails: "Ver todos los días de esta fuente",
  seriesPeriodColumn: "Día",
  seriesValueColumn: "Valor en la fuente",
  sourceLabel: "Fuente",
  sourcesNote:
    "Fuentes acreditadas de esta pestaña. Cadastur es el registro público de alojamientos del Ministerio de Turismo de Brasil: aparece aquí como crédito y enlace, sin ningún conteo calculado por el Observatorio.",
  sourcesTitle: "Fuentes y licencias",
  tabLabel: "Contexto externo",
  tabsLabel: "Capas del panel",
  termsLabel: "Términos de uso",
  tideNote:
    "Mientras la tabla oficial no llegue, esta caja no muestra curva ni horario de pleamar o bajamar.",
  title: "Contexto externo, fuera de la metodología del Observatorio",
  unavailableLabel: "No disponible",
  units: {
    celsius: "°C",
    millimetre: "mm",
    metre: "m",
    metre_per_second: "m/s",
    pageview: "vistas de página",
    person: "personas",
    brl: "BRL",
    count: "ocurrencias",
    degree: "°",
  },
};

const CONTEXT_CATALOG: Record<Locale, ContextCopy> = { pt, en, es };

export function contextCopyFor(locale: Locale): ContextCopy {
  return CONTEXT_CATALOG[locale];
}
