# Observatório Turístico de Cumuruxatiba

Blueprint técnico, funcional e de governança para uma plataforma de registro de
estadias, pesquisa de perfil turístico, previsão de fluxo e publicação de
indicadores anônimos.

## Situação do projeto

As Fases 1, 2, 3 e 4 estão implementadas com gate técnico local `PASS`. A Fase
4 inclui domínio, handlers, worker, dashboards público e interno e jornada
real em Chromium. Todas permanecem exclusivamente como protótipo
`PROTOTYPE_ONLY`; os gates externos continuam bloqueando as fases seguintes.

Essa conclusão técnica não autoriza operação real: os gates externos permanecem
obrigatórios. Antes de tornar qualquer cadastro municipal obrigatório, cumpra o
gate jurídico descrito em
[`docs/06-legal-e-governanca.md`](docs/06-legal-e-governanca.md).
Cumuruxatiba é distrito do Município de Prado; portanto, a iniciativa deve ser
formalmente patrocinada pela Prefeitura de Prado ou por entidade com competência
e delegação válidas.

## Resultado pretendido

O sistema deverá:

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

O Observatório local foi desenhado para os dois casos: uma pousada formal e
uma pessoa física que aluga uma casa podem cadastrar o local, registrar
estadias e participar dos indicadores sem informar CPF, CNPJ, Cadastur ou
chave FNRH. No protótipo, o cadastro pede somente nome do local, tipo de
hospedagem e capacidade aproximada. Esses dados não comprovam regularidade,
licenciamento ou enquadramento jurídico.

A integração FNRH é uma trilha separada e opcional. Quando a Fase 5 e seus
gates externos forem autorizados, cada meio de hospedagem elegível deverá usar
sua própria chave oficial; o Observatório não emite nem compartilha essa
credencial. Consulte a decisão completa em
[`ADR-035`](docs/decisoes/ADR-035-participacao-local-sem-cnpj-e-onboarding-prototipo.md).

Materiais simples para apresentação e treinamento:

- [`Guia do Observatório para a Prefeitura (PDF)`](apps/web/public/guias/observatorio-prefeitura.pdf);
- [`Guia para gerar a chave FNRH (PDF)`](apps/web/public/guias/chave-fnrh-hospedagens.pdf).

## Stack de referência

| Camada | Escolha |
|---|---|
| API e workers | Go 1.26.x, biblioteca padrão `net/http` |
| Banco | PostgreSQL 17+ |
| Acesso ao banco | `pgx/v5` e SQL explícito, gerado com `sqlc` |
| Front-end | React 19.2, TypeScript e Vite |
| Dados remotos no front | TanStack Query |
| Formulários | React Hook Form + Zod |
| Gráficos | Apache ECharts |
| Autenticação | E-mail e senha local (Argon2id, sessão opaca) e OIDC/OAuth 2.1 em paralelo |
| Contrato | OpenAPI 3.1 |
| Observabilidade | OpenTelemetry, métricas Prometheus e logs JSON |
| Implantação | Contêineres OCI; serviço gerenciado de PostgreSQL |

As versões exatas devem ser travadas no primeiro commit e atualizadas de forma
controlada. Não use Create React App.

## Por que esta arquitetura

- **Monólito modular:** uma implantação para API e worker, com limites de domínio
  claros; menor custo operacional que microsserviços.
- **PostgreSQL como infraestrutura principal:** dados, idempotência,
  `transactional outbox`, jobs e agregados no mesmo sistema transacional.
- **Front-end estático:** distribuído por CDN, rápido e barato.
- **Integrações por adaptadores:** a FNRH pode mudar de versão sem contaminar o
  domínio principal.
- **Privacidade por construção:** o dashboard nunca consulta respostas brutas ou
  tabelas de identidade.

## Mapa da documentação

1. [`docs/00-visao-e-escopo.md`](docs/00-visao-e-escopo.md)
2. [`docs/01-arquitetura.md`](docs/01-arquitetura.md)
3. [`docs/02-dominios-e-fluxos.md`](docs/02-dominios-e-fluxos.md)
4. [`docs/03-modelo-de-dados.md`](docs/03-modelo-de-dados.md)
5. [`docs/04-api-e-idempotencia.md`](docs/04-api-e-idempotencia.md)
6. [`docs/05-privacidade-e-seguranca.md`](docs/05-privacidade-e-seguranca.md)
7. [`docs/06-legal-e-governanca.md`](docs/06-legal-e-governanca.md)
8. [`docs/07-dashboard-e-previsao.md`](docs/07-dashboard-e-previsao.md)
9. [`docs/08-operacao-e-resiliencia.md`](docs/08-operacao-e-resiliencia.md)
10. [`docs/09-roadmap-e-aceite.md`](docs/09-roadmap-e-aceite.md)
11. [`docs/10-questionario-inicial.md`](docs/10-questionario-inicial.md)
12. [`docs/11-decisoes-tecnicas.md`](docs/11-decisoes-tecnicas.md)
13. [`contracts/openapi.yaml`](contracts/openapi.yaml)
14. [`database/schema.sql`](database/schema.sql)
15. [`prompts/BOOTSTRAP-CODEX.md`](prompts/BOOTSTRAP-CODEX.md)
16. [`Guia do Observatório para a Prefeitura`](apps/web/public/guias/observatorio-prefeitura.pdf)
17. [`Guia para gerar a chave FNRH`](apps/web/public/guias/chave-fnrh-hospedagens.pdf)

## Como começar com o Codex

1. Copie este diretório para o repositório definitivo.
2. Abra o diretório como workspace.
3. Entregue ao Codex o conteúdo de
   [`prompts/BOOTSTRAP-CODEX.md`](prompts/BOOTSTRAP-CODEX.md).
4. Peça a execução por fases, nunca o sistema inteiro em uma única alteração.
5. Exija que cada fase termine com testes, migrações, documentação e critérios de
   aceite verificados.

## Harness de execução

O repositório inclui o skill Codex `cumuru-bootstrap` para executar o prompt por
fase com estudo paralelo, implementação com ownership e QA incremental. Como
skills e agentes novos são descobertos no início da sessão, abra uma nova tarefa
no diretório raiz antes do primeiro uso.

Comandos de inspeção:

```bash
make harness-validate
make harness-test
make harness-status
make harness-dry-run PHASE=1
make harness-prompt PHASE=1
make harness-snapshot PHASE=1
```

O `dry-run` não altera a aplicação. Enquanto este diretório não estiver em um
repositório Git, o harness permite estudos paralelos, mas limita a implementação
a um único writer por vez e não promete worktrees ou rollback por commit.
Antes de reexecutar uma fase ampla, `harness-snapshot` preserva o plano, os
artifacts e o QA atuais em `attempts/` e fecha o gate da fase como
`UNVERIFIED` até a nova validação.

Para iniciar no Codex:

```text
$cumuru-bootstrap Execute a Fase 1, com estudo antes da implementação.
```

## Fundação técnica — Fase 1

A implementação da fundação opera exclusivamente como `PROTOTYPE_ONLY`: use
somente dados, identidades e credenciais fictícios. Governança, provedor OIDC
institucional, KMS, backup/restore e infraestrutura de staging/produção ainda
não foram verificados. O contrato OpenAPI é um contrato-alvo; a extensão
`x-cumuru-implementation-phase` distingue operações futuras das três operações
entregues pela fundação.

Versões principais fixadas:

- Go 1.26.5;
- PostgreSQL 17.10;
- React e React DOM 19.2.8;
- TypeScript 7.0.2;
- Vite 8.1.5;
- `sqlc` 1.31.1 e `golang-migrate` 4.19.1;
- `openapi-typescript` 7.13.0 e Redocly CLI 2.41.0.

Pré-requisitos locais: Go, Node/npm e Docker com o daemon ativo.

```bash
cp .env.example .env
make install
make tools
make generate
make generated-check
make migration-test
make local-restore-drill
make local-demo-test
make local-demo-e2e
make phase2-integration
make phase2-proxy-test
make phase2-full-stack
make complexity
make up
make smoke
```

`make up` constrói e inicia PostgreSQL, migrations, API, worker e web. O target
usa `compose.local.yaml` para executar o comando Go `/app/local-demo`, que
aplica fixtures fictícias idempotentes pelos serviços de domínio, semeia a conta
local de demonstração e espera a primeira publicação anônima. Nenhum arquivo SQL
de fixtures é montado ou executado.

O bundle não carrega credencial: a entrada é por e-mail e senha em
`/acesso`. A conta fictícia é `operador@cumuru.local`, com a senha definida em
`LOCAL_DEMO_ACCOUNT_PASSWORD` (o Compose local usa
`demonstracao-local-2026`). A sessão vive apenas na memória da aba — recarregar
a página exige entrar de novo, por construção — e nada é gravado em
`localStorage`, `sessionStorage` ou cache do service worker.
Abra:

- `http://127.0.0.1:4173/` para o dashboard fictício;
- `http://127.0.0.1:4173/acesso` para a jornada do operador.

Na área da hospedagem, escolha uma hospedagem, crie uma estadia informando
chegada, saída e número de pessoas, e use as ações oferecidas pelo próprio
cartão da estadia — só aparecem as transições que o servidor aceita naquele
estado. Nenhum identificador ou ETag é digitado: ambos vêm da própria listagem.
O botão **Abrir registro neste navegador** mantém a capability somente em
memória, remove-a da URL e conduz ao registro e à pesquisa. As rotas
`/registro` e `/pesquisa` abertas diretamente continuam bloqueadas por design.

`make smoke` exige publicação, séries públicas, preferências e ao menos uma
acomodação do operador; lista vazia ou endpoint público `4xx/5xx` falha.
`make phase4-remediation` reúne o check de código gerado, as guardas dos builds
web, PostgreSQL fresh/persistente, rollover, colisão fail-closed, full-stack e
a jornada completa em Chromium, inclusive Service Worker e Cache API sem
authority material. Antes do primeiro E2E local, instale o browser fixado pelo
Playwright com `npx playwright install chromium`.

Para parar sem apagar o volume:

```bash
make down
```

Reversão destrutiva de uma migration só é aceita em banco local descartável e
exige confirmação explícita:

```bash
ALLOW_DESTRUCTIVE_MIGRATION_DOWN=yes make migrate-down-local
```

Os gates completos são:

```bash
make openapi-lint
make generated-check
make migration-test
make local-restore-drill
make phase2-integration
make phase2-proxy-test
make phase2-full-stack
make test
make typecheck
make post-task-quality
make build
make images
make image-scan
make sbom
make scan
```

`make local-restore-drill` cria uma base PostgreSQL descartável, aplica a
migration consolidada, grava apenas canários fictícios, executa `pg_dump` e
`pg_restore` em outra base isolada e compara schema, ownership, grants e dados.
O cleanup remove dump, base restaurada, containers, rede e volume temporários.
Essa prova é estritamente local: não comprova backup contínuo, PITR, RPO, RTO,
custódia, retenção, reaplicação de exclusões ou restore institucional.

## Plataforma da Fase 2 — contrato e onboarding local

A baseline `000001_initial_schema` materializa acomodações, memberships,
estadias, grupos, convites, comandos de estado e o onboarding local contratado
na versão `0.6.0`, sem exigir CNPJ, CPF, Cadastur ou chave FNRH. Consumidores só
podem ser considerados integrados depois do QA cruzar OpenAPI, PostgreSQL, Go,
cliente gerado e React.

O recorte continua `PROTOTYPE_ONLY`: nome, documento, contato, referência
externa e texto livre são rejeitados; questionário, analytics, dashboard e
FNRH permanecem fora da Fase 2. O gate `make complexity` exige complexidade
ciclomática e cognitiva no máximo 9 por função, inclui testes e código gerado e
não admite suppressions.

Depois de cada tarefa que altera código, o owner executa
`make post-task-quality`, que roda `make complexity` e `make lint`
sequencialmente e só então emite `POST_TASK_QUALITY=PASS`. Um `DONE` ou handoff
sem esse marcador e sem o exit code zero registrado no artifact é inválido.

`make images` aplica a tag determinística `0.2.0` às duas imagens runtime
distintas: `cumuru-api:0.2.0`, compartilhada por API e worker, e
`cumuru-web:0.2.0`. A CI substitui a tag pelo SHA exato do commit. Versão,
revisão e instante reprodutível da fonte são derivados por
`deploy/scripts/with-build-metadata.sh`: usa commit e timestamp quando há um
checkout limpo; sem SCM, usa hash determinístico da fonte e o
`SOURCE_DATE_EPOCH` versionado em `deploy/build-metadata.env`.

`make image-scan` exporta as imagens para tar e executa Trivy 0.69.3 fixado por
digest, sem montar o socket Docker no scanner. O ref `tag@sha256` é
materializado e conferido antes de qualquer inspeção, portanto o fluxo também
funciona em um runner sem cache e falha se o registry ou digest não puder ser
validado. O gate falha em findings HIGH ou CRITICAL. `make sbom` gera
CycloneDX para Go, npm e ambas as imagens e publica
`artifacts/sbom/image-manifest.json` com os IDs/digests e a revisão exata. O
target `make scan` continua cobrindo dependências, filesystem e segredos; as
ferramentas executadas em container também são fixadas por digest.

No Compose, o web usa `/api/v1` no mesmo origin e é a única entrada para a API:
o serviço `api` não publica porta no host. Nginx mantém access log desligado,
descarta error log na location de capability, remove `Referer`/`Forwarded` e
sobrescreve `X-Forwarded-For`/`X-Real-IP` com o socket remoto. O smoke usa
somente o fake OIDC local para provar uma rota autenticada da Fase 2 pelo
proxy. Web e PostgreSQL publicam portas apenas em `127.0.0.1`.

O proxy web local usa `172.30.0.10` e
`COMPOSE_TRUSTED_PROXY_CIDRS=172.30.0.10/32`; nenhuma subnet inteira é
confiável. API e Vite executados nativamente no host usam
`TRUSTED_PROXY_CIDRS=127.0.0.1/32,::1/128`.

Overlays de migration, integration e full-stack usam subnets disjuntas para
não colidir com a stack local.

`.env.example` inicializa todos os TTLs, limites, CORS e os cinco keyrings da
Fase 2 com fixtures públicas e independentes, exclusivamente para
`local`/`test`. Elas não são segredos e não podem ser promovidas. O target
`make phase2-integration` cria PostgreSQL e porta efêmeros, aplica migrations
com `cumuru_migration` e executa a suíte tagged sob o papel runtime
`cumuru_app`, falhando se qualquer DSN estiver ausente ou apontar para outro
papel.

`make phase2-proxy-test` usa upstreams locais controlados para provar
sobrescrita de headers e ausência de capability/dados de erro em stdout e
stderr de Nginx/Vite. `make phase2-full-stack` constrói API, worker e web em
projeto descartável, valida timezone, service worker, proxy e rota autenticada,
derruba/restaura a API para o canário de logs e confirma a remoção de
containers, rede e volume.

Toda a cadeia pré-lançamento — incluindo onboarding local, autenticação por
e-mail e senha e rotação da senha semeada — está consolidada no único par
`000001_initial_schema`. A suíte exige exatamente esse par e exercita
`zero → 1 → zero → 1`, grants, categorias fechadas e isolamento de tenant
fictício.

## Plataforma da Fase 3 — contrato congelado

O contrato `0.4.0`, o trecho da Fase 3 incorporado à migration consolidada
`000001_initial_schema`, as queries `sqlc` e o cliente TypeScript gerado
materializam a fronteira compartilhada do questionário. Isso ainda não declara
domínio, handlers ou UI concluídos.

A pesquisa é opcional e usa `Survey-Capability`. O header aparece somente
quando a submissão de grupo chega a `pre_registered` e existe uma versão
`tourism_profile` publicada; sua ausência não falha cadastro ou check-in. O
mesmo POST e seu replay exato devolvem o mesmo segredo sem persistir plaintext.

Local e teste exigem keyrings independentes, TTL máximo de 24 horas e cleanup
de texto livre habilitado. A Fase 3 permanece `PROTOTYPE_ONLY`.

```bash
make openapi-lint
make generated-check
make migration-test
make phase3-integration
make phase3-proxy-test
```

## Fase 4 — analytics e publicação materializados

O contrato `0.5.2`, o trecho da Fase 4 incorporado à migration consolidada
`000001_initial_schema`, as queries `sqlc` e o cliente TypeScript gerado
estabelecem analytics e publicação anônima. O papel `public_runtime` consulta
somente quatro views `security_barrier` em `public_data`; a API usa um
`PUBLIC_DATABASE_URL` separado para essa superfície e não recebe acesso às
tabelas-base.

Os indicadores são células pré-agregadas com catálogo tipado, política
`prototype-v1`, arredondamento mínimo de 10 e supressão com `k >= 10` e pelo
menos três acomodações. Preferências elegíveis usam somente respostas
estruturadas, consentidas e não sensíveis. Texto livre criptografado, IDs,
contagens de amostra e motivos de supressão não entram no contrato público.

O backend reconcilia presença incremental e integral, publica releases
imutáveis e serve a última válida durante falha. Os dashboards usam as
operações tipadas e preservam estados protegidos, indisponíveis e `N/A`. Os
targets abaixo validam contrato, migrations e privilégios em PostgreSQL real,
HTTP runtime, frontend, recomposição e cleanup Compose com fixtures fictícias:

```bash
make openapi-lint
make generated-check
make migration-test
make phase4-integration
make phase4-proxy-test
make phase4-full-stack
make phase4-benchmark
make local-demo-test
make local-demo-e2e
make phase4-remediation
```

Esses targets da Fase 4 são gates locais `PROTOTYPE_ONLY`. O benchmark
registra duas recomposições determinísticas de três anos, tempo, heap e
hardware, depois que o full-stack prova a preservação do último snapshot
válido. A remediação adicional prova build padrão sem identidade fake, seed
fresh e persistente sem sobrescrever colisões, duas execuções concorrentes
serializadas, rollover das fixtures, dashboard com presença/cobertura e o fluxo
operador → convite → grupo → pesquisa em Chromium, sem persistir authorities
no navegador ou no cache do Service Worker. Eles não autorizam deploy, release
ou uso com dados reais.

## Princípios não negociáveis

- Nenhuma obrigação municipal entra em produção sem fundamento jurídico formal.
- O comércio não recebe dados pessoais, respostas brutas ou exportações
  individualizadas.
- Perguntas publicadas são imutáveis; mudanças criam nova versão.
- Toda escrita repetível é idempotente.
- Nenhum segredo da FNRH é armazenado em texto puro.
- Falha na integração externa não perde o registro local.
- O total público nunca é somado diretamente pelo navegador a partir de registros
  individuais.
- Um administrador não pode transformar uma pergunta comum em dado sensível sem
  revisão do encarregado de dados.
