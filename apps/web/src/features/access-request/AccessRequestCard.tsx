import type {
  AccessRequest,
  AccessRequestRejectionReason,
} from "../../shared/api/invite-request-client";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import type { Translate } from "../../shared/i18n/translate";
import { formatCivilDate, stayCivilDateOf } from "../operator/stay-lifecycle";
import {
  accessRequestStateKeys,
  capacityLabel,
  categoryKeys,
  contactOf,
  rejectionReasonKeys,
  waitingLabel,
} from "./access-request-vocabulary";
import { AccessRequestRejectionForm } from "./AccessRequestRejectionForm";

interface AccessRequestCardProps {
  busy: boolean;
  errorId: string | undefined;
  onApprove: (request: AccessRequest) => void;
  onCancelRejection: () => void;
  onReject: (
    request: AccessRequest,
    reasonCode: AccessRequestRejectionReason,
  ) => void;
  onStartRejection: (request: AccessRequest) => void;
  rejecting: boolean;
  request: AccessRequest;
}

interface FactRow {
  term: string;
  value: string;
}

function FactList({ rows }: { rows: readonly FactRow[] }) {
  return (
    <dl className="access-request-facts">
      {rows.map((row) => (
        <div key={row.term}>
          <dt>{row.term}</dt>
          <dd>{row.value}</dd>
        </div>
      ))}
    </dl>
  );
}

function lodgingRows(t: Translate, request: AccessRequest): FactRow[] {
  return [
    {
      term: t("accessRequest.field.category"),
      value: t(categoryKeys[request.category]),
    },
    {
      term: t("accessRequest.field.capacity"),
      value: capacityLabel(t, request.capacity),
    },
    {
      term: t("accessRequest.field.location"),
      value: t("accessRequest.locationValue", {
        city: request.city_label,
        state: request.state_code,
      }),
    },
  ];
}

/**
 * O contato é nulo depois de recusado ou vencido, e a tela diz isso em vez de
 * mostrar campo vazio: o dado não sumiu por defeito, foi apagado de propósito.
 */
function ContactFacts({ request }: { request: AccessRequest }) {
  const { t } = useLocale();
  const contact = contactOf(request);
  if (contact === null) {
    return (
      <p className="access-request-erased">{t("accessRequest.contactErased")}</p>
    );
  }
  const rows: FactRow[] = [
    { term: t("accessRequest.field.contactName"), value: contact.name },
    { term: t("accessRequest.field.contactEmail"), value: contact.email },
  ];
  if (contact.phone !== null) {
    rows.push({
      term: t("accessRequest.field.contactPhone"),
      value: contact.phone,
    });
  }
  return <FactList rows={rows} />;
}

/** Estado, espera e prazo. Texto sempre, nunca só cor. */
function CardBadges({ request }: { request: AccessRequest }) {
  const { t } = useLocale();
  const waiting = waitingLabel(t, request.created_at);
  return (
    <p className="access-request-badges">
      <span className={`access-request-state state-${request.approval_state}`}>
        {t(accessRequestStateKeys[request.approval_state])}
      </span>
      {waiting === null ? null : (
        <span className="access-request-waiting">{waiting}</span>
      )}
      {request.approval_state === "pending" ? (
        <span className="approval-deadline">
          {t("accessRequest.expires", {
            date: formatCivilDate(stayCivilDateOf(request.expires_at)),
          })}
        </span>
      ) : null}
    </p>
  );
}

function RejectionNote({ request }: { request: AccessRequest }) {
  const { t } = useLocale();
  if (request.rejection_reason_code === null) {
    return null;
  }
  return (
    <p className="access-request-erased">
      {t("accessRequest.rejectedReason", {
        reason: t(rejectionReasonKeys[request.rejection_reason_code]),
      })}
    </p>
  );
}

function CardActions({
  busy,
  errorId,
  onApprove,
  onStartRejection,
  request,
}: Pick<
  AccessRequestCardProps,
  "busy" | "errorId" | "onApprove" | "onStartRejection" | "request"
>) {
  const { t } = useLocale();
  if (request.approval_state !== "pending") {
    return null;
  }
  return (
    <div className="stay-actions">
      <button
        type="button"
        className="primary-action"
        disabled={busy}
        aria-describedby={errorId}
        onClick={() => onApprove(request)}
      >
        {t("accessRequest.approve")}
      </button>
      <button
        type="button"
        className="quiet-action"
        disabled={busy}
        aria-describedby={errorId}
        onClick={() => onStartRejection(request)}
      >
        {t("accessRequest.reject")}
      </button>
    </div>
  );
}

export function AccessRequestCard(props: AccessRequestCardProps) {
  const { t } = useLocale();
  const { request } = props;

  return (
    <li className="stay-card approval-card access-request-card">
      <div className="stay-summary">
        <strong>{request.accommodation_name}</strong>
        <CardBadges request={request} />
      </div>
      <FactList rows={lodgingRows(t, request)} />
      <ContactFacts request={request} />
      <RejectionNote request={request} />
      {props.rejecting ? (
        <AccessRequestRejectionForm
          busy={props.busy}
          onCancel={props.onCancelRejection}
          onConfirm={(reasonCode) => props.onReject(request, reasonCode)}
        />
      ) : (
        <CardActions
          busy={props.busy}
          errorId={props.errorId}
          onApprove={props.onApprove}
          onStartRejection={props.onStartRejection}
          request={request}
        />
      )}
    </li>
  );
}
