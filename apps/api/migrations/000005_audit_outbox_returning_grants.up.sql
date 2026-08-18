BEGIN;

-- Fecha a classe de incidente aberta em CreateActivationAccount
-- (queries/auth.sql:140-146): app_runtime tinha INSERT sem nenhum SELECT em
-- platform.audit_events e platform.outbox_events, então a PRIMEIRA query com
-- RETURNING sobre qualquer uma delas derruba a transação inteira com
-- "permission denied for table", no próprio INSERT. O incidente já mordeu
-- auth.accounts; aqui é prevenção, não reação — nenhuma query nova passa a
-- usar RETURNING nesta migração.
--
-- O escopo é o menor que evita a classe: só a coluna `id`. É a única coluna
-- que uma cláusula RETURNING precisaria projetar quando o valor já é gerado
-- pelo cliente (como acontece hoje: id vem de sqlc.arg(id) nas duas tabelas,
-- não de DEFAULT do servidor) — o padrão real já observado em
-- CreateActivationAccount, onde o Go só consumia row.ID. Nenhuma outra
-- coluna destas tabelas é concedida: actor_subject_hmac, metadata e o
-- payload do outbox continuam sem SELECT nenhum para app_runtime.

GRANT SELECT (id) ON TABLE platform.audit_events TO app_runtime;
GRANT SELECT (id) ON TABLE platform.outbox_events TO app_runtime;

COMMIT;
