BEGIN;

REVOKE INSERT (
  id,
  name
) ON TABLE core.organizations
FROM app_runtime;

REVOKE INSERT (
  id,
  organization_id,
  name,
  category,
  status,
  capacity,
  onboarding_submission_id
) ON TABLE core.accommodations
FROM app_runtime;

GRANT UPDATE (cadastur_id) ON TABLE core.accommodations
TO app_runtime;

REVOKE SELECT (
  id,
  name,
  legal_name,
  created_at,
  updated_at,
  version
) ON TABLE core.organizations
FROM app_runtime, worker_runtime;

GRANT SELECT ON TABLE core.organizations
TO app_runtime, worker_runtime;

DROP INDEX core.accommodations_onboarding_submission_idx;

ALTER TABLE core.accommodations
  DROP CONSTRAINT accommodations_category_valid;

UPDATE core.accommodations AS accommodation
SET
  category = CASE accommodation.id
    WHEN '019fae11-0000-7000-8000-000000000001'::uuid THEN 'pousada'
    WHEN '019fae11-0000-7000-8000-000000000002'::uuid THEN 'hospedaria'
    WHEN '019fae11-0000-7000-8000-000000000003'::uuid THEN 'chale'
    WHEN '019fae11-0000-7000-8000-000000000004'::uuid THEN 'casa'
  END,
  cadastur_id = NULL
WHERE accommodation.organization_id =
    '019fae10-0000-7000-8000-000000000001'::uuid
  AND (
    (
      accommodation.id = '019fae11-0000-7000-8000-000000000001'::uuid
      AND accommodation.name = 'Pousada Farol Fictícia'
      AND accommodation.category = 'formal_lodging'
      AND accommodation.cadastur_id = 'CADASTUR-FICTICIO-NAO-VALIDO'
    )
    OR
    (
      accommodation.id = '019fae11-0000-7000-8000-000000000002'::uuid
      AND accommodation.name = 'Hospedaria Rio Fictícia'
      AND accommodation.category = 'formal_lodging'
      AND accommodation.cadastur_id IS NULL
    )
    OR
    (
      accommodation.id = '019fae11-0000-7000-8000-000000000003'::uuid
      AND accommodation.name = 'Chalés Areia Fictícios'
      AND accommodation.category = 'seasonal_rental'
      AND accommodation.cadastur_id IS NULL
    )
    OR
    (
      accommodation.id = '019fae11-0000-7000-8000-000000000004'::uuid
      AND accommodation.name = 'Casa Silenciosa Fictícia'
      AND accommodation.category = 'family_hosting'
      AND accommodation.cadastur_id IS NULL
    )
  );

ALTER TABLE core.accommodations
  DROP COLUMN onboarding_submission_id;

COMMENT ON COLUMN core.accommodations.cadastur_id IS NULL;

COMMIT;
