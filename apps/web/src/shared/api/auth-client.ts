import createClient from "openapi-fetch";

import type { components, paths } from "../../generated/schema";
import { resolveApiBaseUrl } from "./platform-client";

export type Session = components["schemas"]["Session"];
export type SessionAccount = components["schemas"]["SessionAccount"];

type Problem = components["schemas"]["Problem"];

type FetchImplementation = (request: Request) => Promise<Response>;

interface AuthClientOptions {
  baseUrl?: string;
  fetch?: FetchImplementation;
}

/**
 * AuthError carries the transport status so the login screen can separate a
 * rejected credential from a temporary lockout without parsing prose.
 */
export class AuthError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "AuthError";
    this.status = status;
  }
}

const rejectionMessages: Record<number, string> = {
  400: "Revise o e-mail e a senha informados.",
  401: "E-mail ou senha incorretos.",
  422: "A nova senha precisa ter ao menos 12 caracteres e ser diferente da atual.",
  429: "Muitas tentativas seguidas. Aguarde alguns minutos e tente de novo.",
  503: "O serviço está indisponível no momento. Tente novamente em instantes.",
};

function messageFor(status: number, error: Problem | undefined) {
  return (
    rejectionMessages[status] ??
    error?.title ??
    "Não foi possível concluir a operação."
  );
}

export type AuthClient = ReturnType<typeof createAuthClient>;

export function createAuthClient({
  baseUrl = resolveApiBaseUrl(import.meta.env.VITE_API_BASE_URL),
  fetch = (request) => globalThis.fetch(request),
}: AuthClientOptions = {}) {
  const client = createClient<paths>({ baseUrl, credentials: "same-origin", fetch });

  return {
    async login(email: string, password: string): Promise<Session> {
      const { data, error, response } = await client.POST("/auth/login", {
        body: { email, password },
      });
      if (data) {
        return data;
      }
      throw new AuthError(response.status, messageFor(response.status, error));
    },

    /**
     * Rehydrates a session held only in memory. A reload therefore ends the
     * session by design: nothing is written to localStorage or sessionStorage.
     */
    async describe(token: string): Promise<Session> {
      const { data, error, response } = await client.GET("/auth/session", {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (data) {
        return data;
      }
      throw new AuthError(response.status, messageFor(response.status, error));
    },

    /**
     * Replaces the password of the current session. The server revokes every
     * session opened with the previous secret, this one included, so the caller
     * has to authenticate again with the new password.
     */
    async rotatePassword(
      token: string,
      currentPassword: string,
      newPassword: string,
    ): Promise<void> {
      const { error, response } = await client.POST("/auth/password", {
        headers: { Authorization: `Bearer ${token}` },
        body: { current_password: currentPassword, new_password: newPassword },
      });
      if (response.ok) {
        return;
      }
      throw new AuthError(response.status, messageFor(response.status, error));
    },

    /**
     * Revokes the session server-side. It never throws: the caller always drops
     * the local credential, so a failed round trip must not trap the operator
     * in a signed-in interface.
     */
    async logout(token: string): Promise<void> {
      await client
        .POST("/auth/logout", {
          headers: { Authorization: `Bearer ${token}` },
        })
        .catch(() => undefined);
    },
  };
}

export const authClient = createAuthClient();
