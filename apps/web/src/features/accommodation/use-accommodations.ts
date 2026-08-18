import { useCallback, useEffect, useState } from "react";

import { useAuthSession } from "../../shared/auth/AuthSession";
import type { Accommodation } from "../operator/stay-lifecycle";
import { useOperation } from "../operator/use-operation";

/**
 * `GET /api/v1/accommodations` sempre devolve o que a conta alcança por
 * `core.memberships` — nunca o catálogo inteiro. Quem consome decide o que a
 * lista significa: para a hospedagem é o lugar que ela opera; para a
 * administração é o cadastro que ela admitiu e ainda precisa entregar o acesso.
 * O carregamento é o mesmo, então mora aqui em vez de nascer duas vezes.
 */
export function useAccommodations(label: string, autoSelect = true) {
  const { coreClient: client } = useAuthSession();
  const operation = useOperation();
  const { run } = operation;
  const [accommodations, setAccommodations] = useState<readonly Accommodation[]>(
    [],
  );
  const [selected, setSelected] = useState<Accommodation | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    const result = await run(label, () => client.listAccommodations());
    const items = result?.data.items ?? [];
    setAccommodations(items);
    // A hospedagem abre já no seu lugar; a administração escolhe. Selecionar
    // sozinho ali abriria o formulário de acesso de uma hospedagem que ninguém
    // pediu para tocar.
    setSelected((current) => (autoSelect ? (current ?? items[0] ?? null) : current));
    setLoading(false);
  }, [autoSelect, client, label, run]);

  useEffect(() => {
    void load();
  }, [load]);

  return { accommodations, load, loading, operation, selected, setSelected };
}
