BEGIN;

-- Reverte ao vocabulário anterior na mesma ordem: a constraint só pode voltar
-- a exigir 'phase_not_implemented' depois que as linhas carregarem esse valor.

ALTER TABLE analytics.quality_snapshots
  DROP CONSTRAINT IF EXISTS quality_snapshots_fnrh_valid;

UPDATE analytics.quality_snapshots
  SET fnrh_failures_reason = 'phase_not_implemented'
  WHERE fnrh_failures_reason = 'not_implemented';

ALTER TABLE analytics.quality_snapshots
  ADD CONSTRAINT quality_snapshots_fnrh_valid
  CHECK (
    fnrh_failures IS NULL
    AND fnrh_failures_reason = 'phase_not_implemented'
  );

COMMIT;
