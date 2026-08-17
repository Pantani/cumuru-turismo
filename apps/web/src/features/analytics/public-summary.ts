import { useQuery } from "@tanstack/react-query";

import {
  publicAnalyticsClient,
  type AnalyticsClient,
} from "../../shared/api/analytics-client";

/** Documento público e cacheável: cinco minutos evitam refetch a cada âncora. */
export const PUBLIC_STALE_TIME = 300_000;

/**
 * Chave única do resumo. A capa e o painel montam na mesma página; com a mesma
 * chave o React Query resolve as duas leituras com uma requisição só.
 */
export const PUBLIC_SUMMARY_KEY = ["analytics", "public", "summary"] as const;

export function usePublicSummary(client: AnalyticsClient = publicAnalyticsClient) {
  return useQuery({
    queryKey: PUBLIC_SUMMARY_KEY,
    queryFn: () => client.getSummary(),
    staleTime: PUBLIC_STALE_TIME,
  });
}
