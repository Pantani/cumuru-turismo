import { InviteRequestForm } from "../features/invite-request/InviteRequestForm";
import { useLocale } from "../shared/i18n/LocaleProvider";

/**
 * Superfície pública de entrada da hospedagem. Antes desta página, o convite da
 * capa levava para `/acesso`, que pede credencial — inútil para quem ainda não
 * tem conta. Aqui a pousada descreve o que tem e deixa um contato; o acesso em
 * si continua sendo criado pela administração, depois da análise da fila.
 */
export default function InviteRequestPage() {
  const { t } = useLocale();
  return (
    <article className="page page-narrow">
      <div className="page-eyebrow-row">
        <div className="eyebrow">{t("inviteRequest.eyebrow")}</div>
        <span className="prototype-badge">{t("landing.ticker.prototype")}</span>
      </div>
      <h1 data-route-heading tabIndex={-1}>
        {t("inviteRequest.pageTitle")}
      </h1>
      <p className="onboarding-intro">{t("inviteRequest.intro")}</p>
      <InviteRequestForm />
    </article>
  );
}
