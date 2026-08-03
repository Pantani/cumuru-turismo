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
  createPhase2Client,
  type Phase2Client,
} from "../api/phase2-client";
import {
  createPhase3Client,
  type Phase3Client,
} from "../api/phase3-client";
import {
  createPhase4Client,
  type Phase4Client,
} from "../api/phase4-client";
import { clearAllDrafts } from "../offline/encrypted-drafts";
import { clearInviteCapability } from "../security/invite-capability";
import { clearSurveyCapability } from "../security/survey-capability";

interface AuthSessionValue {
  administrativeNavigation: boolean;
  analyticsClient: Phase4Client;
  authenticated: boolean;
  client: Phase2Client;
  localDemo: boolean;
  questionnaireClient: Phase3Client;
  endSession: () => Promise<void>;
}

interface AuthSessionProviderProps extends PropsWithChildren {
  accessToken?: string | null;
  localDemo?: boolean;
}

export function localDemoAccessToken(
  enabled: boolean,
  configuredToken: string | undefined,
  hostname: string,
) {
  if (
    !enabled ||
    configuredToken === undefined ||
    configuredToken.length === 0 ||
    !isLoopbackHostname(hostname)
  ) {
    return null;
  }
  return configuredToken;
}

export function isLoopbackHostname(hostname: string) {
  const normalized = hostname.trim().toLowerCase();
  return normalized === "localhost" ||
    normalized === "127.0.0.1" ||
    normalized === "::1" ||
    normalized === "[::1]";
}

const failClosedClient = createPhase2Client({ getAccessToken: () => null });
const failClosedQuestionnaireClient = createPhase3Client({
  getAccessToken: () => null,
});
const failClosedAnalyticsClient = createPhase4Client({
  getAccessToken: () => null,
});
const failClosedSession: AuthSessionValue = {
  administrativeNavigation: false,
  analyticsClient: failClosedAnalyticsClient,
  authenticated: false,
  client: failClosedClient,
  localDemo: false,
  questionnaireClient: failClosedQuestionnaireClient,
  endSession: async () => {
    clearInviteCapability();
    clearSurveyCapability();
    await clearAllDrafts();
  },
};
const AuthSessionContext =
  createContext<AuthSessionValue>(failClosedSession);

export function AuthSessionProvider({
  accessToken = null,
  children,
  localDemo = false,
}: AuthSessionProviderProps) {
  const queryClient = useQueryClient();
  const initialToken =
    accessToken === null || accessToken.length === 0 ? null : accessToken;
  const tokenRef = useRef<string | null>(initialToken);
  const [authenticated, setAuthenticated] = useState(initialToken !== null);
  const client = useMemo(
    () => createPhase2Client({ getAccessToken: () => tokenRef.current }),
    [],
  );
  const questionnaireClient = useMemo(
    () => createPhase3Client({ getAccessToken: () => tokenRef.current }),
    [],
  );
  const analyticsClient = useMemo(
    () => createPhase4Client({ getAccessToken: () => tokenRef.current }),
    [],
  );
  const endSession = useCallback(async () => {
    tokenRef.current = null;
    setAuthenticated(false);
    clearInviteCapability();
    clearSurveyCapability();
    queryClient.clear();
    await clearAllDrafts();
  }, [queryClient]);

  const value = useMemo(
    () => ({
      administrativeNavigation:
        authenticated && !localDemo,
      analyticsClient,
      authenticated,
      client,
      localDemo,
      questionnaireClient,
      endSession,
    }),
    [
      analyticsClient,
      authenticated,
      client,
      initialToken,
      localDemo,
      questionnaireClient,
      endSession,
    ],
  );

  return (
    <AuthSessionContext.Provider value={value}>
      {children}
    </AuthSessionContext.Provider>
  );
}

export function useAuthSession() {
  return useContext(AuthSessionContext);
}
