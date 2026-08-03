import {
  InviteRegistration,
  RegistrationCompletion,
} from "../features/invite/InviteRegistration";
import { peekInviteCapability } from "../shared/security/invite-capability";
import { peekSurveyCapability } from "../shared/security/survey-capability";

export default function RegistrationPage() {
  const hasCapability = peekInviteCapability() !== null;
  const hasSurveyCapability = peekSurveyCapability() !== null;

  return (
    <article className="page registration-page">
      <div className="eyebrow">Registro por convite · Fase 2</div>
      <h1 data-route-heading tabIndex={-1}>
        Registro de estadias
      </h1>
      {hasCapability ? (
        <InviteRegistration />
      ) : hasSurveyCapability ? (
        <RegistrationCompletion />
      ) : (
        <section className="boundary-note" aria-labelledby="invite-required">
          <h2 id="invite-required">Convite necessário</h2>
          <p>
            Abra o convite fornecido pela acomodação. A capability é removida
            da barra de endereço antes desta página ser exibida.
          </p>
        </section>
      )}
    </article>
  );
}
