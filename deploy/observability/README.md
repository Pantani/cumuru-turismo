# Observabilidade local

O overlay `deploy/compose.observability.yaml` adiciona OpenTelemetry Collector,
Prometheus, Tempo e Grafana ao Compose base. Todo endpoint publicado permanece
limitado a `127.0.0.1`; credenciais e armazenamento são fixtures locais.

```bash
deploy/scripts/local-infra.sh up
deploy/scripts/local-infra.sh smoke
deploy/scripts/local-infra.sh down
```

O Grafana fica em `http://127.0.0.1:3000`, com usuário `admin` e senha local
`cumuru-local-only` por padrão. Não reutilize essas credenciais fora de
`local|test`.
