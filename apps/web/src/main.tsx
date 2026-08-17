import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { AppProviders } from "./app/AppProviders";
import { App } from "./app/App";
import { registerServiceWorker } from "./shared/offline/register-service-worker";
import { captureInviteCapability } from "./shared/security/invite-capability";
import { purgeExpiredDrafts } from "./shared/offline/encrypted-drafts";
// Fontes empacotadas no próprio bundle: a CSP serve `default-src 'self'`, então
// nenhum host externo é alcançável. A entrada do pacote traz só os pesos
// normais; cada subconjunto tem unicode-range, então o navegador baixa apenas o
// latino.
import "@fontsource-variable/geist";
import "@fontsource-variable/geist-mono";
// Display editorial da capa pública: um eixo de peso só, mesmo empacotamento.
import "@fontsource-variable/bricolage-grotesque";
import "./styles.css";
import "./landing.css";

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
