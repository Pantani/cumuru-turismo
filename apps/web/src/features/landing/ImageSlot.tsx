import { useLocale } from "../../shared/i18n/LocaleProvider";
import type { MessageKey } from "../../shared/i18n/translate";

/**
 * Foto real com atribuição obrigatória (Wikimedia Commons/CC exige crédito
 * visível ao autor e à licença — não é estilo, é condição de uso).
 */
export type SlotPhoto = {
  src: string;
  srcWebp?: string;
  author: string;
  /** Chave do dicionário: só "domínio público" muda de idioma; CC é sigla fixa. */
  license: MessageKey;
  sourceUrl: string;
};

/**
 * Vaga de imagem da capa. Toda vaga carrega uma foto real de Cumuruxatiba, e
 * `caption` é o texto alternativo dela — descreve o que a imagem mostra, não a
 * cena que se gostaria de ter. `photo` é obrigatório justamente para que não
 * volte a existir quadro vazio anunciando foto que nunca chega.
 */
export function ImageSlot({
  caption,
  photo,
}: {
  caption: MessageKey;
  photo: SlotPhoto;
}) {
  const { t } = useLocale();

  return (
    <figure className="lp-image-slot">
      <picture>
        {photo.srcWebp === undefined ? null : (
          <source srcSet={photo.srcWebp} type="image/webp" />
        )}
        <img src={photo.src} alt={t(caption)} loading="lazy" />
      </picture>
      <figcaption className="lp-image-slot-credit">
        {t("landing.photoCredit", {
          author: photo.author,
          license: t(photo.license),
        })}
        <a href={photo.sourceUrl} rel="noreferrer" target="_blank">
          ↗
        </a>
      </figcaption>
    </figure>
  );
}
