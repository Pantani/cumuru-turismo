# ADR-032 — Consolidação pré-lançamento da baseline de migrations

**Status:** aceito.

> **Emendado pelo [ADR-045](ADR-045-camada-de-contexto-externo.md).** A
> partir de `000003`, o regime é **append-only**: não há quarta onda de
> squash. O regime já havia sido retomado na prática por
> `000002_presence_history_window`. Nova consolidação, se algum dia houver,
> exige emenda explícita a este ADR, não escolha silenciosa de uma onda.

## Contexto

As Fases 1–4 foram implementadas antes da criação ou lançamento de qualquer
banco persistente. A cadeia executável chegou a `000019` durante o
desenvolvimento e contém a evolução completa de schemas, tabelas, tipos,
constraints, índices, funções, triggers, views, comentários, owners, grants,
default privileges, hardening do banco e seeds técnicos.

Não existe release anterior nem banco implantado que dependa das versões
intermediárias. Manter dezenove versões como histórico de upgrade antes da
primeira baseline publicada aumentaria o custo de instalação, rollback
descartável e validação sem oferecer compatibilidade real.

`database/schema.sql` permanece um modelo lógico e não é equivalente à cadeia
executável: contém objetos futuros e não materializa sozinho todos os guards,
owners e ACLs. Portanto, ele não pode substituir as migrations como fonte do
squash.

## Decisão

Consolidar a cadeia pré-lançamento `000001`–`000019` em um único par:

```text
apps/api/migrations/000001_initial_schema.up.sql
apps/api/migrations/000001_initial_schema.down.sql
```

O `up` preserva o SQL efetivo das migrations atuais na mesma ordem. O `down`
preserva os efeitos dos arquivos atuais em ordem reversa. Os pares
`000002`–`000019` deixam de existir.

A baseline consolidada deve preservar exatamente o estado final da cadeia,
incluindo:

- a definição determinística final de `analytics.current_quality`;
- funções `SECURITY DEFINER`, seus owners e `search_path`;
- triggers e guards append-only/imutáveis;
- ACLs de schemas, tabelas, colunas, funções e banco;
- default privileges e hardening dos logins públicos;
- as três linhas técnicas de `analytics.metric_catalog`;
- comentários e views `security_barrier`.

A prova obrigatória usa dois PostgreSQL 17.10 descartáveis, provisionados com
as mesmas roles:

1. banco A aplica a cadeia preservada `000001`–`000019`;
2. banco B aplica somente a nova `000001`;
3. dumps de schema, catálogos, ACLs e seeds são comparados;
4. a única diferença intencional é
   `public.schema_migrations.version` (`19` em A e `1` em B);
5. ambos devem permanecer com `dirty=false`.

Tokens não determinísticos `\restrict` e `\unrestrict` do `pg_dump` são
removidos antes do diff, sem remover owners, comments ou grants.

`deploy/scripts/test-migrations.sh` passa a exigir exatamente um par
`000001`, testar `zero → 1 → zero → 1` e preservar as invariantes finais e
negações de privilégio. Backfills e upgrades entre versões internas de
desenvolvimento tornam-se `N/A`.

O bootstrap externo de roles em
`deploy/postgres/init/001-create-local-roles.sql` não pertence à cadeia
`golang-migrate` e permanece inalterado.

Depois que a primeira baseline for lançada ou aplicada em ambiente
persistente, `000001` torna-se imutável. Toda evolução posterior volta a usar
migrations append-only a partir de `000002`.

### Segunda onda de consolidação

Nenhum banco persistente foi montado ou lançado depois da primeira onda, e a
cadeia voltou a crescer durante o desenvolvimento até `000004`. A mesma decisão
é reaplicada: `000002_accommodation_onboarding`,
`000003_local_password_auth` e `000004_admin_seed_password_rotation` passam a
integrar o par `000001_initial_schema`, na mesma ordem no `up` e em ordem
reversa no `down`, com os comentários de proveniência marcados como
`second pre-launch wave`.

Só dois blocos foram omitidos, e ambos migram dados preexistentes, não schema:
a guarda `DO` de categoria legada e o backfill `UPDATE` de
`000002_accommodation_onboarding`, mais o `DO` simétrico do `down`
correspondente. Em uma baseline única `core.accommodations` é criada vazia no
mesmo arquivo e o CHECK `accommodations_category_valid` passa a valer desde a
instalação, de modo que nenhuma linha legada pode existir para ser convertida.
Os seeds versionados já usam as categorias finais. O schema final, os grants e
os comentários permanecem idênticos.

### Terceira onda de consolidação

Nenhum banco persistente foi montado ou lançado depois da segunda onda, e a
cadeia voltou a crescer durante o desenvolvimento até `000005`. A mesma
decisão é reaplicada: `000002_organization_document`,
`000003_self_service_and_approval`, `000004_rename_fnrh_failures_reason` e
`000005_audit_outbox_returning_grants` passam a integrar o par
`000001_initial_schema`, na mesma ordem no `up` e em ordem reversa no `down`,
com o comentário de proveniência marcado como `third pre-launch wave`.

Diferente da segunda onda, nenhum bloco foi omitido: a concatenação é
literal, cada migração absorvida preserva seus próprios `BEGIN`/`COMMIT`
internos, e nenhuma delas continha backfill de dados legados a eliminar — só
DDL, mudança de vocabulário de dado técnico (`fnrh_failures_reason`) e
grants. O schema final, os grants e os comentários permanecem idênticos aos
da cadeia `000001`–`000005` aplicada em sequência.

## Consequências

- instalações novas executam uma única versão de migration;
- bancos locais de desenvolvimento na versão 2–19 devem ser descartados e
  recriados, nunca forçados para a versão 1;
- não há manifesto ou checksum específico de migrations para atualizar;
- documentação e testes que tratavam versões intermediárias como superfície
  atual precisam apontar para a baseline consolidada;
- a política geral de expand/contract e forward-fix continua válida depois do
  primeiro lançamento;
- esta decisão não autoriza deploy, release ou uso com dados reais e mantém o
  projeto `PROTOTYPE_ONLY`.
