import { useEffect, useRef } from "react";

import { useLocale } from "../../shared/i18n/LocaleProvider";

/**
 * Tela própria, e não um aviso ao lado do formulário: o pedido saiu do aparelho
 * e não há mais nada a preencher, então deixar os campos na tela convidaria a
 * mandar de novo. O foco vai para o título porque a troca de tela precisa ser
 * percebida por quem navega só com teclado ou leitor.
 *
 * O texto não promete prazo: a fila é analisada por gente, e o sistema não tem
 * como garantir data. Diz o que aconteceu, quem decide e por onde vem a resposta.
 */
export function InviteRequestCompletion({ email }: { email: string }) {
  const { t } = useLocale();
  const headingRef = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    headingRef.current?.focus();
  }, []);

  return (
    <section className="form-card" aria-labelledby="invite-request-done">
      <h2 id="invite-request-done" ref={headingRef} tabIndex={-1}>
        {t("inviteRequest.completion.title")}
      </h2>
      <p role="status" aria-live="polite">
        {t("inviteRequest.completion.body", { email })}
      </p>
      <ul>
        <li>{t("inviteRequest.completion.review")}</li>
        <li>{t("inviteRequest.completion.noDeadline")}</li>
        <li>{t("inviteRequest.completion.keepEmail")}</li>
      </ul>
      <a className="primary-action" href="/">
        {t("inviteRequest.completion.back")}
      </a>
    </section>
  );
}
