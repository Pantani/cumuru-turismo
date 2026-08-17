# ADR-012 — Contrato faseado e superfície operacional

**Status:** aceito.

## Contexto

`contracts/openapi.yaml` descreve operações de estadias, questionários,
privacidade e analytics que pertencem a fases posteriores. A Fase 1, por outro
lado, precisa entregar health, readiness, build info, métricas e um cliente
TypeScript gerado, sem criar handlers prematuros para as operações futuras.

Tratar todo o arquivo como API já entregue faria o cliente prometer
funcionalidades inexistentes. Remover as operações futuras apagaria o contrato
de referência do blueprint.

## Decisão

`contracts/openapi.yaml` permanece o **contrato-alvo faseado** da API. Cada
operação recebe `x-cumuru-feature`, e a documentação de entrega
deixa explícito quais fases estão implementadas.

A Fase 1 adiciona, sob o servidor `/api/v1`:

- `GET /platform/health`, público, para liveness do processo;
- `GET /platform/readiness`, público, para readiness do PostgreSQL;
- `GET /platform/build`, autenticado com o escopo `platform:read`.

Os contratos são:

- health: `200 application/json` com `{"status":"ok"}`;
- readiness saudável: `200 application/json` com `{"status":"ready"}`;
- readiness degradada: `503 application/problem+json`;
- build: `200 application/json` com `version`, `revision` e `built_at`;
- build sem credencial válida: `401 application/problem+json`;
- build sem `platform:read`: `403 application/problem+json`.

Todos os sucessos usam schemas fechados (`additionalProperties: false`).
Respostas JSON incluem `X-Request-ID` e `Cache-Control: no-store`. Liveness não
consulta dependências. Readiness usa timeout curto e não expõe DSN, host, SQL
ou erro bruto. `revision` e `built_at` seguem a proveniência reprodutível e
fail-closed definida no ADR-015, inclusive quando `SCM=ABSENT`.

Métricas Prometheus ficam fora do contrato público e são servidas em listener
operacional separado, no path `/metrics`. Esse listener deve ser restrito pela
topologia de rede e não usa IDs, sujeito OIDC, organização ou path variável
como labels.

Health e readiness não prometem `429` na Fase 1 porque não há limiter na
aplicação e probes não devem ser limitadas silenciosamente. Antes de exposição
fora da topologia local, o controle de taxa deve ser decidido na borda ou
implementado e contratado junto com `Retry-After`.

O cliente web é composto por tipos gerados de todo o contrato-alvo e um wrapper
humano com `openapi-fetch`. A geração dos tipos futuros não autoriza handlers,
telas ou comportamento antes da fase proprietária.

Mudanças no contrato atualizam OpenAPI, geração, testes e `CHANGELOG.md` na
mesma alteração.

## Consequências

- O contrato preserva o desenho futuro sem fingir que ele está entregue.
- Consumidores precisam observar a fase de implementação registrada.
- A Fase 1 possui uma rota autenticada real para provar rejeição de token
  inválido.
- `/metrics` não pode ser publicado pelo mesmo roteamento público por acidente.
