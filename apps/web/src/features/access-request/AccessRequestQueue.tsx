import { useCallback, useEffect, useId, useState } from "react";

import type {
  AccessRequest,
  AccessRequestRejectionReason,
  AccessRequestState,
} from "../../shared/api/invite-request-client";
import { useAuthSession } from "../../shared/auth/AuthSession";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import type { Translate } from "../../shared/i18n/translate";
import { useDeferredFocus } from "../invite-request/invite-request-focus";
import { entityTagFor } from "../operator/stay-lifecycle";
import { useAccessRequestOperation } from "./access-request-operation";
import {
  accessRequestStateKeys,
  accessRequestStates,
} from "./access-request-vocabulary";
import { AccessRequestCard } from "./AccessRequestCard";
import {
  AccessRequestIssuance,
  ACCESS_REQUEST_ISSUANCE_ID,
} from "./AccessRequestIssuance";

/**
 * A região viva existe desde o primeiro render, mesmo vazia. Criada só quando
 * há texto, a mensagem chegaria junto com o elemento, e leitores de tela
 * costumam perder o primeiro anúncio de uma região recém-inserida.
 */
export const ACCESS_REQUEST_STATUS_ID = "access-request-status";

function useAccessRequestQueue(t: Translate) {
  const { inviteRequestClient } = useAuthSession();
  const operation = useAccessRequestOperation(t);
  const { run } = operation;
  const [items, setItems] = useState<readonly AccessRequest[]>([]);
  const [approvalState, setApprovalState] =
    useState<AccessRequestState>("pending");
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    setLoading(true);
    const result = await run(t("accessRequest.loading"), "", () =>
      inviteRequestClient.listAccessRequests({ approvalState }),
    );
    setItems(result?.data.items ?? []);
    setLoading(false);
  }, [approvalState, inviteRequestClient, run, t]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return {
    approvalState,
    items,
    loading,
    operation,
    refresh,
    setApprovalState,
  };
}

interface QueueListProps {
  busy: boolean;
  errorId: string | undefined;
  items: readonly AccessRequest[];
  loading: boolean;
  onApprove: (request: AccessRequest) => void;
  onCancelRejection: () => void;
  onReject: (
    request: AccessRequest,
    reasonCode: AccessRequestRejectionReason,
  ) => void;
  onStartRejection: (request: AccessRequest) => void;
  rejectingId: string | null;
}

function QueueList(props: QueueListProps) {
  const { t } = useLocale();
  if (props.loading) {
    return <p className="loading-note">{t("accessRequest.loading")}</p>;
  }
  if (props.items.length === 0) {
    return <p className="empty-note">{t("accessRequest.empty")}</p>;
  }
  return (
    <ul className="stay-list">
      {props.items.map((request) => (
        <AccessRequestCard
          key={request.id}
          busy={props.busy}
          errorId={props.errorId}
          onApprove={props.onApprove}
          onCancelRejection={props.onCancelRejection}
          onReject={props.onReject}
          onStartRejection={props.onStartRejection}
          rejecting={props.rejectingId === request.id}
          request={request}
        />
      ))}
    </ul>
  );
}

function StateFilter({
  disabled,
  onChange,
  value,
}: {
  disabled: boolean;
  onChange: (next: AccessRequestState) => void;
  value: AccessRequestState;
}) {
  const { t } = useLocale();
  const fieldId = useId();
  return (
    <label className="field-control" htmlFor={fieldId}>
      {t("accessRequest.filter.label")}
      <select
        id={fieldId}
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value as AccessRequestState)}
      >
        {accessRequestStates.map((option) => (
          <option key={option} value={option}>
            {t(accessRequestStateKeys[option])}
          </option>
        ))}
      </select>
    </label>
  );
}

function IssuanceSlot({
  approved,
  onDismiss,
}: {
  approved: AccessRequest | null;
  onDismiss: () => void;
}) {
  if (approved === null || approved.accommodation_id === null) {
    return null;
  }
  return (
    <AccessRequestIssuance
      accommodationId={approved.accommodation_id}
      onDismiss={onDismiss}
      request={approved}
    />
  );
}

/**
 * Fila da administração para os pedidos vindos da página pública. Aprovar cria
 * a acomodação e nada mais: a entrega do acesso continua sendo o ato separado
 * da ADR-041, oferecido logo abaixo e já preenchido com o que o pedido trouxe.
 */
export function AccessRequestQueue() {
  const { t } = useLocale();
  const { inviteRequestClient } = useAuthSession();
  const queue = useAccessRequestQueue(t);
  const { refresh } = queue;
  const decision = useAccessRequestOperation(t);
  const { run } = decision;
  const [rejectingId, setRejectingId] = useState<string | null>(null);
  const [approved, setApproved] = useState<AccessRequest | null>(null);
  const requestFocus = useDeferredFocus();

  const approve = useCallback(
    async (request: AccessRequest) => {
      const result = await run(
        t("accessRequest.status.approving"),
        t("accessRequest.status.approved"),
        () =>
          inviteRequestClient.approveAccessRequest(
            request.id,
            entityTagFor(request.version),
            crypto.randomUUID(),
          ),
      );
      setRejectingId(null);
      await refresh();
      if (result === null) {
        return;
      }
      setApproved(result.data);
      requestFocus(ACCESS_REQUEST_ISSUANCE_ID);
    },
    [inviteRequestClient, refresh, requestFocus, run, t],
  );

  const reject = useCallback(
    async (request: AccessRequest, reasonCode: AccessRequestRejectionReason) => {
      await run(
        t("accessRequest.status.rejecting"),
        t("accessRequest.status.rejected"),
        () =>
          inviteRequestClient.rejectAccessRequest(
            request.id,
            { reason_code: reasonCode },
            entityTagFor(request.version),
            crypto.randomUUID(),
          ),
      );
      setRejectingId(null);
      requestFocus(ACCESS_REQUEST_STATUS_ID);
      await refresh();
    },
    [inviteRequestClient, refresh, requestFocus, run, t],
  );

  const busy = decision.busy || queue.operation.busy;
  const message =
    decision.message === "" ? queue.operation.message : decision.message;
  const errorId =
    decision.tone === "failed" ? ACCESS_REQUEST_STATUS_ID : undefined;

  return (
    <section className="approval-queue" aria-labelledby="access-request-title">
      <div className="board-heading">
        <h2 id="access-request-title">{t("accessRequest.title")}</h2>
        <button
          type="button"
          className="ghost-action"
          disabled={busy}
          onClick={() => void refresh()}
        >
          {t("accessRequest.refresh")}
        </button>
      </div>
      <p className="queue-hint">{t("accessRequest.hint")}</p>

      <StateFilter
        disabled={busy}
        onChange={queue.setApprovalState}
        value={queue.approvalState}
      />

      <p
        className={`operation-status tone-${decision.tone}`}
        id={ACCESS_REQUEST_STATUS_ID}
        tabIndex={-1}
        role="status"
        aria-live="polite"
      >
        {message}
      </p>

      <QueueList
        busy={busy}
        errorId={errorId}
        items={queue.items}
        loading={queue.loading}
        onApprove={(request) => void approve(request)}
        onCancelRejection={() => setRejectingId(null)}
        onReject={(request, reasonCode) => void reject(request, reasonCode)}
        onStartRejection={(request) => setRejectingId(request.id)}
        rejectingId={rejectingId}
      />

      <IssuanceSlot approved={approved} onDismiss={() => setApproved(null)} />
    </section>
  );
}
