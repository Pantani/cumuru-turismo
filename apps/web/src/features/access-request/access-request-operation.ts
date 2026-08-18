import { useCallback, useState } from "react";

import { ApiError } from "../../shared/api/http-client";
import type { Translate } from "../../shared/i18n/translate";

/**
 * A fila tem operação própria em vez de reusar `use-operation` porque aquela
 * fala português fixo e fala de estadia ("Alguém alterou esta estadia"). Aqui a
 * frase precisa ser do idioma escolhido e falar de pedido.
 */
export interface AccessRequestOperationState {
  busy: boolean;
  message: string;
  tone: "idle" | "working" | "done" | "failed";
}

const idle: AccessRequestOperationState = {
  busy: false,
  message: "",
  tone: "idle",
};

/**
 * O 412 e o 409 são a mesma história para quem decide: outra pessoa já mexeu
 * neste pedido. O código não vai à tela; a instrução, sim.
 */
export function describeAccessRequestFailure(t: Translate, cause: unknown) {
  if (!(cause instanceof ApiError)) {
    return t("accessRequest.error.offline");
  }
  if (cause.status === 412 || cause.status === 409) {
    return t("accessRequest.error.conflict");
  }
  return t("accessRequest.error.generic");
}

export function isVersionConflict(cause: unknown) {
  return (
    cause instanceof ApiError && (cause.status === 412 || cause.status === 409)
  );
}

export function useAccessRequestOperation(t: Translate) {
  const [state, setState] = useState<AccessRequestOperationState>(idle);

  const run = useCallback(
    async <T,>(
      working: string,
      done: string,
      execute: () => Promise<T>,
    ): Promise<T | null> => {
      setState({ busy: true, message: working, tone: "working" });
      try {
        const result = await execute();
        setState({ busy: false, message: done, tone: "done" });
        return result;
      } catch (error) {
        setState({
          busy: false,
          message: describeAccessRequestFailure(t, error),
          tone: "failed",
        });
        return null;
      }
    },
    [t],
  );

  return { ...state, run };
}
