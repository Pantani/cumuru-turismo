import { Phase7ApiError } from "../../shared/api/phase7-client";

const strongEtagPattern = /^"([1-9][0-9]*)"$/u;

/**
 * Commands on the accommodation share one row version, so the panel that just
 * wrote has to hand the new version to the panel that writes next; otherwise
 * the second command fails `If-Match` for no reason the operator can see.
 */
export function versionFromEtag(etag: string | null) {
  const matched = etag === null ? null : strongEtagPattern.exec(etag);
  if (matched === null) {
    return null;
  }
  return Number(matched[1]);
}

/** A missing poster is a state of the screen, not a failure to report. */
export function isNotFound(error: unknown) {
  return error instanceof Phase7ApiError && error.status === 404;
}
