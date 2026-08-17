import type { components } from "../../generated/schema";
import type { Translate } from "../../shared/i18n/translate";

type Coverage = components["schemas"]["PublicMetadata"]["coverage"];

/**
 * Cobertura em uma frase. Capa e painel dizem a mesma coisa sobre a mesma
 * célula, então a regra mora em um lugar só.
 */
export function coverageText(t: Translate, coverage: Coverage): string {
  if (coverage.status === "published") {
    return t("analytics.coverage.published", { ratio: coverage.ratio });
  }
  return coverage.status === "protected"
    ? t("analytics.coverage.protected")
    : t("analytics.coverage.unavailable");
}
