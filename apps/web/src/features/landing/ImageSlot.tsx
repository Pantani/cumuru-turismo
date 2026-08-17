import { useLocale } from "../../shared/i18n/LocaleProvider";
import type { MessageKey } from "../../shared/i18n/translate";

/**
 * Espaço reservado de foto. O design chega com seis vagas e nenhuma imagem da
 * vila ainda foi fotografada; até lá o quadro anuncia o que deve entrar em vez
 * de esconder a lacuna atrás de uma foto genérica de banco de imagens.
 *
 * É decorativo para a árvore de acessibilidade (`aria-hidden`): o texto da vaga
 * descreve a foto que virá, não conteúdo que o leitor precise ouvir hoje.
 */
export function ImageSlot({ caption }: { caption: MessageKey }) {
  const { t } = useLocale();
  return (
    <div className="lp-image-slot" aria-hidden="true">
      <span className="lp-image-slot-tag">{t("landing.imagePending")}</span>
      <span className="lp-image-slot-caption">{t(caption)}</span>
    </div>
  );
}
