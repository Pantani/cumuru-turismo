BEGIN;

COMMENT ON COLUMN core.organizations.document_hmac IS NULL;

DROP INDEX IF EXISTS core.organizations_document_hmac_idx;

ALTER TABLE core.organizations
  DROP CONSTRAINT IF EXISTS organizations_document_hmac_length;

COMMIT;
