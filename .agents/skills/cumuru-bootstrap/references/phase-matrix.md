# Matriz de fases do Cumuru

## Regra global

Implementações seguem:

```text
Fase 0 externa
  → Fase 1 Fundação
  → Fase 2 Hospedagens e estadias
  → Fase 3 Questionário
  → Fase 4 Analytics e dashboard
  → Fase 5 FNRH
  → 6A Auditoria de prontidão
  → 6B Piloto operacional

Fase 7 Autoatendimento e aprovação depende de 2 e 3, não de 6.
```

O Prompt 6 executa **6A**, não conclui o piloto de 30–60 dias descrito como
Fase 6 no roadmap. O piloto 6B depende de decisões e evidências externas.

A Fase 7 é uma wave de produto, não uma etapa do roadmap original. Seu número é
posterior por ordem de criação, não por dependência: ela exige apenas as Fases 2
e 3 `PASS`. Concluir a Fase 7 invalida qualquer 6A anterior, que passa a
`UNVERIFIED` até nova auditoria.

Todos os estudos podem ser antecipados em modo read-only. Somente uma fase pode
estar em implementação.

## Fase 0 — Governança e descoberta

### Evidências externas

- patrocinador e controlador;
- mapa de hospedagens e capacidade;
- fluxo FNRH atual;
- parecer jurídico;
- inventário de dados;
- RIPD inicial;
- política de publicação;
- OIDC e infraestrutura selecionados.

Sem todas as evidências, marque `PROTOTYPE_ONLY`. Nenhum agente pode inventar
aprovação, documento ou escolha institucional.

## Fase 1 — Fundação

### Dependências

- Blueprint completo lido.
- Fase 0 `PASS` ou modo explícito `PROTOTYPE_ONLY`.

### Estudo paralelo

1. Backend/dados: módulo Go, banco, migrações, OIDC, health e observabilidade.
2. Frontend: React/Vite, TypeScript strict, roteamento, cliente gerado e a11y.
3. Plataforma/risco: CI, SBOM, scanners, compose, logs, contratos e isolamento.

### Ownership de implementação

| Owner | Escopo |
| --- | --- |
| backend | `apps/api/**`, exceto migrations e gerados compartilhados |
| frontend | `apps/web/**`, exceto `src/generated/**` |
| platform | `contracts/**`, migrations, gerados, `deploy/**`, CI, compose e raiz delegada |

Sem Git, execute esses owners serialmente. O owner de platform gera o cliente
somente após congelar o OpenAPI.

### Gate

- sobe do zero com um comando documentado;
- migração do zero e política de rollback/forward;
- OIDC rejeita tokens inválidos;
- isolamento negativo por organização;
- cliente OpenAPI reproduzível;
- logs sem payload pessoal;
- CI, SBOM e scanners;
- gates Go/Node aplicáveis.

### Fora de escopo

Estadias, questionário, dashboard e FNRH.

## Fase 2 — Hospedagens e estadias

### Dependência

Fase 1 `PASS`.

### Estudo paralelo

1. Domínio/DB: estados, tenant, identidade, invites, outbox e auditoria.
2. Contrato/UI: endpoints, ETags, idempotência, offline e formulários.
3. Ameaças/QA: repetição, concorrência, isolamento, logs e datas.

### Ordem de escrita

1. Supervisor congela as decisões de contrato, migration plan e testes.
2. Platform materializa OpenAPI/migrations e executa geração.
3. Backend implementa domínio e handlers contra o contrato congelado.
4. Frontend implementa fluxo e IndexedDB sem tocar gerados.
5. QA cruza OpenAPI → handler → cliente → hooks/forms.

Backend e frontend só podem rodar juntos quando o contrato estiver congelado e
o SCM/ownership permitir.

### Gate

- replay idêntico não duplica visitante e reproduz resposta;
- mesma chave com corpo diferente retorna `409`;
- concorrência em processamento é controlada;
- `ETag`/`If-Match` e `412`;
- isolamento A/B;
- convite salvo como HMAC;
- transições válidas e inválidas;
- cancelado/no-show fora da presença;
- `[entrada, saída)`;
- IndexedDB, nunca `localStorage`;
- mutação, outbox e auditoria atômicas.

### Fora de escopo

Questionário, dashboard e FNRH.

## Fase 3 — Questionário

### Dependência

Fase 2 `PASS`.

### Estudo paralelo

1. Domínio: versões, estados, DSL e consentimentos.
2. UI/contrato: editor, revisão, publicação e submissão separada.
3. Privacidade/QA: classificação, texto livre, menores e semântica histórica.

### Ordem de escrita

Contrato e estados são congelados primeiro. Backend e frontend usam owners
disjuntos; migrações, OpenAPI e cliente gerado permanecem serializados.

### Gate

- `draft → privacy_review → approved → published → retired`;
- publicada é imutável;
- edição clona nova versão;
- respostas antigas mantêm versão/semântica;
- DSL rejeita código, regex arbitrária e referência externa;
- sensível não publica;
- texto livre não entra em analytics/publicação;
- recusa não bloqueia check-in;
- consentimento específico e versionado;
- autorização negativa e idempotência.

## Fase 4 — Analytics e dashboard

### Dependência

Fase 3 `PASS`. Presença depende da Fase 2 e preferências dependem da Fase 3;
portanto, a fase completa não começa antes das duas.

### Estudo paralelo

1. Analytics/DB: presença, reconciliação, cubos, grants e publicação.
2. Frontend: metodologia, cobertura, observado/previsão e a11y.
3. Privacidade/QA: supressão, differencing, arredondamento e payloads.

### Ordem de escrita

1. Congelar métricas, dimensões e política.
2. Backend implementa materialização e API.
3. Frontend implementa dashboard a partir do contrato congelado.
4. Custodiante aplica grants e geração.
5. QA testa differencing e acesso real do papel público.

### Gate

- reconciliação idempotente;
- presença `[entrada, saída)`;
- `public_runtime` lê somente `public_data`;
- payload público sem IDs, estabelecimento ou texto livre;
- supressão primária e complementar;
- differencing;
- arredondamento estável;
- cobertura e atualização;
- observado versus previsto;
- última versão válida em falha do agregador.

## Fase 5 — FNRH

### Dependências técnicas e de roadmap

- Fase 4 `PASS`;
- autorização formal;
- documentação oficial vigente;
- homologação disponível;
- regra aprovada de credencial por estabelecimento;
- mapeamento de campos aprovado.

O preflight externo pode ocorrer cedo. Implementação só ocorre depois da Fase 4
e com todos os gates externos `PASS`. Qualquer ausência resulta em `BLOCKED`.

Registre o estado sem segredos em
`_workspace/cumuru-bootstrap/phase-5/external-gates.env`:

```text
AUTHORIZATION=PASS
AUTHORIZATION_EVIDENCE=evidence/authorization.md
CURRENT_DOCS=PASS
CURRENT_DOCS_EVIDENCE=evidence/current-docs.md
HOMOLOGATION=PASS
HOMOLOGATION_EVIDENCE=evidence/homologation.md
PER_ACCOMMODATION_CREDENTIALS=PASS
PER_ACCOMMODATION_CREDENTIALS_EVIDENCE=evidence/credential-policy.md
APPROVED_MAPPING=PASS
APPROVED_MAPPING_EVIDENCE=evidence/approved-mapping.md
```

Cada referência é relativa a `_workspace/cumuru-bootstrap/`, deve apontar para
um arquivo local não vazio e não pode escapar do workspace. O arquivo de
evidência registra fonte, data, responsável e conclusão; o manifesto nunca
contém credencial ou payload.

### Estudo paralelo após elegibilidade

1. Adaptador/contrato oficial.
2. Segredos, KMS, retries, jobs e reconciliação.
3. UI de status e QA de repetição/indisponibilidade.

### Gate

- credencial nunca aparece em log ou leitura posterior;
- timeout não perde registro local;
- retry com jitter e dead letter;
- reenvio sem efeito duplicado;
- falha permanente acionável e sanitizada;
- contrato comprovado em homologação;
- credencial isolada por estabelecimento.

## 6A — Auditoria de prontidão

### Dependências

- Fases 1–4 `PASS`;
- Fase 5 `PASS` ou `BLOCKED` explicitamente documentada;
- nenhuma autorização de release é inferida.

### Revisão paralela

1. Segurança, privacidade, legal e retenção.
2. Contrato, dados, backend, frontend e acessibilidade.
3. Operação, restore, incidente, SLOs, FNRH e cobertura.

Correções de baixo risco são serializadas pelo owner original e revalidadas.

### Saída

Matriz `PASS/FAIL/UNVERIFIED/BLOCKED`, sem release.

## 6B — Piloto operacional

Não é executável apenas por código. Exige gates legais, dados reais
autorizados, 5–10 hospedagens, 30–60 dias, suporte, canal do titular, restore e
incidente simulados, métricas observadas e decisão do comitê.

## Fase 7 — Autoatendimento e aprovação

### Dependências

- Fase 2 `PASS`, porque o convite reutilizável estende `core.invites` e a
  estadia autocadastrada reusa a máquina de estados;
- Fase 3 `PASS`, porque o formulário aberto exibe consentimento versionado;
- governança `PASS` ou `PROTOTYPE_ONLY`;
- Fase 5 é irrelevante para a elegibilidade.

### Gate externo

O link genérico coleta sem operador identificado e sem contato com o titular.
Não coleta identidade: nome, documento, e-mail e telefone são rejeitados no
canal aberto, e a identidade só é preenchida pelo próprio titular depois da
aprovação, pelo convite nominal já existente da Fase 2. O risco remanescente
não é técnico: é base legal para coleta aberta sem intermediário. Registre em
`_workspace/cumuru-bootstrap/phase-7/external-gates.env`:

```text
SELF_SERVICE_LEGAL_BASIS=PASS
SELF_SERVICE_LEGAL_BASIS_EVIDENCE=evidence/self-service-legal-basis.md
```

As regras de contenção e formato são as mesmas da Fase 5. Sem esse gate, a fase
continua implementável e pode alcançar `PASS` técnico, porém apenas com dados
fictícios; o QA registra `REAL_DATA=BLOCKED` e nenhum agente declara a fase apta
a titulares reais.

### Estudo paralelo

1. Domínio/DB: proveniência de estadia, aprovação, expiração de pendência,
   convite por acomodação e ativação de conta.
2. Contrato/UI: endpoints de ativação, convite reutilizável, submissão aberta,
   fila de aprovação e QR no navegador.
3. Privacidade/segurança: consentimento sem operador, minimização do canal
   aberto, segunda prova, rate limit e ausência de token em log.

### Ordem de escrita

1. Supervisor congela contrato, migrações e política de aprovação.
2. Platform materializa OpenAPI, migrações e geração.
3. Backend implementa proveniência, aprovação e capabilities.
4. Frontend implementa ativação, formulário aberto e fila de aprovação.
5. QA cruza OpenAPI → handler → cliente → hooks e prova o filtro de presença.

### Ownership de implementação

| Owner | Escopo |
| --- | --- |
| backend | `apps/api/**`, exceto migrations e gerados compartilhados |
| frontend | `apps/web/**`, exceto `src/generated/**` |
| platform | `contracts/**`, migrations, gerados, `deploy/scripts/test-self-service-*.sh`, Makefile |

### Gate

- capability de ativação é de uso único, revogável e nunca reconstruível sem o
  keyring;
- conta pendente não autentica antes da ativação;
- convite reutilizável é revogável e a rotação invalida o token anterior;
- `max_uses` nulo significa ilimitado, `invites_usage_valid` nunca avalia o
  termo comparativo como `UNKNOWN`, e o convite de estadia continua obrigado a
  `max_uses NOT NULL`;
- `ConsumeInvite` e `FinalizeInviteSubmission` operam sobre convite ilimitado;
  o teste que reproduz a falha silenciosa vem antes da correção;
- o `purpose` entra no MAC e é conferido no `Verify`;
- o token trafega no fragmento da URL e nunca no caminho;
- token, URL e HMAC ausentes de log, trace, métrica, audit e outbox;
- estadia autocadastrada nasce sem membership autora e com proveniência
  `self_service`;
- o canal aberto rejeita nome, documento, e-mail, telefone e `role='minor'`;
- estadia não aprovada não aparece em `analytics.presence_days` nem em
  `public_data`, provado nos três pontos de filtro;
- aprovação e rejeição são idempotentes e respeitam `If-Match`;
- aprovação exige operação própria; operador com `update_stay` não aprova;
- rejeição exige motivo de lista fechada e é auditada sem valor pessoal;
- rejeição **e** expiração eliminam os dados do autocadastro, provado por
  varredura de `information_schema.columns` em ambos os caminhos;
- pendência expira em 72 horas e a expiração é auditada;
- consentimento e aviso de privacidade versionados no formulário aberto;
- rate limit e proof-of-work provados com `429` e `Retry-After`, sem serviço de
  terceiro e sem cookie, com gasto de nonce impedindo replay do desafio;
- `RequestHash` é com chave;
- isolamento A/B do convite reutilizável entre acomodações.

### Fora de escopo

Envio de e-mail, FNRH, cofre `identity.visitor_identities`, alteração do
dashboard público além do filtro de aprovação e qualquer novo valor no enum
`core.stay_status`.

## Fase 8 — Contexto externo

### Dependências

- Fase 1 `PASS`, porque a ingestão usa worker, config, migrations e telemetria;
- Fase 4 `PASS`, porque a disciplina de pool público e views do ADR-030 é o
  padrão a **imitar sem contaminar**, e a aba nova vive ao lado do painel;
- governança `PASS` ou `PROTOTYPE_ONLY`;
- Fases 2, 3, 5, 6 e 7 são irrelevantes para a elegibilidade.

A fase **invalida a auditoria 6A**, porque muda a fronteira pública: o acesso
positivo do papel público passa de quatro para seis views de `public_data`.

### Gate externo

Duas naturezas diferentes, e nenhuma delas é de disponibilidade de dado.

`EXTERNAL_SOURCE_LICENSE` é **por fonte**: licença identificada por URL
versionada, texto de atribuição redigido, limite de uso declarado e
compatibilidade com o uso pretendido. Fonte sem esse gate **não renderiza card
público**, mesmo que a ingestão funcione. `PROTOTYPE_ONLY` **não** o dispensa,
porque atribuição CC-BY não depende de o software ser protótipo.

`TIDE_HARMONIC_CONSTANTS` é **gate de direitos**, não de dado. As fichas F-41 do
CHM são públicas e entregam o datum vertical, mas não as constantes harmônicas.
As constantes ficam no BNDO, só por solicitação, e pode ser exigido Termo de
Compromisso que veda repasse a terceiro — publicar predição derivada num painel
público é exatamente repasse. Destrava com ato humano: pedido a
`chm.bndo@marinha.mil.br` das constantes da estação 40154 (Porto Seguro), com
pedido expresso de autorização para publicar predições derivadas.

Registre em `_workspace/cumuru-bootstrap/phase-8/external-gates.env`:

```text
EXTERNAL_SOURCE_LICENSE=PASS
EXTERNAL_SOURCE_LICENSE_EVIDENCE=evidence/external-source-license.md
TIDE_HARMONIC_CONSTANTS=PASS
TIDE_HARMONIC_CONSTANTS_EVIDENCE=evidence/tide-harmonic-constants.md
```

As regras de contenção e formato são as mesmas da Fase 5. Sem
`TIDE_HARMONIC_CONSTANTS`, a fase continua implementável e pode alcançar `PASS`
técnico; o card de maré nasce `unavailable` com
`reason_code = constants_not_imported` e o QA registra `TIDE_CARD=BLOCKED`.

### Estudo paralelo

1. Domínio/DB: schema `external`, revisão de série, `fetch_runs`, retenção e
   direcionalidade por ACL.
2. Contrato/UI: `GET /public/context`, três estados, proveniência por card,
   `data_mode` por card e as views novas em `public_data`.
3. Privacidade/segurança: egresso, differencing entre universo externo e
   cobertura, licenças e atribuição.

### Ordem de escrita

1. Supervisor congela ADR-045, contrato, migration e grants.
2. Platform materializa OpenAPI, `000003`, view, grants e geração.
3. Backend implementa ingestão, breaker, handler e stub local de fixtures.
4. Frontend implementa a aba separada e os três estados.
5. QA prova isolamento com o upstream parado.

### Ownership de implementação

| Owner | Escopo |
| --- | --- |
| platform | `contracts/**`, `apps/api/migrations/000003_*`, `database/schema.sql`, gerados, `store/queries/**`, config, provisionamento de papéis, `deploy/scripts/test-migrations.sh`, `test-local-restore.sh` |
| backend | `apps/api/internal/external/**`, worker, handler, stub de fixtures |
| frontend | `apps/web/src/features/analytics/**` (aba nova apenas) |

### Gate

- dado externo não entra em `analytics`, `public_data.metric_cells`, forecast
  nem cobertura; `policy.go`, `presence*.go`, `publication.go` e `coverage.go`
  ficam byte a byte iguais;
- `worker_runtime` **falha** ao ler `external.*` — é a prova da direcionalidade;
- `external_runtime` **falha** ao ler `core`, `survey`, `analytics` e
  `public_data`;
- `public_runtime` lê as **seis** views de `public_data` — as quatro da série
  protegida mais `current_external_context` e `current_external_sources` — e
  **falha** nas tabelas base de `external`;
  não recebe `USAGE` em `external` e `publicRuntimeSearchPath` não muda;
- nenhuma FK atravessa `external` → `core|survey|analytics|public_data`;
- o `down` dropa o schema `external`, **e a contagem de schemas remanescentes
  inclui `external`**, senão um `down` incompleto passa em silêncio;
- **com o stub parado, as quatro rotas públicas de analytics respondem corpo e
  `ETag` idênticos**; `/public/context` responde `200` com o card `unavailable`,
  nunca `503` por fonte de terceiro;
- reconciliação de analytics produz digest idêntico com `external` populado e
  vazio;
- ingestão idempotente: duas execuções sobre a mesma fixture produzem as mesmas
  linhas; digest diferente insere `revision + 1`;
- upstream com `500`, timeout, corpo truncado, JSON inválido e corpo acima do
  limite: o ciclo termina, marca `unavailable`, incrementa métrica, **não**
  aborta o worker e **não** apaga a última observação válida;
- host fora da allowlist, `http://` e redirect para host não permitido são
  recusados antes do DNS;
- nenhum parâmetro controlado pelo cliente alcança a URL externa;
- requisição ao painel **não** dispara chamada de egresso;
- `User-Agent` constante, sem tenant, organização, acomodação, operador nem
  sujeito OIDC;
- log de egresso sem URL com query, corpo ou headers;
- CSP servida mantém exatamente `connect-src 'self'` e `img-src 'self' data:`;
- payload não contém `coverage`, `ratio`, `sample_size`, `accommodation_count`
  nem contagem de acomodações;
- nenhuma view, handler ou componente calcula razão entre número externo e
  cobertura ou célula protegida — **inclusive no cliente**;
- card renderiza fonte, licença, timestamp de origem e `data_mode` próprio no
  render inicial, sem interação; ausência de qualquer um falha o teste;
- **com `value` nulo o DOM não contém numeral** na região de medida — título,
  estado, motivo e lugar do valor —, não desenha ponto no eixo nem barra de
  altura zero, e mostra o motivo em linguagem leiga. A região de proveniência
  (`[data-card-provenance]`) é exceção declarada: ela carrega data e rótulo de
  licença, que a ADR-045 §7 exige justamente no ramo indisponível;
- `stale` é visualmente distinto de `fresh`, com `observed_at` visível;
- a aba não compartilha eixo, escala ou legenda com o gráfico de presença;
- sem constantes do CHM, o card de maré é `unavailable` explícito, sem curva e
  sem horário de extremo;
- service worker não intercepta a rota nova;
- nenhum gate depende de rede pública.

### Fora de escopo

Card público de maré com valor, Wikimedia Pageviews, ANAC, IBGE, Observatório
do Turismo da Bahia, BrasilAPI feriados (primeiro incremento após a entrega 1),
contagem publicada derivada do Cadastur, qualquer dado externo em `analytics` ou
`public_data.metric_cells`, e retenção histórica longa na entrega 1.
