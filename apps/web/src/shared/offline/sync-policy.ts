export type DraftDisposition = "preserve" | "purge";

function accepted(status: number) {
  return status >= 200 && status < 300;
}

/**
 * A draft is discarded only once the server has certainly consumed it: an
 * accepted response, or a 404 on an invite already validated (the invite was
 * spent). Everything else — including an unknown status — preserves the draft,
 * because losing an operator's typing is worse than keeping a stale one.
 */
export function draftDispositionFor(
  status: number,
  inviteWasValidated: boolean,
): DraftDisposition {
  if (accepted(status)) {
    return "purge";
  }
  return status === 404 && inviteWasValidated ? "purge" : "preserve";
}
