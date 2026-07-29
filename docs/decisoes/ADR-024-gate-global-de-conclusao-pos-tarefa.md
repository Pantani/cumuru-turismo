# ADR-024 — Gate global de conclusão pós-tarefa

**Status:** aceito.

## Contexto

O ADR-021 limita a complexidade ciclomática e cognitiva a 9 e exige o gate em
cada onda. Entretanto, os contratos de `CLAIM`, `DONE`, handoff, QA e CI não
exigiam uma prova única depois de cada tarefa que alterasse código. Além disso,
o script de complexidade podia perder o status de um analisador e terminar com
sucesso por causa de um comando posterior bem-sucedido.

Um gate final de fase não substitui a prova no ponto em que cada owner entrega
seu slice. Sem essa barreira, dívida ou falha operacional pode atravessar
handoffs e tornar o diagnóstico posterior ambíguo.

## Decisão

`make post-task-quality` é o gate único de conclusão para toda tarefa que:

- altera código próprio ou código gerado;
- nasce de um `CLAIM` de implementação ou correção;
- pretende emitir `DONE` ou handoff a outro owner.

O target executa, nesta ordem e de forma fail-closed:

1. `make complexity`;
2. `make lint`;
3. emissão de `POST_TASK_QUALITY=PASS`.

O marcador só é impresso quando os dois comandos anteriores retornam exit code
zero. O owner registra no artifact o comando, o exit code e a linha exata
`POST_TASK_QUALITY=PASS`. Falha, ferramenta indisponível, saída mascarada,
comando ausente ou marcador ausente impede `DONE` e handoff.

O QA verifica essa evidência em todo artifact de tarefa code-changing e recusa
conclusão sem ela. Mudanças exclusivamente documentais continuam sujeitas aos
testes estreitos aplicáveis, mas não inventam métrica de complexidade.

O target `ci` executa seus gates por chamadas recursivas sequenciais, mesmo
quando o make externo recebe paralelismo. O workflow remoto chama
`post-task-quality`, não combina chamadas independentes de complexidade e lint.

## Complexidade e suppressions

O limite por função permanece 9 para complexidade ciclomática e cognitiva em
todo código próprio Go, JavaScript, JSX, TypeScript e TSX, incluindo variantes
de módulo `.mjs`, `.cjs`, `.mts` e `.cts`, testes, configuração, código gerado
e JavaScript público.

O gate:

- prova explicitamente que valor 9 passa e 10 falha em Go e nas extensões web
  próprias;
- captura status e saída de cada analisador sem depender de `set -e`;
- falha diante de erro operacional, mesmo sem diagnóstico em stdout;
- enumera arquivos web próprios e exclui somente dependências e build;
- usa `--no-ignore` e `--disable-nested-config` quando suportados;
- rejeita `ignorePatterns`, `overrides`, `extends`, directives inline e
  suppressions `nolint` relacionadas a `gocyclo` ou `gocognit`.
- trata `rg=0` como findings, `rg=1` como ausência de findings e qualquer outro
  status como falha operacional.

Código gerado nunca é corrigido à mão. Uma violação gerada deve ser resolvida
no contrato, gerador, template ou desenho produtor.

Shell e SQL não recebem uma métrica de complexidade inventada. O target
`lint-shell`, integrado a `make lint`, executa `bash -n` de forma fail-closed em
todo `.sh` próprio; ShellCheck permanece `UNVERIFIED` enquanto a ferramenta não
estiver disponível. SQL continua coberto por lint, migrations e testes
específicos.

## Consequências

- cada handoff possui prova local e reproduzível de complexidade e lint;
- CI, Makefile, agents Codex/Claude e harness compartilham o mesmo marcador;
- o dry-run expõe o completion gate antes de qualquer escrita;
- o validator falha quando qualquer superfície remove ou desvia o contrato;
- `POST_TASK_QUALITY=PASS` não substitui testes funcionais, integração, scans ou
  gates externos aplicáveis.
