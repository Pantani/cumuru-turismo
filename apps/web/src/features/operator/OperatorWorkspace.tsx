import { useCallback, useEffect, useState } from "react";

import { useAuthSession } from "../../shared/auth/AuthSession";
import { AccessRequestQueue } from "../access-request/AccessRequestQueue";
import { AccommodationAccessPanel } from "../accommodation/AccommodationAccessPanel";
import { ApprovalQueue } from "../approval/ApprovalQueue";
import { AccommodationOnboarding } from "./AccommodationOnboarding";
import { AccommodationPicker } from "./AccommodationPicker";
import { StayBoard } from "./StayBoard";
import type { Accommodation } from "./stay-lifecycle";
import { useOperation } from "./use-operation";

/**
 * Owns the accommodation list and the current selection. Extracted so the view
 * below stays declarative and the loading rules live in one place.
 */
function LoadFailure({ operation }: { operation: { message: string; tone: string } }) {
  if (operation.tone !== "failed") {
    return null;
  }
  return (
    <p className="operation-status tone-failed" role="alert">
      {operation.message}
    </p>
  );
}

/**
 * O servidor não registra rota alguma do autoatendimento quando `SELF_SERVICE_ENABLED` é falso
 * — e o default é falso —, mas o contrato não expõe nenhum campo de capacidade:
 * `HealthStatus` e `ReadinessStatus` são `additionalProperties: false` com valor
 * `const`, logo não há onde ler a flag sem mudar o contrato, que é lane do
 * platform.
 *
 * O canal que já existe é o escopo da sessão, o mesmo que `App.tsx` usa para as
 * Fases 3 e 4. `stays:approve` serve porque é concedido **exclusivamente** pelo
 * caminho de ativação do autoatendimento, e nenhuma conta semeada o carrega; ele vem do
 * servidor a cada login, então a mesma imagem atende runtime com a fase ligada
 * e desligada. Devolver `null` antes de montar é o que importa: painel não
 * montado não dispara efeito, e efeito não disparado não gera `404`.
 */
const SELF_SERVICE_SCOPE = "stays:approve";

/**
 * Aprovar um pedido cria a acomodação, então a fila custa a mesma permissão que
 * criar o cadastro à mão: `accommodations:onboard` (ADR-042). O painel devolve
 * `null` antes de montar, e painel não montado não dispara efeito nem `403`.
 */
const ACCESS_REQUEST_SCOPE = "accommodations:onboard";

function AccessRequestPanel() {
  const { hasScope } = useAuthSession();
  if (!hasScope(ACCESS_REQUEST_SCOPE)) {
    return null;
  }
  return <AccessRequestQueue />;
}

function SelfServicePanels({ accommodation }: { accommodation: Accommodation }) {
  const { hasScope } = useAuthSession();
  if (!hasScope(SELF_SERVICE_SCOPE)) {
    return null;
  }
  return (
    <>
      <ApprovalQueue accommodation={accommodation} />
      <AccommodationAccessPanel accommodation={accommodation} />
    </>
  );
}

function useAccommodations() {
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
    const result = await run("Carregando suas hospedagens", () =>
      client.listAccommodations(),
    );
    const items = result?.data.items ?? [];
    setAccommodations(items);
    setSelected((current) => current ?? items[0] ?? null);
    setLoading(false);
  }, [client, run]);

  useEffect(() => {
    void load();
  }, [load]);

  return { accommodations, load, loading, operation, selected, setSelected };
}

export function OperatorWorkspace() {
  const { accommodations, load, loading, operation, selected, setSelected } =
    useAccommodations();
  const [onboarding, setOnboarding] = useState(false);

  const handleCreated = useCallback(
    (accommodation: Accommodation) => {
      setOnboarding(false);
      setSelected(accommodation);
      void load();
    },
    [load, setSelected],
  );

  return (
    <div className="workspace">
      <section className="workspace-section" aria-labelledby="properties-title">
        <h2 id="properties-title">Suas hospedagens</h2>
        <LoadFailure operation={operation} />
        <AccommodationPicker
          accommodations={accommodations}
          loading={loading}
          onSelect={setSelected}
          onStartOnboarding={() => setOnboarding(true)}
          selectedId={selected?.id ?? null}
        />
        {onboarding ? (
          <AccommodationOnboarding
            onCancel={() => setOnboarding(false)}
            onCreated={handleCreated}
          />
        ) : null}
      </section>

      <AccessRequestPanel />

      {selected === null ? null : (
        <>
          <StayBoard accommodation={selected} />
          <SelfServicePanels accommodation={selected} />
        </>
      )}
    </div>
  );
}
