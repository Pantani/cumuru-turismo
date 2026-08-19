# Changelog

Todas as mudanças relevantes do contrato e da plataforma são registradas neste
arquivo.

## 0.7.0 — em desenvolvimento

### Adicionado

- material de campo em `docs/guias/`: o guia da hospedagem (onde colar o cartaz,
  o que escrever ao lado do QR, a frase do anfitrião na chegada e na saída, as
  respostas para as perguntas do hóspede) e o guia do piloto de 30 dias (dois
  grupos comparados, calendário semana a semana e as três conclusões possíveis).
  Ambos em linguagem para quem não é da área e ambos abrindo com o aviso de que
  os gates legais precedem o primeiro hóspede real;
- guia da administração para o dia a dia em `docs/guias/`: decidir a fila de
  pedidos com critério escrito e os quatro motivos de recusa, cadastrar
  hospedagem à mão, entregar o acesso de uso único, ler a qualidade e o funil, e
  registrar pedido de titular. Este último documenta uma lacuna em vez de
  escondê-la: não existe tela para direito de titular, então o caderno de
  registro é o controle e será pedido em auditoria;
- funil de adesão dos últimos 30 dias em `GET /analytics/funnel`, escopo
  `analytics:read:internal`, exibido no painel `/qualidade`: convite emitido,
  usado e expirado; pesquisa emitida, respondida, recusada e vencida;
  autocadastro iniciado, pendente e decidido. Não abre canal de telemetria e
  não coleta nada novo — conta estados que o registro já guarda. A mediana de
  latência só aparece a partir de dez submissões na janela, porque abaixo disso
  ela descreve uma pessoa e não um comportamento. Na pesquisa, concluída é a
  capability consumida: resposta e recusa explícita contam juntas, porque
  `app_runtime` grava em `survey.responses` e não lê de volta — privilégio
  deliberado, não lacuna a corrigir com GRANT;
- [ADR-045](docs/decisoes/ADR-045-retorno-do-dado-a-hospedagem-e-funil-de-adesao.md)
  registrando as três decisões de privacidade desta entrega: limiar próprio do
  comparativo, recusa de publicar ocupação como célula pública e funil que conta
  estados em vez de instrumentar pessoas;
- comparativo da própria hospedagem em
  `GET /accommodations/{accommodation_id}/performance`: a série observada da
  acomodação, exata porque é dado dela, ao lado do índice da publicação
  corrente. Nenhuma família de célula nova é publicada — o lado da vila é o
  mesmo documento da capa, com a mesma supressão — e o valor absoluto da vila
  fica de fora do payload, porque quem conhece o próprio número é o único que
  consegue calcular `outros = total - meu`. O comparativo fecha, mantendo só o
  lado próprio, quando menos de cinco hospedagens reportaram na janela ou
  quando a capacidade própria passa de um quarto do denominador reportante;
- taxa de ocupação da janela no mesmo comparativo: a própria exata, a da vila
  em banda de cinco pontos e só com o comparativo aberto. Ela **não** vira
  célula publicada: presença já sai em múltiplos de 10 e a cobertura em
  múltiplos de 5, então uma ocupação pública tornaria a capacidade da vila
  inferível. O arredondamento impede determinar o valor exato — capacidades
  vizinhas produzem publicações idênticas —, mas 730 dias estreitam o intervalo
  a uma faixa de poucos leitos, e a diferença entre duas publicações estreita do
  mesmo modo a capacidade do estabelecimento recém-admitido. Atributo de
  estabelecimento identificável, ainda que por faixa, é dado individualizado
  (Portaria MTur nº 41/2025). Um número por janela, para um leitor identificado,
  é outra escala de exposição;
- lista pública de hospedagens em `/hospedagens` e
  `GET /api/v1/public/accommodations`, com nome, categoria, localidade,
  capacidade, telefone em E.164, WhatsApp e site das hospedagens ativas que
  consentiram em publicar; documento único, ordenado por nome, sem cursor,
  `max-age=300` e ETag forte
  ([ADR-043](docs/decisoes/ADR-043-lista-publica-de-hospedagens.md));
- consentimento de publicação em `core.accommodations`
  (`public_listing_enabled`, `public_contact_phone`, `public_contact_whatsapp`,
  `public_website_url`, `public_listing_consented_at`), exposto no recurso da
  acomodação como `public_listing` e editável por
  `PATCH /accommodations/{id}`; publicar sem telefone é recusado, e despublicar
  elimina o carimbo de consentimento;
- painel "Aparecer na lista pública" na área da hospedagem, que publica e
  retira o contato sob a mesma versão otimista das demais edições;
- importação do calendário do Booking.com por iCal: a hospedagem cadastra o
  endereço `.ics` do anúncio, o worker lê a cada duas horas e as datas entram
  como reserva importada. `POST /accommodations/{accommodation_id}/calendar-feeds`,
  `GET` da mesma rota, `POST /calendar-feeds/{feed_id}/remove`,
  `GET /accommodations/{accommodation_id}/calendar-reservations`,
  `POST /calendar-reservations/{reservation_id}/confirm` e `/dismiss` no
  contrato, atrás de `CALENDAR_FEED_ENABLED`
  ([ADR-044](docs/decisoes/ADR-044-importacao-de-calendario-da-plataforma-de-hospedagem.md));
- `core.calendar_feeds` e `core.calendar_reservations` com a URL do feed cifrada
  em repouso por AES-GCM com keyring versionado, o identificador da reserva na
  origem guardado sob HMAC e nenhuma coluna de identidade — a estadia só nasce
  por confirmação humana informando o número de hóspedes, que o arquivo não
  traz;
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

- **a hospedagem fictícia do `local-demo` deixou de administrar o catálogo de
  questionários.** A conta de operação carregava `questionnaires:manage` e
  `questionnaires:approve`, então `/questionários` aparecia no menu de quem
  apenas opera uma hospedagem. O catálogo é um instrumento único da plataforma:
  nem o serviço nem o repositório de questionário cruzam `core.memberships`, de
  modo que os escopos editam, aprovam, publicam e aposentam a versão que todas as
  hospedagens respondem. Desenhar o instrumento é ato da administração; a
  hospedagem responde. A publicação da fixture continua intacta, porque usa
  principals próprios (`fixture-questionnaire-editor` e
  `-reviewer`). Sem migration e sem escopo novo; banco local já semeado mantém
  os escopos antigos na linha da conta, então rode `make docker-rm` antes do
  próximo `local-demo`.
- **o administrador deixou de ver "Suas hospedagens" e ganhou área própria.** A
  mesma tela servia às duas autoridades: quem entrava com a conta semeada de
  administração via a lista de hospedagens como se fossem suas, com quadro de
  estadias, cartaz de autocadastro e fila de hóspedes. `/acesso` passa a escolher
  a área por `accommodations:onboard`, que é o que a API tem no lugar de um papel:
  a administração cadastra hospedagem, entrega o acesso de cada uma e decide os
  pedidos vindos do site; a hospedagem registra estadia e decide o cadastro do
  hóspede que chegou pelo código dela. O seed também deixou de conceder
  `stays:write` ao administrador — registrar estadia é ato do estabelecimento —,
  e o vínculo do administrador com o catálogo semeado continua, porque entregar
  o acesso lê a linha da hospedagem e escreve a ativação. Sem migration e sem
  escopo novo.
- **as fixtures do perfil `test` passaram a povoar dois anos de operação, não
  oito semanas de estadias de uma noite.** O catálogo fictício foi de 4 para 16
  hospedagens — pelo menos três em cada categoria publicada, porque a cobertura
  só reporta uma categoria que alcança o mínimo de estabelecimentos —, e o
  calendário passou a cobrir os 730 dias da janela publicada com estadias de
  vários dias, alta e baixa temporada, grupos dimensionados pela capacidade,
  hóspedes com faixas etárias e residências variadas (inclusive fora do Brasil)
  e respostas de pesquisa que só parte dos grupos responde. Uma stack semeada
  abre com ~1400 estadias, ~8800 hóspedes e ~950 respostas, e todas as janelas
  da capa — 30, 90, 365 e 730 dias, mês civil, previsão e preferências — têm
  série publicada em vez de buraco. O calendário é ancorado no calendário
  civil, não na data da execução, então semear de novo amanhã mantém o que já
  existe e só acrescenta o dia que passou;

- **a reconciliação de analytics ganhou orçamento de lote onde era de
  requisição.** `DATABASE_TIMEOUT` dimensiona uma requisição, e uma
  reconciliação percorre todas as estadias da história publicada: com dois anos
  de dados o worker passava a falhar todo ciclo no relógio, não em problema
  real. O repositório de analytics do worker passa a usar um store próprio com
  5 minutos, e o seed usa 10 minutos tanto na reconciliação final quanto na
  espera de um segundo semeador — `LocalDemoRepository.AcquireRunLock` recebe a
  espera explicitamente em vez de herdar o timeout de statement;

- **o código passou a ser nomeado por funcionalidade, não pela fase que a
  entregou.** `Phase2Config`, `Phase3Config`, `Phase4Config` e `Phase7Config`
  viraram `CoreConfig`, `QuestionnaireConfig`, `AnalyticsConfig` e
  `SelfServiceConfig`; os arquivos `config/phase*.go` viraram `core.go`,
  `questionnaire.go`, `analytics.go` e `selfservice.go`, com `keyring.go` e
  `url.go` extraídos. Fase continua existindo apenas como ordem de entrega no
  harness. O mapa completo está em **Vocabulário de funcionalidades** no
  [`README`](README.md);

- **quebra de configuração:** `PHASE3_ENABLED`, `PHASE4_*` e `PHASE7_ENABLED`
  passam a ser `QUESTIONNAIRE_ENABLED`, `ANALYTICS_*` e
  `SELF_SERVICE_ENABLED`. Runtimes já provisionados precisam renomear as chaves
  antes do próximo deploy; não há alias de compatibilidade;

- **quebra de contrato:** a extensão `x-cumuru-implementation-phase` passa a ser
  `x-cumuru-feature`, com valores `platform`, `core`, `questionnaire`,
  `analytics`, `self-service` e `deferred` no lugar do número da fase; e o valor
  de enum `phase_not_implemented` de `UnavailableQualityCount.reason_code`
  passa a ser `not_implemented`;

- os quatro clients web foram reconstruídos sobre um transporte único,
  `shared/api/http-client.ts`, que concentra sessão, cabeçalhos, contrato de
  resposta e uma só classe `ApiError` no lugar de `Phase2ApiError`,
  `Phase3ApiError`, `Phase4ApiError` e `Phase7ApiError` — `use-operation`
  deixou de precisar testar `instanceof` duas vezes para uma única falha. Os
  validadores de payload público saíram para `shared/api/analytics-payload.ts`;

- os alvos de teste e os scripts `deploy/scripts/test-phase*.sh` e
  `deploy/compose.phase*.yaml` passaram a nomear a funcionalidade:
  `core-integration`, `questionnaire-integration`, `analytics-full-stack`,
  `self-service-integration`, entre outros;

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

- caixa de texto fora do padrão em todo formulário que não morava num cartão
  conhecido: o estilo de campo estava preso a `.form-card`, `.panel-card`,
  `.login-card`, `.operation-card`, `.onboarding-card` e `.new-stay-form`, então
  os filtros do diretório, o calendário do Booking.com e a emissão de acesso
  caíam no controle nativo do navegador — mais estreito, cinza e fora do ritmo
  da página. O estilo passa a ser base global de `input`, `select` e `textarea`
  com especificidade zero, e os campos deliberadamente menores continuam
  vencendo pela própria regra; a borda de erro de `aria-invalid` deixa de valer
  só dentro daqueles seis contêineres;
- rótulo colado ao campo nos filtros de `/hospedagens`: os dois filtros usavam
  `className="field"`, classe que não existe na folha de estilo;
- seta do `select` desenhada pelo sistema, que não seguia o tema nem o tamanho
  do campo e mudava de forma conforme o navegador: passa a ser um chevron da
  própria folha de estilo, feito de gradiente para acompanhar `currentColor`,
  inclusive quando o campo desabilita;
- `POST /accommodations` descrito no contrato como onboarding "com OIDC fake",
  trilho que a [ADR-037](docs/decisoes/ADR-037-autenticacao-local-por-email-e-senha.md)
  substituiu: `NewChainVerifier(session, federated)` aceita a sessão local de
  e-mail e senha, então a rota passa a declarar também o esquema `session`;
- capa prometendo o que o produto não faz: o passo 1 e a FAQ pediam CPF ou CNPJ
  quando o cadastro não coleta documento nenhum; o passo 3 dizia que o QR do
  balcão leva o visitante ao próprio registro, quando o canal aberto recusa
  nome, documento, e-mail e telefone e a estadia sempre nasce pendente de
  aprovação ([ADR-040](docs/decisoes/ADR-040-autocadastro-generalizado-e-aprovacao.md));
  o cartão de cadastro prometia acesso "no mesmo dia" e mandava para uma tela de
  login sem criação de conta, e a seção de privacidade anunciava um fluxo de
  direitos do titular que segue `deferred` no contrato. A capa passa a descrever
  o caminho real — conversa com a equipe, link de ativação, e-mail do encarregado
  — e saem o depoimento fictício marcado como exemplo e o horário de atendimento
  presencial inexistente;
- âncoras da capa parando atrás do cabeçalho de rotas e do índice de seções: as
  duas barras ficam coladas no topo e nenhum alvo descontava a altura delas, então
  clicar em "Números" ou usar "Pular para o conteúdo" deixava a primeira linha da
  seção escondida sob as barras. O índice não tem altura fixa — quebra em duas
  linhas em espanhol e abaixo de 50rem —, então a altura é medida no navegador e
  publicada como `--lp-nav-h` para o `scroll-margin-top`;
- navegação para `/registro` depois de "Abrir registro neste navegador": a
  guarda de rota comparava com a URL que `captureInviteCapability` já havia
  reescrito, o que anulava o clique e deixava o fluxo do convite sem saída;
- rótulos de categoria exibidos em código (`family_hosting`) na tela de
  qualidade agregada;
- botão de remover visitante ocupando uma célula inteira do formulário de
  registro, porque a distribuição em colunas estava no `fieldset` em vez do
  grupo de campos aninhado;
- `PHASE4_PRE_REGISTERED_WEIGHT` malformado era convertido em `NaN` e passava
  pela checagem de política congelada, porque toda comparação com `NaN` é falsa;
  o peso inválido chegava à aritmética de previsão. Toda leitura numérica de
  ambiente passa a falhar nomeando o próprio campo, em vez de devolver um
  sentinela para um validador posterior notar;
- `DOCUMENT_HMAC_KEYS` estava fora da lista que a Fase 3 percorre ao exigir
  chaves distintas, então uma chave de pesquisa podia reutilizar em silêncio a
  chave que cega o CPF sob a
  [ADR-038](docs/decisoes/ADR-038-documento-do-responsavel-cpf-ou-cnpj-por-organizacao.md);
- keyring de cursor inválido era descartado em silêncio e deixava toda listagem
  paginada sem cursor de próxima página; `httpapi.New` passa a recusar a
  construção quando registra uma superfície que pagina;
- conexão IndexedDB dos rascunhos cifrados vazava quando a operação falhava,
  porque o `close()` ficava na última linha; uma conexão vazada bloqueia o
  próximo upgrade de versão indefinidamente.

### Removido

- injeção de identidade em tempo de build (`VITE_LOCAL_DEMO_MODE` e
  `VITE_LOCAL_DEMO_IDENTITY`), com o guarda `local-demo-build` e o alvo
  `local-demo-build-test` correspondentes: o bundle não carrega mais credencial;
- metade não alcançável do pacote `idempotency` (`KeyHasher`, `Digest`,
  `Identity` e `StoredResponse`), que reimplementava o mesmo HMAC já aplicado
  pelo store; a lista de operações permitidas deixou de ser código morto e passa
  a barrar, no `runIdempotent`, uma operação não registrada antes de gravar a
  claim;
- telas de espaço reservado `AuthenticatedPlaceholderPage` e
  `RegistrationPlaceholderPage`, substituídas pelas telas reais, e a instância
  `platformClient` sem nenhum consumidor.

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
