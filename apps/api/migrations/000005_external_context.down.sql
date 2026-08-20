-- Reverte a camada de contexto externo. O `down` precisa dropar a view nova
-- **e** o schema `external`: um `down` incompleto deixaria schema órfão, e a
-- contagem de schemas remanescentes em `test-migrations.sh` passou a incluir
-- `external` justamente para que isso não passe em silêncio (ADR-045, emenda
-- ao ADR-032).
--
-- O papel `external_runtime` não é removido aqui: papéis são provisionados
-- fora da cadeia de migrations (`deploy/postgres/init/001-create-local-roles.sql`
-- em local, ansible em deploy), exatamente como os quatro anteriores, e a
-- 000001 também não os cria nem os dropa.
BEGIN;

DROP VIEW public_data.current_external_sources;
DROP VIEW public_data.current_external_context;

-- Sem CASCADE de propósito: se algo fora de `external` ainda depender do
-- schema, o rollback falha alto em vez de arrastar objeto de outra camada.
DROP TABLE external.tide_harmonics;
DROP TABLE external.tide_stations;
DROP INDEX external.observations_series_period_idx;
DROP INDEX external.observations_idempotency_idx;
DROP TABLE external.observations;
DROP INDEX external.fetch_runs_source_started_idx;
DROP TABLE external.fetch_runs;
DROP TABLE external.series;
DROP TABLE external.sources;

-- As entradas de default privileges deste schema saem com ele: `DROP SCHEMA`
-- remove as linhas de `pg_default_acl`. Reconceder ALL a PUBLIC aqui seria
-- abrir o que o `up` fechou, não desfazer.

DROP SCHEMA external;

COMMIT;
