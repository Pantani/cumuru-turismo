BEGIN;

ALTER TABLE auth.accounts
  ADD COLUMN password_must_change boolean NOT NULL DEFAULT false;

COMMENT ON COLUMN auth.accounts.password_must_change IS
  'Marca a credencial semeada como provisória. Enquanto for verdadeira a sessão '
  'só alcança troca de senha, logout e leitura da própria sessão, de modo que a '
  'senha de bootstrap não sobrevive ao primeiro acesso.';

-- A rotação é executada pelo processo da API em nome do titular da sessão, e é
-- o único caminho que escreve a credencial fora do provisionamento.
GRANT SELECT (password_must_change) ON TABLE auth.accounts TO app_runtime;

GRANT UPDATE (
  password_hash,
  password_changed_at,
  password_must_change
) ON TABLE auth.accounts TO app_runtime;

REVOKE ALL ON TABLE auth.accounts
  FROM worker_runtime, public_runtime, privacy_officer;

COMMIT;
