import { QuestionnaireAdmin } from "../features/questionnaire/QuestionnaireAdmin";
import { useAuthSession } from "../shared/auth/AuthSession";

export default function QuestionnaireAdminPage() {
  const session = useAuthSession();

  return (
    <article className="page operator-page">
      <div className="eyebrow">Questionários · Fase 3</div>
      <h1 data-route-heading tabIndex={-1}>
        Questionários
      </h1>
      {session.authenticated ? (
        <QuestionnaireAdmin client={session.questionnaireClient} />
      ) : (
        <section className="boundary-note" aria-labelledby="questionnaire-auth">
          <h2 id="questionnaire-auth">Acesso institucional necessário</h2>
          <p>
            O editor e o revisor dependem exclusivamente dos escopos OIDC
            institucionais. Esta aplicação não oferece autenticação própria.
          </p>
        </section>
      )}
    </article>
  );
}
