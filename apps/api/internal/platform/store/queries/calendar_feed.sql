-- name: CreateCalendarFeed :one
-- A URL não aparece no RETURNING e não aparecerá: ela é segredo portador, e
-- devolvê-la transformaria a tela que qualquer operador abre num jeito de ler o
-- calendário do anúncio em outro lugar (ADR-044).
INSERT INTO core.calendar_feeds (
  id,
  accommodation_id,
  provider,
  label,
  url_ciphertext,
  url_nonce,
  url_key_version,
  url_fingerprint,
  url_fingerprint_key_version,
  created_at,
  updated_at
)
VALUES (
  sqlc.arg(feed_id),
  sqlc.arg(accommodation_id),
  sqlc.arg(provider),
  sqlc.arg(label),
  sqlc.arg(url_ciphertext),
  sqlc.arg(url_nonce),
  sqlc.arg(url_key_version),
  sqlc.arg(url_fingerprint),
  sqlc.arg(url_fingerprint_key_version),
  sqlc.arg(now),
  sqlc.arg(now)
)
RETURNING
  id,
  accommodation_id,
  provider,
  label,
  status,
  last_synced_at,
  last_sync_outcome,
  consecutive_failures,
  version,
  created_at,
  updated_at;

-- name: ListCalendarFeeds :many
-- O removido fica fora da listagem e dentro da tabela: as estadias confirmadas
-- a partir dele continuam de pé, e a origem permanece como explicação.
SELECT
  feed.id,
  feed.accommodation_id,
  feed.provider,
  feed.label,
  feed.status,
  feed.last_synced_at,
  feed.last_sync_outcome,
  feed.consecutive_failures,
  feed.version,
  feed.created_at,
  feed.updated_at
FROM core.calendar_feeds AS feed
WHERE feed.accommodation_id = sqlc.arg(accommodation_id)
  AND feed.status <> 'removed'
ORDER BY feed.created_at, feed.id;

-- name: LockCalendarFeedForDecision :one
-- A acomodação volta junto para que a autorização decida sobre a linha travada,
-- e não sobre o que o cliente disse que ela era.
SELECT
  feed.id,
  feed.accommodation_id,
  feed.status,
  feed.version
FROM core.calendar_feeds AS feed
WHERE feed.id = sqlc.arg(feed_id)
FOR UPDATE OF feed;

-- name: RemoveCalendarFeed :one
-- Zero linhas significa que a linha mudou entre a trava e a escrita, o que é
-- conflito e não ausência.
UPDATE core.calendar_feeds AS feed
SET
  status = 'removed',
  updated_at = sqlc.arg(removed_at),
  version = feed.version + 1
WHERE feed.id = sqlc.arg(feed_id)
  AND feed.version = sqlc.arg(expected_version)
  AND feed.status <> 'removed'
RETURNING
  feed.id,
  feed.accommodation_id,
  feed.provider,
  feed.label,
  feed.status,
  feed.last_synced_at,
  feed.last_sync_outcome,
  feed.consecutive_failures,
  feed.version,
  feed.created_at,
  feed.updated_at;

-- name: ListDueCalendarFeeds :many
-- Sem FOR UPDATE de propósito: entre a leitura e a escrita do resultado há
-- uma requisição HTTP para fora, e segurar trava de linha durante ela
-- prenderia a transação no relógio de outra pessoa. O recorte por
-- last_synced_at já basta — no pior caso um ciclo concorrente refaz a mesma
-- sincronização, que é idempotente.
--
-- Nunca sincronizado vem primeiro, porque é o feed que a hospedagem acabou
-- de cadastrar e está olhando a tela para ver funcionar.
SELECT
  feed.id,
  feed.accommodation_id,
  feed.provider,
  feed.url_ciphertext,
  feed.url_nonce,
  feed.url_key_version,
  feed.consecutive_failures,
  feed.version
FROM core.calendar_feeds AS feed
WHERE feed.status = 'active'
  AND (
    feed.last_synced_at IS NULL
    OR feed.last_synced_at < sqlc.arg(cutoff)
  )
ORDER BY feed.last_synced_at NULLS FIRST
LIMIT sqlc.arg(batch_size);

-- name: MarkCalendarFeedSynced :exec
-- O sucesso zera a contagem de falhas: o que interessa é a sequência corrente,
-- não o histórico, porque o objetivo é suspender quem está quebrado agora.
UPDATE core.calendar_feeds AS feed
SET
  last_synced_at = sqlc.arg(synced_at),
  last_sync_outcome = 'ok',
  consecutive_failures = 0,
  updated_at = sqlc.arg(synced_at),
  version = feed.version + 1
WHERE feed.id = sqlc.arg(feed_id);

-- name: MarkCalendarFeedFailed :exec
-- A suspensão vem decidida do domínio e chega como argumento: o SQL carimba, e
-- o limite continua sendo uma regra legível em Go em vez de uma aritmética
-- escondida numa cláusula CASE.
UPDATE core.calendar_feeds AS feed
SET
  last_synced_at = sqlc.arg(synced_at),
  last_sync_outcome = sqlc.arg(outcome),
  consecutive_failures = feed.consecutive_failures + 1,
  status = CASE WHEN sqlc.arg(suspend)::boolean THEN 'suspended' ELSE feed.status END,
  updated_at = sqlc.arg(synced_at),
  version = feed.version + 1
WHERE feed.id = sqlc.arg(feed_id);

-- name: UpsertCalendarReservation :exec
-- O UID cego é a identidade da reserva na origem, e por isso o conflito é o
-- caminho normal e não a exceção: toda sincronização revê o mesmo calendário.
--
-- A atualização não toca estado já decidido. Datas mudadas na plataforma
-- alcançam o que ainda está na fila; o que a hospedagem já confirmou é estadia,
-- e estadia se corrige na tela dela, não por um arquivo remoto.
INSERT INTO core.calendar_reservations (
  id,
  feed_id,
  external_uid_hmac,
  external_uid_key_version,
  arrival_on,
  departure_on,
  kind,
  first_seen_at,
  last_seen_at,
  updated_at
)
VALUES (
  sqlc.arg(reservation_id),
  sqlc.arg(feed_id),
  sqlc.arg(external_uid_hmac),
  sqlc.arg(external_uid_key_version),
  sqlc.arg(arrival_on),
  sqlc.arg(departure_on),
  sqlc.arg(kind),
  sqlc.arg(seen_at),
  sqlc.arg(seen_at),
  sqlc.arg(seen_at)
)
ON CONFLICT (feed_id, external_uid_hmac) DO UPDATE
SET
  arrival_on = EXCLUDED.arrival_on,
  departure_on = EXCLUDED.departure_on,
  kind = EXCLUDED.kind,
  last_seen_at = EXCLUDED.last_seen_at,
  updated_at = EXCLUDED.updated_at,
  version = core.calendar_reservations.version + 1
WHERE core.calendar_reservations.state = 'pending';

-- name: ReviveWithdrawnCalendarReservation :exec
-- Uma reserva que sumiu e voltou é a mesma reserva. Sem isto ela permaneceria
-- retirada para sempre, porque o upsert acima só alcança o que está pendente.
--
-- As datas e a natureza vêm junto, e não só o estado: uma reserva costuma sumir
-- e voltar justamente porque foi alterada na plataforma, e reviver só o estado
-- devolveria à fila as datas antigas — que a hospedagem confirmaria como se
-- fossem as de agora.
UPDATE core.calendar_reservations AS reservation
SET
  state = 'pending',
  withdrawn_at = NULL,
  arrival_on = sqlc.arg(arrival_on),
  departure_on = sqlc.arg(departure_on),
  kind = sqlc.arg(kind),
  last_seen_at = sqlc.arg(seen_at),
  updated_at = sqlc.arg(seen_at),
  version = reservation.version + 1
WHERE reservation.feed_id = sqlc.arg(feed_id)
  AND reservation.external_uid_hmac = sqlc.arg(external_uid_hmac)
  AND reservation.state = 'withdrawn';

-- name: WithdrawUnseenCalendarReservations :exec
-- Some do feed quem foi cancelado na plataforma, e o arquivo não diz isso: diz
-- apenas que não está mais lá. Por isso a retirada alcança só a fila pendente —
-- desaparecimento nunca cancela estadia já confirmada (ADR-044).
UPDATE core.calendar_reservations AS reservation
SET
  state = 'withdrawn',
  withdrawn_at = sqlc.arg(withdrawn_at),
  updated_at = sqlc.arg(withdrawn_at),
  version = reservation.version + 1
WHERE reservation.feed_id = sqlc.arg(feed_id)
  AND reservation.state = 'pending'
  AND reservation.last_seen_at < sqlc.arg(cycle_started_at);

-- name: ListCalendarReservations :many
-- A junção com o feed é a autorização: a fila pertence à acomodação, e não ao
-- feed, porque a hospedagem que tem três anúncios vê uma fila só.
SELECT
  reservation.id,
  reservation.feed_id,
  reservation.arrival_on,
  reservation.departure_on,
  reservation.kind,
  reservation.state,
  reservation.stay_id,
  reservation.first_seen_at,
  reservation.last_seen_at,
  reservation.version
FROM core.calendar_reservations AS reservation
JOIN core.calendar_feeds AS feed ON feed.id = reservation.feed_id
WHERE feed.accommodation_id = sqlc.arg(accommodation_id)
  AND (
    sqlc.narg(state)::text IS NULL
    OR reservation.state = sqlc.narg(state)::text
  )
ORDER BY reservation.arrival_on, reservation.id
LIMIT sqlc.arg(page_limit);

-- name: LockCalendarReservationForDecision :one
SELECT
  reservation.id,
  reservation.feed_id,
  reservation.arrival_on,
  reservation.departure_on,
  reservation.kind,
  reservation.state,
  reservation.version,
  feed.accommodation_id
FROM core.calendar_reservations AS reservation
JOIN core.calendar_feeds AS feed ON feed.id = reservation.feed_id
WHERE reservation.id = sqlc.arg(reservation_id)
FOR UPDATE OF reservation;

-- name: ConfirmCalendarReservation :one
-- A estadia foi criada na mesma transação: a constraint recusa 'confirmed' sem
-- stay_id, então a fila não tem como carimbar confirmação sem que a estadia
-- exista.
UPDATE core.calendar_reservations AS reservation
SET
  state = 'confirmed',
  stay_id = sqlc.arg(stay_id),
  updated_at = sqlc.arg(decided_at),
  version = reservation.version + 1
WHERE reservation.id = sqlc.arg(reservation_id)
  AND reservation.version = sqlc.arg(expected_version)
  AND reservation.state = 'pending'
RETURNING
  reservation.id,
  reservation.feed_id,
  reservation.arrival_on,
  reservation.departure_on,
  reservation.kind,
  reservation.state,
  reservation.stay_id,
  reservation.first_seen_at,
  reservation.last_seen_at,
  reservation.version;

-- name: DismissCalendarReservation :one
-- Dispensar é dizer que aquilo não era estadia — bloqueio de manutenção, uso do
-- dono, reserva que a hospedagem já registrou à mão. Não há motivo de lista
-- fechada porque nada disso descreve pessoa alguma.
UPDATE core.calendar_reservations AS reservation
SET
  state = 'dismissed',
  updated_at = sqlc.arg(decided_at),
  version = reservation.version + 1
WHERE reservation.id = sqlc.arg(reservation_id)
  AND reservation.version = sqlc.arg(expected_version)
  AND reservation.state = 'pending'
RETURNING
  reservation.id,
  reservation.feed_id,
  reservation.arrival_on,
  reservation.departure_on,
  reservation.kind,
  reservation.state,
  reservation.stay_id,
  reservation.first_seen_at,
  reservation.last_seen_at,
  reservation.version;
