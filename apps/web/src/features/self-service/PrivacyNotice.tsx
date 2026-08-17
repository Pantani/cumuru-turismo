interface PrivacyNoticeProps {
  accommodationName: string;
  version: string;
}

/**
 * There is no operator between the person and the collection in this channel,
 * so the notice is part of the form, shown before the first field and pinned to
 * the version the poster was printed under. A poster printed against an older
 * notice fails with a conflict instead of succeeding silently — that is the
 * intended behaviour, and it is why the version is on screen.
 */
export function PrivacyNotice({ accommodationName, version }: PrivacyNoticeProps) {
  return (
    <section className="privacy-notice" aria-labelledby="privacy-notice-title">
      <h2 id="privacy-notice-title">Aviso de privacidade</h2>
      <p>
        Você está iniciando um cadastro para <strong>{accommodationName}</strong>.
        Este formulário é aberto e não pede identificação: coletamos apenas
        faixa etária, papel no grupo, residência e datas previstas.
      </p>
      <ul>
        <li>
          Não informe nome, documento, e-mail, telefone, endereço ou qualquer
          observação pessoal. Estes dados não são aceitos aqui.
        </li>
        <li>
          A hospedagem precisa aprovar o cadastro. Sem aprovação, nada entra em
          estatística nem no painel público.
        </li>
        <li>
          Se a hospedagem recusar, ou se ninguém decidir em 72 horas, os dados
          enviados são eliminados e resta apenas o registro auditável da decisão.
        </li>
        <li>
          Se preferir não usar este formulário, peça à hospedagem para registrar
          a estadia com você, pelo canal assistido.
        </li>
      </ul>
      <p className="notice-version">
        Versão do aviso: <code>{version}</code>
      </p>
    </section>
  );
}
