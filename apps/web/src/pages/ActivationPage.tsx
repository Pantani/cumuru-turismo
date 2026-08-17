import { AccountActivationForm } from "../features/activation/AccountActivationForm";
import { peekActivationCapability } from "../shared/security/activation-capability";

export default function ActivationPage() {
  const hasCapability = peekActivationCapability() !== null;

  return (
    <article className="page page-narrow">
      <div className="eyebrow">Acesso da hospedagem</div>
      <h1 data-route-heading tabIndex={-1}>
        Ativação da conta
      </h1>
      {hasCapability ? (
        <AccountActivationForm />
      ) : (
        <section className="boundary-note" aria-labelledby="activation-required">
          <h2 id="activation-required">Link de acesso necessário</h2>
          <p>
            Abra o link ou leia o código que a administração da hospedagem
            entregou a você. O token fica no fragmento do endereço, nunca é
            enviado ao servidor e é apagado da barra de endereço.
          </p>
        </section>
      )}
    </article>
  );
}
