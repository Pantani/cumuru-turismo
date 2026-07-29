export default function NotFoundPage() {
  return (
    <article className="page placeholder-page">
      <div className="eyebrow">Erro 404</div>
      <h1 data-route-heading tabIndex={-1}>
        Página não encontrada
      </h1>
      <p className="lead">
        O endereço informado não corresponde a uma área disponível do
        observatório.
      </p>
      <a className="text-link" href="/">
        Voltar à página inicial
      </a>
    </article>
  );
}
