import { useCallback, useEffect, useState } from "react";

import { useAuthSession } from "../../shared/auth/AuthSession";
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

function useAccommodations() {
  const { client } = useAuthSession();
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

      {selected === null ? null : <StayBoard accommodation={selected} />}
    </div>
  );
}
