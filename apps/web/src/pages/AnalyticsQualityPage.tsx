import { AnalyticsQuality } from "../features/analytics/AnalyticsQuality";
import { useAuthSession } from "../shared/auth/AuthSession";

export default function AnalyticsQualityPage() {
  const session = useAuthSession();

  return (
    <article className="page operator-page">
      <div className="eyebrow">Analytics interno · Fase 4</div>
      <h1 data-route-heading tabIndex={-1}>
        Qualidade dos dados
      </h1>
      {session.authenticated ? (
        <AnalyticsQuality client={session.analyticsClient} />
      ) : (
        <section className="boundary-note" aria-labelledby="quality-auth">
          <h2 id="quality-auth">Acesso institucional necessário</h2>
          <p>
            O painel exige sessão OIDC e o escopo
            {" "}
            <code>analytics:read:internal</code>. A aplicação não oferece
            autenticação própria.
          </p>
        </section>
      )}
    </article>
  );
}
