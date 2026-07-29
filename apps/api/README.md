# Backend da fundação

Este módulo contém somente a fundação técnica da Fase 1. Ele não implementa
estadias, questionários, analytics, dashboard ou FNRH.

## Processos

- `api`: API pública e listener operacional separado.
- `worker`: worker de infraestrutura e métricas, ainda sem jobs de domínio.

Os binários encerram listeners, conexões e telemetria ao receber `SIGINT` ou
`SIGTERM`.

Use `make up` na raiz para construir e iniciar os processos. Builds diretos sem
os `ldflags` produzidos por `deploy/scripts/with-build-metadata.sh` são
deliberadamente rejeitados no startup: versão, revisão e instante da fonte não
podem ser ausentes, `unknown`, inválidos ou substituídos por Unix epoch.

## Configuração

`APP_ENV`, `DATABASE_URL`, `OIDC_MODE`, `OIDC_ISSUER` e `OIDC_AUDIENCE` são
obrigatórios. A API também exige `HTTP_ADDRESS` e `OPERATIONS_ADDRESS`. O worker
usa `WORKER_OPERATIONS_ADDRESS`, cujo default local é `127.0.0.1:9091`.

O fake OIDC é aceito exclusivamente em `local` e `test`. O token fictício
`cumuru-local-platform-read` permite testar `GET /api/v1/platform/build`; ele
nunca deve ser utilizado com dados reais. `staging` e `production` exigem
`OIDC_MODE=real`, HTTPS, exportação OTLP configurada e conexão PostgreSQL com
exatamente um `sslmode=verify-full`. Modos que somente cifram ou validam a CA
sem autenticar o hostname são rejeitados.

Timeouts opcionais:

```text
DATABASE_TIMEOUT=3s
OIDC_HTTP_TIMEOUT=5s
OTEL_TIMEOUT=5s
SHUTDOWN_TIMEOUT=10s
HTTP_READ_HEADER_TIMEOUT=5s
HTTP_READ_TIMEOUT=15s
HTTP_WRITE_TIMEOUT=15s
HTTP_IDLE_TIMEOUT=60s
```

## Superfícies

- `GET /api/v1/platform/health`
- `GET /api/v1/platform/readiness`
- `GET /api/v1/platform/build` com `platform:read`
- `GET /metrics` somente no listener operacional

Logs, spans e labels são formados por allowlist. Corpos, query strings,
credenciais, cookies, tokens, sujeitos OIDC e identificadores de tenant não são
registrados.

## Validação

Na raiz:

```bash
go -C apps/api test ./...
go -C apps/api vet ./...
make lint
make scan
make build
```

Os testes de migração e isolamento com PostgreSQL real pertencem ao gate
integrado `make migration-test`.
