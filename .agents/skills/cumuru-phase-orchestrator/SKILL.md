---
name: cumuru-phase-orchestrator
description: "Motor interno de implementação de uma única fase do Cumuru depois que cumuru-bootstrap concluiu preflight e estudo. Use para executar slices backend, frontend, plataforma e integração com ownership explícito, incremental QA, retry controlado, retomada ou correção parcial. Não use isoladamente para escolher a próxima fase, explicar o blueprint, estudar legislação, alterar o harness, deploy ou release."
---

# Cumuru Phase Orchestrator

Execute exatamente uma fase já selecionada e planejada pelo
`cumuru-bootstrap`.

## Entradas obrigatórias

- número da fase;
- prompt extraído de `prompts/BOOTSTRAP-CODEX.md`;
- `plan.md` da fase;
- estado das dependências;
- situação do SCM;
- ownership de arquivos;
- comandos de gate.

Se qualquer entrada faltar, retorne ao orquestrador principal.

## Wave A: contrato e testes

1. Atribua OpenAPI, migrações, gerados e arquivos compartilhados ao
   `cumuru-platform-builder`.
2. Escreva ou ajuste testes que demonstrem o comportamento do slice.
3. Congele request, response, estados e erros antes de iniciar consumidores.
4. Gere código somente pelo gerador oficial do projeto.
5. Execute o teste estreito e registre o resultado.

## Wave B: implementação por ownership

Escolha apenas os agentes necessários:

| Agente | Ownership primário |
| --- | --- |
| `cumuru-backend-builder` | `apps/api/**`, exceto migrations e gerados |
| `cumuru-frontend-builder` | `apps/web/**`, exceto `src/generated/**` |
| `cumuru-platform-builder` | OpenAPI, migrations, gerados, CI, deploy, compose, observabilidade e raiz delegada |

Sem Git, execute um writer por vez. Com Git, use `spawn_agent` em paralelo apenas
quando nenhum arquivo ou diretório compartilhado estiver no escopo de dois
agentes.

Cada writer:

1. lê sua definição em `.codex/agents/`;
2. lê estudos, plano e mudanças atuais antes de editar;
3. altera somente o ownership atribuído;
4. executa testes estreitos;
5. quando a tarefa altera código, executa `make post-task-quality` na raiz;
6. grava comando, exit code e `POST_TASK_QUALITY=PASS` em
   `_workspace/cumuru-bootstrap/phase-N/implementation/{agent}.md`;
7. somente depois do gate verde emite `DONE` ou handoff ao supervisor;
8. envia ao supervisor contrato alterado, bloqueio ou risco de conflito.

Falha ou ausência de `make post-task-quality` impede `DONE`. O último `echo` ou
outro comando bem-sucedido nunca pode mascarar falha de complexidade ou lint.

## Wave C: integração

O supervisor:

1. relê todos os arquivos compartilhados antes de integrar;
2. resolve sequencialmente mudanças de OpenAPI, migração e geração;
3. compara produtor e consumidor;
4. executa o menor gate integrado;
5. confirma `POST_TASK_QUALITY=PASS` em todo artifact code-changing;
6. chama `cumuru-integration-qa`.

Não use a existência de arquivos como prova de integração.

## Retomada e retry

- Leia artefatos anteriores e preserve o que já passou.
- Reexecute apenas o slice afetado por feedback ou falha.
- Faça um retry somente para falha operacional transitória.
- Falha de teste, contrato, segurança ou gate externo exige correção ou
  `FAIL`/`BLOCKED`, não retry cego.

## Restrições por fase

Leia `../cumuru-bootstrap/references/phase-matrix.md`.

- Fase 1 não implementa domínios posteriores.
- Fase 2 não implementa questionário, dashboard ou FNRH.
- Fase 3 não altera a semântica histórica de respostas.
- Fase 4 só publica a partir de `public_data`.
- Fase 5 não inicia sem todos os gates externos.
- Fase 6 é auditoria; mudanças automáticas devem ser técnicas, localizadas e de
  baixo risco.

## Saída

Retorne ao orquestrador:

- requisitos entregues;
- arquivos e migrations;
- contratos alterados;
- testes por slice;
- conflitos evitados ou resolvidos;
- riscos e itens não verificados;
- recomendação de gate.
