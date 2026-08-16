---
name: cumuru-bootstrap
description: "Orquestra a implementação faseada do Observatório de Cumuruxatiba a partir de prompts/BOOTSTRAP-CODEX.md. Use obrigatoriamente para executar, continuar, retomar, reexecutar, atualizar, corrigir ou auditar uma fase do bootstrap; para 'próxima fase', 'fase N', 'fundação', 'estadias', 'questionário', 'dashboard', 'FNRH', 'auditoria de prontidão do Prompt 6' ou para a Fase 7 de autoatendimento: 'autocadastro', 'QR da pousada', 'link genérico', 'convite reutilizável', 'fila de aprovação' e 'ativação da acomodação'. Não use para simples explicação do blueprint, typo isolado, review de PR, alteração do próprio harness, piloto operacional, deploy ou release."
---

# Cumuru Bootstrap

Coordene o roadmap do Cumuru com estudo paralelo, implementação controlada e QA
incremental. A ordem das fases é uma restrição de segurança, não uma sugestão.

## Superfícies canônicas

- Prompt fonte: `prompts/BOOTSTRAP-CODEX.md`
- Roadmap: `docs/09-roadmap-e-aceite.md`
- Decisão do harness: `docs/decisoes/ADR-011-harness-codex-faseado.md`
- Motor por fase:
  `.agents/skills/cumuru-phase-orchestrator/SKILL.md`
- QA:
  `.agents/skills/cumuru-integration-qa/SKILL.md`
- Matriz detalhada:
  `references/phase-matrix.md`
- Protocolo de execução:
  `references/execution-protocol.md`
- Protocolo de remediação:
  `references/remediation-protocol.md`
- Evals de trigger e dry-run:
  `references/trigger-evals.md`

## Modo de execução

Use fan-out/fan-in para estudo e geração-verificação para implementação. O
agente principal é o supervisor e integrador. Há no máximo três subagentes
ativos, pois a thread principal ocupa o quarto slot.

Não implemente duas fases ao mesmo tempo. Estudos de fases futuras podem ocorrer
em paralelo, mas seus resultados são hipóteses até as dependências anteriores
estarem `PASS`.

## Fase 0: contexto e retomada

1. Leia integralmente `README.md`, `AGENTS.md`, `docs/*.md`,
   `contracts/openapi.yaml`, `database/schema.sql` e
   `prompts/BOOTSTRAP-CODEX.md`.
2. Execute `make harness-validate` e `make harness-status`.
3. Detecte o modo:
   - sem `_workspace/cumuru-bootstrap/`: execução inicial;
   - workspace existente + pedido parcial: reexecute apenas o slice pedido;
   - workspace existente + nova execução ampla: execute
     `make harness-snapshot PHASE=N` antes de criar a nova tentativa. O snapshot
     preserva a evidência anterior e fecha o gate ativo como `UNVERIFIED` até
     novo QA.
4. Detecte Git. Sem Git, registre `SCM=ABSENT`, não prometa worktrees, commits ou
   rollback e use um único escritor.
5. Identifique a fase solicitada. Se não houver, selecione a primeira fase
   dependente ainda não `PASS`.
6. Quando a solicitação for sobre pendências, débitos, correção de uma fase ou
   divergência entre QA e runtime, leia e aplique
   `references/remediation-protocol.md` antes do fan-out.

## Fase 1: preflight da fase

1. Extraia o prompt exato com `make harness-prompt PHASE=N`.
2. Consulte `references/phase-matrix.md`.
3. Classifique todos os pré-requisitos como:
   `PASS`, `FAIL`, `BLOCKED`, `UNVERIFIED` ou `N/A`.
4. Pare fail-closed quando um gate rígido falhar.
5. Se a governança da Fase 0 estiver incompleta, permita Fases 1–4 somente como
   protótipo com dados fictícios e registre `PROTOTYPE_ONLY` em
   `_workspace/cumuru-bootstrap/governance-status.txt`; documente itens
   presentes/ausentes em `governance.md` e encerre com
   `GOVERNANCE_STATUS=PROTOTYPE_ONLY`.
6. Na Fase 5, qualquer gate externo ausente resulta em `BLOCKED`; não crie
   adaptador fictício para simular conclusão.
7. Antes da Fase 5, registre cada gate e sua referência de evidência em
   `_workspace/cumuru-bootstrap/phase-5/external-gates.env`, sem segredos.
8. Fases 2–4 herdam `PROTOTYPE_ONLY`; a Fase 5 exige governança `PASS` e nunca
   é elegível em modo protótipo.

## Fase 2: estudo paralelo

Carregue as definições em `.codex/agents/` e dispare até três subagentes com
`spawn_agent`:

1. `cumuru-phase-analyst`: requisitos, dependências e plano de aceite;
2. `cumuru-contract-reviewer`: OpenAPI, schema, migrações e fronteiras;
3. escolha contextual entre:
   - `cumuru-privacy-reviewer`, para privacidade/segurança;
   - `cumuru-compliance-gatekeeper`, para gates legais/externos;
   - `cumuru-integration-qa`, para baseline e estratégia de testes.

Cada prompt de subagente deve mandar ler sua definição TOML e gravar um resumo
em `_workspace/cumuru-bootstrap/phase-N/study/`. Estudos não editam aplicação.
Subagentes não criam netos; toda nova delegação volta ao supervisor.

Espere todos os resultados. Se um subagente falhar por erro operacional, tente
uma vez. Falha determinística ou gate externo não recebe retry.

## Fase 3: síntese antes da implementação

O supervisor consolida os estudos em:

`_workspace/cumuru-bootstrap/phase-N/plan.md`

O plano deve conter:

- escopo incluído e explicitamente excluído;
- dependências e gates;
- ADRs necessários;
- contrato e migrações previstas;
- ownership sem sobreposição;
- testes primeiro e comandos de gate;
- riscos, blockers e itens não verificáveis.

Registre qualquer decisão nova ou divergente em `docs/decisoes/` antes de
editar aplicação. Não avance enquanto o plano estiver incompleto ou
contraditório.

## Fase 4: implementação

Leia e siga integralmente
`.agents/skills/cumuru-phase-orchestrator/SKILL.md`.

Princípios:

- sem Git: um implementador por vez;
- com Git: no máximo três writers e apenas em ownership disjunto;
- cada writer relê integralmente `README.md` e `docs/*.md` antes do primeiro
  patch e registra isso no próprio artefato;
- cada writer envia `CLAIM phase=N paths=<globs> intent=<resumo>` e espera o
  supervisor confirmar o ownership;
- `contracts/openapi.yaml`, migrações, código gerado, changelog e arquivos raiz
  compartilhados pertencem ao `cumuru-platform-builder`;
- o supervisor congela as decisões do contrato no plano; o platform builder as
  materializa antes de backend e frontend dependerem delas;
- código gerado nunca é editado à mão;
- nenhum agente reverte alterações de outro agente ou do usuário;
- cada slice termina com teste estreito e QA incremental;
- toda tarefa que altera código executa `make post-task-quality` depois dos
  testes estreitos e antes de `DONE` ou handoff;
- o artifact registra comando, exit code e `POST_TASK_QUALITY=PASS`; ausência
  ou falha impede conclusão.

## Fase 5: gate e handoff

Leia e siga integralmente
`.agents/skills/cumuru-integration-qa/SKILL.md`.

Depois do QA:

1. grave `status.txt` com exatamente `PASS`, `FAIL`, `BLOCKED`, `UNVERIFIED` ou
   `N/A`, e encerre `qa.md` com `GATE_STATUS=<mesmo estado>`;
   `PROTOTYPE_ONLY` pertence ao contexto da execução, não ao resultado técnico
   da fase;
2. grave `qa.md` com comandos, exit codes e evidências;
3. recuse qualquer artifact code-changing sem `make post-task-quality` verde e
   sem o marcador exato `POST_TASK_QUALITY=PASS`;
4. informe requisitos atendidos, arquivos principais, migrações, testes,
   riscos, pontos não verificados e próxima fase segura;
5. preserve `_workspace/`;
6. não faça commit, push, PR, deploy ou release sem pedido explícito.

## Dados e comunicação

Use arquivos para artefatos grandes, mensagens para alertas e follow-ups para
correções específicas. Um agente que descobre problema de fronteira deve avisar
os dois owners e o supervisor.

## Erros

| Situação | Resposta |
| --- | --- |
| Gate anterior não `PASS` | `BLOCKED`; não iniciar implementação dependente |
| Git ausente | estudo paralelo; escrita serial |
| Dois writers precisam do mesmo arquivo | serializar e definir owner único |
| Teste falha | `FAIL`; corrigir uma vez e repetir o comando exato |
| Ferramenta/comando ausente | `UNVERIFIED`, nunca `PASS` |
| Fase 5 sem gate externo | `BLOCKED`, sem mock de conclusão |
| Mais da metade dos estudos falha | parar e reportar lacunas |
| Mudança de produto/contrato não prevista | criar ADR e replanejar |

## Cenários de teste

### Fluxo normal

Fase anterior verde → estudo em três lanes → síntese → writers disjuntos ou
serializados → QA incremental → gates completos → `PASS` → próxima fase segura.

### Sem Git

Estudos rodam em paralelo, o dry-run informa `SCM=ABSENT`, a implementação usa
um escritor, e nenhuma alegação de worktree ou rollback é feita.

### Gate externo

Pedido da Fase 5 sem homologação ou autorização → estudo registra a ausência →
status `BLOCKED` → nenhum código de integração é criado.

### Retomada parcial

Workspace existente + pedido para corrigir um gate → leia estudos e QA
anteriores → reexecute somente o owner afetado → QA repete o comando que falhou
e os gates impactados.
