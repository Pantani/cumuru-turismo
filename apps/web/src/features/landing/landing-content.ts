import type { SlotPhoto } from "./ImageSlot";
import type { MessageKey } from "../../shared/i18n/translate";

/** Endereço público de contato do Observatório, reutilizado nos dois botões. */
export const CONTACT_EMAIL = "observatorio@cumuruxatiba.tur.br";
export const DPO_EMAIL = "dpo@cumuruxatiba.tur.br";

/**
 * Única vaga de foto hoje preenchida com uma imagem real do lugar — as
 * outras cinco descrevem cenas que ainda não existem em banco livre (recepção,
 * retrato de anfitrião, mapa) e continuam como vaga anunciada em vez de foto
 * fora do enquadramento. Fonte: Wikimedia Commons, licença livre com
 * atribuição obrigatória.
 */
export const HERO_PHOTO: SlotPhoto = {
  src: "/images/hero-falesia-cumuruxatiba.jpg",
  srcWebp: "/images/hero-falesia-cumuruxatiba.webp",
  author: "Tristão José Macedo",
  license: "CC BY-SA 4.0",
  sourceUrl:
    "https://commons.wikimedia.org/wiki/File:Vista_do_mar_de_cima_de_uma_fal%C3%A9sia_em_Cumuruxatiba,_Bahia,_Brasil.jpg",
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
  { title: "landing.hosts.benefit1.title", body: "landing.hosts.benefit1.body" },
  { title: "landing.hosts.benefit2.title", body: "landing.hosts.benefit2.body" },
  { title: "landing.hosts.benefit3.title", body: "landing.hosts.benefit3.body" },
  { title: "landing.hosts.benefit4.title", body: "landing.hosts.benefit4.body" },
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
