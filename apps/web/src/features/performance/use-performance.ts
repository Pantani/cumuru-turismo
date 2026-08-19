import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import type { PerformanceWindow } from "../../shared/api/core-client";
import { useAuthSession } from "../../shared/auth/AuthSession";
import { describeFailure } from "../operator/use-operation";
import type { PerformancePayload } from "./performance-summary";

const PERFORMANCE_QUERY_KEY = "accommodation-performance";

/**
 * As janelas do comparativo são as do contrato, menos a previsão. O catálogo é
 * fechado no servidor; repeti-lo aqui é o seletor da tela, não uma segunda
 * fonte de verdade.
 */
export const PERFORMANCE_WINDOWS = [
  "recent_30_days",
  "recent_90_days",
  "recent_365_days",
  "recent_730_days",
] as const satisfies readonly PerformanceWindow[];

export const PERFORMANCE_WINDOW_LABELS: Record<
  (typeof PERFORMANCE_WINDOWS)[number],
  string
> = {
  recent_30_days: "30 dias",
  recent_90_days: "90 dias",
  recent_365_days: "1 ano",
  recent_730_days: "2 anos",
};

export function usePerformance(accommodationId: string) {
  const { coreClient: client } = useAuthSession();
  const [window, setWindow] = useState<PerformanceWindow>("recent_90_days");

  const query = useQuery({
    queryKey: [PERFORMANCE_QUERY_KEY, accommodationId, window],
    queryFn: () => client.getAccommodationPerformance(accommodationId, window),
    select: (result): PerformancePayload => result.data,
  });

  return {
    failure: query.isError ? describeFailure(query.error) : null,
    loading: query.isPending,
    performance: query.data ?? null,
    setWindow,
    window,
  };
}
