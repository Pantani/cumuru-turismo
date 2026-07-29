import {
  createContext,
  type PropsWithChildren,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
} from "react";

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
  analyticsClient: Phase4Client;
  authenticated: boolean;
  client: Phase2Client;
  questionnaireClient: Phase3Client;
  endSession: () => Promise<void>;
}

interface AuthSessionProviderProps extends PropsWithChildren {
  accessToken?: string | null;
}

const failClosedClient = createPhase2Client({ getAccessToken: () => null });
const failClosedQuestionnaireClient = createPhase3Client({
  getAccessToken: () => null,
});
const failClosedAnalyticsClient = createPhase4Client({
  getAccessToken: () => null,
});
const failClosedSession: AuthSessionValue = {
  analyticsClient: failClosedAnalyticsClient,
  authenticated: false,
  client: failClosedClient,
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
}: AuthSessionProviderProps) {
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
    await clearAllDrafts();
  }, []);

  const value = useMemo(
    () => ({
      analyticsClient,
      authenticated,
      client,
      questionnaireClient,
      endSession,
    }),
    [
      analyticsClient,
      authenticated,
      client,
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
