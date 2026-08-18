import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import type { AccessRequest } from "../../shared/api/invite-request-client";
import { useAuthSession } from "../../shared/auth/AuthSession";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import type { Translate } from "../../shared/i18n/translate";
import { ActivationIssuePanel } from "../accommodation/ActivationIssuePanel";
import type { Accommodation } from "../operator/stay-lifecycle";
import { describeAccessRequestFailure } from "./access-request-operation";
import { contactOf } from "./access-request-vocabulary";

/** Alvo estável de foco: depois de aprovar, o cursor vem parar aqui. */
export const ACCESS_REQUEST_ISSUANCE_ID = "access-request-issuance-title";

interface AccessRequestIssuanceProps {
  accommodationId: string;
  onDismiss: () => void;
  request: AccessRequest;
}

/**
 * A emissão exige `If-Match` da acomodação, e a aprovação devolve só o
 * identificador dela. Ler a linha recém-criada é o que evita adivinhar a versão
 * — um palpite certo hoje viraria precondição falhada no dia em que a criação
 * passar a escrever duas vezes.
 */
function useApprovedAccommodation(accommodationId: string, t: Translate) {
  const { coreClient } = useAuthSession();
  const query = useQuery({
    queryKey: ["accommodation", accommodationId],
    queryFn: () => coreClient.getAccommodation(accommodationId),
    select: (result) => result.data,
  });

  return {
    accommodation: query.data ?? null,
    message: statusMessage(t, query),
  };
}

function statusMessage(
  t: Translate,
  query: { error: unknown; isError: boolean; isPending: boolean },
) {
  if (query.isError) {
    return describeAccessRequestFailure(t, query.error);
  }
  return query.isPending ? t("accessRequest.issuance.loading") : "";
}

function IssuanceForm({
  accommodation,
  request,
}: {
  accommodation: Accommodation;
  request: AccessRequest;
}) {
  const { t } = useLocale();
  const [version, setVersion] = useState(accommodation.version);
  const contact = contactOf(request);

  return (
    <ActivationIssuePanel
      accommodationId={accommodation.id}
      initialDraft={
        contact === null
          ? undefined
          : { displayName: contact.name, email: contact.email }
      }
      note={
        <p className="access-request-unverified">
          {t("accessRequest.issuance.unverifiedEmail")}
        </p>
      }
      onVersionChange={setVersion}
      version={version}
    />
  );
}

/**
 * O ganho do recurso inteiro está aqui: aprovar leva direto à emissão já
 * preenchida com o que o pedido trouxe, e a administração não redigita nada.
 *
 * A ressalva da ADR-042 fica em uma linha, colada ao formulário: aprovar não
 * provou que quem pediu controla aquele endereço, e mandar o acesso para lá é
 * julgamento de quem aprova.
 */
export function AccessRequestIssuance({
  accommodationId,
  onDismiss,
  request,
}: AccessRequestIssuanceProps) {
  const { t } = useLocale();
  const { accommodation, message } = useApprovedAccommodation(
    accommodationId,
    t,
  );

  return (
    <section
      className="approval-followup access-request-issuance"
      aria-labelledby={ACCESS_REQUEST_ISSUANCE_ID}
    >
      <h3 id={ACCESS_REQUEST_ISSUANCE_ID} tabIndex={-1}>
        {t("accessRequest.issuance.title", {
          name: request.accommodation_name,
        })}
      </h3>
      <p>{t("accessRequest.issuance.hint")}</p>
      <p className="operation-status" role="status" aria-live="polite">
        {message}
      </p>
      {accommodation === null ? null : (
        <IssuanceForm accommodation={accommodation} request={request} />
      )}
      <div className="button-row">
        <button type="button" className="ghost-action" onClick={onDismiss}>
          {t("accessRequest.issuance.dismiss")}
        </button>
      </div>
    </section>
  );
}
