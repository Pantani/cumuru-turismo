import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useState } from "react";

import { useAuthSession } from "../../shared/auth/AuthSession";
import { describeFailure } from "../operator/use-operation";
import type { Accommodation } from "../operator/stay-lifecycle";

const ACCOMMODATIONS_QUERY_KEY = "accommodations";

interface LoadStatus {
  message: string;
  tone: "idle" | "working" | "done" | "failed";
}

function loadStatus(label: string, query: {
  error: unknown;
  isError: boolean;
  isPending: boolean;
}): LoadStatus {
  if (query.isError) {
    return { message: describeFailure(query.error), tone: "failed" };
  }
  return query.isPending
    ? { message: `${label}…`, tone: "working" }
    : { message: "", tone: "idle" };
}

/**
 * `GET /api/v1/accommodations` sempre devolve o que a conta alcança por
 * `core.memberships` — nunca o catálogo inteiro. Quem consome decide o que a
 * lista significa: para a hospedagem é o lugar que ela opera; para a
 * administração é o cadastro que ela admitiu e ainda precisa entregar o acesso.
 * O carregamento é o mesmo — e é estado de servidor, então vive no TanStack
 * Query (AGENTS.md) em vez de `useState` alimentado por efeito.
 */
export function useAccommodations(label: string, autoSelect = true) {
  const { coreClient: client } = useAuthSession();
  const queryClient = useQueryClient();
  const [selected, setSelected] = useState<Accommodation | null>(null);

  const query = useQuery({
    queryKey: [ACCOMMODATIONS_QUERY_KEY],
    queryFn: () => client.listAccommodations(),
    select: (result) => result.data.items,
  });
  const accommodations = query.data ?? [];

  // A hospedagem abre já no seu lugar; a administração escolhe. `current ?? …`
  // é o que evita que um refetch troque a seleção de quem já escolheu outra
  // hospedagem.
  useEffect(() => {
    const first = accommodations[0];
    if (!autoSelect || first === undefined) {
      return;
    }
    setSelected((current) => current ?? first);
  }, [autoSelect, accommodations]);

  const load = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: [ACCOMMODATIONS_QUERY_KEY] });
  }, [queryClient]);

  return {
    accommodations,
    load,
    loading: query.isPending,
    operation: loadStatus(label, query),
    selected,
    setSelected,
  };
}
