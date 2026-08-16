export default function AuthenticatedPlaceholderPage() {
  return (
    <article className="page placeholder-page">
      <div className="eyebrow">Área da hospedagem</div>
      <h1 data-route-heading tabIndex={-1}>
        Área autenticada
      </h1>
      <p className="lead">
        A fronteira administrativa está reservada, mas ainda não oferece login
        ou funções operacionais. A integração institucional OIDC permanece
        pendente de configuração.
      </p>
      <div className="boundary-note" role="note">
        <strong>Autenticação fail-closed.</strong>
        <span>
          Esta interface não armazena tokens nem implementa autenticação
          própria.
        </span>
      </div>
    </article>
  );
}
