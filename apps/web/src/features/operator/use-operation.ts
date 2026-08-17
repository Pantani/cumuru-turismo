import { useCallback, useState } from "react";

import { Phase2ApiError } from "../../shared/api/phase2-client";
import { Phase7ApiError } from "../../shared/api/phase7-client";

export interface OperationState {
  busy: boolean;
  message: string;
  tone: "idle" | "working" | "done" | "failed";
}

const idle: OperationState = { busy: false, message: "", tone: "idle" };

/**
 * describeFailure turns a transport failure into something an innkeeper can act
 * on. It never surfaces an identifier, a status code or a stack.
 */
type ApiFailure = Phase2ApiError | Phase7ApiError;

function apiFailure(error: unknown): ApiFailure | null {
  if (error instanceof Phase2ApiError || error instanceof Phase7ApiError) {
    return error;
  }
  return null;
}

export function describeFailure(cause: unknown) {
  const error = apiFailure(cause);
  if (error === null) {
    return "Não foi possível falar com o serviço. Nada foi perdido — tente de novo.";
  }
  if (error.retryAfterSeconds !== null) {
    return `${error.problem.title}. Tente novamente em ${error.retryAfterSeconds} segundos.`;
  }
  if (error.status === 412 || error.status === 409) {
    return "Alguém alterou esta estadia enquanto você trabalhava. Atualizamos a lista; confira e repita a ação.";
  }
  return error.problem.title;
}

/**
 * useOperation serializes one user-visible action and reports it in plain
 * language. Callers stay free of try/catch and of status string handling.
 */
export function useOperation() {
  const [state, setState] = useState<OperationState>(idle);

  const run = useCallback(
    async <T,>(label: string, execute: () => Promise<T>): Promise<T | null> => {
      setState({ busy: true, message: `${label}…`, tone: "working" });
      try {
        const result = await execute();
        setState({ busy: false, message: `${label}: pronto.`, tone: "done" });
        return result;
      } catch (error) {
        setState({ busy: false, message: describeFailure(error), tone: "failed" });
        return null;
      }
    },
    [],
  );

  const reset = useCallback(() => setState(idle), []);

  return { ...state, reset, run };
}
