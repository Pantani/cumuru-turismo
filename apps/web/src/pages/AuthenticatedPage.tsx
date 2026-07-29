import { OperatorWorkspace } from "../features/operator/OperatorWorkspace";
import { useAuthSession } from "../shared/auth/AuthSession";

export default function AuthenticatedPage() {
  const session = useAuthSession();

  return (
    <article className="page operator-page">
      <div className="eyebrow">Área autenticada · Fase 2</div>
      <h1 data-route-heading tabIndex={-1}>
        Área autenticada
      </h1>
      {session.authenticated ? (
        <OperatorWorkspace />
      ) : (
        <section className="boundary-note" aria-labelledby="auth-required">
          <h2 id="auth-required">Acesso institucional necessário</h2>
          <p>
            O provedor OIDC institucional não entregou uma sessão. O acesso
            permanece bloqueado e esta aplicação não oferece login próprio.
          </p>
        </section>
      )}
    </article>
  );
}
