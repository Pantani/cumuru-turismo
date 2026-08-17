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
| 2026-08-16 | Convenção shfmt declarada | `.editorconfig` e scripts do harness | `shfmt` passou a existir no ambiente e assumia tabs contra os 2 espaços do repositório; `SHFMT` saiu de `UNVERIFIED` para `PASS` sem suprimir regra |
| 2026-08-16 | Fase 7 reescrita após o estudo | `prompts/BOOTSTRAP-CODEX.md`, `phase-matrix.md`, `harness.sh`, `test-harness.sh`, ADR-039/040/041 | O estudo provou que `identity.visitor_identities` não existe na cadeia executável (ADR-020 a vetou); o canal aberto passa a coletar só dado generalizado e a identidade vem depois pelo convite nominal da Fase 2. Gate renomeado para `SELF_SERVICE_LEGAL_BASIS`, porque o nome anterior descrevia algo que o desenho não faz mais |
| 2026-08-16 | Wave Fase 7 de autoatendimento e aprovação | `prompts/BOOTSTRAP-CODEX.md`, `phase-matrix.md`, `harness.sh`, `test-harness.sh`, `trigger-evals.md`, `Makefile`, `deploy/scripts/test-phase7-*.sh` | Cadastrar acomodação com ativação por capability, convite reutilizável por acomodação, autocadastro e aprovação do estabelecimento |
| 2026-08-16 | Correção do gate ShellCheck | `.agents/skills/cumuru-bootstrap/scripts/harness.sh` e `test-harness.sh` | ShellCheck 0.11 passou a acusar SC1007/SC2016 e quebrou `make harness-validate`; `CDPATH=''` e `$'...'` restauram o gate sem suprimir regra |
| 2026-07-29 | Protocolo e build de remediação | bootstrap e Fase 4 | Transformar débitos/runtime divergente em ondas reproduzíveis com PostgreSQL, full-stack e navegador real |
| 2026-07-28 | Gate global pós-tarefa | harness, agents e CI | Impedir handoff com complexidade/lint ausentes ou mascarados |
| 2026-07-28 | Configuração inicial | harness completo | Execução faseada no Codex com compatibilidade Claude |
