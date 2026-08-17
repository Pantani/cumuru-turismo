import { useLocale } from "../../shared/i18n/LocaleProvider";
import type { MessageKey } from "../../shared/i18n/translate";

/**
 * Numeração editorial das seções: o número dá ao leitor a posição na página
 * inteira, o traço separa e o texto diz o assunto. O número é decorativo, então
 * fica fora da árvore de acessibilidade — o cabeçalho seguinte já nomeia a seção.
 */
export function SectionIndex({
  index,
  kicker,
}: {
  index: MessageKey;
  kicker: MessageKey;
}) {
  const { t } = useLocale();
  return (
    <p className="lp-index">
      <span className="lp-index-number" aria-hidden="true">
        {t(index)}
      </span>
      <span className="lp-index-rule" aria-hidden="true" />
      {t(kicker)}
    </p>
  );
}
