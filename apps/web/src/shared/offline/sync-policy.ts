export type DraftDisposition = "preserve" | "purge";

const preservedStatuses = new Set([0, 409, 412, 422, 429, 503]);

export function draftDispositionFor(
  status: number,
  inviteWasValidated: boolean,
): DraftDisposition {
  if (status >= 200 && status < 300) {
    return "purge";
  }
  if (status === 404 && inviteWasValidated) {
    return "purge";
  }
  return preservedStatuses.has(status) ? "preserve" : "preserve";
}
