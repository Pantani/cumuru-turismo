\set ON_ERROR_STOP on

-- Somente desenvolvimento local, com credenciais deliberadamente fictícias.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'migration_admin') THEN
    CREATE ROLE migration_admin NOLOGIN;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_runtime') THEN
    CREATE ROLE app_runtime NOLOGIN;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'worker_runtime') THEN
    CREATE ROLE worker_runtime NOLOGIN;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'public_runtime') THEN
    CREATE ROLE public_runtime NOLOGIN;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'privacy_officer') THEN
    CREATE ROLE privacy_officer NOLOGIN;
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'cumuru_migration') THEN
    CREATE ROLE cumuru_migration
      LOGIN PASSWORD 'cumuru-local-migration-only'
      IN ROLE migration_admin;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'cumuru_app') THEN
    CREATE ROLE cumuru_app
      LOGIN PASSWORD 'cumuru-local-app-only'
      IN ROLE app_runtime;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'cumuru_worker') THEN
    CREATE ROLE cumuru_worker
      LOGIN PASSWORD 'cumuru-local-worker-only'
      IN ROLE worker_runtime;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'cumuru_public') THEN
    CREATE ROLE cumuru_public
      LOGIN PASSWORD 'cumuru-local-public-only'
      IN ROLE public_runtime;
  END IF;
END
$$;

GRANT CONNECT, CREATE, TEMPORARY ON DATABASE cumuru TO migration_admin;
GRANT USAGE, CREATE ON SCHEMA public TO migration_admin;
GRANT CONNECT ON DATABASE cumuru
  TO app_runtime, worker_runtime, public_runtime, privacy_officer;
REVOKE CREATE, TEMPORARY ON DATABASE cumuru
  FROM PUBLIC, public_runtime, cumuru_public;

ALTER ROLE cumuru_migration SET search_path = public;
ALTER ROLE cumuru_app SET search_path = core, platform, public;
ALTER ROLE cumuru_worker SET search_path = core, platform, public;
ALTER ROLE cumuru_public SET search_path = public_data, public;
