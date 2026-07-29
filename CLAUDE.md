## Harness: Cumuru Bootstrap

**Objetivo:** executar `prompts/BOOTSTRAP-CODEX.md` por fase, com estudo
paralelo, implementação controlada e QA incremental.

**Trigger:** para executar, continuar, retomar, reexecutar, atualizar ou auditar
uma fase do Cumuru, use o skill `cumuru-bootstrap`. Perguntas conceituais simples
podem ser respondidas diretamente.

**Completion gate:** toda tarefa que altera código e possui `CLAIM` só pode
emitir `DONE` ou handoff depois de `make post-task-quality` concluir com exit
code zero e o artifact anexar `POST_TASK_QUALITY=PASS`. O QA recusa ausência do
comando, resultado ou marcador.

**Histórico de mudanças:**

| Data | Mudança | Alvo | Motivo |
| --- | --- | --- | --- |
| 2026-07-28 | Gate global pós-tarefa | harness, agents e CI | Impedir handoff com complexidade/lint ausentes ou mascarados |
| 2026-07-28 | Configuração inicial | harness completo | Execução faseada no Codex com compatibilidade Claude |
