import type { components, operations } from "../../generated/schema";
import {
  isMethodology,
  isPreferences,
  isPresence,
  isFunnel,
  isQuality,
  isSummary,
} from "./analytics-payload";
import { type HttpClientOptions } from "./http-client";
import {
  createDocumentReader,
  type DocumentResult,
} from "./public-document-client";

type Schemas = components["schemas"];
type PresenceWindow = components["parameters"]["PresenceWindow"];
type PresenceMonth = components["parameters"]["PresenceMonth"];
type PreferencePeriod = components["parameters"]["PreferencePeriod"];
type QualityWindow = components["parameters"]["QualityWindow"];

/**
 * The published metrics surface. It does not share the mutation transport: all
 * five operations are GETs, four of them are anonymous and cached by shared
 * caches, and every body is checked against a runtime allowlist before it
 * reaches a chart.
 */
export const analyticsOperationNames = [
  "getPublicSummary",
  "getPublicPresence",
  "getPublicPreferences",
  "getPublicMethodology",
  "getAnalyticsQuality",
  "getAnalyticsFunnel",
] as const satisfies readonly (keyof operations)[];

/**
 * `month` só acompanha `window=month`: enviado em qualquer outra janela o
 * servidor recusa a requisição, e omiti-lo dentro dela não nomeia documento.
 */
function presenceQuery(window: PresenceWindow, month?: PresenceMonth) {
  const query = new URLSearchParams({ window });
  if (window === "month" && month !== undefined) {
    query.set("month", month);
  }
  return query.toString();
}

export type AnalyticsResult<T> = DocumentResult<T>;

export function createAnalyticsClient(options: HttpClientOptions) {
  const reader = createDocumentReader(options);
  const published = reader.published;

  return {
    getSummary: () =>
      published<Schemas["PublicSummary"]>("/api/v1/public/summary", isSummary),
    getPresence: (window: PresenceWindow, month?: PresenceMonth) =>
      published<Schemas["PublicPresence"]>(
        `/api/v1/public/presence?${presenceQuery(window, month)}`,
        isPresence,
      ),
    getPreferences: (period: PreferencePeriod = "last_complete_month") =>
      published<Schemas["PublicPreferences"]>(
        `/api/v1/public/preferences?period=${encodeURIComponent(period)}`,
        isPreferences,
      ),
    getMethodology: () =>
      published<Schemas["PublicMethodology"]>(
        "/api/v1/public/methodology",
        isMethodology,
      ),
    getFunnel: (window: QualityWindow = "last_30_days") =>
      reader.authenticated<Schemas["AdoptionFunnel"]>(
        `/api/v1/analytics/funnel?window=${encodeURIComponent(window)}`,
        isFunnel,
      ),
    getQuality: (window: QualityWindow = "last_30_days") =>
      reader.authenticated<Schemas["QualitySnapshot"]>(
        `/api/v1/analytics/quality?window=${encodeURIComponent(window)}`,
        isQuality,
      ),
  };
}

export type AnalyticsClient = ReturnType<typeof createAnalyticsClient>;

export const publicAnalyticsClient = createAnalyticsClient({
  getAccessToken: () => null,
});
