import {
  describedBy,
  FieldError,
  invalidFlag,
  issueMessage,
} from "../../shared/forms/FieldFeedback";
import type { ValidationIssue } from "../../shared/validation/core-validation";

interface PlannedWindowFieldsProps {
  arrival: string;
  departure: string;
  disabled: boolean;
  issues: readonly ValidationIssue[];
  onChange: (field: "arrival" | "departure", value: string) => void;
}

interface DateFieldProps {
  field: "arrival" | "departure";
  id: string;
  issue: string | undefined;
  label: string;
  onChange: (field: "arrival" | "departure", value: string) => void;
  value: string;
}

function DateField({ field, id, issue, label, onChange, value }: DateFieldProps) {
  // The error stays outside the label so it never becomes part of the field's
  // accessible name; `aria-describedby` is what associates the two.
  return (
    <div className="field-control">
      <label htmlFor={id}>{label}</label>
      <input
        id={id}
        type="date"
        value={value}
        onChange={(event) => onChange(field, event.target.value)}
        aria-invalid={invalidFlag(issue)}
        aria-describedby={describedBy(`${id}-error`, issue)}
        required
      />
      <FieldError id={`${id}-error`} message={issue} />
    </div>
  );
}

export function PlannedWindowFields({
  arrival,
  departure,
  disabled,
  issues,
  onChange,
}: PlannedWindowFieldsProps) {
  return (
    <fieldset className="planned-window" disabled={disabled}>
      <legend>Período previsto</legend>
      <div className="field-grid">
        <DateField
          field="arrival"
          id="self-registration-arrival"
          issue={issueMessage(issues, "planned_arrival_on")}
          label="Data de chegada"
          onChange={onChange}
          value={arrival}
        />
        <DateField
          field="departure"
          id="self-registration-departure"
          issue={issueMessage(issues, "planned_departure_on")}
          label="Data de saída"
          onChange={onChange}
          value={departure}
        />
      </div>
    </fieldset>
  );
}
