import { peekSurveyCapability } from "../../shared/security/survey-capability";

function SurveyContinuation() {
  if (peekSurveyCapability() === null) {
    return null;
  }
  return (
    <button
      className="primary-action"
      type="button"
      onClick={() => {
        window.history.pushState(null, "", "/pesquisa");
        window.dispatchEvent(new PopStateEvent("popstate"));
      }}
    >
      Responder pesquisa voluntária
    </button>
  );
}

/**
 * The confirmation is rendered from what the browser already had, never from
 * anything the server echoed back: the 201 body of the open channel carries no
 * personal field, and this screen must not create a reason for it to.
 */
export function SelfRegistrationCompletion() {
  return (
    <section className="form-card" aria-labelledby="self-registration-done">
      <h2 id="self-registration-done">Autocadastro enviado</h2>
      <p role="status" aria-live="polite">
        A hospedagem precisa aprovar este cadastro. Se ninguém decidir em 72
        horas, o pedido expira e os dados enviados são eliminados. Nada entra em
        estatística ou no painel público antes da aprovação.
      </p>
      <SurveyContinuation />
    </section>
  );
}
