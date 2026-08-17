# Changelog

Todas as mudanças relevantes do contrato e da plataforma são registradas neste
arquivo.

## 0.7.0 — em desenvolvimento

### Adicionado

- autenticação local por e-mail e senha com Argon2id, sessão opaca, bloqueio
  por tentativas e `POST /auth/login`, `POST /auth/logout` e `GET /auth/session`
  no contrato, permitindo entrar sem CNPJ, Cadastur ou chave federal
  ([ADR-037](docs/decisoes/ADR-037-autenticacao-local-por-email-e-senha.md));
- schema `auth` com `accounts` e `sessions`, grants restritos a `app_runtime` e
  `password_hash` fora de worker, público e privacy officer;
- tela de login e área da hospedagem orientada a tarefa: lista de hospedagens
  selecionável, criação de estadia por datas e número de pessoas, e ações de
  ciclo de vida derivadas do estado da estadia;
- gráfico de presença em SVG no painel público, com a série dia a dia mantida
  como alternativa acessível;
- sistema visual em tokens de cor, tipografia, superfície e movimento como
  fonte única do layout, com tema escuro automático por
  `prefers-color-scheme`;
- Geist e Geist Mono empacotadas no próprio bundle, servidas na mesma origem e
  compatíveis com a CSP `default-src 'self'`; apenas o subconjunto latino é
  baixado;
- grade, eixo de valores e rótulos de data no gráfico de presença, com escala
  arredondada para intervalos legíveis, e barra proporcional por categoria nas
  preferências agregadas;
- leitura dia a dia no gráfico de presença por ponteiro e por teclado (setas,
  `Home`, `End` e `Esc`), com tooltip e leitura viva equivalentes, faixa de fim
  de semana, linha de média da janela e rótulos de data intermediários;
- seis indicadores derivados da janela exibida — média diária, dia mais cheio,
  dia mais vazio, tendência entre metades iguais, total acumulado e dias
  publicados —, cada um com a dica que explica o que entra na conta; dias
  protegidos são pulados, nunca contados como zero;
- coluna "ante a média" na série dia a dia e dica de interpretação nos cartões
  de presença de hoje e de pico previsto;
- janela combinada de 60 dias no painel público, unindo observado e previsto em
  uma leitura só, com a divisa "previsão a partir daqui"; os indicadores e todas
  as comparações continuam medidos contra o nível observado;
- média móvel de 7 dias sobre a série, que só aparece com a janela completa e
  com metade dos dias publicados, e interrompe onde faltam valores;
- bloco "ritmo da semana" com a média por dia da semana da janela observada;
- `make compose-config`, `make prod-config-example`, `make smoke-local`,
  `make phase7-integration` e `make phase7-full-stack` no gate `make ci`,
  cobrindo os overlays do Compose, os loaders reais de produção sobre
  `deploy/runtime.env.example`, o smoke da stack local e as duas provas da
  Fase 7, que existiam como targets sem nenhuma execução automatizada;
- `make smoke-local` e `make prod-config-example` como targets públicos:
  `smoke-local` sobe a stack, executa o smoke e derruba a stack mesmo quando o
  smoke falha, propagando o primeiro exit code diferente de zero.

### Alterado

- a CI deixou de ser um job sequencial único e passa a executar catorze jobs
  paralelos com uma porta única `ci` exigida pela proteção de branch; cada job
  instala apenas a cadeia de ferramentas que usa através da action composta
  `.github/actions/setup`, e as ferramentas Go fixadas são restauradas de cache
  em vez de recompiladas a cada execução;

- a cadeia pré-lançamento `000002`–`000004` foi absorvida pelo par
  `000001_initial_schema`, preservando a ordem efetiva de `up` e a ordem reversa
  de `down`; `make migration-test` passa a exigir exatamente esse par e a
  exercitar `zero → 1 → zero → 1`
  ([ADR-032](docs/decisoes/ADR-032-consolidacao-pre-lancamento-da-baseline-de-migrations.md));
- limite de complexidade passa de `9` único para ciclomática `5` e cognitiva
  `8` no código de aplicação, gerado e no web inteiro; o Go de teste permanece
  em `9`
  ([ADR-036](docs/decisoes/ADR-036-limite-de-complexidade-cinco-e-oito.md));
- a área autenticada deixou de expor identificadores internos: ETag e
  `Idempotency-Key` passam a ser derivados da versão já listada em vez de
  digitados;
- densidade e tipografia do layout revistas; rótulos de fase interna removidos
  das telas voltadas ao público e ao operador;
- a paleta deixa de ser papel bege com serifada e passa a codificar significado:
  presença observada em teal, previsão em coral e dado protegido em neutro, com
  a mesma correspondência no gráfico, nos cartões e nos selos de estado;
- a jornada em Chromium passa a entrar por e-mail e senha e a operar pela lista
  de hospedagens e pelas ações do próprio cartão de estadia, sem identificadores
  digitados.

### Corrigido

- `make scan` acusava `nanoid` em `3.3.16`, sob
  [GHSA-2v37-7h3g-55p8](https://github.com/advisories/GHSA-2v37-7h3g-55p8). O
  pacote chega por `vite → postcss` e não tinha override; passa a `3.3.18`,
  dentro da mesma major e sem alterar a superfície de `postcss`;
- navegação para `/registro` depois de "Abrir registro neste navegador": a
  guarda de rota comparava com a URL que `captureInviteCapability` já havia
  reescrito, o que anulava o clique e deixava o fluxo do convite sem saída;
- rótulos de categoria exibidos em código (`family_hosting`) na tela de
  qualidade agregada;
- botão de remover visitante ocupando uma célula inteira do formulário de
  registro, porque a distribuição em colunas estava no `fieldset` em vez do
  grupo de campos aninhado.

### Removido

- injeção de identidade em tempo de build (`VITE_LOCAL_DEMO_MODE` e
  `VITE_LOCAL_DEMO_IDENTITY`), com o guarda `local-demo-build` e o alvo
  `local-demo-build-test` correspondentes: o bundle não carrega mais credencial.

## 0.6.0

### Adicionado

- onboarding local de acomodações para pousadas formais, casas de temporada,
  hospedagens familiares, campings e locais em regularização, sem exigir CPF,
  CNPJ, Cadastur ou chave FNRH;
- `POST /accommodations` com escopo OIDC específico, `Idempotency-Key`,
  `client_submission_id`, categorias fechadas e criação atômica da organização,
  acomodação e vínculo gestor quando necessário;
- feature flag fail-closed para limitar o onboarding ao protótipo local com
  provedor OIDC fictício;
- guias na raiz para apresentar o Observatório à Prefeitura e orientar a
  obtenção da chave oficial FNRH pelas hospedagens elegíveis.

### Alterado

- cadastro e edição local deixam de tratar Cadastur como pré-requisito ou campo
  editável; quando previamente provisionado, ele continua apenas informativo;
- `PATCH /accommodations/{accommodation_id}` agora restringe `category` aos
  códigos de `AccommodationInputCategory` e remove `cadastur_id` do request;
  clientes existentes devem migrar categorias livres para o enum publicado e
  deixar de enviar Cadastur em alterações genéricas;
- o fluxo e a interface passam a separar explicitamente participação local de
  integração FNRH, que permanece bloqueada pelos gates externos da Fase 5 e
  exige credencial oficial própria de cada estabelecimento.

### Corrigido

- o contrato de residência exige país e UF em maiúsculas, UF e município IBGE
  para o Brasil e ausência desses campos para outros países, em paridade com o
  domínio Go e a validação React;
- o rollback do onboarding falha fechado diante de fixtures reservadas
  parciais ou divergentes, sem remover a coluna antes de restaurar os dados;
- a CI executa o gate canônico `make ci`, instala Chromium explicitamente e o
  Trivy grava artefatos com a identidade numérica do runner; a suíte de
  migrations serializa o uso da subnet Docker fixa e recupera lock órfão;
- o protótipo local distingue falha de banco de conflito de fixture, não inclui
  conteúdo persistido em mensagens de teste e preserva o QR diante de URL
  malformada.

### Segurança

- o onboarding rejeita identificadores de organização, CPF, CNPJ, Cadastur,
  contato, senha gov.br, chave FNRH e qualquer texto livre fora do contrato;
- credenciais do OIDC fictício são aceitas pela API somente quando o endereço
  de cliente resolvido é loopback; cabeçalhos de proxy não confiáveis são
  ignorados e valores confiáveis divergentes falham fechados;
- criação, replay idempotente, auditoria e outbox são persistidos na mesma
  transação, sem registrar credenciais ou dados pessoais em logs.

## 0.5.2 — em desenvolvimento

### Corrigido

- o runtime local agora aplica fixtures fictícias idempotentes pelo comando Go
  `/app/local-demo` e pelos serviços de domínio, sem carga SQL direta; espera a
  primeira publicação e deixa de aprovar dashboard `503` ou lista vazia no
  smoke;
- um gate PostgreSQL descartável prova seed em banco novo, repetição sem
  duplicação, UUIDv7 e preservação de linhas fora do namespace da fixture;
- o frontend local inicia o principal do fake OIDC apenas na variante explícita
  servida em loopback, mantém o estado da sessão em memória, exibe
  `PROTOTYPE_ONLY`, limpa cache e authorities no logout e conecta acomodação,
  estadia, convite, registro e pesquisa sem copiar UUID/ETag entre formulários;
- o seed local reconcilia presença antes de publicar analytics, mantém cohorts
  históricas estáveis dentro do mês, reconcilia as estadias correntes e falha
  fechado em qualquer colisão de ID reservado, sem atualizar dados existentes;
  um advisory lock de sessão serializa toda a execução concorrente;
- o consumo do convite preserva a confirmação da submissão enquanto a
  capability de pesquisa existe somente em memória, evitando desmontagem
  prematura da jornada pela atualização da navegação contextual;
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

- build reproduzível `phase4-remediation`, com guardas da variante web,
  PostgreSQL fresh/persistente, rollover, colisão fail-closed, full-stack e
  jornada Playwright em Chromium com Service Worker/Cache API e cleanup
  verificáveis;
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
