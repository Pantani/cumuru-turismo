import { useAuthSession } from "../../shared/auth/AuthSession";
import {
  accommodationCategoryLabels,
  type Accommodation,
} from "./stay-lifecycle";

/**
 * Admitting an establishment is the administrator's act: `accommodations:onboard`
 * gates POST /accommodations, and the operator does not hold it (ADR-042). The
 * gate returns `null` before mounting, and an affordance that never mounts fires
 * neither the request nor the `403`.
 */
const ONBOARD_SCOPE = "accommodations:onboard";

interface AccommodationPickerProps {
  accommodations: readonly Accommodation[];
  loading: boolean;
  onSelect: (accommodation: Accommodation) => void;
  onStartOnboarding: () => void;
  selectedId: string | null;
}

function AccommodationCard({
  accommodation,
  onSelect,
  selected,
}: {
  accommodation: Accommodation;
  onSelect: (accommodation: Accommodation) => void;
  selected: boolean;
}) {
  return (
    <li>
      <button
        type="button"
        className="property-card"
        aria-pressed={selected}
        onClick={() => onSelect(accommodation)}
      >
        <strong>{accommodation.name}</strong>
        <span>{accommodationCategoryLabels[accommodation.category]}</span>
        <span className="property-capacity">
          {accommodation.capacity === undefined || accommodation.capacity === null
            ? "Capacidade não informada"
            : `Até ${accommodation.capacity} pessoas`}
        </span>
      </button>
    </li>
  );
}

/**
 * Without the scope the empty screen states the path instead of offering a
 * button that can only answer `403`. Saying nothing would be worse than the
 * dead button: whoever lands here has an account and no lodging, and the way
 * out is the access request of ADR-042, which the administration decides.
 */
function EmptyStateGuidance() {
  return (
    <>
      <p>Você ainda não tem hospedagem cadastrada nesta conta.</p>
      <p>
        Quem cadastra hospedagem na plataforma é a administração. Peça o
        cadastro pela <a href="/convite">página de pedido de acesso</a>. Depois
        que a administração aprovar, ela envia o link que liga a hospedagem à
        sua conta.
      </p>
    </>
  );
}

function EmptyState({ onStartOnboarding }: { onStartOnboarding: () => void }) {
  const { hasScope } = useAuthSession();
  return (
    <div className="empty-state">
      {hasScope(ONBOARD_SCOPE)
        ? (
          <>
            <p>
              Você ainda não tem hospedagem cadastrada. Cadastre a sua para
              começar a registrar as estadias.
            </p>
            <button
              type="button"
              className="primary-action"
              onClick={onStartOnboarding}
            >
              Cadastrar minha hospedagem
            </button>
          </>
        )
        : <EmptyStateGuidance />}
    </div>
  );
}

function OnboardAnotherAction({
  onStartOnboarding,
}: {
  onStartOnboarding: () => void;
}) {
  const { hasScope } = useAuthSession();
  if (!hasScope(ONBOARD_SCOPE)) {
    return null;
  }
  return (
    <button type="button" className="ghost-action" onClick={onStartOnboarding}>
      Cadastrar outra hospedagem
    </button>
  );
}

export function AccommodationPicker({
  accommodations,
  loading,
  onSelect,
  onStartOnboarding,
  selectedId,
}: AccommodationPickerProps) {
  if (loading) {
    return (
      <p className="loading-note" role="status">
        Carregando suas hospedagens…
      </p>
    );
  }
  if (accommodations.length === 0) {
    return <EmptyState onStartOnboarding={onStartOnboarding} />;
  }
  return (
    <>
      <ul className="property-grid">
        {accommodations.map((accommodation) => (
          <AccommodationCard
            key={accommodation.id}
            accommodation={accommodation}
            onSelect={onSelect}
            selected={accommodation.id === selectedId}
          />
        ))}
      </ul>
      <OnboardAnotherAction onStartOnboarding={onStartOnboarding} />
    </>
  );
}
