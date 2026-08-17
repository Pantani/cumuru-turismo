import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, type RenderResult } from "@testing-library/react";
import { useEffect, useRef, type ReactNode } from "react";

import type { AuthClient, Session } from "../shared/api/auth-client";
import { AuthSessionProvider, useAuthSession } from "../shared/auth/AuthSession";
import { LocaleProvider } from "../shared/i18n/LocaleProvider";

export const testAccountScopes = [
  "platform:read",
  "accommodations:onboard",
  "accommodations:manage",
  "stays:read:own",
  "stays:write",
  "questionnaires:manage",
  "questionnaires:approve",
  "analytics:read:internal",
];

export function testSession(
  scopes: readonly string[] = testAccountScopes,
  mustChangePassword = false,
): Session {
  return {
    token: "cms_test-session-token",
    expires_at: "2026-08-16T12:00:00Z",
    account: {
      id: "019fae14-0000-7000-8000-000000000001",
      email: "operador@cumuru.local",
      display_name: "Operadora de teste",
      scopes: [...scopes],
      must_change_password: mustChangePassword,
    },
  };
}

export function stubAuthClient(session = testSession()): AuthClient {
  return {
    login: async () => session,
    describe: async () => session,
    rotatePassword: async () => undefined,
    logout: async () => undefined,
  };
}

/**
 * SignInOnMount drives the real sign-in path instead of seeding provider state,
 * so tests exercise the same code an operator does.
 */
function SignInOnMount({ children }: { children: ReactNode }) {
  const { authenticated, signIn } = useAuthSession();
  const started = useRef(false);
  useEffect(() => {
    if (started.current) {
      return;
    }
    started.current = true;
    void signIn("operador@cumuru.local", "fixture-fixture-");
  }, [signIn]);
  return authenticated ? <>{children}</> : null;
}

interface RenderOptions {
  authClient?: AuthClient;
  signedIn?: boolean;
}

function newQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0 } },
  });
}

export function renderWithSession(
  ui: ReactNode,
  { authClient = stubAuthClient(), signedIn = true }: RenderOptions = {},
): RenderResult {
  return render(
    <QueryClientProvider client={newQueryClient()}>
      <LocaleProvider initial="pt">
        <AuthSessionProvider authClient={authClient}>
          {signedIn ? <SignInOnMount>{ui}</SignInOnMount> : ui}
        </AuthSessionProvider>
      </LocaleProvider>
    </QueryClientProvider>,
  );
}
