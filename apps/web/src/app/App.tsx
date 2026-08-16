import {
  Suspense,
  lazy,
  type ComponentType,
  type ReactNode,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

import { AppLink } from "./AppLink";
import { normalizePathname, type AppPath } from "./routes";
import { useAuthSession } from "../shared/auth/AuthSession";
import { CAPABILITY_CHANGE_EVENT } from "../shared/security/capability-events";
import { peekInviteCapability } from "../shared/security/invite-capability";
import { peekSurveyCapability } from "../shared/security/survey-capability";

const PublicFoundationPage = lazy(() => import("../pages/PublicFoundationPage"));
const RegistrationPage = lazy(() => import("../pages/RegistrationPage"));
const AuthenticatedPage = lazy(() => import("../pages/AuthenticatedPage"));
const SurveyPage = lazy(() => import("../pages/SurveyPage"));
const QuestionnaireAdminPage = lazy(
  () => import("../pages/QuestionnaireAdminPage"),
);
const AnalyticsQualityPage = lazy(() => import("../pages/AnalyticsQualityPage"));
const NotFoundPage = lazy(() => import("../pages/NotFoundPage"));

const routePages: Record<AppPath, ComponentType> = {
  "/": PublicFoundationPage,
  "/registro": RegistrationPage,
  "/pesquisa": SurveyPage,
  "/acesso": AuthenticatedPage,
  "/questionarios": QuestionnaireAdminPage,
  "/qualidade": AnalyticsQualityPage,
};

const routeTitles: Record<AppPath, string> = {
  "/": "Painel público do turismo",
  "/registro": "Registro de visitantes",
  "/pesquisa": "Pesquisa com o visitante",
  "/acesso": "Área da hospedagem",
  "/questionarios": "Questionários",
  "/qualidade": "Qualidade dos dados",
};

function renderRoute(pathname: string) {
  const Page = routePages[pathname as AppPath] ?? NotFoundPage;
  return <Page />;
}

interface NavigationEntry {
  href: AppPath;
  label: string;
  visible: boolean;
}

interface AppProps {
  routeRenderer?: (pathname: string) => ReactNode;
}

interface RouteContentProps {
  children: ReactNode;
  focusOnMount: boolean;
  onFocused: () => void;
}

interface PrimaryNavigationProps {
  entries: readonly NavigationEntry[];
  navigate: (nextPath: AppPath) => void;
  pathname: string;
}

function PrimaryNavigation({
  entries,
  navigate,
  pathname,
}: PrimaryNavigationProps) {
  return (
    <nav aria-label="Navegação principal">
      <ul className="navigation">
        {entries
          .filter((entry) => entry.visible)
          .map((entry) => (
            <li key={entry.href}>
              <AppLink
                href={entry.href}
                navigate={navigate}
                aria-current={pathname === entry.href ? "page" : undefined}
              >
                {entry.label}
              </AppLink>
            </li>
          ))}
      </ul>
    </nav>
  );
}

/** Moves focus to the new route heading so a keyboard user lands on the change. */
function focusRouteHeading(container: HTMLDivElement | null) {
  const heading = container?.querySelector<HTMLElement>("[data-route-heading]");
  if (heading === undefined || heading === null) {
    return false;
  }
  heading.focus();
  return true;
}

function RouteContent({ children, focusOnMount, onFocused }: RouteContentProps) {
  const contentRef = useRef<HTMLDivElement>(null);
  const hasFocused = useRef(false);

  useLayoutEffect(() => {
    if (!focusOnMount || hasFocused.current) {
      return;
    }
    if (!focusRouteHeading(contentRef.current)) {
      return;
    }
    hasFocused.current = true;
    onFocused();
  }, [focusOnMount, onFocused]);

  return <div ref={contentRef}>{children}</div>;
}

function useCapabilityRevision() {
  const [, setRevision] = useState(0);
  useEffect(() => {
    const handle = () => setRevision((current) => current + 1);
    window.addEventListener(CAPABILITY_CHANGE_EVENT, handle);
    return () => window.removeEventListener(CAPABILITY_CHANGE_EVENT, handle);
  }, []);
}

function useRouting() {
  const [pathname, setPathname] = useState(() =>
    normalizePathname(window.location.pathname),
  );
  const pendingRouteFocus = useRef(false);

  useEffect(() => {
    const handlePopState = () => {
      pendingRouteFocus.current = true;
      setPathname(normalizePathname(window.location.pathname));
    };
    window.addEventListener("popstate", handlePopState);
    return () => window.removeEventListener("popstate", handlePopState);
  }, []);

  /**
   * The guard compares against the rendered route, not the address bar.
   * `captureInviteCapability` scrubs the token by replacing the URL with
   * `/registro` while the view is still the operator area; guarding on
   * `window.location` made every later navigation to that path a no-op and left
   * the invite flow with no way forward. The push is still skipped when the
   * address already matches, so no duplicate history entry is created.
   */
  const navigate = useCallback(
    (nextPath: AppPath) => {
      if (pathname === nextPath) {
        return;
      }
      pendingRouteFocus.current = true;
      if (window.location.pathname !== nextPath) {
        window.history.pushState(null, "", nextPath);
      }
      setPathname(nextPath);
    },
    [pathname],
  );

  const completeRouteFocus = useCallback(() => {
    pendingRouteFocus.current = false;
  }, []);

  return { completeRouteFocus, navigate, pathname, pendingRouteFocus };
}

function navigationEntries(
  authenticated: boolean,
  hasScope: (scope: string) => boolean,
): NavigationEntry[] {
  return [
    { href: "/", label: "Painel público", visible: true },
    {
      href: "/registro",
      label: "Registro",
      visible: peekInviteCapability() !== null,
    },
    {
      href: "/pesquisa",
      label: "Pesquisa",
      visible: peekSurveyCapability() !== null,
    },
    { href: "/acesso", label: "Área da hospedagem", visible: true },
    {
      href: "/questionarios",
      label: "Questionários",
      visible: authenticated && hasScope("questionnaires:manage"),
    },
    {
      href: "/qualidade",
      label: "Qualidade",
      visible: authenticated && hasScope("analytics:read:internal"),
    },
  ];
}

function SessionBadge() {
  const { account, authenticated, endSession } = useAuthSession();
  if (!authenticated || account === null) {
    return null;
  }
  return (
    <div className="session-badge">
      <span className="session-name">{account.display_name}</span>
      <button type="button" className="quiet-action" onClick={() => void endSession()}>
        Sair
      </button>
    </div>
  );
}

export function App({ routeRenderer = renderRoute }: AppProps) {
  const { authenticated, hasScope } = useAuthSession();
  const { completeRouteFocus, navigate, pathname, pendingRouteFocus } =
    useRouting();
  const mainRef = useRef<HTMLElement>(null);
  useCapabilityRevision();

  const currentTitle =
    routeTitles[pathname as AppPath] ?? "Página não encontrada";

  return (
    <div className="site-shell">
      <a
        className="skip-link"
        href="#conteudo"
        onClick={() => mainRef.current?.focus()}
      >
        Ir para o conteúdo
      </a>

      <header className="site-header">
        <div className="header-inner">
          <AppLink className="brand" href="/" navigate={navigate}>
            <span className="brand-mark" aria-hidden="true">
              C
            </span>
            <span>
              <strong>Observatório Turístico</strong>
              <small>Cumuruxatiba · Prado, Bahia</small>
            </span>
          </AppLink>

          <div className="header-actions">
            <PrimaryNavigation
              entries={navigationEntries(authenticated, hasScope)}
              navigate={navigate}
              pathname={pathname}
            />
            <SessionBadge />
          </div>
        </div>
      </header>

      <p className="route-announcer" aria-live="polite" aria-atomic="true">
        Página atual: {currentTitle}
      </p>

      <main id="conteudo" ref={mainRef} tabIndex={-1}>
        <Suspense
          fallback={
            <div className="route-status" role="status" aria-live="polite">
              Carregando página…
            </div>
          }
        >
          <RouteContent
            key={pathname}
            focusOnMount={pendingRouteFocus.current}
            onFocused={completeRouteFocus}
          >
            {routeRenderer(pathname)}
          </RouteContent>
        </Suspense>
      </main>

      <footer className="site-footer">
        <p>
          Protótipo técnico com dados fictícios. Uso real depende dos gates de
          governança do Município de Prado.
        </p>
      </footer>
    </div>
  );
}
