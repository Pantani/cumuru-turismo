import {
  Suspense,
  lazy,
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
import { peekInviteCapability } from "../shared/security/invite-capability";
import { peekSurveyCapability } from "../shared/security/survey-capability";

const PublicFoundationPage = lazy(
  () => import("../pages/PublicFoundationPage"),
);
const RegistrationPage = lazy(
  () => import("../pages/RegistrationPage"),
);
const AuthenticatedPage = lazy(
  () => import("../pages/AuthenticatedPage"),
);
const SurveyPage = lazy(() => import("../pages/SurveyPage"));
const QuestionnaireAdminPage = lazy(
  () => import("../pages/QuestionnaireAdminPage"),
);
const AnalyticsQualityPage = lazy(
  () => import("../pages/AnalyticsQualityPage"),
);
const NotFoundPage = lazy(() => import("../pages/NotFoundPage"));

const routeTitles: Record<AppPath, string> = {
  "/": "Fundação do observatório",
  "/registro": "Registro de estadias",
  "/pesquisa": "Pesquisa turística",
  "/acesso": "Área autenticada",
  "/questionarios": "Questionários",
  "/qualidade": "Qualidade dos dados",
};

function renderRoute(pathname: string) {
  switch (pathname) {
    case "/":
      return <PublicFoundationPage />;
    case "/registro":
      return <RegistrationPage />;
    case "/pesquisa":
      return <SurveyPage />;
    case "/acesso":
      return <AuthenticatedPage />;
    case "/questionarios":
      return <QuestionnaireAdminPage />;
    case "/qualidade":
      return <AnalyticsQualityPage />;
    default:
      return <NotFoundPage />;
  }
}

interface AppProps {
  routeRenderer?: (pathname: string) => ReactNode;
}

interface RouteContentProps {
  children: ReactNode;
  focusOnMount: boolean;
  onFocused: () => void;
}

interface QualityNavigationItemProps {
  authenticated: boolean;
  navigate: (nextPath: AppPath) => void;
  pathname: string;
}

function QualityNavigationItem({
  authenticated,
  navigate,
  pathname,
}: QualityNavigationItemProps) {
  if (!authenticated) {
    return null;
  }
  return (
    <li>
      <AppLink
        href="/qualidade"
        navigate={navigate}
        aria-current={pathname === "/qualidade" ? "page" : undefined}
      >
        Qualidade
      </AppLink>
    </li>
  );
}

interface PrimaryNavigationProps {
  administrativeNavigation: boolean;
  authenticated: boolean;
  hasInviteCapability: boolean;
  hasSurveyCapability: boolean;
  navigate: (nextPath: AppPath) => void;
  pathname: string;
}

function ContextualNavigationItems({
  hasInviteCapability,
  hasSurveyCapability,
  navigate,
  pathname,
}: Pick<
  PrimaryNavigationProps,
  "hasInviteCapability" | "hasSurveyCapability" | "navigate" | "pathname"
>) {
  return (
    <>
      {hasInviteCapability ? (
        <li>
          <AppLink
            href="/registro"
            navigate={navigate}
            aria-current={pathname === "/registro" ? "page" : undefined}
          >
            Registro
          </AppLink>
        </li>
      ) : null}
      {hasSurveyCapability ? (
        <li>
          <AppLink
            href="/pesquisa"
            navigate={navigate}
            aria-current={pathname === "/pesquisa" ? "page" : undefined}
          >
            Pesquisa
          </AppLink>
        </li>
      ) : null}
    </>
  );
}

function InstitutionalNavigationItems({
  administrativeNavigation,
  authenticated,
  navigate,
  pathname,
}: Pick<
  PrimaryNavigationProps,
  "administrativeNavigation" | "authenticated" | "navigate" | "pathname"
>) {
  return (
    <>
      {authenticated ? (
        <li>
          <AppLink
            href="/acesso"
            navigate={navigate}
            aria-current={pathname === "/acesso" ? "page" : undefined}
          >
            Área autenticada
          </AppLink>
        </li>
      ) : null}
      {administrativeNavigation ? (
        <li>
          <AppLink
            href="/questionarios"
            navigate={navigate}
            aria-current={pathname === "/questionarios" ? "page" : undefined}
          >
            Questionários
          </AppLink>
        </li>
      ) : null}
      {administrativeNavigation ? (
        <QualityNavigationItem
          authenticated={authenticated}
          navigate={navigate}
          pathname={pathname}
        />
      ) : null}
    </>
  );
}

function PrimaryNavigation({
  administrativeNavigation,
  authenticated,
  hasInviteCapability,
  hasSurveyCapability,
  navigate,
  pathname,
}: PrimaryNavigationProps) {
  return (
    <nav aria-label="Navegação principal">
      <ul className="navigation">
        <li>
          <AppLink
            href="/"
            navigate={navigate}
            aria-current={pathname === "/" ? "page" : undefined}
          >
            Visão pública
          </AppLink>
        </li>
        <ContextualNavigationItems
          hasInviteCapability={hasInviteCapability}
          hasSurveyCapability={hasSurveyCapability}
          navigate={navigate}
          pathname={pathname}
        />
        <InstitutionalNavigationItems
          administrativeNavigation={administrativeNavigation}
          authenticated={authenticated}
          navigate={navigate}
          pathname={pathname}
        />
      </ul>
    </nav>
  );
}

function RouteContent({
  children,
  focusOnMount,
  onFocused,
}: RouteContentProps) {
  const contentRef = useRef<HTMLDivElement>(null);
  const hasFocused = useRef(false);

  useLayoutEffect(() => {
    if (!focusOnMount || hasFocused.current) {
      return;
    }

    const heading =
      contentRef.current?.querySelector<HTMLElement>("[data-route-heading]");
    if (heading === undefined || heading === null) {
      return;
    }

    heading.focus();
    hasFocused.current = true;
    onFocused();
  }, [focusOnMount, onFocused]);

  return <div ref={contentRef}>{children}</div>;
}

export function App({ routeRenderer = renderRoute }: AppProps) {
  const session = useAuthSession();
  const [, setCapabilityRevision] = useState(0);
  const [pathname, setPathname] = useState(() =>
    normalizePathname(window.location.pathname),
  );
  const mainRef = useRef<HTMLElement>(null);
  const pendingRouteFocus = useRef(false);

  useEffect(() => {
    const handlePopState = () => {
      pendingRouteFocus.current = true;
      setPathname(normalizePathname(window.location.pathname));
    };

    window.addEventListener("popstate", handlePopState);
    return () => window.removeEventListener("popstate", handlePopState);
  }, []);

  useEffect(() => {
    const handleCapabilityChange = () => {
      setCapabilityRevision((current) => current + 1);
    };
    window.addEventListener(
      "cumuru:capability-change",
      handleCapabilityChange,
    );
    return () =>
      window.removeEventListener(
        "cumuru:capability-change",
        handleCapabilityChange,
      );
  }, []);

  const navigate = useCallback((nextPath: AppPath) => {
    if (window.location.pathname === nextPath) {
      return;
    }

    pendingRouteFocus.current = true;
    window.history.pushState(null, "", nextPath);
    setPathname(nextPath);
  }, []);

  const completeRouteFocus = useCallback(() => {
    pendingRouteFocus.current = false;
  }, []);

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

          <PrimaryNavigation
            administrativeNavigation={session.administrativeNavigation}
            authenticated={session.authenticated}
            hasInviteCapability={peekInviteCapability() !== null}
            hasSurveyCapability={peekSurveyCapability() !== null}
            navigate={navigate}
            pathname={pathname}
          />
        </div>
      </header>

      {session.authenticated && session.localDemo ? (
        <aside
          className="local-session-banner"
          aria-label="Sessão fictícia local"
        >
          <strong>Sessão fictícia local</strong>
          <span>PROTOTYPE_ONLY</span>
          <button type="button" onClick={() => void session.endSession()}>
            Encerrar sessão fictícia
          </button>
        </aside>
      ) : null}

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
