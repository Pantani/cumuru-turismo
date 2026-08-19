import { useCallback, useEffect, useId, useState, type FormEvent } from "react";

import { useAuthSession } from "../../shared/auth/AuthSession";
import { OperationStatus } from "../../shared/forms/FieldFeedback";
import { describeFailure, useOperation } from "../operator/use-operation";
import {
  entityTagFor,
  useCalendarFeeds,
  type CalendarFeed,
} from "./use-calendar-feeds";

interface CalendarFeedPanelProps {
  accommodationId: string;
}

const outcomeCopy: Record<string, string> = {
  ok: "última leitura funcionou",
  unreachable: "o site da plataforma não respondeu",
  not_calendar: "o endereço não abriu um calendário — provavelmente expirou",
  malformed: "o calendário veio quebrado",
};

const statusCopy: Record<string, string> = {
  active: "ativo",
  suspended: "pausado depois de várias falhas seguidas",
  removed: "removido",
};

function describeSync(feed: CalendarFeed) {
  if (feed.last_synced_at === null || feed.last_sync_outcome === null) {
    return "ainda não foi lido";
  }
  const when = new Date(feed.last_synced_at).toLocaleString("pt-BR");
  return `${outcomeCopy[feed.last_sync_outcome] ?? "resultado desconhecido"} (${when})`;
}

function FeedRow({
  feed,
  onRemove,
  busy,
}: {
  busy: boolean;
  feed: CalendarFeed;
  onRemove: (feed: CalendarFeed) => void;
}) {
  return (
    <li className="calendar-feed-row">
      <div>
        <strong>{feed.label}</strong>
        <p className="calendar-feed-detail">
          Booking.com · {statusCopy[feed.status] ?? feed.status} ·{" "}
          {describeSync(feed)}
        </p>
      </div>
      <button type="button" onClick={() => onRemove(feed)} disabled={busy}>
        Remover
      </button>
    </li>
  );
}

/**
 * A falha de leitura vem antes da lista vazia: "nenhum calendário" e "não
 * consegui perguntar" são fatos diferentes, e mostrar o primeiro no lugar do
 * segundo faria a hospedagem achar que perdeu o cadastro.
 */
function FeedList({
  busy,
  error,
  feeds,
  loading,
  onRemove,
}: {
  busy: boolean;
  error: unknown;
  feeds: readonly CalendarFeed[];
  loading: boolean;
  onRemove: (feed: CalendarFeed) => void;
}) {
  if (loading) {
    return <p>Carregando os calendários…</p>;
  }
  if (error !== null) {
    return (
      <p className="operation-status tone-failed" role="alert">
        {describeFailure(error)}
      </p>
    );
  }
  if (feeds.length === 0) {
    return <p>Nenhum calendário cadastrado ainda.</p>;
  }
  return (
    <ul className="calendar-feed-list">
      {feeds.map((feed) => (
        <FeedRow key={feed.id} feed={feed} busy={busy} onRemove={onRemove} />
      ))}
    </ul>
  );
}

/**
 * O painel nunca mostra o endereço do calendário depois de salvo, e a API não o
 * devolve: quem tem o link lê o calendário do anúncio inteiro. O que a tela
 * mostra é o rótulo, o estado e quando foi lido pela última vez (ADR-044).
 */
export function CalendarFeedPanel({ accommodationId }: CalendarFeedPanelProps) {
  const { coreClient: client } = useAuthSession();
  const { error, feeds, loading, reload } = useCalendarFeeds(accommodationId);
  const operation = useOperation();
  const formId = useId();
  const [draft, setDraft] = useState({ label: "", url: "" });

  // O rascunho pertence à hospedagem selecionada. Sem isto, trocar de
  // hospedagem levaria junto o endereço digitado para a anterior — e cadastrá-lo
  // no lugar errado é justamente o que ninguém consegue desfazer olhando a tela,
  // porque a URL não é exibida depois de salva.
  useEffect(() => {
    setDraft({ label: "", url: "" });
  }, [accommodationId]);

  const submit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      const result = await operation.run("Cadastrando o calendário", () =>
        client.createCalendarFeed(
          accommodationId,
          { provider: "booking", label: draft.label.trim(), url: draft.url.trim() },
          crypto.randomUUID(),
        ),
      );
      if (result !== null) {
        setDraft({ label: "", url: "" });
        await reload();
      }
    },
    [accommodationId, client, draft, operation, reload],
  );

  const remove = useCallback(
    async (feed: CalendarFeed) => {
      const done = await operation.run("Removendo o calendário", () =>
        client.removeCalendarFeed(
          feed.id,
          entityTagFor(feed.version),
          crypto.randomUUID(),
        ),
      );
      if (done !== null) {
        await reload();
      }
    },
    [client, operation, reload],
  );

  return (
    <section className="workspace-section" aria-labelledby={`${formId}-title`}>
      <h2 id={`${formId}-title`}>Calendário do Booking.com</h2>
      <p className="lead">
        Se a sua hospedagem vende pelo Booking.com, cole aqui o endereço do
        calendário do anúncio. As datas passam a chegar sozinhas e você só
        confirma quantas pessoas vieram. Ninguém aqui vê o nome do hóspede: a
        plataforma não envia essa informação.
      </p>

      <form className="calendar-feed-form" onSubmit={(event) => void submit(event)}>
        <label className="field-control" htmlFor={`${formId}-label`}>
          Nome do anúncio
          <input
            id={`${formId}-label`}
            value={draft.label}
            maxLength={120}
            placeholder="Chalé 3"
            onChange={(event) =>
              setDraft((current) => ({ ...current, label: event.target.value }))
            }
            required
          />
          <span className="field-hint">
            É só para você se achar — "Chalé 3", "Suíte da frente".
          </span>
        </label>
        <label className="field-control" htmlFor={`${formId}-url`}>
          Endereço do calendário
          <input
            id={`${formId}-url`}
            type="url"
            value={draft.url}
            maxLength={2048}
            placeholder="https://ical.booking.com/..."
            onChange={(event) =>
              setDraft((current) => ({ ...current, url: event.target.value }))
            }
            required
          />
          <span className="field-hint">
            No Booking.com: Extranet → Tarifas e disponibilidade → Calendário →
            Sincronizar calendários → Exportar. Copie o endereço que aparece.
          </span>
        </label>
        <p className="privacy-warning" role="note">
          O rótulo nunca deve conter nome, documento, contato, credencial ou dado
          sensível: ele aparece na tela de quem opera a hospedagem.
        </p>
        <OperationStatus operation={operation} />
        <button className="primary-action" type="submit" disabled={operation.busy}>
          Cadastrar calendário
        </button>
      </form>

      <FeedList
        busy={operation.busy}
        error={error}
        feeds={feeds}
        loading={loading}
        onRemove={(target) => void remove(target)}
      />
    </section>
  );
}
