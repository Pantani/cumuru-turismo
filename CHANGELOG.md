# Changelog

Todas as mudanças relevantes do contrato e da plataforma são registradas neste
arquivo.

## 0.5.2 — em desenvolvimento

### Corrigido

- a cadeia pré-lançamento `000001`–`000019` foi consolidada no único par
  `000001_initial_schema`, preservando a ordem efetiva de `up`, a ordem reversa
  de `down`, o schema final, owners, ACLs, comentários e seeds técnicos;
- as 41 operações implementadas agora declaram a resposta transversal
  `500 application/problem+json` já produzida pelo middleware de recuperação
  de panic, por meio de um componente OpenAPI reutilizável;
- a baseline consolidada substitui o acesso direto do worker às tabelas de
  idempotência e rate limit por uma função `SECURITY DEFINER` limitada, que
  preserva registros válidos e toda idempotência em processamento e retorna
  somente contagens agregadas;
- `presence_today` aceita somente presença observada, inclusive quando
  protegida ou indisponível;
- séries `recent_30_days` aceitam somente pontos observados e
  `next_30_days` somente pontos previstos; a baseline consolidada aplica a
  mesma invariante às células publicadas;
- cobertura interna exige `ratio` para o estado `available` e o proíbe para
  `not_available`, preservando a invariante do banco no OpenAPI e no cliente
  gerado;
- o recovery HTTP agora limita a resposta intermediária, descarta status,
  headers e corpo parciais após panic e retorna Problem 500 limpo;
- o rate limit de pesquisas é durável fora do rollback de negócio, preserva
  replay idempotente e evita starvation ao compartilhar o permit por pool;
- shutdown e cleanup do worker são limitados, observáveis e mantêm pool e
  telemetria vivos até a saída real dos pollers;
- o editor de questionários ignora leituras, escritas, transições e erros
  obsoletos após troca de seleção, preserva páginas carregadas em falha de
  paginação e oferece retry explícito.

### Adicionado

- gate `phase4-benchmark` com duas recomposições determinísticas da janela
  fictícia de três anos, digest SHA-256, orçamento de tempo/heap e registro do
  hardware;
- o benchmark depende do full-stack que prova a disponibilidade byte a byte do
  último snapshot válido durante falha de publicação e integra o CI;
- métricas agregadas tipadas de quantidade e idade da outbox, sem payload,
  labels variáveis ou consumo implícito;
- gate `infra-validation` no Makefile e GitHub Actions, com imagens externas
  fixadas por digest, associação exata de `RepoDigest` e fixtures negativas;
- race detector do backend incorporado ao CI completo.

## 0.5.1 — em desenvolvimento

### Alterado

- metodologia pública passa a distinguir a faixa nominal de forecast
  `forecast_bounds_percent = [85,115]` da faixa de fallback
  `forecast_fallback_bounds_percent = [70,130]`;
- a baseline consolidada expõe também a faixa de fallback na view pública de
  metodologia.

## 0.5.0 — em desenvolvimento

### Adicionado

- contrato fechado da Fase 4 para resumo, presença, preferências, metodologia
  pública e qualidade agregada interna;
- trecho da Fase 4 na baseline consolidada para derivação de analytics,
  releases públicas imutáveis, células protegidas e privilégios mínimos;
- queries `sqlc`, cliente OpenAPI gerado e configuração fail-closed para
  política, metodologia, periodicidade, arredondamento e limiares de proteção;
- login PostgreSQL público separado, limitado a quatro views
  `security_barrier`, sem acesso às tabelas-base;
- scaffolds `phase4-integration`, `phase4-proxy-test` e `phase4-full-stack`.

### Segurança

- métricas públicas não contêm identificadores, tamanho de amostra ou motivo de
  supressão;
- preferências aceitam somente respostas estruturadas, consentidas e não
  sensíveis; texto livre criptografado é excluído por grants e predicados;
- publicação usa células já arredondadas e status `published`, `protected` ou
  `unavailable`, com `k >= 10` e ao menos três acomodações;
- `public_runtime` não herda grants futuros no schema `public_data` e a API
  exige `PUBLIC_DATABASE_URL` distinto do DSN transacional;
- domínio, handlers, worker, dashboard, dados reais, deploy e release não são
  declarados entregues pela onda de plataforma.

## 0.4.0 — em desenvolvimento

### Adicionado

- contrato fechado da Fase 3 para catálogo, versões, workflow de privacy
  review, projeção pública por `stable_key` e submissão opcional;
- capability exclusiva no header `Survey-Capability`, emitida opcionalmente
  no sucesso e replay dos dois fluxos de submissão de grupo;
- trecho da Fase 3 na baseline consolidada, queries `sqlc` e cliente OpenAPI
  gerados oficialmente;
- unicidade de capability e resultado por estadia e versão, sem endpoint de
  reemissão;
- configuração fail-closed com keyrings separados, TTL máximo de 24 horas e
  cleanup obrigatório;
- gates `phase3-integration` e `phase3-proxy-test`.

### Segurança

- `active` significa exclusivamente `status=published`, sem agendamento;
- `tourism_profile` é a stable key primária fixa para emissão;
- resposta é group-level e não aceita identificadores de visitante ou
  capability no JSON;
- `declined` usa listas vazias e `submitted` exige decisões exatas por purpose
  e notice;
- texto livre possui somente ciphertext, nonce, key version e `erase_after`
  máximo de 24 horas;
- papéis público e de privacidade não recebem acesso ao schema survey.

## 0.3.0 — em desenvolvimento

### Adicionado

- contrato OpenAPI da Fase 2 para acomodações previamente provisionadas,
  memberships, estadias, grupos, convites, check-in, check-out, cancelamento e
  no-show;
- respostas idempotentes com ETag, `If-Match`, `Location`,
  `Idempotency-Replayed` e `Retry-After` nas condições contratadas;
- resposta fechada `StayMutationResult`, limitada a `id`, `status` e `version`,
  para criação, check-in, check-out, cancelamento e no-show;
- IDs de submissão e visitante restringidos a UUIDv7 hifenizado, incluindo
  versão 7 e variant RFC, com hexadecimal em qualquer capitalização;
- declaração por operação dos erros `400`, `403` (incluindo rejeição CORS) e
  `503` já produzidos pelo runtime da Fase 2;
- trecho da Fase 2 na baseline consolidada para o domínio minimizado de
  estadias, idempotência, outbox, rate limit persistido e privilégios mínimos;
- ciclo PostgreSQL real descartável `zero → 1 → zero → 1`, com preservação das
  fixtures e invariantes finais da fundação;
- gate de complexidade ciclomática e cognitiva por função com limite 9 para
  Go e todo TypeScript/TSX/JavaScript próprio, incluindo testes, configuração e
  cliente gerado;
- configuração local/teste completa da Fase 2, com TTLs, limites, CORS e cinco
  keyrings independentes, além de harness PostgreSQL efêmero que separa
  migrations/fixtures do papel runtime `cumuru_app`;
- dependências locais fixadas para QR e IndexedDB em testes, e estratégia de
  service worker limitada ao shell estático;
- gate full-stack efêmero para API, worker, web, PostgreSQL, timezone, proxy,
  rota autenticada, indisponibilidade e cleanup sem recursos residuais.

### Segurança

- `MembershipCreated` não persiste nem reproduz `oidc_issuer` ou
  `oidc_subject`;
- nome, documento, contato, referência externa e texto livre permanecem fora
  do contrato e das migrations desta fase;
- convites guardam apenas HMAC versionado; audit, outbox, idempotência e rate
  limit usam estruturas minimizadas;
- replays de mutações de estadia persistem e retornam somente a projeção
  `StayMutationResult`, sem duplicar detalhes operacionais;
- `app_runtime` continua sem acesso a `identity`, `public_runtime` continua sem
  acesso ao domínio e auditoria continua append-only;
- Nginx e Vite encaminham toda a superfície `/api/v1`, removem `Referer`, não
  habilitam access log e o smoke autenticado usa somente o fake OIDC local;
- a API deixa de publicar porta no host; o único proxy local confiável é o IP
  estático do web em `/32`, e headers encaminhados pelo cliente são removidos
  ou sobrescritos a partir do socket;
- desenvolvimento nativo separa a confiança explícita nos loopbacks
  `127.0.0.1/32,::1/128` da confiança `/32` do proxy Compose;
- capability de convite e dados estruturados de erro são descartados dos logs
  de proxy, inclusive no canário com upstream indisponível;
- `GET /invites/{token}` declara `400` para headers de proxy ausentes,
  múltiplos ou malformados quando a conexão vem de proxy confiável;
- o service worker ignora API, capabilities de convite em qualquer
  capitalização de query key, requests autenticadas e qualquer método diferente
  de `GET`, não cacheia asset com query e não usa background sync;
- o runtime web fixa `libpng` 1.6.58-r1, removendo o pacote 1.6.57-r0 afetado
  pela CVE-2026-40930 sem alterar a imagem base ou permitir exceção no scanner;
- a Fase 2 permanece `PROTOTYPE_ONLY`; backend de negócio, UI, dados reais,
  deploy, release, dashboard, questionário e FNRH não são declarados entregues
  por esta onda de plataforma.

## 0.2.0 — 2026-07-28

### Adicionado

- contrato-alvo faseado com `x-cumuru-implementation-phase`;
- endpoints da Fase 1 para health, readiness e build info;
- baseline inicial para schemas, tenancy mínima, auditoria e privilégios,
  posteriormente consolidada com as demais fases pré-lançamento;
- principal OIDC lógico identificado pelo par `(issuer, subject)`;
- geração reproduzível do cliente TypeScript e queries `sqlc`;
- Compose local com PostgreSQL 17.10 e roles exclusivamente fictícias;
- bindings locais funcionais no Docker Desktop e proxy same-origin limitado às
  rotas públicas da fundação;
- pipeline de CI, SBOM, scanners, Dockerfiles não-root e gates do Makefile.
- tags determinísticas e scan Trivy HIGH/CRITICAL das imagens runtime
  efetivamente construídas para API/worker e web.
- bases runtime atualizadas para Alpine 3.23.5 e Nginx unprivileged
  1.29.8-alpine3.23 após o novo gate detectar correções OpenSSL disponíveis.
- scanner Trivy fixado por digest e isolado do daemon por exportação de imagem
  para tar somente leitura;
- SBOM CycloneDX para as imagens runtime, acompanhado de manifesto com
  ID/digest, revisão, versão e hash de cada SBOM;
- metadados de build derivados de commit ou hash da fonte e
  `SOURCE_DATE_EPOCH`, sem fallback silencioso para `unknown`;
- respostas `429` removidas de health/readiness enquanto não há controle de
  taxa contratado e implementado para essa superfície.
- suíte de migrações sem porta publicada, permitindo validação isolada mesmo
  quando a stack local principal já ocupa a porta PostgreSQL.
- materialização explícita e verificação de `RepoDigests` dos scanners
  digest-pinned antes de qualquer inspeção, cobrindo runners sem cache local.

### Segurança

- respostas operacionais sem cache e com request ID;
- papel público restrito a `public_data`;
- eventos de auditoria sem permissão de `UPDATE` ou `DELETE` para runtimes;
- dados reais, staging, produção, deploy, release e FNRH continuam bloqueados.
- testes e configuração Vite entram no typecheck estrito, sem pular checagem de
  tipos das bibliotecas.

## 0.1.0 — 2026-07-27

- blueprint inicial do Observatório Turístico de Cumuruxatiba.
