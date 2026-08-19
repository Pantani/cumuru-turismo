# ADR-030 — Fronteira pública, snapshots e HTTP da Fase 4

**Status:** aceito para `PROTOTYPE_ONLY`.

> Nota de baseline: o ADR-032 incorpora as migrations citadas neste documento
> à `000001_initial_schema`; a numeração permanece como contexto histórico.

> **Emendado pelo [ADR-045](ADR-045-camada-de-contexto-externo.md).** O acesso
> positivo do papel público passa de quatro para **cinco** views de
> `public_data`, com o acréscimo de `current_external_context`. A quinta view
> mora em `public_data` e lê `external` sob os privilégios do dono da view, de
> modo que `public_runtime` continua sem qualquer privilégio no schema
> `external` e o `search_path` do pool público permanece
> `pg_catalog, public_data`. O schema `external` é acrescentado à lista de
> schemas varridos pela validação de sessão apenas do lado **negativo**, para
> que um grant indevido seja detectado. A auditoria 6A fica invalidada por esta
> emenda.

## Contexto

A API atual usa somente o pool de `app_runtime`, e a migration `000004`
concede por default `SELECT` em futuras tabelas de `public_data`. O schema
lógico também coloca valores exatos e metadata interna na mesma tabela. Esses
atalhos permitiriam que uma falha do handler contornasse a proteção.

## Decisão

As migrations aplicadas `000001`–`000010` não serão editadas. A Fase 4 cria:

1. `000011_create_analytics_domain`;
2. `000012_create_public_snapshots`;
3. `000013_apply_phase4_privileges`;
4. `000014_harden_public_runtime_session`;
5. `000015_secure_preference_aggregation`;
6. `000016_expose_forecast_fallback_bounds`.

`000011` começa revogando o default `SELECT` futuro de `public_runtime` em
`public_data`. Microdados pseudonimizados, catálogo, runs, staging, qualidade e
valores exatos ficam em `analytics`.

`public_data` contém releases imutáveis e células já protegidas, mas o papel
público não lê tabelas base. Ele recebe `SELECT` explícito apenas nas views:

- `current_summary`;
- `current_presence`;
- `current_preferences`;
- `current_methodology`.

`PUBLIC` e os runtimes recebem revokes explícitos antes dos grants. Uma tabela
canário criada depois das migrations não pode herdar acesso público.

### Pools

A API mantém `DATABASE_URL` para as rotas autenticadas e exige
`PUBLIC_DATABASE_URL` para a superfície pública da Fase 4. O pool público
executa `SET ROLE public_runtime` ao conectar e valida:

- `current_user = 'public_runtime'`;
- `session_user` distinto, `LOGIN`, sem superuser, `CREATEDB`, `CREATEROLE`,
  `REPLICATION` ou `BYPASSRLS`;
- closure transitiva de papéis alcançáveis limitada a `public_runtime`;
- ausência de `CREATE`/`TEMPORARY` no banco e de `CREATE` nos schemas;
- acesso positivo somente às quatro views;
- acesso negativo aos demais schemas, tabelas base, funções, sequences e
  operações de escrita.

Falha de DSN, role ou ACL impede startup; não existe fallback para
`app_runtime`. O repository público só conhece DTOs das views correntes.
O provisionamento local revoga `CREATE` e `TEMPORARY` de `PUBLIC`, do papel e
do login público antes das migrations; `000014` repete e valida essa fronteira
para upgrades.

`app_runtime` recebe `SELECT` somente na view agregada
`analytics.current_quality`. `worker_runtime` recebe leitura mínima de
core/outbox, sem acesso direto às tabelas de respostas. A agregação de
preferências ocorre pela função fechada
`analytics.aggregate_eligible_preferences`, `SECURITY DEFINER`, com owner
`migration_admin` `NOLOGIN`, `search_path = pg_catalog`, SQL estático e
retorno agregado sem IDs, source values ou texto livre.

### Snapshot

O worker constrói staging interno, aplica elegibilidade, supressão,
differencing, arredondamento e invariantes. Depois:

1. inicia transação;
2. insere a release e as células protegidas;
3. troca o singleton `current_publication`;
4. marca o run publicado;
5. confirma.

O `build_fingerprint` torna retry idempotente. Falha antes do commit preserva
integralmente o ponteiro anterior. Sem release válida, a API responde `503`
sanitizado; nunca consulta `core`, `survey` ou `analytics` como fallback.
Releases anteriores e staging não são enumeráveis pelo papel público.

### HTTP

O contrato `0.5.1` oferece:

- `GET /public/summary`, sem filtro;
- `GET /public/presence?window=recent_30_days|next_30_days`;
- `GET /public/preferences?period=last_complete_month`;
- `GET /public/methodology`;
- `GET /analytics/quality?window=last_30_days`, com
  `analytics:read:internal`.

Schemas são fechados. A metodologia é estruturada e versionada; não transporta
texto administrativo livre. As faixas nominal `[85,115]` e fallback
`[70,130]` são propriedades distintas e obrigatórias.

Sucesso público usa ETag forte do JSON canônico e
`Cache-Control: public, max-age=300, stale-if-error=86400`. `If-None-Match`
correspondente retorna `304`. Resposta autenticada, Problem, ausência de
snapshot e erros usam `no-store`. Cache e ETag variam somente por operação e
seletor catalogado.

O service worker não intercepta `/api/`. Endpoints públicos não aceitam
cookie, bearer, IDs, filtros livres ou seleção de release.

## Consequências

- a fronteira é aplicada pelo PostgreSQL, não apenas pelo mapper Go;
- OpenAPI, migrations, schema lógico, queries e gerados têm owner único e são
  congelados antes de backend e frontend;
- o QA precisa provar `current_user`, ACL positiva/negativa, instalação e
  down/up da baseline vigente, falha parcial e última release válida; antes do
  ADR-032, isso incluía o upgrade interno `000015→000016`;
- CDN, OIDC institucional, deploy e produção permanecem `UNVERIFIED` ou
  `BLOCKED`.
