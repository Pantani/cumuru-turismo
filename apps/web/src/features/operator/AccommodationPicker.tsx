import {
  accommodationCategoryLabels,
  type Accommodation,
} from "./stay-lifecycle";

interface AccommodationPickerProps {
  accommodations: readonly Accommodation[];
  loading: boolean;
  onSelect: (accommodation: Accommodation) => void;
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
 * Cadastrar hospedagem é ato da administração (`accommodations:onboard`, ADR-042),
 * e a área da hospedagem não o oferece a ninguém — nem a quem carrega o escopo,
 * porque para esse a tela é outra. Calar seria pior que um botão morto: quem
 * chega aqui tem conta e nenhuma hospedagem, e a saída é o pedido de acesso.
 */
function EmptyState() {
  return (
    <div className="empty-state">
      <p>Você ainda não tem hospedagem cadastrada nesta conta.</p>
      <p>
        Quem cadastra hospedagem na plataforma é a administração. Peça o
        cadastro pela <a href="/convite">página de pedido de acesso</a>. Depois
        que a administração aprovar, ela envia o link que liga a hospedagem à
        sua conta.
      </p>
    </div>
  );
}

export function AccommodationPicker({
  accommodations,
  loading,
  onSelect,
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
    return <EmptyState />;
  }
  return (
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
  );
}
