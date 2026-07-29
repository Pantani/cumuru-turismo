import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { AppProviders } from "./app/AppProviders";
import { App } from "./app/App";
import { registerServiceWorker } from "./shared/offline/register-service-worker";
import { captureInviteCapability } from "./shared/security/invite-capability";
import { purgeExpiredDrafts } from "./shared/offline/encrypted-drafts";
import "./styles.css";

captureInviteCapability(
  new URL(window.location.href),
  (path) => window.history.replaceState(null, "", path),
);
void purgeExpiredDrafts().catch(() => {
  window.dispatchEvent(new Event("cumuru:draft-cleanup-failed"));
});
registerServiceWorker();

const rootElement = document.getElementById("root");

if (rootElement === null) {
  throw new Error("Elemento raiz da aplicação não encontrado.");
}

createRoot(rootElement).render(
  <StrictMode>
    <AppProviders>
      <App />
    </AppProviders>
  </StrictMode>,
);
