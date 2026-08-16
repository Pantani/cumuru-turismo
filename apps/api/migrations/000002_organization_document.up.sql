BEGIN;

-- ADR-038: the responsible party is identified by exactly one CPF or CNPJ,
-- persisted only as a keyed HMAC. The plaintext is never stored, so this column
-- can answer "already registered" and nothing else.

ALTER TABLE core.organizations
  ADD CONSTRAINT organizations_document_hmac_length
  CHECK (document_hmac IS NULL OR octet_length(document_hmac) = 32);

-- Partial: organizations provisioned before ADR-038, and the fictitious seed
-- tenants, carry no document and must not collide with each other.
CREATE UNIQUE INDEX organizations_document_hmac_idx
  ON core.organizations (document_hmac)
  WHERE document_hmac IS NOT NULL;

COMMENT ON COLUMN core.organizations.document_hmac IS
  'HMAC-SHA256 do CPF ou CNPJ do responsável, com chave rotacionável e separada '
  'da chave de dados pessoais. O valor em claro nunca é persistido nem '
  'recuperável: serve apenas para recusar documento já cadastrado. Ver ADR-038.';

COMMIT;
