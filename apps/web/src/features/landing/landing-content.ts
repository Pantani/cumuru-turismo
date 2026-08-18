import type { SlotPhoto } from "./ImageSlot";
import type { MessageKey } from "../../shared/i18n/translate";

/** Endereço público de contato do Observatório, reutilizado nos dois botões. */
export const CONTACT_EMAIL = "observatorio@cumuruxatiba.tur.br";
export const DPO_EMAIL = "dpo@cumuruxatiba.tur.br";

/**
 * As seis vagas de foto da página, todas preenchidas com imagem real de
 * Cumuruxatiba vinda do Wikimedia Commons. Nenhuma cena é encenada ou gerada:
 * a legenda de cada vaga descreve o que a foto mostra de fato, porque um site
 * público de transparência não pode ilustrar a vila com imagem inventada.
 *
 * O crédito ao autor e à licença é obrigatório em CC BY/CC BY-SA e o
 * `ImageSlot` o imprime junto da foto — por isso `author`, `license` e
 * `sourceUrl` não são opcionais.
 */
export const HERO_PHOTO: SlotPhoto = {
  src: "/images/hero-falesia-cumuruxatiba.jpg",
  srcWebp: "/images/hero-falesia-cumuruxatiba.webp",
  author: "Tristão José Macedo",
  license: "landing.license.ccBySa40",
  sourceUrl:
    "https://commons.wikimedia.org/wiki/File:Vista_do_mar_de_cima_de_uma_fal%C3%A9sia_em_Cumuruxatiba,_Bahia,_Brasil.jpg",
};

/** Pórtico de entrada da vila, na estrada de acesso. */
export const GATE_PHOTO: SlotPhoto = {
  src: "/images/portico-entrada-cumuruxatiba.jpg",
  srcWebp: "/images/portico-entrada-cumuruxatiba.webp",
  author: "Vilamir Azevedo",
  license: "landing.license.ccBySa30",
  sourceUrl:
    "https://commons.wikimedia.org/wiki/File:Cumuruxatiba_-_panoramio.jpg",
};

/** Rua da vila sob os coqueiros. */
export const STREET_PHOTO: SlotPhoto = {
  src: "/images/rua-coqueiros-cumuruxatiba.jpg",
  srcWebp: "/images/rua-coqueiros-cumuruxatiba.webp",
  author: "Rômulo Gama Ferreira",
  license: "landing.license.ccBy20",
  sourceUrl:
    "https://commons.wikimedia.org/wiki/File:Cumuruxatiba_-_BA_(50988762481).jpg",
};

/** Praça do centro, onde ficam a pousada, o camping e o comércio da vila. */
export const SQUARE_PHOTO: SlotPhoto = {
  src: "/images/praca-centro-cumuruxatiba.jpg",
  srcWebp: "/images/praca-centro-cumuruxatiba.webp",
  author: "Márcio Filho / MTur Destinos",
  license: "landing.license.publicDomain",
  sourceUrl:
    "https://commons.wikimedia.org/wiki/File:MARCIO_FILHO_PRACA_DO_CENTRO_DE_CUMURUXATIBA_PRADO_BAHIA_(26207038957).jpg",
};

/** Rede armada na varanda de uma hospedagem da vila. */
export const HAMMOCK_PHOTO: SlotPhoto = {
  src: "/images/rede-varanda-cumuruxatiba.jpg",
  srcWebp: "/images/rede-varanda-cumuruxatiba.webp",
  author: "Pit Thomspon",
  license: "landing.license.ccBy20",
  sourceUrl:
    "https://commons.wikimedia.org/wiki/File:Cumuruxatiba,_BA_-_5142824408.jpg",
};

/** Praia do Centro na maré baixa, com as falésias da Costa das Baleias ao fundo. */
export const BEACH_PHOTO: SlotPhoto = {
  src: "/images/praia-centro-falesias-cumuruxatiba.jpg",
  srcWebp: "/images/praia-centro-falesias-cumuruxatiba.webp",
  author: "Pit Thomspon",
  license: "landing.license.ccBy20",
  sourceUrl:
    "https://commons.wikimedia.org/wiki/File:Cumuruxatiba,_BA_-_4971094684.jpg",
};

export interface LandingEntry {
  body: MessageKey;
  title: MessageKey;
}

export interface SectionAnchor {
  id: string;
  label: MessageKey;
}

/**
 * Âncoras da barra de seções. A ordem espelha a ordem de leitura da página:
 * um índice fora de ordem faria o marcador de seção ativa mentir.
 */
export const SECTION_ANCHORS: readonly SectionAnchor[] = [
  { id: "numeros", label: "landing.nav.numbers" },
  { id: "como", label: "landing.nav.how" },
  { id: "anfitrioes", label: "landing.nav.hosts" },
  { id: "comercio", label: "landing.nav.commerce" },
  { id: "privacidade", label: "landing.nav.privacy" },
  { id: "sobre", label: "landing.nav.about" },
];

export const HOW_STEPS: readonly LandingEntry[] = [
  { title: "landing.how.step1.title", body: "landing.how.step1.body" },
  { title: "landing.how.step2.title", body: "landing.how.step2.body" },
  { title: "landing.how.step3.title", body: "landing.how.step3.body" },
  { title: "landing.how.step4.title", body: "landing.how.step4.body" },
];

export const HOST_BENEFITS: readonly LandingEntry[] = [
  {
    title: "landing.hosts.benefit1.title",
    body: "landing.hosts.benefit1.body",
  },
  {
    title: "landing.hosts.benefit2.title",
    body: "landing.hosts.benefit2.body",
  },
  {
    title: "landing.hosts.benefit3.title",
    body: "landing.hosts.benefit3.body",
  },
  {
    title: "landing.hosts.benefit4.title",
    body: "landing.hosts.benefit4.body",
  },
];

export const COMMERCE_ITEMS: readonly LandingEntry[] = [
  {
    title: "landing.commerce.item1.title",
    body: "landing.commerce.item1.body",
  },
  {
    title: "landing.commerce.item2.title",
    body: "landing.commerce.item2.body",
  },
  {
    title: "landing.commerce.item3.title",
    body: "landing.commerce.item3.body",
  },
  {
    title: "landing.commerce.item4.title",
    body: "landing.commerce.item4.body",
  },
];

export const PRIVACY_ITEMS: readonly LandingEntry[] = [
  { title: "landing.privacy.item1.title", body: "landing.privacy.item1.body" },
  { title: "landing.privacy.item2.title", body: "landing.privacy.item2.body" },
  { title: "landing.privacy.item3.title", body: "landing.privacy.item3.body" },
  { title: "landing.privacy.item4.title", body: "landing.privacy.item4.body" },
];

export interface FaqEntry {
  answer: MessageKey;
  question: MessageKey;
}

export const FAQ_ENTRIES: readonly FaqEntry[] = [
  { question: "landing.faq.q1.question", answer: "landing.faq.q1.answer" },
  { question: "landing.faq.q2.question", answer: "landing.faq.q2.answer" },
  { question: "landing.faq.q3.question", answer: "landing.faq.q3.answer" },
  { question: "landing.faq.q4.question", answer: "landing.faq.q4.answer" },
  { question: "landing.faq.q5.question", answer: "landing.faq.q5.answer" },
  { question: "landing.faq.q6.question", answer: "landing.faq.q6.answer" },
];

export interface GuideLink {
  href: string;
  meta: MessageKey;
  title: MessageKey;
}

/** Os dois PDFs já versionados no repositório, servidos por `public/guias`. */
export const GUIDE_LINKS: readonly GuideLink[] = [
  {
    href: "/guias/observatorio-prefeitura.pdf",
    meta: "landing.about.guideCityHallMeta",
    title: "landing.about.guideCityHall",
  },
  {
    href: "/guias/chave-fnrh-hospedagens.pdf",
    meta: "landing.about.guideFnrhMeta",
    title: "landing.about.guideFnrh",
  },
];
