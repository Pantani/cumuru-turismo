# Observatório Turístico de Cumuruxatiba

Plataforma de registro de estadias, pesquisa de perfil turístico, previsão de
fluxo e publicação de indicadores anônimos. Este repositório contém o blueprint
técnico, funcional e de governança e a implementação de referência.

## Estado atual

O sistema roda de ponta a ponta em ambiente local com dados fictícios: uma
hospedagem se cadastra, registra estadias, o hóspede responde à pesquisa, o
worker agrega presença e o painel público mostra os indicadores anônimos.

| Funcionalidade | O que entrega | Estado |
| --- | --- | --- |
| `platform` | Saúde, prontidão, build, observabilidade e CI | implementada |
| `core` | Hospedagens, estadias, grupos, convite nominal e auditoria | implementada |
| `questionnaire` | Autoria e versionamento do questionário e pesquisa pública | implementada |
| `analytics` | Presença, cobertura, supressão, publicação anônima e painéis | implementada |
| `self-service` | Autocadastro por cartaz/QR, ativação de conta e fila de aprovação | implementada |
| `calendar-feed` | Importação do calendário do Booking.com por iCal e fila de confirmação | implementada |
| `fnrh` | Envio à FNRH Digital | não implementada; bloqueada por gates externos |

**Todo o runtime opera como `PROTOTYPE_ONLY`.** Use exclusivamente dados,
identidades e credenciais fictícios. Os gates locais (`make ci`) provam código,
contrato, banco, privilégios e navegador real — nada mais. Eles não autorizam
deploy, release, uso com dados reais nem obrigação municipal.

O que continua pendente e não pode ser inferido do código:

- **base legal e patrocínio institucional.** Cumuruxatiba é distrito do
  Município de Prado; a iniciativa precisa ser patrocinada pela Prefeitura de
  Prado ou por entidade com competência e delegação válidas. Antes de tornar
  qualquer cadastro obrigatório, cumpra o gate descrito em
  [`docs/06-legal-e-governanca.md`](docs/06-legal-e-governanca.md);
- **base legal do canal aberto de autocadastro.** O formulário público coleta
  sem operador identificado e sem contato com o titular; enquanto essa base não
  existir, o canal só pode receber dados fictícios;
- **infraestrutura real:** provedor OIDC institucional, KMS/secret manager,
  PostgreSQL gerenciado, TLS, DNS, backup, restore e staging/produção;
- **FNRH:** autorização formal, documentação oficial vigente, homologação e
  política de credencial por estabelecimento;
- **piloto controlado** de 5 a 10 hospedagens por 30 a 60 dias, com suporte,
  canal do titular, restore e incidente simulados.

Os critérios de aceite por funcionalidade estão em
[`docs/09-roadmap-e-aceite.md`](docs/09-roadmap-e-aceite.md).

## O que o sistema faz hoje

Jornadas disponíveis na aplicação web:

| Rota | Para quem | O que faz |
| --- | --- | --- |
| `/` | qualquer pessoa | Capa pública trilíngue e painel de indicadores anônimos |
| `/hospedagens` | quem procura onde ficar | Lista pública das hospedagens que consentiram em publicar o contato |
| `/acesso` | hospedagem ou administração | Login por e-mail e senha; a conta cai na área da hospedagem ou na da administração, conforme o escopo `accommodations:onboard` |
| `/registro` | hóspede convidado | Registro do grupo a partir do convite nominal |
| `/pesquisa` | hóspede convidado | Pesquisa turística opcional |
| `/i` | hóspede sem convite | Autocadastro pelo cartaz/QR da hospedagem |
| `/ativacao` | hospedagem convidada | Ativação da conta por capability de uso único |
| `/questionarios` | administração | Autoria, revisão e publicação das versões do questionário |
| `/qualidade` | administração | Painel interno de cobertura e qualidade dos dados |

`/registro`, `/pesquisa`, `/i` e `/ativacao` só funcionam com a capability no
fragmento da URL: abertas diretamente, permanecem bloqueadas por construção.

## Calendário do Booking.com

A hospedagem que vende pelo Booking.com pode colar, na área dela, o endereço do
calendário do anúncio — o mesmo `.ics` que o extranet exporta em **Tarifas e
disponibilidade → Calendário → Sincronizar calendários → Exportar**. As datas
passam a chegar sozinhas e a hospedagem só confirma quantas pessoas vieram.

Três limites que o desenho assume em vez de esconder:

- **o arquivo não traz identidade.** As plataformas pararam de exportá-la, e
  isso aqui é a fronteira certa: o Observatório não guarda nome de hóspede;
- **o arquivo não traz número de hóspedes.** Por isso a estadia só nasce quando
  alguém confirma informando quantas pessoas vieram;
- **o arquivo não separa reserva de bloqueio de manutenção com confiabilidade.**
  A fila mostra o que a plataforma disse e pergunta; nada vira presença
  publicada sem confirmação humana.

Integração direta com a API do Booking.com não está no escopo e não é possível
para este projeto: o Connectivity API é reservado a parceiro homologado que
gerencia preço, disponibilidade e conteúdo em tempo real — a descrição de um
channel manager. A decisão completa está em
[`ADR-044`](docs/decisoes/ADR-044-importacao-de-calendario-da-plataforma-de-hospedagem.md).

## Resultado pretendido

O sistema deve:

- registrar estadias e grupos sem contar a mesma pessoa duas vezes;
- permitir autopreenchimento pelo hóspede ou operação pela hospedagem;
- funcionar em celular e tolerar internet instável;
- versionar perguntas e respostas;
- separar dados necessários à estadia de uma pesquisa turística opcional;
- integrar-se à FNRH Digital quando autorizado pelo estabelecimento;
- entregar previsão de pessoas presentes por dia e intervalos;
- publicar somente métricas pré-agregadas, arredondadas e protegidas;
- manter trilha de auditoria, retenção configurável e direitos do titular;
- ser simples de operar e evoluir.

## Participação local com ou sem CNPJ

O Observatório foi desenhado para os dois casos: uma pousada formal e uma
pessoa física que aluga uma casa podem cadastrar o local, registrar estadias e
participar dos indicadores sem informar CPF, CNPJ, Cadastur ou chave FNRH. O
cadastro pede somente nome do local, tipo de hospedagem e capacidade
aproximada. Esses dados não comprovam regularidade, licenciamento ou
enquadramento jurídico.

A integração FNRH é uma trilha separada e opcional. Quando ela for autorizada,
cada meio de hospedagem elegível deverá usar sua própria chave oficial; o
Observatório não emite nem compartilha essa credencial. Consulte a decisão
completa em
[`ADR-035`](docs/decisoes/ADR-035-participacao-local-sem-cnpj-e-onboarding-prototipo.md).

Materiais simples para apresentação e treinamento:

- [`Guia do Observatório para a Prefeitura (PDF)`](apps/web/public/guias/observatorio-prefeitura.pdf);
- [`Guia para gerar a chave FNRH (PDF)`](apps/web/public/guias/chave-fnrh-hospedagens.pdf).

## Stack

| Camada | Escolha | Versão fixada |
|---|---|---|
| API e workers | Go, biblioteca padrão `net/http` | 1.26.6 |
| Banco | PostgreSQL | 17.10 |
| Acesso ao banco | `pgx/v5` e SQL explícito gerado com `sqlc` | `sqlc` 1.31.1 |
| Migrations | `golang-migrate` | 4.19.1 |
| Front-end | React, TypeScript e Vite | React 19.2.8, TS 7.0.2, Vite 8.1.5 |
| Dados remotos no front | TanStack Query | — |
| Formulários | React Hook Form + Zod | — |
| Gráficos | Apache ECharts e SVG próprio | — |
| Autenticação | E-mail e senha local (Argon2id, sessão opaca) e OIDC/OAuth 2.1 | — |
| Contrato | OpenAPI | 3.1, documento `0.7.0` |
| Geração do cliente | `openapi-typescript` e Redocly CLI | 7.13.0 e 2.41.0 |
| Observabilidade | OpenTelemetry, métricas Prometheus e logs JSON | — |
| Implantação | Contêineres OCI; PostgreSQL gerenciado | — |

As versões exatas ficam travadas no lockfile e no toolchain e só mudam de forma
controlada. Não use Create React App.

## Por que esta arquitetura

- **Monólito modular:** uma implantação para API e worker, com limites de
  domínio claros; menor custo operacional que microsserviços.
- **PostgreSQL como infraestrutura principal:** dados, idempotência,
  `transactional outbox`, jobs e agregados no mesmo sistema transacional.
- **Front-end estático:** distribuído por CDN, rápido e barato.
- **Integrações por adaptadores:** a FNRH pode mudar de versão sem contaminar o
  domínio principal.
- **Privacidade por construção:** o painel público nunca consulta respostas
  brutas ou tabelas de identidade.

## Vocabulário de funcionalidades

O código é nomeado por **funcionalidade**, nunca pela ordem em que foi
entregue. Os mesmos nomes valem no back-end, no front-end, na configuração, no
contrato e nos alvos de teste:

| Funcionalidade | Config Go | Client web | Variável de ambiente | Alvo de integração |
| --- | --- | --- | --- | --- |
| `platform` | — | `platform-client.ts` | — | `make migration-test` |
| `core` | `CoreConfig` | `core-client.ts` | sempre ativa | `make core-integration` |
| `questionnaire` | `QuestionnaireConfig` | `questionnaire-client.ts` | `QUESTIONNAIRE_ENABLED` | `make questionnaire-integration` |
| `analytics` | `AnalyticsConfig` | `analytics-client.ts` | `ANALYTICS_ENABLED` | `make analytics-integration` |
| `self-service` | `SelfServiceConfig` | `self-service-client.ts` | `SELF_SERVICE_ENABLED` | `make self-service-integration` |
| `external-context` | `ExternalContextConfig` | `context-client.ts` | `EXTERNAL_CONTEXT_ENABLED` | `make external-context-isolation` |

No `contracts/openapi.yaml`, a extensão `x-cumuru-feature` marca a
funcionalidade dona de cada operação, com o valor `deferred` para o que ainda
não foi implementado.

Todo client web é construído sobre um único transporte,
`apps/web/src/shared/api/http-client.ts`, que concentra sessão, cabeçalhos,
contrato de resposta e a classe de erro `ApiError`. Não existe classe de erro
por funcionalidade: quem trata falha de API captura `ApiError` e nada mais.

Ordem de entrega e retomada de trabalho são vocabulário do harness, não do
código: veja [`prompts/BOOTSTRAP-CODEX.md`](prompts/BOOTSTRAP-CODEX.md).

## Como rodar na sua máquina

Pré-requisitos: Go, Node/npm e Docker com o daemon ativo.

```bash
cp .env.example .env
make setup
make up
make seed-test-fixtures
```

`make up` constrói e inicia PostgreSQL, migrations, API, worker e web, e semeia
o administrador incondicionalmente, com `SEED_ADMIN_EMAIL`/`SEED_ADMIN_PASSWORD`
do `.env` — hoje `administracao@cumuru.local`. Uma stack recém-subida sempre tem
por onde entrar.

As fixtures fictícias — operador, acomodações, estadias e respostas de pesquisa
— ficam fora desse fluxo, no perfil `test` do Compose, e entram com
`make seed-test-fixtures`. Esse alvo executa o comando Go `/app/local-demo`,
que aplica as fixtures de forma idempotente pelos serviços de domínio, semeia a
conta local de demonstração e espera a primeira publicação anônima. Nenhum
arquivo SQL de fixtures é montado ou executado. Sem ele o painel público sobe
vazio e `make smoke` falha por ausência de dado, não por bug.

O conjunto fictício é uma vila inteira em operação, e não um exemplo mínimo: 16
hospedagens de todas as categorias e portes, dois anos de estadias de vários
dias com alta e baixa temporada, cerca de 8800 hóspedes com faixas etárias e
residências variadas e cerca de 950 respostas de pesquisa. É esse volume que faz
todas as janelas da capa — 30, 90, 365 e 730 dias, mês civil, previsão e
preferências — abrirem com gráfico em vez de aviso de dado insuficiente, e que
deixa alguns poucos dias protegidos, que é como a supressão aparece na prática.
A primeira aplicação leva cerca de 20 segundos; as seguintes só acrescentam os
dias que passaram desde a anterior.

Depois abra:

- `http://127.0.0.1:4173/` — painel público com dados fictícios;
- `http://127.0.0.1:4173/acesso` — entrada de quem tem conta. A tela depende de
  quem entra: o administrador cai na área da administração, que só cadastra
  hospedagem, entrega o acesso e decide os pedidos vindos do site; a conta da
  hospedagem cai na área dela, com as estadias e a fila de hóspedes.

O bundle não carrega credencial: a entrada é por e-mail e senha. O
administrador é `administracao@cumuru.local`, com a senha de
`SEED_ADMIN_PASSWORD` (o `.env.example` usa `administracao-local-2026`); a
conta fictícia de operador é `operador@cumuru.local`, criada por
`make seed-test-fixtures`, com a senha de `LOCAL_DEMO_ACCOUNT_PASSWORD` (o
Compose local usa `demonstracao-local-2026`). A sessão vive apenas na memória
da aba — recarregar a página exige entrar de novo, por construção — e nada é
gravado em `localStorage`, `sessionStorage` ou cache do service worker.

O administrador não opera hospedagem nenhuma: ele não tem quadro de estadias
nem aprova hóspede. Cadastrar a hospedagem e entregar o acesso são atos dele;
registrar estadia e aprovar hóspede são atos de cada hospedagem, na conta que
recebeu esse acesso.

Na área da hospedagem, escolha uma hospedagem, crie uma estadia informando
chegada, saída e número de pessoas, e use as ações oferecidas pelo próprio
cartão da estadia — só aparecem as transições que o servidor aceita naquele
estado. Nenhum identificador ou ETag é digitado: ambos vêm da própria listagem.
O botão **Abrir registro neste navegador** mantém a capability somente em
memória, remove-a da URL e conduz ao registro e à pesquisa.

Para parar sem apagar o volume:

```bash
make down
```

Outros modos de execução:

| Comando | O que faz |
| --- | --- |
| `make dev` | Alias de `make up`; stack Compose comprovada, sem hot reload |
| `make docker-dev` | Stack com hot reload em `http://127.0.0.1:5173`, projeto `cumuru-dev` |
| `make dev-web` | Só o Vite; pressupõe uma API local respondendo em `127.0.0.1:8080` |
| `make seed` | Reaplica administrador e catálogo numa stack de pé; idempotente, não repõe senha trocada |
| `make seed-test-fixtures` | Aplica as fixtures fictícias do operador (perfil `test`); idempotente |
| `make docker-dev-seed-test-fixtures` | O mesmo, contra a stack de hot reload |
| `make docker-status` / `make docker-logs` | Estado e logs da stack local |
| `make docker-rm` | **Destrutivo:** derruba as stacks e apaga volumes, banco incluído |
| `make help` | Lista todos os targets públicos com descrição |

Reversão destrutiva de uma migration só é aceita em banco local descartável e
exige confirmação explícita:

```bash
ALLOW_DESTRUCTIVE_MIGRATION_DOWN=yes make migrate-down-local
```

## Comandos de desenvolvimento

O ciclo curto, sem Docker:

```bash
make test-unit      # Go short + Vitest
make typecheck      # typecheck estrito do web
make lint           # shell, go vet, Staticcheck e lint do web
make complexity     # complexidade ciclomática e cognitiva
make check          # gate local sequencial, sem Docker nem scanners
```

Depois de qualquer tarefa que altera código, o owner executa
`make post-task-quality`, que roda `make complexity` e `make lint`
sequencialmente e só então emite `POST_TASK_QUALITY=PASS`. Um `DONE` ou handoff
sem esse marcador e sem o exit code zero registrado no artifact é inválido.

O gate de complexidade exige, sem nenhuma suppression:

- Go de produção e código gerado: ciclomática ≤ 5 e cognitiva ≤ 8;
- Go de teste: ciclomática e cognitiva ≤ 9, porque um teste table-driven perde
  legibilidade quando as asserções viram helpers;
- web, testes incluídos: ciclomática ≤ 5 e cognitiva ≤ 8.

Regeneração de artefatos derivados (nunca edite os arquivos gerados à mão):

```bash
make generate         # cliente TypeScript do OpenAPI + queries sqlc
make generated-check  # prova que o gerado é reprodutível
make openapi-lint
```

## Gate completo e CI

`make ci` é a referência única local e executa sequencialmente as mesmas provas
da CI; use os targets isolados quando quiser reexecutar apenas uma delas.
`make compose-config` e `make smoke-local` leem os overlays locais do Compose,
então o `.env` precisa existir.

```bash
make ci
```

```text
openapi-lint             generated-check           compose-config
prod-config-example      migration-test            local-restore-drill
local-demo-test          core-integration          core-proxy-test
questionnaire-integration questionnaire-proxy-test analytics-integration
analytics-proxy-test     self-service-integration  test
test-backend-race        typecheck                 post-task-quality
infra-validation         build                     images
core-full-stack          analytics-benchmark       self-service-full-stack
local-demo-e2e           smoke-local               sbom
scan                     image-scan
```

Na GitHub Actions esses mesmos gates rodam em paralelo, um job por prova.
`.github/workflows/ci.yml` define os jobs e `.github/actions/setup` instala em
cada um somente a cadeia de ferramentas que ele usa — Go, Node, `.tools/bin`,
Terraform, ripgrep, Chromium ou `.env` —, o que evita pagar `npm ci` num job que
só compila Go. O job `ci` é a porta única: depende de todos os outros e falha
quando qualquer um não termina em `success`, inclusive se for pulado ou
cancelado. É esse o check que a proteção de branch deve exigir. Ao adicionar um
gate, adicione nos dois lugares.

Antes do primeiro E2E local, instale o browser fixado pelo Playwright com
`npx playwright install chromium`.

### O que cada prova cobre

| Prova | O que garante |
| --- | --- |
| `migration-test` | Migrations, grants e papéis em PostgreSQL real, com `zero → 1 → zero → 1` |
| `local-restore-drill` | `pg_dump`/`pg_restore` em base isolada, comparando schema, ownership, grants e dados |
| `core-integration` | Domínio do núcleo sob o papel runtime `cumuru_app`, em PostgreSQL efêmero |
| `*-proxy-test` | Sobrescrita de headers e ausência de capability ou dado de erro nos logs de Nginx/Vite |
| `*-full-stack` | API, worker e web construídos em projeto descartável, com teardown verificado |
| `analytics-benchmark` | Duas recomposições determinísticas de três anos, com tempo, heap e hardware registrados |
| `local-demo-e2e` | Jornada operador → convite → grupo → pesquisa em Chromium, sem authority em cache |
| `smoke` / `smoke-local` | Publicação, séries públicas, preferências e ao menos uma acomodação do operador |
| `sbom` / `scan` / `image-scan` | CycloneDX, dependências, filesystem, segredos e imagens; falha em HIGH/CRITICAL |

`make local-restore-drill` é estritamente local: não comprova backup contínuo,
PITR, RPO, RTO, custódia, retenção, reaplicação de exclusões ou restore
institucional.

## Notas de implementação

### Núcleo (`core`)

A baseline `000001_initial_schema` materializa acomodações, memberships,
estadias, grupos, convites, comandos de estado, autenticação local e o
onboarding sem CNPJ, CPF, Cadastur ou chave FNRH. Toda a cadeia pré-lançamento
está consolidada nesse único par de migrations, e a suíte exige exatamente esse
par.

Nome, documento, contato, referência externa e texto livre são rejeitados no
núcleo. Mutação, outbox e auditoria são atômicas; convites são guardados como
HMAC; a presença usa o intervalo `[entrada, saída)`; rascunhos offline usam
IndexedDB e nunca `localStorage`.

### Questionário (`questionnaire`)

Versões seguem `draft → privacy_review → approved → published → retired`.
Versão publicada é imutável e editar clona uma nova versão, de modo que
respostas antigas preservam a semântica com que foram coletadas.

A pesquisa é opcional e usa `Survey-Capability`. O header aparece somente
quando a submissão de grupo chega a `pre_registered` e existe uma versão
`tourism_profile` publicada; sua ausência não falha cadastro ou check-in. O
mesmo POST e seu replay exato devolvem o mesmo segredo sem persistir plaintext.
Local e teste exigem keyrings independentes, TTL máximo de 24 horas e cleanup
de texto livre habilitado.

### Analytics (`analytics`)

O papel `public_runtime` consulta somente quatro views `security_barrier` em
`public_data`; a API usa um `PUBLIC_DATABASE_URL` separado para essa superfície
e não recebe acesso às tabelas-base.

Os indicadores são células pré-agregadas com catálogo tipado, política
`prototype-v1`, arredondamento mínimo de 10 e supressão com `k >= 10` e pelo
menos três acomodações. Preferências elegíveis usam somente respostas
estruturadas, consentidas e não sensíveis. Texto livre criptografado, IDs,
contagens de amostra e motivos de supressão não entram no contrato público.

O backend reconcilia presença incremental e integral, publica releases
imutáveis e serve a última válida durante falha. Os painéis usam as operações
tipadas e preservam estados protegidos, indisponíveis e `N/A`.

### Autoatendimento (`self-service`)

O cartaz/QR da hospedagem abre um canal sem operador identificado. Ele coleta
apenas dado generalizado: nome, documento, e-mail, telefone e `role='minor'`
são rejeitados, e a identidade só é preenchida pelo próprio titular depois da
aprovação, pelo convite nominal do núcleo.

A estadia autocadastrada nasce sem membership autora, com proveniência
`self_service`, e não entra em `analytics.presence_days` nem em `public_data`
antes da aprovação. Aprovação e rejeição são idempotentes, respeitam `If-Match`
e exigem operação própria: um operador com `update_stay` não aprova. A rejeição
exige motivo de lista fechada; rejeição e expiração eliminam os dados do
autocadastro, e a pendência expira em 72 horas com auditoria. O canal é
protegido por rate limit e proof-of-work, sem serviço de terceiro e sem cookie.
A capability de ativação é de uso único e revogável; o token trafega no
fragmento da URL e nunca em log, trace, métrica, audit ou outbox.

### Lista pública de hospedagens

`/hospedagens` e `GET /api/v1/public/accommodations` publicam nome, categoria,
localidade, capacidade, telefone, WhatsApp e site das hospedagens ativas que
pediram para aparecer — e nada além disso. A publicação é ato da própria
hospedagem, no painel "Aparecer na lista pública" da área da hospedagem: nasce
desligada, exige telefone em E.164 e guarda o instante do consentimento.
Desmarcar retira a hospedagem da lista na mesma transação, sem fila e sem
espera.

O documento é único, ordenado por nome, sem cursor e cacheável por inteiro
(`max-age=300`, ETag forte); filtrar por tipo e buscar por nome acontecem no
navegador. Não há reserva, preço, disponibilidade nem avaliação: o Observatório
não intermedeia hospedagem
([ADR-043](docs/decisoes/ADR-043-lista-publica-de-hospedagens.md)).
### Contexto externo (`external-context`)

Esta camada mostra **dado copiado de fora** — hoje o clima — ao lado dos
números que o Observatório mede. As duas coisas aparecem na mesma tela e são de
naturezas diferentes, e a distinção importa:

- os números do Observatório são **medidos aqui**, passam por supressão para
  não identificar ninguém e respondem por uma política de privacidade;
- o contexto externo é **copiado de terceiro**, não tem amostra, não é
  suprimível e responde por uma **licença**.

**O que a camada não é, e nunca será:** dado externo jamais entra em presença,
cobertura ou previsão. Não é um número do Observatório com outra roupa. Essa
separação não é promessa escrita em documento: o banco tem um schema próprio
(`external`) e o papel que calcula presença **não tem permissão nenhuma** ali,
nos dois sentidos. Se alguém tentar cruzar as camadas, o PostgreSQL recusa.

A rota é `GET /public/context`, com endereço e cache próprios, separada das
quatro rotas de indicadores. Isso é de propósito nos dois sentidos: fonte
externa fora do ar não atrasa nem derruba a publicação dos indicadores, e
publicação de indicador não mexe no contexto externo.

Cada card mostra a fonte, o publicador, a licença com link e o texto de
atribuição — **inclusive quando o card está indisponível**, porque a obrigação
de creditar quem produziu o dado existe porque a fonte existe, não porque a
consulta deu certo. Cada card também diz se o dado é real ou fixture de
protótipo, um por um: uma página que misturasse clima real com presença
fictícia sob um único rótulo mentiria nas duas direções.

Um card tem dois estados: **publicado**, com valor, ou **indisponível**, com um
motivo de lista fechada. Indisponível mostra **traço**, nunca zero e nunca o
último valor conhecido servido em silêncio. Zero é um número e afirma algo;
traço afirma que não sabemos. Uma fonte fora do ar deixa aquele card
indisponível e a aba continua de pé.

**O card de maré nasce indisponível de propósito, e isso é decisão, não
defeito.** Para publicar horário de maré é preciso usar as constantes oficiais
da Marinha do Brasil, que só saem mediante pedido e podem exigir um termo que
proíbe repassar os dados a terceiros — e um painel público é exatamente
repasse. Enquanto não houver autorização escrita, o card existe, é creditado e
diz que está indisponível. Nada de estimar por conta própria: maré baixa é
quando se caminha no recife, e um horário errado não é imprecisão estatística,
é uma pessoa na água na hora errada, num painel com a marca da Prefeitura.

O **Cadastur** aparece como fonte creditada, com link, e **sem nenhum número
calculado pela plataforma** — sem total de hospedagens, sem série, sem
percentual. O motivo é concreto: publicar quantos estabelecimentos existem no
município ao lado da cobertura já publicada permitiria a qualquer pessoa
subtrair e descobrir **quantos não reportaram**. Numa vila onde todo mundo sabe
quais são as pousadas, isso vira uma lista de nomes.

A coleta acontece **somente no worker**, com endereço fixo, em horário
ancorado no calendário. O navegador de quem visita o site **não chama nada**
para fora, e nenhum dado de quem acessa vai junto de qualquer requisição.

### Imagens, proveniência e rede local

`make images` aplica a tag determinística `0.2.0` às duas imagens runtime:
`cumuru-api:0.2.0`, compartilhada por API e worker, e `cumuru-web:0.2.0`. A CI
substitui a tag pelo SHA exato do commit. Versão, revisão e instante
reprodutível da fonte são derivados por
`deploy/scripts/with-build-metadata.sh`: usa commit e timestamp quando há um
checkout limpo; sem SCM, usa hash determinístico da fonte e o
`SOURCE_DATE_EPOCH` versionado em `deploy/build-metadata.env`. Builds sem esses
`ldflags` são rejeitados no startup.

`make image-scan` exporta as imagens para tar e executa Trivy fixado por
digest, sem montar o socket Docker no scanner. `make sbom` gera CycloneDX para
Go, npm e ambas as imagens e publica `artifacts/sbom/image-manifest.json`.

No Compose, o web usa `/api/v1` no mesmo origin e é a única entrada para a API:
o serviço `api` não publica porta no host. Nginx mantém access log desligado,
descarta error log na location de capability, remove `Referer`/`Forwarded` e
sobrescreve `X-Forwarded-For`/`X-Real-IP` com o socket remoto. Web e PostgreSQL
publicam portas apenas em `127.0.0.1`. O proxy web local usa `172.30.0.10` e
`COMPOSE_TRUSTED_PROXY_CIDRS=172.30.0.10/32`; nenhuma subnet inteira é
confiável. API e Vite executados nativamente no host usam
`TRUSTED_PROXY_CIDRS=127.0.0.1/32,::1/128`. Overlays de migration, integration
e full-stack usam subnets disjuntas para não colidir com a stack local.

`.env.example` inicializa TTLs, limites, CORS e os keyrings de `platform`,
`core`, `questionnaire` e `analytics` com fixtures públicas e independentes,
exclusivamente para `local`/`test`. Elas não são segredos e não podem ser
promovidas.

O contexto externo vem desligado: `EXTERNAL_CONTEXT_ENABLED` tem default
`false`, e é assim que se quer — ligá-lo abre a primeira saída do sistema para
a internet pública. Quando ligado, apenas o worker usa
`EXTERNAL_DATABASE_URL`, que conecta com um usuário de banco próprio
(`cumuru_external`) e diferente do usuário que calcula os indicadores. É essa
diferença que faz as permissões valerem alguma coisa: o loader recusa a
configuração se os dois usuários forem o mesmo.

**Limitação conhecida:** hoje a ingestão só roda em `local` e `test`. Ligá-la
em servidor exige, **antes**, criar o usuário `cumuru_external` e migrar o
segredo de runtime já gravado — acrescentar a senha nova à validação sem migrar
o segredo quebraria toda instalação existente. O passo a passo está na seção
"Limitação conhecida" da
[`ADR-045`](docs/decisoes/ADR-045-camada-de-contexto-externo.md).

O autoatendimento é a exceção: `SELF_SERVICE_*` e `PROOF_OF_WORK_*` não estão
em `.env.example` e vêm apenas de `compose.yaml`/`compose.local.yaml`. Quem
sobe API e worker nativamente com esse `.env` roda com `SELF_SERVICE_ENABLED`
no default `false` — as rotas de autocadastro e de ativação nem chegam a ser
registradas e respondem `404`, em vez de existirem meio configuradas. Use
`make up` para exercitar a funcionalidade, ou defina as variáveis à mão
espelhando o Compose.

## Mapa da documentação

| Documento | Conteúdo |
| --- | --- |
| [`docs/00-visao-e-escopo.md`](docs/00-visao-e-escopo.md) | Problema, público e escopo |
| [`docs/01-arquitetura.md`](docs/01-arquitetura.md) | Componentes, módulos e fronteiras |
| [`docs/02-dominios-e-fluxos.md`](docs/02-dominios-e-fluxos.md) | Estados, jornadas e regras de domínio |
| [`docs/03-modelo-de-dados.md`](docs/03-modelo-de-dados.md) | Schemas, tabelas e papéis do PostgreSQL |
| [`docs/04-api-e-idempotencia.md`](docs/04-api-e-idempotencia.md) | Contrato HTTP, repetição e concorrência |
| [`docs/05-privacidade-e-seguranca.md`](docs/05-privacidade-e-seguranca.md) | Minimização, criptografia e ameaças |
| [`docs/06-legal-e-governanca.md`](docs/06-legal-e-governanca.md) | Base legal, papéis LGPD e gates jurídicos |
| [`docs/07-dashboard-e-previsao.md`](docs/07-dashboard-e-previsao.md) | Métricas, supressão e previsão |
| [`docs/08-operacao-e-resiliencia.md`](docs/08-operacao-e-resiliencia.md) | Jobs, outbox, retries e incidentes |
| [`docs/09-roadmap-e-aceite.md`](docs/09-roadmap-e-aceite.md) | Estado por funcionalidade e critérios de aceite |
| [`docs/10-questionario-inicial.md`](docs/10-questionario-inicial.md) | Perguntas de referência da pesquisa |
| [`docs/11-decisoes-tecnicas.md`](docs/11-decisoes-tecnicas.md) | Escolhas de stack e referências |
| [`docs/12-infraestrutura-e-deploy.md`](docs/12-infraestrutura-e-deploy.md) | Terraform, Ansible, ambientes e gates de infra |
| [`docs/decisoes/`](docs/decisoes/) | ADRs numerados; toda decisão divergente vira ADR |
| [`contracts/openapi.yaml`](contracts/openapi.yaml) | Contrato público, fonte do cliente gerado |
| [`database/schema.sql`](database/schema.sql) | Schema de referência |
| [`CHANGELOG.md`](CHANGELOG.md) | Mudanças do contrato e da plataforma |
| [`AGENTS.md`](AGENTS.md) | Regras para agentes de implementação |
| [`apps/api/README.md`](apps/api/README.md) | Processos, configuração e superfícies do backend |
| [`apps/web/README.md`](apps/web/README.md) | SPA, proxies, service worker e idiomas |
| [`deploy/README.md`](deploy/README.md) | Compose, infraestrutura e provas de deploy |

## Harness de execução

O repositório inclui o harness `cumuru-bootstrap`, que organiza a implementação
em ondas com estudo paralelo, ownership por owner e QA incremental. Ordem de
entrega, gates externos e retomada vivem só ali —
[`prompts/BOOTSTRAP-CODEX.md`](prompts/BOOTSTRAP-CODEX.md) e
`.agents/skills/cumuru-bootstrap/references/phase-matrix.md` —, nunca no código
da aplicação.

```bash
make harness-validate
make harness-test
make harness-status
make harness-dry-run PHASE=1
make harness-prompt PHASE=1
make harness-snapshot PHASE=1
```

O `dry-run` não altera a aplicação. Antes de reexecutar uma onda ampla,
`harness-snapshot` preserva o plano, os artifacts e o QA atuais em `attempts/` e
fecha o gate como `UNVERIFIED` até a nova validação. Como skills e agentes novos
são descobertos no início da sessão, abra uma nova tarefa no diretório raiz
antes do primeiro uso.

## Princípios não negociáveis

- Nenhuma obrigação municipal entra em produção sem fundamento jurídico formal.
- O comércio não recebe dados pessoais, respostas brutas ou exportações
  individualizadas.
- Perguntas publicadas são imutáveis; mudanças criam nova versão.
- Toda escrita repetível é idempotente.
- Nenhum segredo da FNRH é armazenado em texto puro.
- Falha na integração externa não perde o registro local.
- O total público nunca é somado diretamente pelo navegador a partir de
  registros individuais.
- Um administrador não pode transformar uma pergunta comum em dado sensível sem
  revisão do encarregado de dados.
