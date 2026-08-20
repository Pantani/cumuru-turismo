# ADR-045 — Camada de contexto externo

**Status:** aceito para `PROTOTYPE_ONLY`.

**Nota de numeração:** esta decisão nasceu como ADR-043 e foi renumerada
para 045 no rebase da onda 8. Enquanto ela era escrita, `main` avançou com
a [ADR-043](ADR-043-lista-publica-de-hospedagens.md) (lista pública de
hospedagens) e a
[ADR-044](ADR-044-importacao-de-calendario-da-plataforma-de-hospedagem.md)
(importação de calendário). Referência a "ADR-043" em material anterior a
este rebase, no contexto de contexto externo, é esta decisão.

**Relacionada:**
[ADR-028](ADR-028-politica-tecnica-de-publicacao-da-fase-4.md) e
[ADR-029](ADR-029-presenca-cobertura-e-forecast-da-fase-4.md), que definem a
série protegida e afirmam que o método não chama fonte externa;
[ADR-030](ADR-030-fronteira-publica-snapshots-e-http-da-fase-4.md), **emendado
por este documento** (§7);
[ADR-032](ADR-032-consolidacao-pre-lancamento-da-baseline-de-migrations.md),
**emendado por este documento** (§8);
[ADR-035](ADR-035-participacao-local-sem-cnpj-e-onboarding-prototipo.md), que
admite estabelecimento pessoa natural e por isso remove a defesa "dado de
empresa não é dado pessoal".

## Contexto

O produto passa a exibir dado de terceiro — clima, e no futuro maré — ao lado da
série que ele mesmo mede. São duas naturezas diferentes na mesma tela: uma é
medida pela plataforma, passa por supressão k-anônima e responde por uma política
de privacidade versionada; a outra é copiada de fora, não tem amostra, não é
suprimível e responde por uma licença.

A ADR-029 declara textualmente que o método "não divide por cobertura, não chama
fonte externa e não usa dado futuro no histórico". Introduzir chamada externa no
mesmo runtime, no mesmo worker e no mesmo painel sem decisão registrada
transformaria essa frase numa afirmação que ninguém mais consegue verificar
lendo o repositório. Esta ADR existe para reconstruir a verificabilidade.

A separação já é estrutural, não estilística, e o repositório a impõe hoje:

- `analytics.Cell` exige `SampleSize` e `AccommodationCount`
  (`apps/api/internal/analytics/policy.go:24-56`), que observação externa não
  tem;
- `analytics.metric_catalog` tem forma e política fechadas por CHECK, com
  `privacy_policy_version = 'prototype-v1'` na chave primária — fonte externa não
  tem versão de política de privacidade, tem licença, que é outra coisa;
- `public_data.metric_cells` obriga `published_value % 10 = 0` para observado e
  `% 5 = 0` para preferência. Arredondar temperatura ou altura de maré na base 10
  é absurdo; aquele arredondamento é controle de divulgação, não formatação.

As únicas formas de fundir as camadas seriam preencher `SampleSize` com valor
fictício — fraude — ou abrir um ramo de bypass em `ProtectCells`, que destrói a
invariante "toda célula publicada passou pelas duas passagens de supressão".
Ambas ficam proibidas aqui.

## Decisões do usuário registradas nominalmente

Decisor: usuário, 2026-08-18. Registradas porque algumas divergem da
recomendação dos estudos, e a divergência precisa ter dono.

| # | Decisão |
| --- | --- |
| U-1 | O Cumuru é **não comercial**; o plano gratuito do Open-Meteo é elegível. `commercial_use_allowed` é dado em `external.sources`, não comentário. Mudar a natureza do produto reabre esta escolha |
| U-2 | Emenda ao ADR-030 **autorizada**: a rota pública lê views novas em `public_data` — `current_external_context` e `current_external_sources` (§7) |
| U-3 | *(substituída por U-7)* Cadastur entraria como universo publicado, contra a recomendação do estudo de privacidade, como risco aceito |
| U-4 | Maré permanece `unavailable` com `reason_code = constants_not_imported` até resposta do CHM. **Nunca** fabricar constante, nunca usar `sea_level_height_msl` como maré |
| U-5 | Fonte upstream única na entrega 1: Open-Meteo Forecast + Archive |
| U-6 | Correção do `make harness-validate` primeiro, em commit isolado — feita em `77c5a64` |
| **U-7** | **Cadastur entra como atribuição e link, sem contagem calculada pela plataforma.** Substitui U-3 e converge com a recomendação M5 do estudo de privacidade. Elimina o differencing na origem: sem universo publicado, não há complemento a deduzir |

Consequência direta de U-7: `coverage_ratio_percent` permanece em passos de 5 pp,
`PublishedCoverage` não é tocado, e as quatro rotas públicas de analytics mantêm
corpo e ETag idênticos. Engrossar o passo da cobertura teria exigido emenda à
ADR-028, alteração da camada protegida e quebra do próprio critério de aceite
desta onda; U-7 torna a questão sem objeto.

## Decisão

### 1. Direcionalidade: dado externo é sumidouro, nunca insumo

Fonte externa é **contexto**. Não entra em `analytics`, em
`public_data.metric_cells`, no forecast nem na cobertura, em nenhuma hipótese e
sob nenhuma pressão de produto.

A garantia é **ACL do PostgreSQL**, não convenção de código:

- o papel que reconcilia presença, cobertura e forecast — que é
  **`worker_runtime`**, e não `app_runtime`: ele detém a escrita em
  `analytics.*` e `public_data.*` e a leitura colunar de `core`/`survey`
  (`000001_initial_schema.up.sql:1862-1939`) — não recebe **nenhum** privilégio
  no schema `external`;
- a ingestão externa corre sob papel **novo e dedicado, `external_runtime`**
  (login `cumuru_external`), que escreve em `external` e **não** recebe `SELECT`
  em `core`, `survey`, `analytics` nem `public_data`.

  O papel é novo porque nenhum dos existentes serve: reusar `worker_runtime`
  violaria a primeira regra desta lista e tornaria a asserção negativa de ACL
  impossível de passar, já que ele lê os quatro schemas hoje. Os dois papéis
  convivem no mesmo processo do worker, em pools distintos: §6 fixa *onde* a
  chamada de rede sai — o worker —, e esta regra fixa *com que privilégio* ela
  grava. São eixos diferentes e não se contradizem;
- `analytics.aggregate_eligible_preferences` permanece `SECURITY DEFINER` com
  `search_path = pg_catalog`, e por isso é estruturalmente incapaz de enxergar
  `external`.

`policy.go`, `presence*.go`, `publication.go` e `coverage.go` ficam **congelados**
nesta onda. Nenhum CHECK de `metric_catalog` ou `metric_cells` é relaxado.
Qualquer tarefa que proponha editá-los para acomodar dado externo é `BLOCKER`.

### 2. Superfície e identidade de cache separadas

Endpoint próprio, ETag própria. É **proibido** fundir contexto externo em
`/public/summary`, `/public/presence`, `/public/preferences` ou
`/public/methodology`.

O motivo é concreto, não estético: payload compartilhado faria a atualização
externa invalidar o ETag do snapshot protegido, e a cadência de invalidação
viraria oráculo do horário de publicação da release.

### 3. Independência de falha nos dois sentidos

Fonte externa velha ou indisponível não bloqueia, não reverte e não atrasa
release de analytics. Release de analytics ausente — o `503` do ADR-030 — não é
compensada exibindo contexto externo como se fosse conteúdo do painel. Sem
fallback cruzado, em nenhuma direção.

Fonte morta é `200` com aquele card `unavailable`. `503` só quando o documento
inteiro não puder ser montado. Uma fonte caída não derruba a aba.

### 4. Proibição de aritmética atravessando a fronteira, inclusive no cliente

Nenhuma superfície — API, payload, componente ou cálculo em navegador — produz
razão, diferença, percentual ou "cobertura sobre universo" que combine número
externo com célula protegida ou com `ratio` de cobertura.

A proibição vale explicitamente no cliente porque é ali que a brecha apareceria:
API pura, UI reconstituindo a quantidade suprimida. Teste negativo obrigatório
nos dois lados.

### 5. Universo nominal externo não é publicado

Série que descreva estabelecimentos nominalmente — Cadastur é o caso — tem
`public_exposable = false`, com **CHECK e ACL**, não convenção de código. A view
pública filtra por essa coluna, e é a view que recebe o `GRANT`.

Por U-7, o Cadastur aparece na plataforma como **fonte creditada e link**, com
`publisher`, `license_code`, `attribution_text` e `terms_url`, e **sem nenhuma
contagem calculada pela plataforma**. Não há série de universo publicada, não há
card com valor e não há série temporal do universo.

O diagnóstico que motivou a regra fica registrado: com `coverage_ratio_percent`
público, publicar o universo `N` entregaria `não_reportantes ≈ N − round(r×N)`.
Numa vila com conjunto enumerável de pousadas, isso individualiza **quem não
reporta** — exatamente a divulgação complementar que
`applyComplementarySuppression` impede dentro de uma camada, executada *entre*
camadas, onde a supressão por camada é cega. Limiar não resolve: `N = 10` com
cobertura 40% entrega a divisão 4/6 num conjunto que se percorre a pé. Agravante:
por ADR-035 o estabelecimento pode ser pessoa natural.

### 6. Egresso

O que vaza numa integração externa não é coordenada nem IP do servidor — é
**cadência por requisição** e **parâmetro derivado do usuário**. Regra:

- egresso **só no worker**, nunca no caminho de requisição. O pool público e o
  runtime de API não fazem chamada de rede externa;
- **nada de chamada a partir do navegador.** `deploy/nginx/default.conf` mantém
  `connect-src 'self'` e `img-src 'self' data:`. A CSP **não é relaxada** nesta
  onda; logo e ícone de fonte são servidos por `self` ou embutidos como `data:`;
- URL, host, path e conjunto de parâmetros são **constantes** de código ou
  configuração. **Nenhum byte vindo de requisição HTTP entra na URL externa.**
  Isso fecha vazamento de sinal e SSRF na mesma trava. Coordenadas fixas,
  arredondadas a 2 casas decimais (~1,1 km);
- somente `https`; allowlist de host em **configuração, não em banco**, para que
  um `UPDATE` não vire SSRF; redirecionamento para host fora da allowlist é
  recusado; teto de tamanho de resposta; timeout próprio e **orçamento de lote
  dedicado** — `DATABASE_TIMEOUT` é de requisição e não serve a ciclo de
  ingestão;
- agenda ancorada no calendário civil `America/Bahia`, constante. Jitter, se
  houver, deriva de semente constante — nunca de tráfego, de contagem de estadias
  ou de horário de acesso;
- `User-Agent` institucional **fixo**: nome do projeto, URL institucional e
  contato. Nunca contém tenant, organização, acomodação, operador, sujeito OIDC
  nem versão que varie por instalação;
- o transport de saída **não honra proxy de ambiente** (`Proxy: nil`).
  `HTTPS_PROXY` roteia a allowlist inteira por um host escolhido por variável de
  ambiente, o que anula a allowlist: o destino efetivo deixa de ser o que o
  código declara e passa a ser o que o ambiente disser. Uma allowlist que um
  `export` contorna não é allowlist;
- log de egresso registra **apenas host, status, duração e resultado**. Nunca URL
  com query, corpo de resposta ou headers.

### 7. Proveniência por célula, e `data_mode` por card

Toda observação externa publicada carrega, no payload **e na tela, sem
interação**: fonte, `publisher`, rótulo da licença com link, texto de atribuição,
`terms_url`, `retrieved_at`, o período coberto e o `observed_at` de origem — o
instante a que o dado se refere, não o horário da nossa coleta.

Três regras que não são detalhe:

- **a proveniência é obrigatória também no ramo `unavailable`.** Fonte, licença e
  atribuição existem porque a fonte existe, não porque a requisição deu certo.
  Sem isso, um card que falha degrada em caixa anônima e a obrigação de
  atribuição CC-BY viraria condicional ao sucesso do fetch;
- **`attribution_text` vem do banco, não é montado em Go.** Texto de licença
  concatenado em código é texto de licença que ninguém revisa;
- **`data_mode` é por card.** Hoje `PublicMetadata.data_mode` é
  `const: prototype_fixtures`. Uma página que mistura clima **real** com presença
  **fictícia** sob um único rótulo global mente nas duas direções. `PublicMetadata`
  permanece intocado.

### Fronteira entre "sem numeral" e proveniência obrigatória

As duas exigências desta seção colidem se lidas ao pé da letra sobre a mesma
região da tela: o card `unavailable` não pode conter numeral, e ao mesmo tempo
precisa exibir `retrieved_at`, período coberto e o rótulo da licença — que não
existem sem algarismo, e que são obrigatórios **justamente** quando a requisição
falha.

A regra, portanto, é territorial e não global. A proibição de numeral incide
sobre **o que o leitor lê como medida**: título, estado, motivo e o lugar do
valor. A proveniência é região declarada e exceção explícita, marcada no DOM
(`[data-card-provenance]`), e é varrida por outras asserções — presença de
fonte, licença com link, timestamp de origem e `data_mode`, tudo no render
inicial e fora de `<details>`.

O que a proibição protege é a confusão entre ausência e zero. Uma data de coleta
não é confundível com a medida; um `0` no lugar do valor é. Ler o critério sobre
o card inteiro tornaria a obrigação de atribuição impossível de cumprir, e o
modo previsível de "resolver" isso seria esconder a proveniência — exatamente o
que a licença CC-BY proíbe.

O status do card é `published` ou `unavailable`. **Nunca `protected`** — essa
palavra, na série protegida, significa "reprovado pelo limiar k-anônimo", e
reusá-la aqui afirmaria que a supressão rodou sobre dado que não passou por ela.

`reason_code` é lista fechada: `source_unavailable`, `source_rate_limited`,
`source_not_licensed`, `source_data_missing`, `constants_not_imported`,
`stale_beyond_declared_lag`. Sem texto livre. Nunca valor inventado, nunca zero,
nunca último valor conhecido servido em silêncio — servir o último valor
conhecido é `published` com `retrieved_at` antigo, e o leitor vê a defasagem no
corpo.

### 8. Maré

Só é lícito chamar de **maré**, e só é lícito publicar **horário de preamar e
baixa-mar**, o que for calculado a partir das constantes harmônicas de uma
estação maregráfica nomeada do CHM.

O obstáculo não é técnico nem de disponibilidade: as fichas F-41 são públicas e
gratuitas e entregam o datum vertical, mas **não** as constantes harmônicas.
As constantes ficam no BNDO, só por solicitação, e a página declara que "poderá
ser exigida a assinatura de um Termo de Compromisso, que compromete a instituição
ou indivíduo a não repassar os dados a terceiro". Publicar predição derivada num
painel público é exatamente repasse a terceiro. **É gate de direitos, não de
dado**, e destrava com ato humano: pedido a `chm.bndo@marinha.mil.br` das
constantes da estação 40154 (Porto Seguro, ~75 km ao norte de Cumuruxatiba),
declarando finalidade de painel público municipal, natureza não comercial e
pedido expresso de autorização para publicar predições derivadas com atribuição
ao CHM.

Até lá, e por U-4, o card nasce `unavailable` com
`reason_code = constants_not_imported`.

É **vetado** derivar o card público de maré de `sea_level_height_msl` do
Open-Meteo Marine: resolução ~0,08° (~8 km), e a própria documentação da fonte
declara acurácia limitada em área costeira e veta uso para navegação costeira.
Maré baixa é quando se caminha no recife; um horário errado não é imprecisão
estatística, é uma pessoa na água na hora errada, num painel com marca da
Prefeitura de Prado.

Se o dado de modelo global for exibido algum dia, ele **não se chama maré**:
rótulo obrigatório "altura de superfície do mar modelada", com resolução e
ressalva da fonte inline no card, sem extremos rotulados e **sem curva com
máximos e mínimos marcados** — uma curva suave sem rótulo ainda comunica horário
de extremo ao olho, e o rótulo não desfaz isso.

### 9. Retenção e não-redistribuição do bruto

Resposta bruta de terceiro é **cache com prazo, não acervo**, e não é exposta
pela API. Retenção é por série (`retention_days`), varrida pelo mesmo padrão
bounded já usado em `platform.cleanup_expired_operational_records`. Dado externo
não é dado pessoal, então retenção aqui é decisão de custo e de licença, não de
privacidade. Termos de fonte podem restringir redistribuição mesmo de dado não
pessoal, e restringem.

### 10. Disciplina de emenda

Acrescentar fonte, card, campo ou geografia exige **emenda desta ADR e nova
revisão de privacidade**, espelhando a regra que a ADR-028 já impõe a métrica,
período, dimensão e categoria. Fonte sem licença identificada por URL versionada,
texto de atribuição redigido e limite de uso declarado **não renderiza card
público**, mesmo que a ingestão funcione. `PROTOTYPE_ONLY` não dispensa esse
gate, porque atribuição CC-BY não depende de o software ser protótipo.

## Emenda ao ADR-030

O ADR-030 diz, na validação do pool público, "acesso positivo somente às quatro
views". Por U-2, passa a ser:

> O papel `public_runtime` recebe `SELECT` explícito **apenas** nas views
> `public_data.current_summary`, `public_data.current_presence`,
> `public_data.current_preferences`, `public_data.current_methodology` e
> `public_data.current_external_context` e
> `public_data.current_external_sources`.

São **duas**, e não uma, por uma razão que só apareceu em runtime: por U-7 o
Cadastur é creditado **sem card**. Uma view com forma de card não tem linha onde
ele caiba, e ler `external.sources` direto pelo papel público é impossível por
construção — a regra §1 nega `USAGE` em `external` a esse papel, e a própria
`000005` aborta se o privilégio existir. A lista de créditos precisa, portanto,
da sua própria view em `public_data`. A alternativa seria abrir `external` ao
papel público, que é exatamente o que esta ADR proíbe.

Três consequências que o texto anterior não cobria, e que a emenda fixa:

1. **As views novas moram em `public_data`, não em `external`.** Elas leem `external`
   sob os privilégios do *dono da view*, então `public_runtime` continua **sem
   nenhum privilégio no schema `external`** e sem `USAGE` nele. O
   `search_path` do pool público permanece `pg_catalog, public_data`, e
   `publicRuntimeSearchPath` (`store/public_pool.go:14`) **não é alterado**. Isso
   evita, por desenho, o modo de falha em que ler `external` pelo pool público
   derruba o startup da API inteira em runtime.
2. **`external` entra na lista de schemas varridos pela validação de sessão, do
   lado negativo.** `application_schemas`, em
   `store/queries/public.sql`, hoje enumera `identity`, `core`, `survey`,
   `analytics`, `public_data` e `platform`. Sem `external` nessa lista, um grant
   indevido a `public_runtime` em `external.*` **não seria detectado** — a
   asserção negativa é cega ao que não varre. `external` é acrescentado a
   `application_schemas`; `expected_schema_usage` continua só com `public_data`;
   `expected_select` ganha as duas views novas e nada mais.
3. **A allowlist de views deixa de ser afirmação e volta a ser teste.**
   `test-migrations.sh:1349-1381` conta views de `public_data` e itera por nome;
   como filtra por nome, uma view nova passaria sem quebrar nada e a afirmação
   do ADR-030 ficaria falsa em silêncio. A lista positiva passa a incluir
   `current_external_context` e `current_external_sources`, e a negativa é
   reafirmada. Idem
   `test-local-restore.sh:157,327`.

A auditoria 6A é **invalidada** por esta emenda, no mesmo padrão já aplicado pela
Fase 7: a fronteira pública mudou, e a auditoria que a carimbou descreve um
estado anterior.

## Emenda ao ADR-032

O ADR-032 autoriza squash da baseline enquanto "nenhum banco persistente foi
montado ou lançado" — premissa sobre o mundo que não se verifica a partir do
repositório. A árvore já traz `000002_presence_history_window` como par separado,
ou seja, o regime append-only já foi retomado na prática.

A partir de `000003`, **append-only**. A camada externa entra como
`000005_external_context.{up,down}.sql`. Não há quarta onda de squash; se algum
dia houver, é emenda explícita ao ADR-032, não escolha silenciosa de uma onda.

O `down` **precisa dropar o schema `external`**, e a contagem de schemas
remanescentes em `test-migrations.sh:1660-1676` e `test-local-restore.sh:141-151`
— hoje uma lista fixa sem `external` — precisa incluí-lo. Sem isso, um `down`
incompleto deixa schema órfão e o teste passa em silêncio.

## Consequências

- a fronteira entre medido e copiado é aplicada pelo PostgreSQL e por CHECK, não
  pelo mapper Go nem por revisão de código;
- o produto ganha o **primeiro egresso HTTP** da sua história para internet
  pública fora de OIDC e OTLP; toda a superfície de risco correspondente
  (SSRF, DNS rebinding, resposta gigante, redirect encadeado, vazamento de
  cadência) é nova e é endereçada em §6;
- as quatro rotas públicas de analytics permanecem byte a byte idênticas, ETag
  inclusive, com ou sem a camada externa, e isso é critério de aceite verificável
  com o upstream parado;
- o card de maré nasce e permanece `unavailable` até ato humano externo ao
  repositório; nenhum código o destrava;
- nenhum gate depende de rede pública: o upstream é stub HTTP local servindo
  fixtures gravadas;
- a promoção de "integração com calendários de eventos e clima" do **Backlog
  posterior** de `docs/09-roadmap-e-aceite.md` para entrega fica registrada aqui;
  a camada não é MVP e não deve ser apresentada como tal;
- permanecem `UNVERIFIED`: o texto exato dos termos do Open-Meteo e dos modelos a
  montante (ERA5/Copernicus), o enquadramento não comercial do limite de 10k
  req/dia, a compatibilidade entre republicar derivado CC-BY sob a
  `LicenseRef-Proprietary` da API, e se o egresso é permitido na infraestrutura
  alvo. Nenhum deles bloqueia a entrega sob `PROTOTYPE_ONLY` com atribuição
  embarcada por card, e todos precisam de resposta antes de release não protótipo;
- permanece `BLOCKED`: o Termo de Compromisso do CHM/BNDO.

### Limitação conhecida: a ingestão só roda em `local` e `test`

O provisionamento de papéis em deploy
(`deploy/ansible/roles/cumuru_host/templates/bootstrap-roles.sql.j2`) cria o
papel de grupo `external_runtime` — **obrigatório em todo ambiente**, porque a
migration `000005` aborta com `required database role is missing` se ele faltar,
e a cadeia de migrations roda em staging e produção. Mas **não** cria o login
`cumuru_external`.

O motivo é operacional e tem gatilho explícito: o login exigiria uma chave nova
(`database_external_password`) em
`deploy/ansible/roles/cumuru_host/files/bootstrap-runtime-secret.sh`, e
acrescentá-la à validação `jq -e` desse script **faria toda instalação existente
falhar**, porque o segredo já gravado no Secrets Manager não a contém. O script
só regenera o segredo quando a leitura falha; um segredo existente e válido
nunca ganha a chave sozinho.

Enquanto isso não for feito, `EXTERNAL_CONTEXT_ENABLED` é `false` fora de
`local` e `test` — o que o loader de configuração já recusa por conta própria —,
e a camada externa em staging e produção existe como schema, view e contrato,
sem nenhuma linha ingerida.

**Gatilho:** ligar `EXTERNAL_CONTEXT_ENABLED` fora de `local`/`test` exige,
**antes**, nesta ordem:

1. acrescentar `database_external_password` ao `jq -n` e à validação `jq -e` de
   `bootstrap-runtime-secret.sh`;
2. migrar o segredo de runtime já existente no Secrets Manager, acrescentando a
   chave sem rotacionar as demais;
3. criar o login `cumuru_external ... IN ROLE external_runtime` no
   `bootstrap-roles.sql.j2`, com `ALTER ROLE ... PASSWORD` no padrão dos quatro
   existentes, e `EXTERNAL_DATABASE_URL` em `runtime.env.j2`;
4. rever a decisão de `PROTOTYPE_ONLY`, porque ingerir fora de local|test
   significa publicar dado de terceiro num ambiente real, e §10 exige licença
   identificada e limite de uso declarado antes disso.

É migração de segredo, não linha de template, e por isso ficou fora da onda 8.
