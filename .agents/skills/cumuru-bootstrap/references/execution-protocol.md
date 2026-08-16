# Protocolo de execução Codex

## Primitives

Use as ferramentas realmente disponíveis:

- `spawn_agent`: delegar lane limitada;
- `wait_agent`: esperar atualizações sem polling ruidoso;
- `send_message`: compartilhar descoberta sem iniciar nova rodada;
- `followup_task`: solicitar correção ou continuação;
- `interrupt_agent`: interromper trabalho divergente;
- `list_agents`: auditar lanes ativas.

Não use `TeamCreate`, `TaskCreate` ou `TeamDelete` quando essas ferramentas não
existirem no runtime.

## Limite de concorrência

A configuração do projeto permite três threads filhas. Com o supervisor, isso
totaliza quatro agentes. Prefira:

- três readers em paralelo durante estudo;
- no máximo dois writers e um QA quando houver Git e ownership disjunto;
- um writer por vez quando Git estiver ausente;
- três reviewers em paralelo na auditoria final.

## Prompt de subagente

Todo prompt deve informar:

1. definição do agente em `.codex/agents/{name}.toml`;
2. fase e objetivo;
3. arquivos de entrada;
4. ownership de escrita ou a proibição de editar;
5. caminho de saída em `_workspace/`;
6. dependências e gates;
7. resumo esperado ao supervisor.

Use `fork_turns="all"` quando a thread contém requisitos relevantes. Não fixe
modelo ou reasoning sem solicitação explícita do usuário.

## Estrutura de workspace

```text
_workspace/cumuru-bootstrap/
├── governance.md
├── governance-status.txt
└── phase-N/
    ├── context.md
    ├── study/
    │   ├── requirements.md
    │   ├── contract.md
    │   └── risk.md
    ├── plan.md
    ├── implementation/
    │   └── {agent}.md
    ├── qa.md
    ├── status.txt
    └── attempts/
        └── {timestamp}/
```

Nunca apague tentativas anteriores. Uma retomada cria novo artefato e aponta a
diferença.

## Ownership

Antes de writers, publique uma tabela:

| Owner | Pode escrever | Não pode escrever |
| --- | --- | --- |
| backend | slice definido em `apps/api/**` | OpenAPI, migrations e gerados |
| frontend | slice definido em `apps/web/**` | `src/generated/**` e backend |
| platform | OpenAPI, migrations, gerados, raiz, CI e deploy delegados | domínios de negócio |
| supervisor | plano, ADRs, state ledger e integração | trabalho já delegado |

Qualquer sobreposição serializa as tarefas.

## Mensagens

- Descoberta informativa: `send_message`.
- Mudança necessária no trabalho de agente já concluído: `followup_task`.
- Agente editando fora de ownership: `interrupt_agent`.
- Bloqueio: informe causa, evidência e decisão necessária.
- `DONE` ou handoff de tarefa code-changing: somente depois de
  `make post-task-quality` retornar zero e o artifact anexar
  `POST_TASK_QUALITY=PASS`.
- Ausência do comando, exit code ou marcador: informe `BLOCKER`; não emita
  `DONE`.

## Retry

1. Falha operacional transitória: um retry.
2. Falha de teste ou contrato: corrigir causa, não repetir cegamente.
3. Gate externo: `BLOCKED`, sem retry.
4. Duas falhas da mesma lane: supervisor assume ou conclui com lacuna.

## Encerramento

Espere todas as lanes solicitadas, consolide resultados e preserve
`_workspace/`. Confirme a evidência `POST_TASK_QUALITY=PASS` de todo CLAIM
code-changing antes do fan-in. Não deixe subagente ativo ao devolver a
conclusão.
