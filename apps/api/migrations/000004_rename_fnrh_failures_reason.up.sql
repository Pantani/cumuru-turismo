BEGIN;

-- O código passou a ser nomeado por funcionalidade, não pela fase que a
-- entregou. O valor 'phase_not_implemented' era o último resquício do
-- vocabulário de fase numa resposta pública da API: quem lê
-- UnavailableQualityCount.reason_code não tem como saber que fase é essa.
--
-- A constraint fixa um único valor, então a ordem é obrigatória: soltar a
-- constraint, reescrever as linhas existentes e só então refazê-la. Recriar a
-- constraint antes do UPDATE recusaria as próprias linhas que já estão na
-- tabela.

ALTER TABLE analytics.quality_snapshots
  DROP CONSTRAINT IF EXISTS quality_snapshots_fnrh_valid;

UPDATE analytics.quality_snapshots
  SET fnrh_failures_reason = 'not_implemented'
  WHERE fnrh_failures_reason = 'phase_not_implemented';

ALTER TABLE analytics.quality_snapshots
  ADD CONSTRAINT quality_snapshots_fnrh_valid
  CHECK (
    fnrh_failures IS NULL
    AND fnrh_failures_reason = 'not_implemented'
  );

COMMIT;
