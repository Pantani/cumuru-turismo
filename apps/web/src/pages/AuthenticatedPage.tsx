import { AdminWorkspace } from "../features/admin/AdminWorkspace";
import { LoginForm } from "../features/auth/LoginForm";
import { PasswordRotationForm } from "../features/auth/PasswordRotationForm";
import { OperatorWorkspace } from "../features/operator/OperatorWorkspace";
import { useAuthSession } from "../shared/auth/AuthSession";

/**
 * `accommodations:onboard` é o que a API tem no lugar de um papel: ele libera
 * `POST /accommodations` e as três rotas de decisão da fila da ADR-042, cuja
 * listagem é global. Quem o carrega administra o cadastro da plataforma inteira,
 * e é por isso que ele — e não uma coluna de papel que não existe — escolhe a
 * tela. A hospedagem recebe seus escopos pela ativação e cai na outra área.
 */
const ADMIN_SCOPE = "accommodations:onboard";

interface Copy {
  title: string;
  lead: string;
}

const adminCopy: Copy = {
  title: "Área da administração",
  lead:
    "Aqui você cadastra hospedagem, decide os pedidos que chegam pelo site e cuida das perguntas do questionário. Registrar estadia e aprovar hóspede são atos de cada hospedagem, na conta dela.",
};

const operatorCopy: Copy = {
  title: "Área da hospedagem",
  lead:
    "Tudo o que você registra aqui vira estatística agregada — nada identificável é publicado.",
};

function SignedOutPage({ mustChangePassword }: { mustChangePassword: boolean }) {
  return (
    <article className="page page-narrow">
      <h1 data-route-heading tabIndex={-1}>
        Área da hospedagem
      </h1>
      <p className="lead">
        {mustChangePassword
          ? "Falta um passo antes de começar: trocar a senha provisória criada na instalação."
          : "Registre as estadias da sua pousada, casa de temporada ou quarto de família. Os dados alimentam o painel público de forma agregada."}
      </p>
      {mustChangePassword ? <PasswordRotationForm /> : <LoginForm />}
    </article>
  );
}

function SignedInPage({
  administrator,
  displayName,
}: {
  administrator: boolean;
  displayName: string;
}) {
  const copy = administrator ? adminCopy : operatorCopy;
  return (
    <article className="page">
      <div className="page-heading">
        <div>
          <h1 data-route-heading tabIndex={-1}>
            {copy.title}
          </h1>
          <p className="lead">
            Olá, {displayName}. {copy.lead}
          </p>
        </div>
      </div>
      {administrator ? <AdminWorkspace /> : <OperatorWorkspace />}
    </article>
  );
}

export default function AuthenticatedPage() {
  const session = useAuthSession();

  if (!session.authenticated) {
    return <SignedOutPage mustChangePassword={session.mustChangePassword} />;
  }

  return (
    <SignedInPage
      administrator={session.hasScope(ADMIN_SCOPE)}
      displayName={session.account?.display_name ?? ""}
    />
  );
}
