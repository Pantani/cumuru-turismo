import {
  createContext,
  type PropsWithChildren,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
} from "react";
import { useQueryClient } from "@tanstack/react-query";

import {
  authClient as defaultAuthClient,
  type AuthClient,
  type SessionAccount,
} from "../api/auth-client";
import { createPhase2Client, type Phase2Client } from "../api/phase2-client";
import { createPhase3Client, type Phase3Client } from "../api/phase3-client";
import { createPhase4Client, type Phase4Client } from "../api/phase4-client";
import { clearAllDrafts } from "../offline/encrypted-drafts";
import { clearInviteCapability } from "../security/invite-capability";
import { clearSurveyCapability } from "../security/survey-capability";

interface AuthSessionValue {
  account: SessionAccount | null;
  analyticsClient: Phase4Client;
  authenticated: boolean;
  client: Phase2Client;
  mustChangePassword: boolean;
  questionnaireClient: Phase3Client;
  signIn: (email: string, password: string) => Promise<void>;
  rotatePassword: (current: string, next: string) => Promise<void>;
  endSession: () => Promise<void>;
  hasScope: (scope: string) => boolean;
}

interface AuthSessionProviderProps extends PropsWithChildren {
  authClient?: AuthClient;
}

const failClosedClient = createPhase2Client({ getAccessToken: () => null });
const failClosedQuestionnaireClient = createPhase3Client({
  getAccessToken: () => null,
});
const failClosedAnalyticsClient = createPhase4Client({
  getAccessToken: () => null,
});

async function clearLocalTraces() {
  clearInviteCapability();
  clearSurveyCapability();
  await clearAllDrafts();
}

const failClosedSession: AuthSessionValue = {
  account: null,
  analyticsClient: failClosedAnalyticsClient,
  authenticated: false,
  client: failClosedClient,
  mustChangePassword: false,
  questionnaireClient: failClosedQuestionnaireClient,
  signIn: async () => {
    throw new Error("Sessão indisponível fora do provedor de autenticação.");
  },
  rotatePassword: async () => {
    throw new Error("Sessão indisponível fora do provedor de autenticação.");
  },
  endSession: clearLocalTraces,
  hasScope: () => false,
};

const AuthSessionContext = createContext<AuthSessionValue>(failClosedSession);

export function AuthSessionProvider({
  authClient = defaultAuthClient,
  children,
}: AuthSessionProviderProps) {
  const queryClient = useQueryClient();
  // The token lives in a ref, never in state or storage: it must not reach a
  // render tree, a devtools snapshot, localStorage or the service worker cache.
  const tokenRef = useRef<string | null>(null);
  const [account, setAccount] = useState<SessionAccount | null>(null);
  const clients = useMemo(() => buildClients(tokenRef), []);

  const signIn = useCallback(
    async (email: string, password: string) => {
      const session = await authClient.login(email, password);
      tokenRef.current = session.token;
      setAccount(session.account);
    },
    [authClient],
  );

  // The rotation revokes every session opened with the previous secret, so the
  // new one is obtained by signing in again rather than by keeping the token
  // that requested the change.
  const rotatePassword = useCallback(
    async (current: string, next: string) => {
      const token = tokenRef.current;
      const email = account?.email;
      if (token === null || email === undefined) {
        throw new Error("Sessão expirada. Entre novamente para trocar a senha.");
      }
      await authClient.rotatePassword(token, current, next);
      tokenRef.current = null;
      const session = await authClient.login(email, next);
      tokenRef.current = session.token;
      setAccount(session.account);
    },
    [account, authClient],
  );

  const endSession = useCallback(async () => {
    const token = tokenRef.current;
    tokenRef.current = null;
    setAccount(null);
    queryClient.clear();
    if (token !== null) {
      await authClient.logout(token);
    }
    await clearLocalTraces();
  }, [authClient, queryClient]);

  const hasScope = useCallback(
    (scope: string) => account?.scopes.includes(scope) ?? false,
    [account],
  );

  const value = useMemo(
    () => ({
      account,
      // A provisional credential authenticates but authorizes nothing, so the
      // interface treats it as not yet signed in: the API closes every scoped
      // route until the rotation lands.
      authenticated: account !== null && !account.must_change_password,
      mustChangePassword: account?.must_change_password ?? false,
      signIn,
      rotatePassword,
      endSession,
      hasScope,
      ...clients,
    }),
    [account, clients, endSession, hasScope, rotatePassword, signIn],
  );

  return (
    <AuthSessionContext.Provider value={value}>
      {children}
    </AuthSessionContext.Provider>
  );
}

function buildClients(tokenRef: { current: string | null }) {
  const getAccessToken = () => tokenRef.current;
  return {
    client: createPhase2Client({ getAccessToken }),
    questionnaireClient: createPhase3Client({ getAccessToken }),
    analyticsClient: createPhase4Client({ getAccessToken }),
  };
}

export function useAuthSession() {
  return useContext(AuthSessionContext);
}
