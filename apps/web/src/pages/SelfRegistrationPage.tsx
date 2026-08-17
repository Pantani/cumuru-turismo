import { SelfRegistrationForm } from "../features/self-service/SelfRegistrationForm";
import { peekSelfServiceCapability } from "../shared/security/self-service-capability";

export default function SelfRegistrationPage() {
  const hasCapability = peekSelfServiceCapability() !== null;

  return (
    <article className="page registration-page">
      <div className="eyebrow">Cartaz da hospedagem</div>
      <h1 data-route-heading tabIndex={-1}>
        Autocadastro pelo cartaz
      </h1>
      {hasCapability ? (
        <SelfRegistrationForm />
      ) : (
        <section className="boundary-note" aria-labelledby="poster-required">
          <h2 id="poster-required">Cartaz necessário</h2>
          <p>
            Leia o código QR exposto pela hospedagem. O token fica no fragmento
            do endereço, nunca é enviado ao servidor e é apagado da barra de
            endereço antes desta página aparecer.
          </p>
        </section>
      )}
    </article>
  );
}
