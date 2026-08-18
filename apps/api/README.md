# Backend modular

Monólito modular em Go. Os domínios implementados são `platform` (saúde,
prontidão, build e observabilidade), `core` (acomodações, estadias, grupos,
convites e auditoria), `questionnaire` (versões, publicação e submissão),
`analytics` (presença, agregação e publicação anônima) e `self-service`
(autocadastro, ativação de conta e fila de aprovação). Cada um vive sob
`internal/`, com a configuração correspondente em
`internal/platform/config/`.

A integração FNRH não está implementada e continua bloqueada por gates
externos; sua ausência não deve ser inferida da existência de adaptadores,
contratos ou fixtures locais. Todo o runtime opera como `PROTOTYPE_ONLY`: use
somente dados, identidades e credenciais fictícios.

## Processos

- `api`: API pública e listener operacional separado.
- `worker`: manutenção de infraestrutura, métricas e jobs internos de
  reconciliação e publicação. Entrega externa da outbox e FNRH não fazem parte
  do runtime concluído.

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
nunca deve ser utilizado com dados reais. Esse token responde pelo sujeito
`fixture-platform-probe`, separado da operadora fictícia do `local-demo`, para
que a trilha de auditoria distinga a sondagem da pessoa que opera a demonstração. `staging` e `production` exigem
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

## Superfícies da fundação

- `GET /api/v1/platform/health`
- `GET /api/v1/platform/readiness`
- `GET /api/v1/platform/build` com `platform:read`
- `GET /metrics` somente no listener operacional

As demais operações implementadas são definidas em `contracts/openapi.yaml` e
devem ser consumidas pelo cliente TypeScript gerado, sem contratos paralelos.

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
