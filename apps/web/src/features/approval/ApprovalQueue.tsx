import { useCallback, useEffect, useState } from "react";

import type { components } from "../../generated/schema";
import { useAuthSession } from "../../shared/auth/AuthSession";
import { OperationStatus } from "../../shared/forms/FieldFeedback";
import { InviteQr } from "../invite/InviteQr";
import { createInvite } from "../operator/stay-commands";
import {
  entityTagFor,
  type Accommodation,
  type Stay,
} from "../operator/stay-lifecycle";
import { useOperation } from "../operator/use-operation";
import { ApprovalCard } from "./ApprovalCard";

type RejectionReasonCode =
  components["schemas"]["RejectStayRequest"]["reason_code"];

interface ApprovalQueueProps {
  accommodation: Accommodation;
}

interface ApprovedFollowUpProps {
  busy: boolean;
  onDismiss: () => void;
  onIssueInvite: () => void;
}

/**
 * The identity, when a purpose needs one, is filled in by the person
 * themselves through the nominal invite already built in the core slice. There is no
 * second identity path, and this offer is where the two features meet.
 */
function ApprovedFollowUp({
  busy,
  onDismiss,
  onIssueInvite,
}: ApprovedFollowUpProps) {
  return (
    <div className="approval-followup" role="group" aria-labelledby="followup-title">
      <h3 id="followup-title">Autocadastro aprovado</h3>
      <p>
        Se a finalidade exigir identificação, emita o convite nominal para que a
        própria pessoa preencha os dados dela. A hospedagem não digita nome nem
        documento de ninguém.
      </p>
      <div className="button-row">
        <button
          type="button"
          className="primary-action"
          disabled={busy}
          onClick={onIssueInvite}
        >
          Emitir convite nominal
        </button>
        <button type="button" className="ghost-action" onClick={onDismiss}>
          Agora não
        </button>
      </div>
    </div>
  );
}

interface QueueListProps {
  busy: boolean;
  loading: boolean;
  onApprove: (stay: Stay) => void;
  onCancelRejection: () => void;
  onReject: (stay: Stay, reasonCode: RejectionReasonCode) => void;
  onStartRejection: (stay: Stay) => void;
  rejectingId: string | null;
  stays: readonly Stay[];
}

function QueueList(props: QueueListProps) {
  if (props.loading) {
    return <p className="loading-note">Carregando…</p>;
  }
  if (props.stays.length === 0) {
    return (
      <p className="empty-note">
        Nenhum autocadastro aguardando decisão neste momento.
      </p>
    );
  }
  return (
    <ul className="stay-list">
      {props.stays.map((stay) => (
        <ApprovalCard
          key={stay.id}
          busy={props.busy}
          onApprove={props.onApprove}
          onCancelRejection={props.onCancelRejection}
          onReject={props.onReject}
          onStartRejection={props.onStartRejection}
          rejecting={props.rejectingId === stay.id}
          stay={stay}
        />
      ))}
    </ul>
  );
}

function usePendingStays(accommodationId: string) {
  const { coreClient: client } = useAuthSession();
  const operation = useOperation();
  const { run } = operation;
  const [stays, setStays] = useState<readonly Stay[]>([]);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    setLoading(true);
    const result = await run("Atualizando a fila de aprovação", () =>
      client.listStays({
        accommodationId,
        approvalState: "pending",
        provenance: "self_service",
      }),
    );
    setStays(result?.data.items ?? []);
    setLoading(false);
  }, [accommodationId, client, run]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return { loading, operation, refresh, stays };
}

export function ApprovalQueue({ accommodation }: ApprovalQueueProps) {
  const { coreClient: client, selfServiceClient } = useAuthSession();
  const { loading, operation, refresh, stays } = usePendingStays(accommodation.id);
  const { run } = operation;
  const [rejectingId, setRejectingId] = useState<string | null>(null);
  const [approved, setApproved] = useState<Stay | null>(null);
  const [inviteUrl, setInviteUrl] = useState<string | null>(null);

  const approve = useCallback(
    async (stay: Stay) => {
      const result = await run("Aprovando o autocadastro", () =>
        selfServiceClient.approveStay(
          stay.id,
          entityTagFor(stay.version),
          crypto.randomUUID(),
        ),
      );
      if (result !== null) {
        setApproved({ ...stay, version: result.data.version });
        setRejectingId(null);
        await refresh();
      }
    },
    [refresh, run, selfServiceClient],
  );

  const reject = useCallback(
    async (stay: Stay, reasonCode: RejectionReasonCode) => {
      const result = await run("Recusando o autocadastro", () =>
        selfServiceClient.rejectStay(
          stay.id,
          { reason_code: reasonCode },
          entityTagFor(stay.version),
          crypto.randomUUID(),
        ),
      );
      if (result !== null) {
        setRejectingId(null);
        await refresh();
      }
    },
    [refresh, run, selfServiceClient],
  );

  const issueInvite = useCallback(async () => {
    if (approved === null) {
      return;
    }
    const result = await run("Gerando o convite nominal", () =>
      createInvite({ client, stay: approved }),
    );
    if (result !== null) {
      setInviteUrl(result.data.url);
    }
  }, [approved, client, run]);

  return (
    <section className="approval-queue" aria-labelledby="approval-queue-title">
      <div className="board-heading">
        <h2 id="approval-queue-title">Aguardando aprovação</h2>
        <button
          type="button"
          className="ghost-action"
          disabled={operation.busy}
          onClick={() => void refresh()}
        >
          Atualizar fila
        </button>
      </div>
      <p className="queue-hint">
        Cadastros feitos pelo cartaz da hospedagem. Sem aprovação, nada entra em
        presença nem no painel público, e a pendência expira em 72 horas.
      </p>

      <OperationStatus operation={operation} />

      <QueueList
        busy={operation.busy}
        loading={loading}
        onApprove={(stay) => void approve(stay)}
        onCancelRejection={() => setRejectingId(null)}
        onReject={(stay, reasonCode) => void reject(stay, reasonCode)}
        onStartRejection={(stay) => setRejectingId(stay.id)}
        rejectingId={rejectingId}
        stays={stays}
      />

      {approved === null ? null : (
        <ApprovedFollowUp
          busy={operation.busy}
          onDismiss={() => {
            setApproved(null);
            setInviteUrl(null);
          }}
          onIssueInvite={() => void issueInvite()}
        />
      )}

      {inviteUrl === null ? null : (
        <InviteQr url={inviteUrl} onDiscard={() => setInviteUrl(null)} />
      )}
    </section>
  );
}
