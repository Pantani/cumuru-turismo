import { useCallback, useId, useState, type FormEvent } from "react";

import { OperationStatus } from "../../shared/forms/FieldFeedback";
import { useAuthSession } from "../../shared/auth/AuthSession";
import { entityTagFor, type Accommodation } from "../operator/stay-lifecycle";
import { useOperation } from "../operator/use-operation";

interface PublicListingPanelProps {
  accommodation: Accommodation;
  onSaved: () => void;
}

interface ListingDraft {
  enabled: boolean;
  phone: string;
  whatsapp: boolean;
  website: string;
}

const phonePattern = /^\+[1-9][0-9]{9,14}$/u;

function draftFrom(accommodation: Accommodation): ListingDraft {
  const listing = accommodation.public_listing;
  return {
    enabled: listing.enabled,
    phone: listing.phone ?? "",
    whatsapp: listing.whatsapp,
    website: listing.website ?? "",
  };
}

function phoneIssue(phone: string) {
  if (phone === "" || phonePattern.test(phone)) {
    return null;
  }
  return "Escreva o telefone com país e DDD, só números, assim: +5573999990001.";
}

function publishIssue(enabled: boolean, phone: string) {
  if (!enabled || phone !== "") {
    return null;
  }
  return "Para aparecer na lista, informe o telefone que o hóspede vai usar.";
}

function websiteIssue(website: string) {
  if (website === "" || website.startsWith("https://")) {
    return null;
  }
  return "O endereço do site precisa começar com https://";
}

/** Diz o que impede publicar, antes de gastar uma requisição para ouvir 409. */
function draftIssue(draft: ListingDraft) {
  const phone = draft.phone.trim();
  return (
    phoneIssue(phone) ??
    publishIssue(draft.enabled, phone) ??
    websiteIssue(draft.website.trim())
  );
}

/** Campo vazio é ausência declarada — null —, e não string em branco. */
function optional(value: string) {
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

function listingPatch(draft: ListingDraft) {
  return {
    public_listing_enabled: draft.enabled,
    public_contact_phone: optional(draft.phone),
    public_contact_whatsapp: draft.whatsapp,
    public_website_url: optional(draft.website),
  };
}

function ListingState({ accommodation }: { accommodation: Accommodation }) {
  const listing = accommodation.public_listing;
  if (!listing.enabled || listing.consented_at === null) {
    return <p className="queue-hint">Sua hospedagem não aparece na lista pública.</p>;
  }
  const since = new Date(listing.consented_at).toLocaleDateString("pt-BR");
  return <p className="queue-hint">Na lista pública desde {since}.</p>;
}

/**
 * Publicação é ato da hospedagem e some quando ela desmarca — não há fila nem
 * aprovação no caminho. O painel escreve pelo mesmo `PATCH` das demais edições
 * da acomodação, então lê a versão corrente na hora de salvar em vez de guardar
 * uma cópia: os painéis vizinhos escrevem na mesma linha, e uma versão guardada
 * aqui viraria 412 que a operadora não teria como entender.
 */
export function PublicListingPanel({
  accommodation,
  onSaved,
}: PublicListingPanelProps) {
  const { coreClient } = useAuthSession();
  const operation = useOperation();
  const { run } = operation;
  const formId = useId();
  const [draft, setDraft] = useState(() => draftFrom(accommodation));
  const [issue, setIssue] = useState<string | null>(null);

  const submit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      const blocked = draftIssue(draft);
      setIssue(blocked);
      if (blocked !== null) {
        return;
      }
      const saved = await run("Salvando a publicação", async () => {
        const current = await coreClient.getAccommodation(accommodation.id);
        return coreClient.updateAccommodation(
          accommodation.id,
          listingPatch(draft),
          entityTagFor(current.data.version),
        );
      });
      if (saved !== null) {
        onSaved();
      }
    },
    [accommodation.id, coreClient, draft, onSaved, run],
  );

  return (
    <section className="listing-panel" aria-labelledby={`${formId}-title`}>
      <h2 id={`${formId}-title`}>Aparecer na lista pública</h2>
      <p className="queue-hint">
        A lista é aberta e serve para o hóspede encontrar você. Só entra o que
        você escrever aqui, e você pode sair dela quando quiser.
      </p>
      <ListingState accommodation={accommodation} />

      <form onSubmit={(event) => void submit(event)} noValidate>
        <div className="field-grid">
          <label className="field-control" htmlFor={`${formId}-phone`}>
            Telefone com país e DDD
            <input
              id={`${formId}-phone`}
              type="tel"
              inputMode="tel"
              placeholder="+5573999990001"
              value={draft.phone}
              maxLength={16}
              onChange={(event) =>
                setDraft((current) => ({ ...current, phone: event.target.value }))
              }
            />
          </label>
          <label className="field-control" htmlFor={`${formId}-website`}>
            Site da hospedagem (opcional)
            <input
              id={`${formId}-website`}
              type="url"
              placeholder="https://"
              value={draft.website}
              maxLength={180}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  website: event.target.value,
                }))
              }
            />
          </label>
        </div>
        <label className="checkbox-control" htmlFor={`${formId}-whatsapp`}>
          <input
            id={`${formId}-whatsapp`}
            type="checkbox"
            checked={draft.whatsapp}
            onChange={(event) =>
              setDraft((current) => ({
                ...current,
                whatsapp: event.target.checked,
              }))
            }
          />
          Esse número atende no WhatsApp
        </label>
        <label className="checkbox-control" htmlFor={`${formId}-enabled`}>
          <input
            id={`${formId}-enabled`}
            type="checkbox"
            checked={draft.enabled}
            onChange={(event) =>
              setDraft((current) => ({
                ...current,
                enabled: event.target.checked,
              }))
            }
          />
          Publicar minha hospedagem na lista
        </label>
        {issue === null ? null : (
          <p className="form-error" role="alert">
            {issue}
          </p>
        )}
        <OperationStatus operation={operation} />
        <div className="button-row">
          <button
            type="submit"
            className="primary-action"
            disabled={operation.busy}
          >
            Salvar
          </button>
        </div>
      </form>
    </section>
  );
}
