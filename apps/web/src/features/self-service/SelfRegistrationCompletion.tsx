import { useLocale } from "../../shared/i18n/LocaleProvider";
import { peekSurveyCapability } from "../../shared/security/survey-capability";

function SurveyContinuation() {
  const { t } = useLocale();
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
      {t("selfService.completion.continueSurvey")}
    </button>
  );
}

/**
 * The confirmation is rendered from what the browser already had, never from
 * anything the server echoed back: the 201 body of the open channel carries no
 * personal field, and this screen must not create a reason for it to.
 */
export function SelfRegistrationCompletion() {
  const { t } = useLocale();
  return (
    <section className="form-card" aria-labelledby="self-registration-done">
      <h2 id="self-registration-done">{t("selfService.completion.title")}</h2>
      <p role="status" aria-live="polite">
        {t("selfService.completion.body")}
      </p>
      <SurveyContinuation />
    </section>
  );
}
