import { useCallback, useId, useState } from "react";

import { createUuidV7 } from "../../shared/identity/uuid-v7";
import { useAuthSession } from "../../shared/auth/AuthSession";
import { OperationStatus } from "../../shared/forms/FieldFeedback";
import { useOperation } from "../operator/use-operation";
import {
  entityTagFor,
  useCalendarReservations,
  type CalendarReservation,
} from "./use-calendar-feeds";

interface CalendarReservationQueueProps {
  accommodationId: string;
  /** Confirmar cria estadia, e o quadro de estadias precisa saber. */
  onConfirmed?: () => void;
}

const kindCopy: Record<string, string> = {
  reserved: "a plataforma marcou como reserva",
  blocked: "a plataforma marcou como bloqueio",
  unknown: "a plataforma não disse o que é",
};

function formatDate(value: string) {
  const [year, month, day] = value.split("-");
  return `${day}/${month}/${year}`;
}

/**
 * O campo é texto controlado: apagá-lo daria `Number("") === 0`, e confirmar
 * mandaria uma estadia de zero pessoa para o servidor recusar. O limite é o
 * mesmo de `core.stays`, então a tela recusa antes com uma frase em vez de
 * depois com um código.
 */
function parseGuestCount(raw: string): number | null {
  const value = Number(raw);
  if (!Number.isInteger(value) || value < 1 || value > 100) {
    return null;
  }
  return value;
}

function ReservationRow({
  reservation,
  busy,
  onConfirm,
  onDismiss,
}: {
  busy: boolean;
  onConfirm: (reservation: CalendarReservation, guests: number) => void;
  onDismiss: (reservation: CalendarReservation) => void;
  reservation: CalendarReservation;
}) {
  const fieldId = useId();
  const [guests, setGuests] = useState("2");
  const parsed = parseGuestCount(guests);

  return (
    <li className="calendar-reservation-row">
      <div>
        <strong>
          {formatDate(reservation.arrival_on)} até{" "}
          {formatDate(reservation.departure_on)}
        </strong>
        <p className="calendar-feed-detail">
          {kindCopy[reservation.kind] ?? reservation.kind}
        </p>
      </div>
      <label className="field-control" htmlFor={fieldId}>
        Quantas pessoas
        <input
          id={fieldId}
          type="number"
          inputMode="numeric"
          min={1}
          max={100}
          value={guests}
          aria-invalid={parsed === null}
          onChange={(event) => setGuests(event.target.value)}
        />
        {parsed === null ? (
          <span className="field-error" role="alert">
            Informe de 1 a 100 pessoas.
          </span>
        ) : null}
      </label>
      <div className="calendar-reservation-actions">
        <button
          className="primary-action"
          type="button"
          disabled={busy || parsed === null}
          onClick={() => parsed !== null && onConfirm(reservation, parsed)}
        >
          Confirmar estadia
        </button>
        <button type="button" disabled={busy} onClick={() => onDismiss(reservation)}>
          Não era estadia
        </button>
      </div>
    </li>
  );
}

/**
 * O calendário nunca diz quantas pessoas vieram e nem sempre separa reserva de
 * bloqueio de manutenção. Por isso a fila pergunta em vez de decidir: confirmar
 * é o único caminho em que uma linha importada vira presença publicada
 * (ADR-044).
 */
export function CalendarReservationQueue({
  accommodationId,
  onConfirmed,
}: CalendarReservationQueueProps) {
  const { coreClient: client } = useAuthSession();
  const { reservations, loading, reload } = useCalendarReservations(accommodationId);
  const operation = useOperation();
  const titleId = useId();

  const confirm = useCallback(
    async (reservation: CalendarReservation, guests: number) => {
      const done = await operation.run("Confirmando a estadia", () =>
        client.confirmCalendarReservation(
          reservation.id,
          {
            expected_guest_count: guests,
            client_submission_id: createUuidV7(),
          },
          entityTagFor(reservation.version),
          crypto.randomUUID(),
        ),
      );
      if (done !== null) {
        await reload();
        onConfirmed?.();
      }
    },
    [client, onConfirmed, operation, reload],
  );

  const dismiss = useCallback(
    async (reservation: CalendarReservation) => {
      const done = await operation.run("Dispensando a reserva", () =>
        client.dismissCalendarReservation(
          reservation.id,
          entityTagFor(reservation.version),
          crypto.randomUUID(),
        ),
      );
      if (done !== null) {
        await reload();
      }
    },
    [client, operation, reload],
  );

  if (loading) {
    return <p>Carregando as reservas importadas…</p>;
  }

  return (
    <section className="workspace-section" aria-labelledby={titleId}>
      <h2 id={titleId}>Reservas vindas do Booking.com</h2>
      <p className="lead">
        Estas datas chegaram do calendário do seu anúncio. Diga quantas pessoas
        vieram e confirme — aí a estadia entra no sistema. Se era manutenção, uso
        seu ou uma reserva que você já registrou, use "não era estadia".
      </p>
      <OperationStatus operation={operation} />
      {reservations.length === 0 ? (
        <p>Nenhuma reserva esperando por você.</p>
      ) : (
        <ul className="calendar-reservation-list">
          {reservations.map((reservation) => (
            <ReservationRow
              key={reservation.id}
              reservation={reservation}
              busy={operation.busy}
              onConfirm={(target, guests) => void confirm(target, guests)}
              onDismiss={(target) => void dismiss(target)}
            />
          ))}
        </ul>
      )}
    </section>
  );
}
