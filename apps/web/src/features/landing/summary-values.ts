import type { components } from "../../generated/schema";
import type { Translate } from "../../shared/i18n/translate";
import type { PresenceFormat } from "../analytics/presence-format";

type Schemas = components["schemas"];
type Today = Schemas["PublicSummary"]["presence_today"];
type Peak = Schemas["ForecastPeak"];

function withheldLabel(t: Translate, status: "protected" | "unavailable") {
  return status === "protected"
    ? t("analytics.state.protected")
    : t("analytics.state.unavailable");
}

/**
 * Número de capa de um indicador, ou o motivo pelo qual ele não existe.
 *
 * A capa herda a regra do painel: célula protegida não ganha valor substituto,
 * ela diz que está protegida. Enquanto a leitura não chega, o rótulo é de
 * carregamento — nunca um zero, que se leria como "ninguém dormiu aqui hoje".
 *
 * O número passa pelo mesmo `Intl` do painel: mil e dez pessoas-dia é "1.010"
 * em português e "1,010" em inglês, na capa como na tabela logo abaixo.
 */
export function todayHeadline(
  t: Translate,
  format: PresenceFormat,
  point: Today | undefined,
): string {
  if (point === undefined) {
    return t("landing.hero.todayPending");
  }
  return point.status === "published"
    ? format.count(point.value)
    : withheldLabel(t, point.status);
}

export function peakHeadline(
  t: Translate,
  format: PresenceFormat,
  peak: Peak | undefined,
): string {
  if (peak === undefined) {
    return t("landing.hero.todayPending");
  }
  return peak.status === "published"
    ? format.count(peak.central)
    : withheldLabel(t, peak.status);
}
