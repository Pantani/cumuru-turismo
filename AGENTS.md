# Instruções para agentes de implementação

Leia todo o `README.md` e os documentos em `docs/` antes de alterar a aplicação.

## Missão

Construir o Observatório Turístico de Cumuruxatiba como um monólito modular em Go
e uma aplicação React estática, preservando privacidade, idempotência e
auditabilidade.

## Regras de trabalho

1. Implemente uma onda por vez, seguindo a ordem do harness em
   `.agents/skills/cumuru-bootstrap/references/phase-matrix.md`. Os critérios
   de aceite por funcionalidade estão em `docs/09-roadmap-e-aceite.md`.
2. Antes de codificar, registre decisões novas ou divergentes em
   `docs/decisoes/` como ADR.
3. Não mude o contrato público silenciosamente. Atualize OpenAPI, testes e
   changelog na mesma alteração.
4. Não edite migrações já aplicadas; crie uma nova migração.
5. Não registre nome, documento, e-mail, telefone, token, resposta livre ou
   credencial em logs.
6. Não exponha tabelas transacionais ao dashboard público.
7. Não implemente autenticação caseira. A API valida tokens OIDC emitidos por
   provedor configurado.
8. Não adicione Redis, Kafka, Kubernetes ou microsserviços sem evidência de
   necessidade e ADR aprovado.
9. Não acople o domínio ao formato da FNRH. Use a interface do adaptador.
10. Não publique perguntas novas sem classificação de dados e finalidade.
11. Não crie exportação de dados pessoais no MVP.
12. Não considere uma integração concluída sem testes de contrato e tratamento
    de repetição, timeout e indisponibilidade.

## Estrutura esperada do repositório implementado

```text
.
├── apps/
│   ├── api/
│   │   ├── cmd/api/
│   │   ├── cmd/worker/
│   │   ├── internal/
│   │   │   ├── access/
│   │   │   ├── accommodation/
│   │   │   ├── analytics/
│   │   │   ├── audit/
│   │   │   ├── fnrh/
│   │   │   ├── privacy/
│   │   │   ├── questionnaire/
│   │   │   ├── stay/
│   │   │   └── platform/
│   │   └── migrations/
│   └── web/
│       └── src/
│           ├── app/
│           ├── features/
│           ├── pages/
│           ├── shared/
│           └── generated/
├── contracts/
├── deploy/
├── docs/
├── Makefile
└── compose.yaml
```

## Padrões de backend

- Go na última versão patch suportada da linha 1.26.
- `net/http` e roteamento da biblioteca padrão.
- Contexto com timeout em toda chamada ao banco ou rede.
- SQL explícito; queries geradas por `sqlc`. Exceção deliberada: testes de
  integração em `apps/api/internal/platform/store/*_postgres_test.go` usam
  `pgxpool` com SQL inline para inspecionar estado interno (dumps
  `to_jsonb`, varredura de colunas por `information_schema`, identificador de
  tabela/coluna resolvido em runtime) — padrões que não têm forma fixa
  compatível com o modelo estático de arquivos `.sql` do `sqlc`. Não migrar
  arquivo a arquivo; ver nota no topo desses arquivos de teste.
- IDs UUIDv7 gerados no cliente ou servidor.
- Horários em UTC; datas civis da estadia interpretadas em `America/Bahia`.
- Erros HTTP no formato `application/problem+json`.
- `Idempotency-Key` obrigatório para `POST` de efeito material.
- `ETag` e `If-Match` para alterações concorrentes.
- Eventos e jobs gravados na mesma transação do dado de negócio.
- Testes de unidade, integração com PostgreSQL real e contrato.

## Padrões de front-end

- React, TypeScript estrito e Vite.
- Aplicação responsiva, acessível por teclado e compatível com WCAG 2.2 AA.
- Rotas com divisão de código.
- Estado de servidor no TanStack Query; não duplicar em stores globais.
- Formulários com validação compartilhável e mensagens em português.
- Rascunhos offline no IndexedDB, nunca no `localStorage`.
- Service worker somente para shell e fila de rascunhos; não cachear respostas
  autenticadas ou dados pessoais.
- Campos livres exibem aviso contra inclusão de dados sensíveis.
- Código gerado do OpenAPI não deve ser editado à mão.

## Gates obrigatórios

Antes de considerar uma entrega concluída:

```text
go test ./...
go vet ./...
staticcheck ./...
govulncheck ./...
npm run typecheck
npm run lint
npm run test
npm run build
make post-task-quality
```

Também execute testes de migração do zero e de upgrade da versão anterior.

Toda tarefa que altera código e nasce de um `CLAIM` só pode emitir `DONE` ou
handoff depois de `make post-task-quality` concluir com exit code zero. O
artefato do owner deve anexar a linha exata `POST_TASK_QUALITY=PASS`; ausência
do comando, do resultado ou do marcador impede conclusão e o QA deve recusar o
handoff.

## Evidência de conclusão

Ao finalizar uma entrega, informe:

- requisitos atendidos;
- arquivos principais alterados;
- migrações adicionadas;
- testes executados e resultados;
- riscos ou pontos não verificados;
- próximo passo seguro.

## Harness: Cumuru Bootstrap

Para executar, continuar, retomar, reexecutar, atualizar ou auditar uma fase de
`prompts/BOOTSTRAP-CODEX.md`, use o skill repo-scoped `cumuru-bootstrap` em
`.agents/skills/cumuru-bootstrap/SKILL.md`. O agente principal coordena no
máximo três subagentes; estudos podem ocorrer em paralelo, mas somente uma fase
pode estar em implementação.

Antes do primeiro patch, cada implementador deve confirmar no próprio artefato
que leu integralmente `README.md` e `docs/*.md`, além das fontes exigidas pelo
prompt da fase. Sem Git, use um único escritor e não prometa worktrees, commits
ou rollback.

Um status de fase só é válido quando `status.txt` coincide com
`GATE_STATUS=<estado>` em `qa.md`. Antes da Fase 5, valide também o manifesto
sem segredos dos cinco gates externos. Antes de reexecução ampla, preserve a
tentativa atual com `make harness-snapshot PHASE=N`.

**Histórico do harness:**

| Data | Mudança | Alvo | Motivo |
| --- | --- | --- | --- |
| 2026-07-29 | Protocolo e build de remediação | bootstrap e Fase 4 | Resolver débitos locais em ondas reproduzíveis com PostgreSQL, full-stack e navegador real |
| 2026-07-28 | Configuração inicial | `.agents`, `.codex` e compatibilidade `.claude` | Execução faseada com estudo paralelo e gates fail-closed |
