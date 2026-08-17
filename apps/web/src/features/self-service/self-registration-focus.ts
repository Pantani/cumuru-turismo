import type { ValidationIssue } from "../../shared/validation/core-validation";
import type { VisitorEditorHandle } from "../visitors/VisitorEditor";

/** Group-level issues have no field of their own; the first role owns them. */
function visitorTarget(field: string) {
  return field === "visitors" ? "visitors.0.role" : field;
}

const fieldElementIds: Readonly<Record<string, string>> = {
  planned_arrival_on: "self-registration-arrival",
  planned_departure_on: "self-registration-departure",
};

function focusElement(field: string) {
  const id = fieldElementIds[field];
  if (id !== undefined) {
    document.getElementById(id)?.focus();
  }
}

/** WCAG 2.2 error identification: the refusal must land the caret on the cause. */
export function focusFirstIssue(
  handle: VisitorEditorHandle | null,
  issue: ValidationIssue | undefined,
) {
  if (issue === undefined) {
    return;
  }
  if (issue.field.startsWith("visitors")) {
    handle?.focus(visitorTarget(issue.field));
    return;
  }
  focusElement(issue.field);
}
