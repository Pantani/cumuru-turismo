import type { ValidationIssue } from "../validation/phase2-validation";

/**
 * These primitives exist so a form component's branching lives in one place
 * instead of once per field. Every `message === undefined ? null : <span/>` in a
 * parent adds a branch to that parent; folding it here keeps form components
 * readable and inside the complexity budget.
 */

export function issueMessage(
  issues: readonly ValidationIssue[],
  field: string,
): string | undefined {
  return issues.find((issue) => issue.field === field)?.message;
}

export function invalidFlag(message: string | undefined) {
  return message === undefined ? undefined : true;
}

export function describedBy(id: string, message: string | undefined) {
  return message === undefined ? undefined : id;
}

export function FieldError({
  id,
  message,
}: {
  id?: string;
  message: string | undefined;
}) {
  if (message === undefined) {
    return null;
  }
  return (
    <span className="field-error" id={id}>
      {message}
    </span>
  );
}

export interface OperationFeedback {
  message: string;
  tone: string;
}

export function OperationStatus({ operation }: { operation: OperationFeedback }) {
  if (operation.message === "") {
    return null;
  }
  return (
    <p className={`operation-status tone-${operation.tone}`} role="status">
      {operation.message}
    </p>
  );
}
