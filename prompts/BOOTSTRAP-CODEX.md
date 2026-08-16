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

## Prompt 7 — Autoatendimento e aprovação

```text
Leia AGENTS.md e os documentos do blueprint. Confirme que as Fases 2 e 3 estão
verdes. Implemente somente a Fase 7 de
.agents/skills/cumuru-bootstrap/references/phase-matrix.md.

Esta fase não sucede a Fase 6; ela depende das Fases 2 e 3 e invalida qualquer
auditoria 6A anterior.

Antes de qualquer patch, crie os ADRs:
- supersedência parcial da ADR-019 para convite reutilizável por acomodação;
- autocadastro com aprovação do estabelecimento e proveniência de estadia;
- ativação de conta de acomodação por capability de uso único.

Implemente:

1. Ativação da acomodação.
   O administrador cadastra a acomodação e o sistema emite uma capability de
   ativação de uso único, exibida na própria tela como link e QR gerado no
   navegador. A conta nasce sem senha, em estado pendente, e só se torna
   utilizável quando a acomodação define a senha pela capability. Nenhum envio
   de e-mail pertence a esta fase.

2. Convite reutilizável por acomodação.
   Estenda core.invites com accommodation_id, torne stay_id nulo e acrescente o
   novo purpose. Torne max_uses nulo para representar uso ilimitado; mantenha
   invites_usage_valid válido quando max_uses estiver presente, em vez de
   escolher um teto artificial alto. O convite reutilizável é rotacionável e
   revogável; a rotação invalida o cartaz anterior. Mantenha o armazenamento
   por HMAC e a regra de que token, URL e HMAC nunca entram em log, trace,
   métrica, audit ou outbox.

3. Autocadastro pelo link genérico.
   A submissão cria estadia com proveniência self_service e sem membership
   autora, usando o terceiro valor de collection_channel. A identidade completa
   é cifrada em identity.visitor_identities como no fluxo nominal; os dados
   generalizados continuam em core.visitors. Exiba consentimento e aviso de
   privacidade versionados no formulário aberto, porque não há operador
   intermediando a coleta.

4. Aprovação pelo estabelecimento.
   Fila por acomodação com aprovar e rejeitar, motivo obrigatório na rejeição,
   Idempotency-Key e If-Match. Acrescente uma operação própria de aprovação ao
   mapa allowedOperations de accommodation, disponível somente na acomodação
   ativa; não reaproveite update_stay, porque isso daria aprovação a qualquer
   operador com permissão de edição. Estadia não aprovada nunca entra em
   analytics.presence_days nem em qualquer agregado público. Pendência expira
   automaticamente por prazo configurável e a expiração é auditada.

   A rejeição elimina a identidade cifrada do autocadastro e preserva apenas o
   fato auditável: houve tentativa, foi rejeitada, com motivo e sem valor
   pessoal. Não retenha dado de titular que o próprio estabelecimento declarou
   falso. Prove a eliminação com teste, não apenas o carimbo de rejeição.

5. Abuso.
   Reuse platform.rate_limit_buckets para o convite reutilizável e acrescente
   proof-of-work como segunda prova do formulário aberto: o desafio é emitido e
   verificado pela própria API e resolvido em JavaScript no cliente. Não use
   CAPTCHA nem qualquer serviço de terceiro, porque o formulário público não
   pode depender de rede externa e a ADR-019 já proíbe terceiro na geração do
   QR. O desafio não usa cookie nem qualquer dado do titular. Token e IP bruto
   continuam fora de persistência e log.

Use contracts/openapi.yaml como contrato e atualize-o na mesma mudança.

Não implemente envio de e-mail, FNRH, nem mudança do dashboard público além do
filtro de aprovação. Não acrescente valor ao enum core.stay_status; a espera de
aprovação é proveniência mais carimbo de aprovação, não um novo estado da
máquina de estadia.

A coleta de identidade completa submetida por terceiro sobre titular ausente é
decisão de produto já tomada e registrada. Ela permanece limitada a dados
fictícios enquanto o gate THIRD_PARTY_IDENTITY_BASIS não estiver PASS em
_workspace/cumuru-bootstrap/phase-7/external-gates.env. Não conclua a fase como
apta a dados reais sem esse gate.

Ao terminar, execute todos os gates aplicáveis e apresente PASS/FAIL/UNVERIFIED,
arquivos principais, migrações, riscos e próximo passo.
```
