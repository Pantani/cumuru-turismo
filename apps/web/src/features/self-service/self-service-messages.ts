import { ApiError } from "../../shared/api/http-client";
import { guestCopyFor } from "../../shared/forms/guest-copy";
import type { Translate } from "../../shared/i18n/translate";
import {
  ProofOfWorkAbortedError,
  ProofOfWorkExhaustedError,
} from "../../shared/security/proof-of-work";

/**
 * The uniform 404 is deliberate: a bad token, a revoked poster and an expired
 * one are indistinguishable by design, so the message must not speculate about
 * which one happened.
 */
function statusMessages(t: Translate): Readonly<Record<number, string>> {
  return {
    403: t("selfService.error.forbidden"),
    404: t("selfService.error.notFound"),
    409: t("selfService.error.conflict"),
    422: t("selfService.error.unprocessable"),
    429: t("selfService.error.rateLimited"),
  };
}

function localFailureMessage(t: Translate, error: unknown) {
  if (error instanceof ProofOfWorkExhaustedError) {
    return error.message;
  }
  if (error instanceof ProofOfWorkAbortedError) {
    return t("selfService.error.proofOfWorkAborted");
  }
  return t("selfService.error.offline");
}

export function describeSelfServiceFailure(t: Translate, error: unknown) {
  if (!(error instanceof ApiError)) {
    return localFailureMessage(t, error);
  }
  return guestCopyFor(t, error, statusMessages(t));
}

/** A refusal that will never succeed on retry must not keep a local copy. */
export function shouldPurgeDraft(error: unknown) {
  if (!(error instanceof ApiError)) {
    return false;
  }
  return error.status === 404 || error.status === 403 || error.status === 422;
}
