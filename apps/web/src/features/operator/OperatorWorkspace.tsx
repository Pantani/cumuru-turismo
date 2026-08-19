import { useAuthSession } from "../../shared/auth/AuthSession";
import { AccommodationAccessPanel } from "../accommodation/AccommodationAccessPanel";
import { PublicListingPanel } from "../accommodation/PublicListingPanel";
import { useAccommodations } from "../accommodation/use-accommodations";
import { ApprovalQueue } from "../approval/ApprovalQueue";
import { AccommodationPicker } from "./AccommodationPicker";
import { StayBoard } from "./StayBoard";
import type { Accommodation } from "./stay-lifecycle";

/**
 * Área da hospedagem: registra estadia e decide o cadastro do hóspede que
 * chegou pelo código do lugar. Cadastrar hospedagem e decidir pedido de acesso
 * são atos da administração e vivem em `features/admin`, porque a fila da
 * ADR-042 não filtra por vínculo — quem a abre decide o pedido de qualquer
 * hospedagem da plataforma.
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

export function OperatorWorkspace() {
  const { accommodations, load, loading, operation, selected, setSelected } =
    useAccommodations("Carregando suas hospedagens");

  return (
    <div className="workspace">
      <section className="workspace-section" aria-labelledby="properties-title">
        <h2 id="properties-title">Suas hospedagens</h2>
        <LoadFailure operation={operation} />
        <AccommodationPicker
          accommodations={accommodations}
          loading={loading}
          onSelect={setSelected}
          selectedId={selected?.id ?? null}
        />
      </section>

      {selected === null ? null : (
        <>
          <StayBoard accommodation={selected} />
          <PublicListingPanel
            key={selected.id}
            accommodation={selected}
            onSaved={() => void load()}
          />
          <SelfServicePanels accommodation={selected} />
        </>
      )}
    </div>
  );
}
