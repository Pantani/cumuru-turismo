-- A hospedagem que vende pelo Booking.com já digitou chegada e saída lá. O feed
-- iCal do anúncio traz essas datas de volta sem parceria comercial e sem
-- identidade, porque a plataforma não a exporta. O que ele não traz é o número
-- de hóspedes nem a diferença confiável entre reserva e bloqueio de manutenção,
-- e é por isso que o calendário importado mora em tabela própria: `core.stays`
-- alimenta a presença publicada, e bloqueio de manutenção importado como estadia
-- inflaria o indicador público (ADR-044).
BEGIN;

CREATE TABLE core.calendar_feeds (
  id uuid PRIMARY KEY,
  accommodation_id uuid NOT NULL REFERENCES core.accommodations(id),
  provider text NOT NULL,
  label text NOT NULL,
  -- A URL é segredo portador: quem tem o link lê o calendário do anúncio
  -- inteiro. Fica cifrada em repouso pelo mesmo AES-GCM com keyring versionado
  -- que o texto livre da pesquisa usa, e nunca volta em resposta de API.
  url_ciphertext bytea NOT NULL,
  url_nonce bytea NOT NULL,
  url_key_version text NOT NULL,
  -- A impressão digital existe só para recusar o mesmo feed cadastrado duas
  -- vezes na mesma acomodação. HMAC porque comparar é tudo o que precisamos
  -- fazer com ela, e o texto em claro serviria para mais do que isso.
  url_fingerprint bytea NOT NULL,
  url_fingerprint_key_version text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  last_synced_at timestamptz,
  last_sync_outcome text,
  consecutive_failures integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  CONSTRAINT calendar_feeds_provider_valid
    CHECK (provider IN ('booking')),
  CONSTRAINT calendar_feeds_label_not_blank
    CHECK (btrim(label) <> '' AND char_length(label) <= 120),
  CONSTRAINT calendar_feeds_url_material_present
    CHECK (
      octet_length(url_ciphertext) > 0
      AND octet_length(url_nonce) > 0
      AND octet_length(url_fingerprint) > 0
    ),
  CONSTRAINT calendar_feeds_key_versions_not_blank
    CHECK (
      btrim(url_key_version) <> ''
      AND btrim(url_fingerprint_key_version) <> ''
    ),
  -- 'removed' em vez de DELETE: as estadias já confirmadas a partir deste feed
  -- são fato da hospedagem e continuam de pé, então apagar a origem apagaria a
  -- explicação de como elas entraram.
  CONSTRAINT calendar_feeds_status_valid
    CHECK (status IN ('active', 'suspended', 'removed')),
  CONSTRAINT calendar_feeds_sync_outcome_valid
    CHECK (
      last_sync_outcome IS NULL
      OR last_sync_outcome IN ('ok', 'unreachable', 'not_calendar', 'malformed')
    ),
  CONSTRAINT calendar_feeds_sync_outcome_follows_sync
    CHECK ((last_synced_at IS NULL) = (last_sync_outcome IS NULL)),
  CONSTRAINT calendar_feeds_failures_nonnegative
    CHECK (consecutive_failures >= 0),
  CONSTRAINT calendar_feeds_version_positive
    CHECK (version > 0)
);

CREATE UNIQUE INDEX calendar_feeds_accommodation_url_idx
  ON core.calendar_feeds (accommodation_id, url_fingerprint)
  WHERE status <> 'removed';

CREATE INDEX calendar_feeds_due_idx
  ON core.calendar_feeds (last_synced_at NULLS FIRST)
  WHERE status = 'active';

COMMENT ON TABLE core.calendar_feeds IS
  'Origem declarada das datas de uma acomodação; a URL é segredo cifrado em repouso.';

CREATE TABLE core.calendar_reservations (
  id uuid PRIMARY KEY,
  feed_id uuid NOT NULL REFERENCES core.calendar_feeds(id),
  -- O UID identifica a reserva na plataforma de origem. Sob HMAC ele ainda
  -- serve à única coisa de que precisamos — reconhecer que o evento de hoje é o
  -- mesmo de ontem — sem guardar dado de negócio de terceiro em claro.
  external_uid_hmac bytea NOT NULL,
  external_uid_key_version text NOT NULL,
  arrival_on date NOT NULL,
  departure_on date NOT NULL,
  kind text NOT NULL,
  state text NOT NULL DEFAULT 'pending',
  stay_id uuid REFERENCES core.stays(id),
  first_seen_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  withdrawn_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now(),
  version bigint NOT NULL DEFAULT 1,
  CONSTRAINT calendar_reservations_uid_present
    CHECK (
      octet_length(external_uid_hmac) > 0
      AND btrim(external_uid_key_version) <> ''
    ),
  CONSTRAINT calendar_reservations_dates_valid
    CHECK (departure_on > arrival_on),
  -- O `.ics` do Booking.com não separa reserva de bloqueio de manutenção com
  -- confiabilidade. 'unknown' é o valor honesto para quando a origem não disse,
  -- e a tela pede que a hospedagem decida em vez de adivinhar por ela.
  CONSTRAINT calendar_reservations_kind_valid
    CHECK (kind IN ('reserved', 'blocked', 'unknown')),
  CONSTRAINT calendar_reservations_state_valid
    CHECK (state IN ('pending', 'confirmed', 'dismissed', 'withdrawn')),
  -- Confirmar é criar a estadia; sem estadia não houve confirmação, e uma
  -- estadia pendurada em linha não confirmada seria presença sem autora.
  CONSTRAINT calendar_reservations_stay_follows_state
    CHECK ((state = 'confirmed') = (stay_id IS NOT NULL)),
  -- Só o que ainda estava na fila pode ser retirado por sumiço. Reserva
  -- cancelada some do feed sem dizer que foi cancelada, então desaparecimento
  -- nunca cancela estadia já confirmada (ADR-044).
  CONSTRAINT calendar_reservations_withdrawal_valid
    CHECK ((state = 'withdrawn') = (withdrawn_at IS NOT NULL)),
  CONSTRAINT calendar_reservations_seen_order_valid
    CHECK (last_seen_at >= first_seen_at),
  CONSTRAINT calendar_reservations_version_positive
    CHECK (version > 0),
  UNIQUE (feed_id, external_uid_hmac)
);

CREATE INDEX calendar_reservations_queue_idx
  ON core.calendar_reservations (feed_id, arrival_on, id)
  WHERE state = 'pending';

CREATE UNIQUE INDEX calendar_reservations_stay_idx
  ON core.calendar_reservations (stay_id)
  WHERE stay_id IS NOT NULL;

COMMENT ON TABLE core.calendar_reservations IS
  'Intenção de estadia observada no calendário da plataforma; vira estadia só por confirmação humana.';

REVOKE ALL ON TABLE core.calendar_feeds FROM PUBLIC;
REVOKE ALL ON TABLE core.calendar_reservations FROM PUBLIC;

REVOKE ALL ON TABLE core.calendar_feeds
FROM app_runtime, worker_runtime, public_runtime, privacy_officer;

REVOKE ALL ON TABLE core.calendar_reservations
FROM app_runtime, worker_runtime, public_runtime, privacy_officer;

GRANT SELECT, INSERT, UPDATE ON TABLE core.calendar_feeds TO app_runtime;

-- O worker sincroniza, e o grant descreve isso: lê o material da URL porque
-- precisa decifrá-la para buscar o arquivo, e escreve apenas o resultado da
-- sincronização. Ele não altera provedor, rótulo nem a própria URL.
GRANT SELECT (
  id,
  accommodation_id,
  provider,
  url_ciphertext,
  url_nonce,
  url_key_version,
  status,
  last_synced_at,
  consecutive_failures,
  version
) ON TABLE core.calendar_feeds TO worker_runtime;

GRANT UPDATE (
  status,
  last_synced_at,
  last_sync_outcome,
  consecutive_failures,
  updated_at,
  version
) ON TABLE core.calendar_feeds TO worker_runtime;

GRANT SELECT, INSERT, UPDATE ON TABLE core.calendar_reservations TO app_runtime;

GRANT SELECT ON TABLE core.calendar_reservations TO worker_runtime;

-- O INSERT é por coluna, e não table-wide, para que a fronteira que o comentário
-- abaixo declara seja imposta pelo banco: sem isto o worker poderia escrever
-- stay_id, que é o campo que transforma observação em presença publicada.
GRANT INSERT (
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
) ON TABLE core.calendar_reservations TO worker_runtime;

-- O worker reconcilia o que a origem ainda mostra: atualiza datas, natureza e
-- presença no feed, e retira da fila o que sumiu. Ele nunca escreve `stay_id`
-- nem confirma — confirmar é ato da hospedagem, na aplicação.
GRANT UPDATE (
  arrival_on,
  departure_on,
  kind,
  state,
  last_seen_at,
  withdrawn_at,
  updated_at,
  version
) ON TABLE core.calendar_reservations TO worker_runtime;

REVOKE DELETE ON TABLE core.calendar_feeds, core.calendar_reservations
FROM app_runtime, worker_runtime;

COMMIT;
