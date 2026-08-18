import { useCallback, useState } from "react";

import { useLocale } from "../../shared/i18n/LocaleProvider";
import { AccessRequestQueue } from "../access-request/AccessRequestQueue";
import { ActivationIssuePanel } from "../accommodation/ActivationIssuePanel";
import { useAccommodations } from "../accommodation/use-accommodations";
import { AccommodationOnboarding } from "../operator/AccommodationOnboarding";
import {
  accommodationCategoryLabels,
  type Accommodation,
} from "../operator/stay-lifecycle";

/**
 * A administração cadastra hospedagem e decide pedido de acesso; ela não opera
 * nenhum lugar. Por isso esta área não monta quadro de estadias, painel de
 * cartaz nem fila de hóspedes: o que sobra é o cadastro, a entrega do acesso e a
 * fila da ADR-042. O que a lista mostra é consequência da mesma regra de sempre
 * — `GET /api/v1/accommodations` filtra por `core.memberships` —, e o vínculo do
 * administrador existe só porque emitir o acesso lê a linha e escreve a
 * ativação.
 */
function AccommodationRow({
  accommodation,
  onSelect,
  selected,
}: {
  accommodation: Accommodation;
  onSelect: (accommodation: Accommodation) => void;
  selected: boolean;
}) {
  const { t } = useLocale();
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
        <span className="property-capacity">{t("admin.accommodations.send")}</span>
      </button>
    </li>
  );
}

function AccommodationList({
  accommodations,
  loading,
  onSelect,
  selectedId,
}: {
  accommodations: readonly Accommodation[];
  loading: boolean;
  onSelect: (accommodation: Accommodation) => void;
  selectedId: string | null;
}) {
  const { t } = useLocale();
  if (loading) {
    return (
      <p className="loading-note" role="status">
        {t("admin.accommodations.loading")}
      </p>
    );
  }
  if (accommodations.length === 0) {
    return (
      <div className="empty-state">
        <p>{t("admin.accommodations.empty")}</p>
      </div>
    );
  }
  return (
    <ul className="property-grid">
      {accommodations.map((accommodation) => (
        <AccommodationRow
          key={accommodation.id}
          accommodation={accommodation}
          onSelect={onSelect}
          selected={accommodation.id === selectedId}
        />
      ))}
    </ul>
  );
}

/**
 * A versão da linha vive aqui porque a emissão escreve com `If-Match`: guardada
 * dentro do painel, ela voltaria ao valor antigo a cada troca de hospedagem.
 */
function AccommodationAccess({ accommodation }: { accommodation: Accommodation }) {
  const { t } = useLocale();
  const [version, setVersion] = useState(accommodation.version);

  return (
    <section
      className="workspace-section"
      aria-labelledby="admin-accommodation-access-title"
    >
      <h2 id="admin-accommodation-access-title">
        {t("admin.access.title", { name: accommodation.name })}
      </h2>
      <ActivationIssuePanel
        accommodationId={accommodation.id}
        onVersionChange={setVersion}
        version={version}
      />
    </section>
  );
}

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

function Onboarding({
  onCancel,
  onCreated,
  onStart,
  open,
}: {
  onCancel: () => void;
  onCreated: (accommodation: Accommodation) => void;
  onStart: () => void;
  open: boolean;
}) {
  const { t } = useLocale();
  if (open) {
    return <AccommodationOnboarding onCancel={onCancel} onCreated={onCreated} />;
  }
  return (
    <button type="button" className="primary-action" onClick={onStart}>
      {t("admin.accommodations.onboard")}
    </button>
  );
}

export function AdminWorkspace() {
  const { t } = useLocale();
  const { accommodations, load, loading, operation, selected, setSelected } =
    useAccommodations(t("admin.accommodations.loading"), false);
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
      <section
        className="workspace-section"
        aria-labelledby="admin-accommodations-title"
      >
        <h2 id="admin-accommodations-title">{t("admin.accommodations.title")}</h2>
        <p className="queue-hint">{t("admin.accommodations.hint")}</p>
        <LoadFailure operation={operation} />
        <AccommodationList
          accommodations={accommodations}
          loading={loading}
          onSelect={setSelected}
          selectedId={selected?.id ?? null}
        />
        <Onboarding
          onCancel={() => setOnboarding(false)}
          onCreated={handleCreated}
          onStart={() => setOnboarding(true)}
          open={onboarding}
        />
      </section>

      {selected === null ? null : (
        <AccommodationAccess accommodation={selected} key={selected.id} />
      )}

      <AccessRequestQueue />
    </div>
  );
}
