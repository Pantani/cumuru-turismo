# Prompts de implementação para o Codex

Use um prompt por fase. Não peça todas as fases de uma vez.

## Prompt 1 — Fundação

```text
Leia integralmente README.md, AGENTS.md, docs/*.md,
contracts/openapi.yaml e database/schema.sql.

Implemente somente a Fase 1 descrita em docs/09-roadmap-e-aceite.md.

Crie:
- monorepo com apps/api e apps/web;
- API e worker em Go 1.26.x;
- React 19.2 + TypeScript strict + Vite;
- PostgreSQL local por compose;
- migrações iniciais derivadas de database/schema.sql;
- configuração validada por ambiente;
- health, readiness e build info;
- validação OIDC por interface, com fake somente em desenvolvimento/teste;
- OpenAPI lintado e cliente TypeScript gerado;
- logs JSON sem conteúdo pessoal, métricas e traces;
- CI com testes, lint, govulncheck, build e verificação de migração;
- Makefile e documentação para subir do zero.

Não implemente ainda estadias, questionário, dashboard ou integração FNRH.
Não adicione Redis, Kafka, Kubernetes, Next.js ou framework web Go.
Não altere as decisões arquiteturais sem criar um ADR e explicar a necessidade.

Ao terminar, execute todos os gates aplicáveis e apresente PASS/FAIL/UNVERIFIED,
arquivos principais, riscos e próximo passo.
```

## Prompt 2 — Hospedagens e estadias

```text
Leia AGENTS.md e os documentos do blueprint. Confirme que a Fase 1 está verde.
Implemente somente a Fase 2 de docs/09-roadmap-e-aceite.md.

Priorize:
- isolamento por organização e acomodação;
- Idempotency-Key com hash do request e replay da resposta;
- version/ETag/If-Match;
- máquinas de estado de estadia;
- convite armazenado como HMAC;
- rascunho offline em IndexedDB;
- check-in, check-out, cancelamento e no-show;
- transactional outbox e auditoria sem valores pessoais;
- testes com PostgreSQL real para repetição, concorrência e isolamento.

Use contracts/openapi.yaml como contrato e atualize-o na mesma mudança.
Não implemente questionário, dashboard ou FNRH.
```

## Prompt 3 — Questionários

```text
Implemente somente a Fase 3 do blueprint.

Crie editor, workflow de privacy review, versões imutáveis, tipos de pergunta,
opções, regras condicionais na DSL permitida e submissão separada da estadia.
Impeça publicação de dado sensível e entrada automática de texto livre em
analytics. Consentimentos precisam ser específicos e versionados.

Adicione testes que provem:
- versão publicada é imutável;
- edição cria nova versão;
- recusa não bloqueia check-in;
- respostas antigas mantêm semântica;
- autorização negativa e idempotência.
```

## Prompt 4 — Dashboard e previsão

```text
Implemente somente a Fase 4.

Materialize presença diária de forma idempotente, gere agregados em
public_data, implemente supressão primária e complementar, arredondamento
estável, cobertura e previsão baseline explicável com intervalo.

A API pública deve conectar com papel PostgreSQL que tenha SELECT somente no
schema public_data. Não exponha IDs, estabelecimento, microdados ou texto livre.
O front deve mostrar metodologia, última atualização, observado versus previsto
e cobertura.

Inclua testes de differencing, célula pequena, reconciliação repetida e
autorização do papel público.
```

## Prompt 5 — Integração FNRH

```text
Antes de implementar, confirme:
- autorização formal para o piloto;
- versão vigente da documentação oficial da API FNRH;
- acesso ao ambiente de homologação;
- regra de credencial por estabelecimento;
- mapeamento aprovado dos campos.

Se algum item faltar, pare e marque BLOCKED sem simular integração.

Se os gates estiverem satisfeitos, implemente somente a Fase 5:
- interface de adaptador independente;
- versão vigente isolada em internal/fnrh/adapters;
- segredo cifrado com KMS;
- timeout, retry com jitter, dead letter e reconciliação;
- classificação de erros sem vazar payload;
- testes de contrato e idempotência;
- tela de status por hospedagem.

A criação local nunca deve depender da disponibilidade da FNRH.
```

## Prompt 6 — Auditoria final do piloto

```text
Revise a implementação contra todos os documentos do blueprint sem fazer release.

Entregue uma matriz PASS/FAIL/UNVERIFIED para:
- gates legais;
- minimização e retenção;
- isolamento entre hospedagens;
- idempotência e concorrência;
- criptografia e segredos;
- logs;
- questionários versionados;
- proteção do dashboard;
- backup e restauração;
- incident response;
- acessibilidade;
- FNRH em homologação;
- SLOs e cobertura.

Corrija apenas falhas técnicas claramente dentro do escopo e de baixo risco.
Para lacunas legais, credenciais, infraestrutura externa ou mudança de produto,
pare e solicite decisão.
```
