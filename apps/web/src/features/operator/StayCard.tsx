import {
  actionsFor,
  formatCivilDate,
  stayActionLabels,
  stayStatusLabels,
  type Stay,
  type StayAction,
} from "./stay-lifecycle";

interface StayCardProps {
  busy: boolean;
  onAction: (stay: Stay, action: StayAction) => void;
  stay: Stay;
}

const destructiveActions = new Set<StayAction>(["cancel", "no_show"]);

function actionClass(action: StayAction) {
  if (action === "check_in" || action === "check_out") {
    return "primary-action";
  }
  return destructiveActions.has(action) ? "quiet-action" : "ghost-action";
}

function guestSummary(stay: Stay) {
  if (stay.visitor_count > 0) {
    return `${stay.visitor_count} de ${stay.expected_guest_count} visitantes registrados`;
  }
  return `${stay.expected_guest_count} pessoas esperadas`;
}

export function StayCard({ busy, onAction, stay }: StayCardProps) {
  const actions = actionsFor(stay.status);
  return (
    <li className="stay-card">
      <div className="stay-summary">
        <span className={`status-chip status-${stay.status}`}>
          {stayStatusLabels[stay.status]}
        </span>
        <strong>
          {formatCivilDate(stay.planned_arrival_on)} até{" "}
          {formatCivilDate(stay.planned_departure_on)}
        </strong>
        <span className="stay-guests">{guestSummary(stay)}</span>
      </div>
      {actions.length === 0 ? null : (
        <div className="stay-actions">
          {actions.map((action) => (
            <button
              key={action}
              type="button"
              className={actionClass(action)}
              disabled={busy}
              onClick={() => onAction(stay, action)}
            >
              {stayActionLabels[action]}
            </button>
          ))}
        </div>
      )}
    </li>
  );
}
