import type { components } from "../../generated/schema";
import type { Translate } from "../../shared/i18n/translate";

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
 */
export function todayHeadline(
  t: Translate,
  point: Today | undefined,
): string | number {
  if (point === undefined) {
    return t("landing.hero.todayPending");
  }
  return point.status === "published"
    ? point.value
    : withheldLabel(t, point.status);
}

export function peakHeadline(
  t: Translate,
  peak: Peak | undefined,
): string | number {
  if (peak === undefined) {
    return t("landing.hero.todayPending");
  }
  return peak.status === "published"
    ? peak.central
    : withheldLabel(t, peak.status);
}
