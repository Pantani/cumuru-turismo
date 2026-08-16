import { useCallback, useEffect, useMemo, useState } from "react";

import { useAuthSession } from "../../shared/auth/AuthSession";
import { captureInviteCapability } from "../../shared/security/invite-capability";
import { InviteQr } from "../invite/InviteQr";
import { NewStayForm } from "./NewStayForm";
import { StayCard } from "./StayCard";
import { VisitorGroupPanel } from "./VisitorGroupPanel";
import { createInvite, runDirectAction, type DirectStayAction } from "./stay-commands";
import { isOpenStay, type Accommodation, type Stay, type StayAction } from "./stay-lifecycle";
import { OperationStatus } from "../../shared/forms/FieldFeedback";
import { useOperation } from "./use-operation";

interface StayBoardProps {
  accommodation: Accommodation;
}

type Focus =
  | { kind: "none" }
  | { kind: "group"; stay: Stay }
  | { kind: "invite"; stay: Stay; url: string };

function partition(stays: readonly Stay[]) {
  return {
    open: stays.filter((stay) => isOpenStay(stay.status)),
    closed: stays.filter((stay) => !isOpenStay(stay.status)),
  };
}

export function StayBoard({ accommodation }: StayBoardProps) {
  const { client } = useAuthSession();
  const operation = useOperation();
  const { run } = operation;
  const [stays, setStays] = useState<readonly Stay[]>([]);
  const [loading, setLoading] = useState(true);
  const [focus, setFocus] = useState<Focus>({ kind: "none" });

  const refresh = useCallback(async () => {
    setLoading(true);
    const result = await run("Atualizando as estadias", () =>
      client.listStays(accommodation.id),
    );
    setStays(result?.data.items ?? []);
    setLoading(false);
  }, [accommodation.id, client, run]);

  useEffect(() => {
    setFocus({ kind: "none" });
    void refresh();
    // refresh changes with the selected accommodation, which is exactly when
    // the board must reload.
  }, [refresh]);

  const handleInvite = useCallback(
    async (stay: Stay) => {
      const result = await run("Gerando o convite", () =>
        createInvite({ client, stay }),
      );
      if (result !== null) {
        setFocus({ kind: "invite", stay, url: result.data.url });
        await refresh();
      }
    },
    [client, refresh, run],
  );

  const handleAction = useCallback(
    async (stay: Stay, action: StayAction) => {
      if (action === "submit_group") {
        setFocus({ kind: "group", stay });
        return;
      }
      if (action === "invite") {
        await handleInvite(stay);
        return;
      }
      const done = await run("Registrando a mudança", () =>
        runDirectAction({ client, stay }, action as DirectStayAction),
      );
      if (done !== null) {
        await refresh();
      }
    },
    [client, handleInvite, refresh, run],
  );

  const groups = useMemo(() => partition(stays), [stays]);

  return (
    <section className="stay-board" aria-labelledby="stay-board-title">
      <div className="board-heading">
        <h2 id="stay-board-title">Estadias de {accommodation.name}</h2>
        <button
          type="button"
          className="ghost-action"
          onClick={() => void refresh()}
          disabled={operation.busy}
        >
          Atualizar
        </button>
      </div>

      <OperationStatus operation={operation} />

      <NewStayForm
        accommodationId={accommodation.id}
        onCreated={() => void refresh()}
      />

      {focus.kind === "invite" ? (
        <InviteQr
          url={focus.url}
          onDiscard={() => setFocus({ kind: "none" })}
          onOpenHere={() => {
            captureInviteCapability(new URL(focus.url), (path) =>
              window.history.replaceState(null, "", path),
            );
            setFocus({ kind: "none" });
          }}
        />
      ) : null}

      {focus.kind === "group" ? (
        <VisitorGroupPanel
          stay={focus.stay}
          onClose={() => setFocus({ kind: "none" })}
          onSubmitted={() => {
            setFocus({ kind: "none" });
            void refresh();
          }}
        />
      ) : null}

      <StayGroupList
        busy={operation.busy}
        emptyMessage="Nenhuma estadia em andamento. Crie a primeira acima."
        loading={loading}
        onAction={(stay, action) => void handleAction(stay, action)}
        stays={groups.open}
        title="Em andamento"
      />
      {groups.closed.length === 0 ? null : (
        <details className="closed-stays">
          <summary>Estadias encerradas ({groups.closed.length})</summary>
          <StayGroupList
            busy={operation.busy}
            emptyMessage="Nada encerrado ainda."
            loading={loading}
            onAction={(stay, action) => void handleAction(stay, action)}
            stays={groups.closed}
            title="Encerradas"
          />
        </details>
      )}
    </section>
  );
}

interface StayGroupListProps {
  busy: boolean;
  emptyMessage: string;
  loading: boolean;
  onAction: (stay: Stay, action: StayAction) => void;
  stays: readonly Stay[];
  title: string;
}

function StayGroupList({
  busy,
  emptyMessage,
  loading,
  onAction,
  stays,
  title,
}: StayGroupListProps) {
  return (
    <section className="stay-group">
      <h3>{title}</h3>
      {loading ? <p className="loading-note">Carregando…</p> : null}
      {!loading && stays.length === 0 ? (
        <p className="empty-note">{emptyMessage}</p>
      ) : null}
      <ul className="stay-list">
        {stays.map((stay) => (
          <StayCard key={stay.id} busy={busy} onAction={onAction} stay={stay} />
        ))}
      </ul>
    </section>
  );
}
