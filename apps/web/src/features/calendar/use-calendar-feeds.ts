import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";

import type { components } from "../../generated/schema";
import { useAuthSession } from "../../shared/auth/AuthSession";

type Schemas = components["schemas"];

export type CalendarFeed = Schemas["CalendarFeed"];
export type CalendarReservation = Schemas["CalendarReservation"];

const FEEDS_KEY = "calendar-feeds";
const RESERVATIONS_KEY = "calendar-reservations";

/**
 * A fila e a lista de feeds são estado de servidor: o worker as reescreve a cada
 * ciclo, sem que a tela peça nada. Guardá-las em `useState` faria a tela mostrar
 * um calendário que a origem já mudou (AGENTS.md).
 */
export function useCalendarFeeds(accommodationId: string) {
  const { coreClient: client } = useAuthSession();
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: [FEEDS_KEY, accommodationId],
    queryFn: () => client.listCalendarFeeds(accommodationId),
    select: (result) => result.data.items,
  });

  const reload = useCallback(
    () => queryClient.invalidateQueries({ queryKey: [FEEDS_KEY, accommodationId] }),
    [accommodationId, queryClient],
  );

  return { feeds: query.data ?? [], error: query.error, loading: query.isPending, reload };
}

/**
 * A fila abre no pendente porque é a única lista com algo a fazer nela. O que
 * já foi confirmado virou estadia e aparece no quadro de estadias; o dispensado
 * e o retirado são histórico.
 */
export function useCalendarReservations(accommodationId: string) {
  const { coreClient: client } = useAuthSession();
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: [RESERVATIONS_KEY, accommodationId],
    queryFn: () =>
      client.listCalendarReservations(accommodationId, { state: "pending" }),
    select: (result) => result.data.items,
  });

  const reload = useCallback(async () => {
    await queryClient.invalidateQueries({
      queryKey: [RESERVATIONS_KEY, accommodationId],
    });
    // Confirmar cria estadia, então o quadro de estadias também está velho.
    await queryClient.invalidateQueries({ queryKey: ["stays"] });
  }, [accommodationId, queryClient]);

  return {
    reservations: query.data ?? [],
    error: query.error,
    loading: query.isPending,
    reload,
  };
}

export function entityTagFor(version: number) {
  return `"${version}"`;
}
