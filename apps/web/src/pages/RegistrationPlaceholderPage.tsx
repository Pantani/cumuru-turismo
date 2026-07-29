export default function RegistrationPlaceholderPage() {
  return (
    <article className="page placeholder-page">
      <div className="eyebrow">Área de registro · Fase 2</div>
      <h1 data-route-heading tabIndex={-1}>
        Registro de estadias
      </h1>
      <p className="lead">
        Esta rota já está separada para carregamento sob demanda. O cadastro de
        estadias, integrantes e convites será implementado somente na Fase 2.
      </p>
      <div className="boundary-note" role="note">
        <strong>Nenhum dado é coletado nesta fase.</strong>
        <span>
          Rascunhos offline e formulários ainda não estão habilitados.
        </span>
      </div>
    </article>
  );
}
