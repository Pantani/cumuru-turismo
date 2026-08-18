import { SelfRegistrationCompletion } from "../features/self-service/SelfRegistrationCompletion";
import { SelfRegistrationForm } from "../features/self-service/SelfRegistrationForm";
import { useLocale } from "../shared/i18n/LocaleProvider";
import {
  peekSelfRegistrationCompleted,
  peekSelfServiceCapability,
} from "../shared/security/self-service-capability";

/**
 * O conserto é **consultar o sinal de conclusão**, não a ordem em que ele é
 * consultado: o sucesso descarta o cartaz, o descarte redesenha o `App`, e uma
 * página que olhasse só o token concluiria que nunca houve cartaz nenhum.
 * Verificado por mutação — remover o sinal quebra a jornada; inverter a ordem,
 * não, porque o token já está nulo quando o redesenho chega.
 *
 * A conclusão vem primeiro mesmo assim, por defesa: se algum dia as duas coisas
 * forem verdadeiras no mesmo render, quem concluiu precisa ver a confirmação, e
 * não um formulário em branco.
 */
function SelfRegistrationBody() {
  const { t } = useLocale();
  if (peekSelfRegistrationCompleted()) {
    return <SelfRegistrationCompletion />;
  }
  if (peekSelfServiceCapability() !== null) {
    return <SelfRegistrationForm />;
  }
  return (
    <section className="boundary-note" aria-labelledby="poster-required">
      <h2 id="poster-required">{t("selfService.posterRequired.title")}</h2>
      <p>{t("selfService.posterRequired.body")}</p>
    </section>
  );
}

export default function SelfRegistrationPage() {
  const { t } = useLocale();
  return (
    <article className="page registration-page">
      <div className="page-eyebrow-row">
        <div className="eyebrow">{t("selfService.eyebrow")}</div>
        {/*
          Exigência do brandkit, não enfeite: "rótulo de dado fictício visível
          em toda peça de protótipo". Esta é a superfície mais pública da fase e
          a única alcançável por quem nunca falou com a hospedagem, então o
          aviso precisa estar acima da dobra, não só no rodapé do site.
        */}
        <span className="prototype-badge">{t("landing.ticker.prototype")}</span>
      </div>
      <h1 data-route-heading tabIndex={-1}>
        {t("selfService.pageTitle")}
      </h1>
      <SelfRegistrationBody />
    </article>
  );
}
