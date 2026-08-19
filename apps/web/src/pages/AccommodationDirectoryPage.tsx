import { AccommodationDirectory } from "../features/directory/AccommodationDirectory";
import { publicDirectoryClient } from "../shared/api/directory-client";
import { useLocale } from "../shared/i18n/LocaleProvider";

/**
 * Página aberta de quem procura onde ficar. Usa o cliente anônimo direto, e não
 * o da sessão: a lista é a mesma com ou sem conta, e passar por `useAuthSession`
 * faria a página depender de um provedor que ela não precisa.
 */
export default function AccommodationDirectoryPage() {
  const { t } = useLocale();
  return (
    <article className="page">
      <div className="page-eyebrow-row">
        <div className="eyebrow">{t("directory.eyebrow")}</div>
        <span className="prototype-badge">{t("landing.ticker.prototype")}</span>
      </div>
      <h1 data-route-heading tabIndex={-1}>
        {t("directory.pageTitle")}
      </h1>
      <p className="onboarding-intro">{t("directory.intro")}</p>
      <AccommodationDirectory client={publicDirectoryClient} />
      <section className="boundary-note" aria-labelledby="directory-publish">
        <h2 id="directory-publish">{t("directory.publish.title")}</h2>
        <p>{t("directory.publish.body")}</p>
        <p>
          <a className="quiet-action" href="/convite">
            {t("directory.publish.action")}
          </a>
        </p>
      </section>
    </article>
  );
}
