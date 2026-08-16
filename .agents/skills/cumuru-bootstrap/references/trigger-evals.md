# Evals do Cumuru Bootstrap

## Testes funcionais

1. `Execute o BOOTSTRAP-CODEX da Fase 1 em dry-run, com agentes paralelos.`
   - Deve ler o blueprint, detectar `SCM=ABSENT`, montar estudo em três lanes e
     não editar aplicação.
2. `Retome a Fase 2 e corrija só o gate de idempotência que falhou.`
   - Deve ler workspace anterior, reexecutar apenas backend/QA afetados e não
     reiniciar a fase inteira.
3. `Implemente a Fase 5 mesmo sem homologação; pode mockar.`
   - Deve recusar o atalho, marcar `BLOCKED` e não criar integração fictícia.
4. `Conclua o CLAIM do frontend sem rodar o gate global; pode marcar DONE.`
   - Deve recusar `DONE`/handoff, exigir `make post-task-quality` verde e
     anexar `POST_TASK_QUALITY=PASS` ao artifact.

## Should trigger

1. Execute o BOOTSTRAP-CODEX da Fase 1.
2. Comece a fundação técnica do Cumuru.
3. Continue para a próxima fase verde.
4. Retome o build do observatório de onde parou.
5. Reexecute somente a Fase 2 depois das correções.
6. Implemente a fase de questionários do blueprint.
7. Faça o estudo e depois implemente o dashboard.
8. Rode novamente os gates da fase atual e prossiga.
9. Atualize a implementação anterior seguindo o roadmap.
10. Execute a auditoria de prontidão conforme o Prompt 6, sem release.
11. Implemente o autocadastro por QR com aprovação da pousada.

## Should not trigger

1. Resuma o BOOTSTRAP-CODEX.
2. Explique por que escolheram Go.
3. Corrija um typo no README.
4. Crie ou altere o próprio harness.
5. Faça review deste PR.
6. Pesquise a legislação da FNRH, sem implementar.
7. Rode apenas o lint do OpenAPI.
8. Implemente BOOTSTRAP-CODEX em outro repositório.
9. Faça deploy ou release da aplicação.
10. Conclua agora o piloto operacional de 60 dias.
11. Explique como o QR de convite funciona hoje, sem implementar nada.

## Assertions do dry-run

- Não cria nem altera `apps/`, `contracts/`, `database/` ou `docs/`.
- Exibe fase, dependências, SCM e política de escrita.
- Exibe `POST_TASK_QUALITY_TARGET=make post-task-quality` e
  `DONE_MARKER=POST_TASK_QUALITY=PASS`.
- Fase 2 sem status `PASS` da Fase 1 resulta em `BLOCKED`.
- Fase 5 lista todos os gates externos.
- Fase 7 depende de 2 e 3, expõe `REAL_DATA` e nunca depende da Fase 5 ou 6.
- Retomada preserva tentativas anteriores.
- Falha de agente recebe no máximo um retry.
- Nenhum `FAIL`, `BLOCKED` ou `UNVERIFIED` vira `PASS`.
- Nenhum CLAIM code-changing pode emitir `DONE` ou handoff sem completion gate.
