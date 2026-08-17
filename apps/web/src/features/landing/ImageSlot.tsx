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
  license: string;
  sourceUrl: string;
};

/**
 * Vaga de imagem. O design chega com seis vagas; até uma foto real de
 * Cumuruxatiba ocupar cada uma, o quadro anuncia o que deve entrar em vez de
 * esconder a lacuna atrás de uma foto genérica de banco de imagens.
 *
 * Sem `photo`, o quadro é decorativo para a árvore de acessibilidade
 * (`aria-hidden`): o texto da vaga descreve a foto que virá, não conteúdo que
 * o leitor precise ouvir hoje.
 */
export function ImageSlot({
  caption,
  photo,
}: {
  caption: MessageKey;
  photo?: SlotPhoto;
}) {
  const { t } = useLocale();

  if (photo !== undefined) {
    return (
      <figure className="lp-image-slot lp-image-slot-photo">
        <picture>
          {photo.srcWebp === undefined ? null : (
            <source srcSet={photo.srcWebp} type="image/webp" />
          )}
          <img src={photo.src} alt={t(caption)} loading="lazy" />
        </picture>
        <figcaption className="lp-image-slot-credit">
          {t("landing.photoCredit", {
            author: photo.author,
            license: photo.license,
          })}
          <a href={photo.sourceUrl} rel="noreferrer" target="_blank">
            ↗
          </a>
        </figcaption>
      </figure>
    );
  }

  return (
    <div className="lp-image-slot" aria-hidden="true">
      <span className="lp-image-slot-tag">{t("landing.imagePending")}</span>
      <span className="lp-image-slot-caption">{t(caption)}</span>
    </div>
  );
}
