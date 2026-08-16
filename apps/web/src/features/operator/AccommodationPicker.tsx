import {
  accommodationCategoryLabels,
  type Accommodation,
} from "./stay-lifecycle";

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

function EmptyState({ onStartOnboarding }: { onStartOnboarding: () => void }) {
  return (
    <div className="empty-state">
      <p>
        Você ainda não tem hospedagem cadastrada. Cadastre a sua para começar a
        registrar as estadias.
      </p>
      <button type="button" className="primary-action" onClick={onStartOnboarding}>
        Cadastrar minha hospedagem
      </button>
    </div>
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
      <button type="button" className="ghost-action" onClick={onStartOnboarding}>
        Cadastrar outra hospedagem
      </button>
    </>
  );
}
