-- Lista pública de hospedagens. O hóspede que chega na praia não tem por onde
-- descobrir quem hospeda: o cadastro existe, mas só é legível por dentro, para
-- quem já tem conta. A lista publica nome, categoria, localidade, capacidade,
-- telefone e site — e nada mais.
--
-- A publicação é ato da própria hospedagem, não consequência de estar cadastrada.
-- Telefone de casa de família é telefone de pessoa natural, e publicar por
-- padrão o contato de quem só quis se cadastrar não teria base legal (LGPD
-- art. 7º). Por isso `public_listing_enabled` nasce falso, exige telefone e
-- carimba `public_listing_consented_at`; desmarcar apaga o carimbo e some da
-- lista na mesma transação, sem fila nem espera.
BEGIN;

ALTER TABLE core.accommodations
  ADD COLUMN public_listing_enabled boolean NOT NULL DEFAULT false,
  ADD COLUMN public_contact_phone text,
  ADD COLUMN public_contact_whatsapp boolean NOT NULL DEFAULT false,
  ADD COLUMN public_website_url text,
  ADD COLUMN public_listing_consented_at timestamptz;

-- E.164 normalizado: o número é publicado como link discável e como wa.me, e
-- os dois só funcionam sem separador. A normalização é da aplicação; aqui fica
-- a recusa do que não é discável.
ALTER TABLE core.accommodations
  ADD CONSTRAINT accommodations_public_phone_e164
  CHECK (
    public_contact_phone IS NULL
    OR public_contact_phone ~ '^\+[1-9][0-9]{9,14}$'
  );

ALTER TABLE core.accommodations
  ADD CONSTRAINT accommodations_public_website_https
  CHECK (
    public_website_url IS NULL
    OR public_website_url ~ '^https://[^[:space:]]{4,180}$'
  );

-- WhatsApp é propriedade do número publicado, não um segundo contato: sem
-- telefone não existe o que marcar.
ALTER TABLE core.accommodations
  ADD CONSTRAINT accommodations_public_whatsapp_needs_phone
  CHECK (public_contact_whatsapp = false OR public_contact_phone IS NOT NULL);

-- Publicada exige telefone e consentimento carimbado; não publicada não guarda
-- carimbo de consentimento que já foi retirado.
ALTER TABLE core.accommodations
  ADD CONSTRAINT accommodations_public_listing_consistent
  CHECK (
    (
      public_listing_enabled = false
      AND public_listing_consented_at IS NULL
    )
    OR (
      public_listing_enabled = true
      AND public_contact_phone IS NOT NULL
      AND public_listing_consented_at IS NOT NULL
    )
  );

-- A leitura pública é sempre a lista inteira ordenada por nome, filtrada por
-- publicação e situação; o índice parcial cobre exatamente essa varredura.
CREATE INDEX accommodations_public_directory_idx
  ON core.accommodations (name, id)
  WHERE public_listing_enabled = true AND status = 'active';

COMMENT ON COLUMN core.accommodations.public_listing_enabled IS
  'Consentimento vivo de publicação na lista pública; retirado, a linha some da lista imediatamente.';
COMMENT ON COLUMN core.accommodations.public_contact_phone IS
  'Telefone de contato publicado em E.164, informado pela própria hospedagem; nunca importado de outra fonte.';
COMMENT ON COLUMN core.accommodations.public_listing_consented_at IS
  'Instante do consentimento de publicação vigente; nulo enquanto a hospedagem não estiver publicada.';

GRANT UPDATE (
  public_listing_enabled,
  public_contact_phone,
  public_contact_whatsapp,
  public_website_url,
  public_listing_consented_at
) ON TABLE core.accommodations
TO app_runtime;

COMMIT;
