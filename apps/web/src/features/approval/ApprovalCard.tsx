import type { components } from "../../generated/schema";
import { formatCivilDate, type Stay } from "../operator/stay-lifecycle";
import { formatApprovalDeadline } from "./approval-vocabulary";
import { RejectionReasonForm } from "./RejectionReasonForm";

type RejectionReasonCode =
  components["schemas"]["RejectStayRequest"]["reason_code"];

interface ApprovalCardProps {
  busy: boolean;
  onApprove: (stay: Stay) => void;
  onCancelRejection: () => void;
  onReject: (stay: Stay, reasonCode: RejectionReasonCode) => void;
  onStartRejection: (stay: Stay) => void;
  rejecting: boolean;
  stay: Stay;
}

function Deadline({ expiresAt }: { expiresAt: string | null }) {
  const deadline = formatApprovalDeadline(expiresAt);
  if (deadline === null) {
    return null;
  }
  return <span className="approval-deadline">{deadline}</span>;
}

export function ApprovalCard({
  busy,
  onApprove,
  onCancelRejection,
  onReject,
  onStartRejection,
  rejecting,
  stay,
}: ApprovalCardProps) {
  return (
    <li className="stay-card approval-card">
      <div className="stay-summary">
        <strong>
          {formatCivilDate(stay.planned_arrival_on)} até{" "}
          {formatCivilDate(stay.planned_departure_on)}
        </strong>
        <span className="stay-guests">
          {stay.visitor_count} pessoas, sem identificação
        </span>
        <Deadline expiresAt={stay.approval_expires_at} />
      </div>
      {rejecting ? (
        <RejectionReasonForm
          busy={busy}
          onCancel={onCancelRejection}
          onConfirm={(reasonCode) => onReject(stay, reasonCode)}
        />
      ) : (
        <div className="stay-actions">
          <button
            type="button"
            className="primary-action"
            disabled={busy}
            onClick={() => onApprove(stay)}
          >
            Aprovar
          </button>
          <button
            type="button"
            className="quiet-action"
            disabled={busy}
            onClick={() => onStartRejection(stay)}
          >
            Recusar
          </button>
        </div>
      )}
    </li>
  );
}
