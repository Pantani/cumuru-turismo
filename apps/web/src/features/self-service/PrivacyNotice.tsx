import { useLocale } from "../../shared/i18n/LocaleProvider";

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
  const { t } = useLocale();
  return (
    <section className="privacy-notice" aria-labelledby="privacy-notice-title">
      <h2 id="privacy-notice-title">{t("selfService.privacy.title")}</h2>
      <p>
        {t("selfService.privacy.introBefore")}{" "}
        <strong>{accommodationName}</strong>
        {t("selfService.privacy.introAfter")}
      </p>
      <ul>
        <li>{t("selfService.privacy.noIdentity")}</li>
        <li>{t("selfService.privacy.needsApproval")}</li>
        <li>{t("selfService.privacy.expiry")}</li>
        <li>{t("selfService.privacy.assistedAlternative")}</li>
      </ul>
      <p className="notice-version">
        {t("selfService.privacy.versionLabel")} <code>{version}</code>
      </p>
    </section>
  );
}
