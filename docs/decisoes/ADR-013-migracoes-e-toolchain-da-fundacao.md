# ADR-013 — Migrações e toolchain da fundação

**Status:** aceito.

> Nota de baseline: o ADR-032 consolida os arquivos citados abaixo antes do
> primeiro lançamento. A sequência permanece somente como contexto histórico.

## Contexto

O schema lógico em `database/schema.sql` cobre o produto inteiro, enquanto a
Fase 1 não pode implementar estadias, questionários, dashboard ou FNRH.
Também era necessário fixar ferramentas reproduzíveis para migração, SQL,
OpenAPI, cliente, análise estática, vulnerabilidades, SBOM e scanners.

## Decisão

As migrações serão **incrementais por fase**. `database/schema.sql` continua
sendo o schema-alvo lógico; cada fase materializa somente sua parte. A Fase 1
cria exatamente:

1. `000001`: schemas `identity`, `core`, `survey`, `analytics`, `public_data` e
   `platform`, ainda sem enums de fases futuras;
2. `000002`: `core.organizations`, `core.accommodations` e
   `core.memberships`, com constraints e índices de tenancy;
3. `000003`: `platform.audit_events`, sem grants de update/delete para
   runtimes;
4. `000004`: revokes, grants e default privileges.

Tabelas de estadias, questionários, analytics e FNRH entram somente nas fases
proprietárias, sempre em novas migrações. Migrações aplicadas não são editadas.

Usaremos `golang-migrate` `v4.19.1`, com pares `up/down`. `down` destrutivo é
permitido somente em bancos descartáveis de `local` e `test`. Staging e
produção usam expand/contract e forward-fix; down não é estratégia de
recuperação. Na primeira versão, upgrade de uma release anterior é `N/A`, mas a
suíte deve testar a baseline vigente em `zero → latest → zero → latest` num
banco descartável. Antes da consolidação, esse gate percorria cada transição
interna; o ADR-032 substituiu essa parte por `zero → 1 → zero → 1`.

Papéis de grupo são provisionados fora das migrations de objetos. Os nomes
obrigatórios são `app_runtime`, `worker_runtime`, `public_runtime` e
`privacy_officer`; o usuário que executa migrations é `migration_admin` ou um
equivalente gerenciado. O ambiente local cria esses grupos e logins fictícios
por script de inicialização; ambientes gerenciados devem provisioná-los por
infraestrutura/secret manager. Migrações falham fechado se os papéis exigidos
não existirem.

A matriz inicial de privilégios é:

| Papel | Permitido | Negado |
| --- | --- | --- |
| `app_runtime` | `USAGE` em `core`/`platform`; `SELECT` nas três tabelas de tenancy; `INSERT` em `platform.audit_events` | `identity`, `survey`, `analytics`, `public_data`; `UPDATE/DELETE` de auditoria |
| `worker_runtime` | `USAGE` em `core`/`platform`; `SELECT` nas tabelas de tenancy; `INSERT` em auditoria | mesmos schemas restritos e mutação de auditoria |
| `public_runtime` | `USAGE` somente em `public_data`; `SELECT` somente em tabelas públicas futuras por grant explícito/default controlado | todos os demais schemas |
| `privacy_officer` | `USAGE` em `identity`; acesso futuro apenas por grant explícito e auditado | acesso amplo aos demais schemas |

`PUBLIC` não recebe acesso aos seis schemas nem às suas tabelas. Fases futuras
ampliam privilégios apenas por nova migration.

SQL é explícito e gerado com `sqlc` `v1.31.1`, usando `pgx/v5`. Código gerado é
recriado em diretório temporário e comparado byte a byte quando Git não estiver
disponível.

Versões iniciais fixadas:

| Componente | Versão |
| --- | --- |
| Go | 1.26.5 |
| PostgreSQL | 17.10 |
| React / React DOM | 19.2.8 |
| Vite | 8.1.5 |
| TypeScript | 7.0.2 |
| `pgx/v5` | 5.10.0 |
| `coreos/go-oidc/v3` | 3.20.0 |
| `sqlc` | 1.31.1 |
| `golang-migrate` | 4.19.1 |
| `openapi-typescript` | 7.13.0 |
| Redocly CLI | 2.41.0 |
| Staticcheck | 0.7.0 |
| `govulncheck` | 1.6.0 |
| CycloneDX GoMod | 1.10.0 |

Dependências Go e npm adicionais ficam travadas em `go.sum` e
`package-lock.json`. Atualizações posteriores são mudanças controladas.

SBOM usa CycloneDX e cobre dependências Go/npm e as imagens OCI de API/worker e
web, com manifesto associado aos IDs/digests e à revisão do build. Os gates de
segurança combinam `govulncheck`, `npm audit`, secret scan e Trivy para
filesystem/imagens. Ferramentas em containers são fixadas por digest e não
recebem o socket do Docker; imagens são exportadas para leitura somente local.
Ferramenta ausente ou daemon indisponível resulta em
`UNVERIFIED`/`BLOCKED`, nunca em sucesso silencioso.

## Consequências

- O recorte de fase não é violado apenas para antecipar tabelas futuras.
- O schema lógico e o schema aplicado terão diferença temporária intencional e
  documentada.
- O primeiro upgrade entre releases só poderá ser comprovado depois da primeira
  baseline publicada.
- Geração e ferramentas não dependem de versões globais implícitas.
