-- Remove a lista pública de hospedagens. O consentimento de publicação vai
-- junto: sem a coluna não há o que revogar depois, e manter o telefone
-- publicado sem a lista que o exibia guardaria dado de contato sem finalidade.
BEGIN;

REVOKE UPDATE (
  public_listing_enabled,
  public_contact_phone,
  public_contact_whatsapp,
  public_website_url,
  public_listing_consented_at
) ON TABLE core.accommodations
FROM app_runtime;

DROP INDEX core.accommodations_public_directory_idx;

ALTER TABLE core.accommodations
  DROP CONSTRAINT accommodations_public_listing_consistent;

ALTER TABLE core.accommodations
  DROP CONSTRAINT accommodations_public_whatsapp_needs_phone;

ALTER TABLE core.accommodations
  DROP CONSTRAINT accommodations_public_website_https;

ALTER TABLE core.accommodations
  DROP CONSTRAINT accommodations_public_phone_e164;

ALTER TABLE core.accommodations
  DROP COLUMN public_listing_consented_at,
  DROP COLUMN public_website_url,
  DROP COLUMN public_contact_whatsapp,
  DROP COLUMN public_contact_phone,
  DROP COLUMN public_listing_enabled;

COMMIT;
