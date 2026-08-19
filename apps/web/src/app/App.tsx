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
import { useLocale } from "../shared/i18n/LocaleProvider";
import { LocaleSwitcher } from "../shared/i18n/LocaleSwitcher";
import type { MessageKey, Translate } from "../shared/i18n/translate";
import { CAPABILITY_CHANGE_EVENT } from "../shared/security/capability-events";
import { peekActivationCapability } from "../shared/security/activation-capability";
import { peekInviteCapability } from "../shared/security/invite-capability";
import { peekSelfServiceCapability } from "../shared/security/self-service-capability";
import { peekSurveyCapability } from "../shared/security/survey-capability";

const PublicFoundationPage = lazy(() => import("../pages/PublicFoundationPage"));
const RegistrationPage = lazy(() => import("../pages/RegistrationPage"));
const SelfRegistrationPage = lazy(
  () => import("../pages/SelfRegistrationPage"),
);
const InviteRequestPage = lazy(() => import("../pages/InviteRequestPage"));
const ActivationPage = lazy(() => import("../pages/ActivationPage"));
const AccommodationDirectoryPage = lazy(
  () => import("../pages/AccommodationDirectoryPage"),
);
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
  "/i": SelfRegistrationPage,
  "/convite": InviteRequestPage,
  "/hospedagens": AccommodationDirectoryPage,
  "/ativacao": ActivationPage,
  "/pesquisa": SurveyPage,
  "/acesso": AuthenticatedPage,
  "/questionarios": QuestionnaireAdminPage,
  "/qualidade": AnalyticsQualityPage,
};

const routeTitles: Record<AppPath, MessageKey> = {
  "/": "app.title.public",
  "/registro": "app.title.registration",
  "/i": "app.title.selfRegistration",
  "/convite": "app.title.inviteRequest",
  "/hospedagens": "app.title.directory",
  "/ativacao": "app.title.activation",
  "/pesquisa": "app.title.survey",
  "/acesso": "app.title.workspace",
  "/questionarios": "app.title.questionnaires",
  "/qualidade": "app.title.quality",
};

function renderRoute(pathname: string) {
  const Page = routePages[pathname as AppPath] ?? NotFoundPage;
  return <Page />;
}

interface NavigationEntry {
  href: AppPath;
  label: MessageKey;
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
  const { t } = useLocale();
  return (
    <nav aria-label={t("app.nav.aria")}>
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
                {t(entry.label)}
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

/**
 * O anúncio ao vivo já conta a troca de rota a quem está na página; a aba do
 * navegador continuava dizendo só o nome do site. Quem trabalha com várias
 * abas abertas precisa distinguir "Registro" de "Qualidade" pelo título.
 */
function useDocumentTitle(title: string) {
  useEffect(() => {
    document.title = title;
  }, [title]);
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
    { href: "/", label: "app.nav.public", visible: true },
    { href: "/hospedagens", label: "app.nav.directory", visible: true },
    {
      href: "/registro",
      label: "app.nav.registration",
      visible: peekInviteCapability() !== null,
    },
    {
      href: "/i",
      label: "app.nav.selfRegistration",
      visible: peekSelfServiceCapability() !== null,
    },
    {
      href: "/ativacao",
      label: "app.nav.activation",
      visible: peekActivationCapability() !== null,
    },
    {
      href: "/pesquisa",
      label: "app.nav.survey",
      visible: peekSurveyCapability() !== null,
    },
    {
      href: "/acesso",
      // O rótulo segue a tela que a rota abre: administração e hospedagem são
      // áreas diferentes atrás do mesmo endereço (ver AuthenticatedPage).
      label:
        authenticated && hasScope("accommodations:onboard")
          ? "app.nav.administration"
          : "app.nav.workspace",
      visible: true,
    },
    {
      href: "/questionarios",
      label: "app.nav.questionnaires",
      visible: authenticated && hasScope("questionnaires:manage"),
    },
    {
      href: "/qualidade",
      label: "app.nav.quality",
      visible: authenticated && hasScope("analytics:read:internal"),
    },
  ];
}

function SessionBadge() {
  const { account, authenticated, endSession } = useAuthSession();
  const { t } = useLocale();
  if (!authenticated || account === null) {
    return null;
  }
  return (
    <div className="session-badge">
      <span className="session-name">{account.display_name}</span>
      <button type="button" className="quiet-action" onClick={() => void endSession()}>
        {t("app.signOut")}
      </button>
    </div>
  );
}

function routeTitle(t: Translate, pathname: string): string {
  const key = routeTitles[pathname as AppPath];
  return key === undefined ? t("app.title.notFound") : t(key);
}

export function App({ routeRenderer = renderRoute }: AppProps) {
  const { authenticated, hasScope } = useAuthSession();
  const { t } = useLocale();
  const { completeRouteFocus, navigate, pathname, pendingRouteFocus } =
    useRouting();
  const mainRef = useRef<HTMLElement>(null);
  useCapabilityRevision();

  const currentTitle = routeTitle(t, pathname);
  useDocumentTitle(t("app.documentTitle", { page: currentTitle }));

  return (
    <div className="site-shell">
      <a
        className="skip-link"
        href="#conteudo"
        onClick={() => mainRef.current?.focus()}
      >
        {t("app.skipLink")}
      </a>

      <header className="site-header">
        <div className="header-inner">
          <AppLink className="brand" href="/" navigate={navigate}>
            <span className="brand-mark" aria-hidden="true">
              C
            </span>
            <span>
              <strong>{t("app.brand.name")}</strong>
              <small>{t("app.brand.place")}</small>
            </span>
          </AppLink>

          <div className="header-actions">
            <PrimaryNavigation
              entries={navigationEntries(authenticated, hasScope)}
              navigate={navigate}
              pathname={pathname}
            />
            <LocaleSwitcher />
            <SessionBadge />
          </div>
        </div>
      </header>

      <p className="route-announcer" aria-live="polite" aria-atomic="true">
        {t("app.routeAnnounce", { title: currentTitle })}
      </p>

      <main id="conteudo" ref={mainRef} tabIndex={-1}>
        <Suspense
          fallback={
            <div className="route-status" role="status" aria-live="polite">
              {t("app.routeLoading")}
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
        <p>{t("app.footer.note")}</p>
        <p className="site-footer-mark">{t("landing.contact.mark")}</p>
      </footer>
    </div>
  );
}
