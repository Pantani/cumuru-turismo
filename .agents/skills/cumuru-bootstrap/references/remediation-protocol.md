# Protocolo de remediação de fase

Use este protocolo quando uma fase previamente implementada volta a
`FAIL|UNVERIFIED`, quando o usuário pede débitos técnicos/pendências ou quando
um runtime real contradiz o QA preservado.

## Entrada

1. Preserve a tentativa anterior somente se a retomada for ampla. Em slice
   ativo com `CLAIM`, continue o mesmo artifact.
2. Congele uma fotografia: Git/SCM, dirty state, `status.txt`, `qa.md`, plano,
   processos, banco e endpoints relevantes.
3. Classifique cada item como:
   - técnico local e corrigível;
   - externo e verificável;
   - externo sem autoridade/evidência.
4. Itens externos nunca são simulados para tornar o gate verde.

## Fan-out

Use no máximo três lanes read-only:

1. requisito/aceite e reprodução;
2. contrato e fronteiras produtor-consumidor;
3. privacidade, segurança e runtime.

Cada lane escreve um relatório `remediation-<lane>-<date>.md`, marca
`PASS|FAIL|BLOCKED|UNVERIFIED` e não edita aplicação.

## Síntese e execução

1. Converta achados em ondas serializadas, começando pelo primeiro bloqueio
   observável.
2. Registre divergências no ADR antes do patch correspondente.
3. Mantenha um único writer quando arquivos raiz, gerados, Compose e frontend
   se cruzarem.
4. Para cada correção, exija teste estreito que reproduza a falha, depois o
   gate de integração que a revelou.
5. Runtime demonstrável exige, no mínimo:
   - banco novo e persistente;
   - repetição e rollover;
   - colisão fail-closed e preservação de canário;
   - build padrão sem authorities locais;
   - smoke HTTP com shapes e privacidade;
   - navegador real sem mocks;
   - cleanup de containers, redes e volumes efêmeros.

## Build de remediação da Fase 4

Execute `make phase4-remediation`. O target agrega guardas de build, fixture
PostgreSQL, full-stack com fonte única e E2E Chromium isolado. Instale o browser
fixado uma vez com `npx playwright install chromium`.

O target não substitui `make post-task-quality`, gates globais, QA independente
ou evidência externa. Um `CLAIM` só encerra depois do marcador exato
`POST_TASK_QUALITY=PASS`.

## Saída

Atualize `implementation/<slice>.md`, `qa.md` e `status.txt` somente depois dos
comandos finais. Liste separadamente:

- resolvido e provado localmente;
- dívida remanescente;
- externo `BLOCKED`;
- próxima fase elegível.
