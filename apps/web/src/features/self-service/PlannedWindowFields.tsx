import {
  describedBy,
  FieldError,
  invalidFlag,
  issueMessage,
} from "../../shared/forms/FieldFeedback";
import { useLocale } from "../../shared/i18n/LocaleProvider";
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
  const { t } = useLocale();
  return (
    <fieldset className="planned-window" disabled={disabled}>
      <legend>{t("selfService.window.legend")}</legend>
      <div className="field-grid">
        <DateField
          field="arrival"
          id="self-registration-arrival"
          issue={issueMessage(issues, "planned_arrival_on")}
          label={t("selfService.window.arrival")}
          onChange={onChange}
          value={arrival}
        />
        <DateField
          field="departure"
          id="self-registration-departure"
          issue={issueMessage(issues, "planned_departure_on")}
          label={t("selfService.window.departure")}
          onChange={onChange}
          value={departure}
        />
      </div>
    </fieldset>
  );
}
