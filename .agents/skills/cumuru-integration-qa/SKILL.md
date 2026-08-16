---
name: cumuru-integration-qa
description: "QA incremental e gate final das fases do Cumuru. Use para validar implementação contra blueprint, OpenAPI, migrações, backend, cliente gerado, React, autorização, privacidade, idempotência, analytics, FNRH e gates globais; para revalidar uma correção, retomar QA ou produzir PASS/FAIL/BLOCKED/UNVERIFIED. Não use para review genérico de PR, simples lint isolado, alterar o harness, deploy ou release."
---

# Cumuru Integration QA

Valide comportamento e coerência entre fronteiras. Build verde isolado não
prova que produtor e consumidor concordam.

## Estados

- `PASS`: evidência reproduzível prova o requisito.
- `FAIL`: teste executado falhou por defeito técnico.
- `BLOCKED`: pré-requisito anterior ou externo impede a prova.
- `UNVERIFIED`: a prova não pôde ser executada.
- `N/A`: item não pertence à fase.

Gates dependentes aceitam somente `PASS`, salvo regra explícita de protótipo.

## QA incremental

Depois de cada slice, leia simultaneamente os dois lados:

| Fronteira | Compare |
| --- | --- |
| Migração → sqlc | colunas, enums, constraints, nulabilidade e tenant |
| sqlc → serviço Go | tipos, transação, timeout e autorização |
| Serviço → HTTP/OpenAPI | status, headers, Problem Details e `snake_case` |
| OpenAPI → cliente gerado | operation IDs e request/response reais |
| Cliente → Query/form | shape, unwrap, null, invalidação e estados assíncronos |
| Estado → handlers/UI | transições permitidas e alcançáveis |
| Escrita → idempotência/outbox/auditoria | atomicidade e replay |
| Pessoal → logs/analytics | ausência de identidade, segredo e texto livre |
| Analytics → public_data → API | somente agregado protegido |
| Datas → presença | UTC, `America/Bahia` e `[entrada, saída)` |
| Migração anterior → nova | upgrade sem editar migração aplicada |

Reporte achados com produtor, consumidor, arquivo/linha, severidade, evidência e
owner. Envie a falha aos dois owners quando cruzar uma fronteira.

## Gates globais

Execute quando existirem e forem aplicáveis:

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

Também valide:

- migração do zero e upgrade da versão anterior;
- PostgreSQL real para integração;
- lint do OpenAPI;
- regeneração reproduzível do cliente;
- autorização negativa;
- repetição e concorrência;
- logs sem campos proibidos;
- acessibilidade automatizada e teclado;
- scanner de contêiner quando houver imagem.

Comando inexistente ou não executável é `UNVERIFIED`.

Para cada CLAIM que alterou código, leia o artifact do owner e exija:

- execução de `make post-task-quality` depois dos testes estreitos;
- exit code zero;
- linha exata `POST_TASK_QUALITY=PASS`.

Qualquer ausência, falha ou marcador escrito sem o comando correspondente
invalida `DONE`/handoff e resulta em `FAIL` ou `UNVERIFIED`, conforme a
evidência. QA não reconstrói o marcador retroativamente.

## Evidência

Grave `_workspace/cumuru-bootstrap/phase-N/qa.md` com estes headings exatos:

1. `## Requirements`: matriz de requisitos e boundary checks;
2. `## Commands`: comandos, exit codes, migrations e resumo observado;
3. `## Risks`: segurança, privacidade, blockers e itens não verificados.

Encerre o arquivo com exatamente `GATE_STATUS=PASS`, `GATE_STATUS=FAIL`,
`GATE_STATUS=BLOCKED`, `GATE_STATUS=UNVERIFIED` ou `GATE_STATUS=N/A`. O
`status.txt` deve conter o mesmo valor; o helper rejeita divergência.

Não registre payload pessoal, token, credencial, contato ou texto livre.

## Correção

QA não corrige silenciosamente. Primeiro informe o owner com evidência. Só
edite quando o supervisor atribuir uma correção técnica, localizada e de baixo
risco; depois repita o teste exato e os gates impactados.
