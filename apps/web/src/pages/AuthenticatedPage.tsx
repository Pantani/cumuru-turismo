import { LoginForm } from "../features/auth/LoginForm";
import { PasswordRotationForm } from "../features/auth/PasswordRotationForm";
import { OperatorWorkspace } from "../features/operator/OperatorWorkspace";
import { useAuthSession } from "../shared/auth/AuthSession";

export default function AuthenticatedPage() {
  const session = useAuthSession();

  if (!session.authenticated) {
    return (
      <article className="page page-narrow">
        <h1 data-route-heading tabIndex={-1}>
          Área da hospedagem
        </h1>
        <p className="lead">
          {session.mustChangePassword
            ? "Falta um passo antes de começar: trocar a senha provisória criada na instalação."
            : "Registre as estadias da sua pousada, casa de temporada ou quarto de família. Os dados alimentam o painel público de forma agregada."}
        </p>
        {session.mustChangePassword ? <PasswordRotationForm /> : <LoginForm />}
      </article>
    );
  }

  return (
    <article className="page">
      <div className="page-heading">
        <div>
          <h1 data-route-heading tabIndex={-1}>
            Área da hospedagem
          </h1>
          <p className="lead">
            Olá, {session.account?.display_name}. Tudo o que você registra aqui
            vira estatística agregada — nada identificável é publicado.
          </p>
        </div>
      </div>
      <OperatorWorkspace />
    </article>
  );
}
