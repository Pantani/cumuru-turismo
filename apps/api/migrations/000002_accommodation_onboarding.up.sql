BEGIN;

ALTER TABLE core.accommodations
  ADD COLUMN onboarding_submission_id uuid;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM core.accommodations AS accommodation
    WHERE accommodation.category NOT IN (
      'formal_lodging',
      'seasonal_rental',
      'family_hosting',
      'camping',
      'regularizing',
      'other',
      'unclassified'
    )
      AND NOT (
        accommodation.organization_id =
          '019fae10-0000-7000-8000-000000000001'::uuid
        AND (
          (
            accommodation.id =
              '019fae11-0000-7000-8000-000000000001'::uuid
            AND accommodation.name = 'Pousada Farol Fictícia'
            AND accommodation.category = 'pousada'
          )
          OR
          (
            accommodation.id =
              '019fae11-0000-7000-8000-000000000002'::uuid
            AND accommodation.name = 'Hospedaria Rio Fictícia'
            AND accommodation.category = 'hospedaria'
          )
          OR
          (
            accommodation.id =
              '019fae11-0000-7000-8000-000000000003'::uuid
            AND accommodation.name = 'Chalés Areia Fictícios'
            AND accommodation.category = 'chale'
          )
          OR
          (
            accommodation.id =
              '019fae11-0000-7000-8000-000000000004'::uuid
            AND accommodation.name = 'Casa Silenciosa Fictícia'
            AND accommodation.category = 'casa'
          )
        )
      )
  ) THEN
    RAISE EXCEPTION 'unknown legacy accommodation category blocks migration'
      USING ERRCODE = '23514';
  END IF;
END
$$;

UPDATE core.accommodations AS accommodation
SET
  category = CASE accommodation.id
    WHEN '019fae11-0000-7000-8000-000000000001'::uuid
      THEN 'formal_lodging'
    WHEN '019fae11-0000-7000-8000-000000000002'::uuid
      THEN 'formal_lodging'
    WHEN '019fae11-0000-7000-8000-000000000003'::uuid
      THEN 'seasonal_rental'
    WHEN '019fae11-0000-7000-8000-000000000004'::uuid
      THEN 'family_hosting'
  END,
  cadastur_id = CASE accommodation.id
    WHEN '019fae11-0000-7000-8000-000000000001'::uuid
      THEN 'CADASTUR-FICTICIO-NAO-VALIDO'
    ELSE NULL
  END
WHERE accommodation.organization_id =
    '019fae10-0000-7000-8000-000000000001'::uuid
  AND (
    (
      accommodation.id = '019fae11-0000-7000-8000-000000000001'::uuid
      AND accommodation.name = 'Pousada Farol Fictícia'
      AND accommodation.category = 'pousada'
    )
    OR
    (
      accommodation.id = '019fae11-0000-7000-8000-000000000002'::uuid
      AND accommodation.name = 'Hospedaria Rio Fictícia'
      AND accommodation.category = 'hospedaria'
    )
    OR
    (
      accommodation.id = '019fae11-0000-7000-8000-000000000003'::uuid
      AND accommodation.name = 'Chalés Areia Fictícios'
      AND accommodation.category = 'chale'
    )
    OR
    (
      accommodation.id = '019fae11-0000-7000-8000-000000000004'::uuid
      AND accommodation.name = 'Casa Silenciosa Fictícia'
      AND accommodation.category = 'casa'
    )
  );

ALTER TABLE core.accommodations
  ADD CONSTRAINT accommodations_category_valid
  CHECK (
    category IN (
      'formal_lodging',
      'seasonal_rental',
      'family_hosting',
      'camping',
      'regularizing',
      'other',
      'unclassified'
    )
  );

CREATE UNIQUE INDEX accommodations_onboarding_submission_idx
  ON core.accommodations (organization_id, onboarding_submission_id)
  WHERE onboarding_submission_id IS NOT NULL;

COMMENT ON COLUMN core.accommodations.onboarding_submission_id IS
  'UUIDv7 de repetição do onboarding local; não é documento, tenant externo ou identificador FNRH.';
COMMENT ON COLUMN core.accommodations.cadastur_id IS
  'Metadado opcional provisionado por fonte confiável; nunca prova ou habilita integração FNRH.';

REVOKE SELECT ON TABLE core.organizations
FROM app_runtime, worker_runtime;

GRANT SELECT (
  id,
  name,
  legal_name,
  created_at,
  updated_at,
  version
) ON TABLE core.organizations
TO app_runtime, worker_runtime;

GRANT INSERT (
  id,
  name
) ON TABLE core.organizations
TO app_runtime;

GRANT INSERT (
  id,
  organization_id,
  name,
  category,
  status,
  capacity,
  onboarding_submission_id
) ON TABLE core.accommodations
TO app_runtime;

REVOKE UPDATE (cadastur_id) ON TABLE core.accommodations
FROM app_runtime;

COMMIT;
