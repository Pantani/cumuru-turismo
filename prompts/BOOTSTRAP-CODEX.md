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
- autocadastro generalizado com aprovação do estabelecimento e proveniência de
  estadia;
- ativação de conta de acomodação por capability de uso único.

Implemente:

1. Ativação da acomodação.
   A ADR-035 já implementa autoprovisionamento: o principal cria a acomodação
   e vira manager. Esta fase não inventa um papel de administrador
   provisionador. Ela acrescenta a emissão de uma capability de ativação de uso
   único para uma conta ainda sem credencial, exibida na própria tela como link
   e QR gerado no navegador. auth.accounts ganha o estado pending_activation:
   o CHECK do algoritmo passa a ser condicional a password_hash IS NOT NULL, e
   um CHECK novo amarra a ausência de hash exclusivamente a esse estado, para
   que conta active sem credencial continue impossível. Nenhum envio de e-mail
   pertence a esta fase.

2. Convite reutilizável por acomodação.
   Estenda core.invites com accommodation_id, torne stay_id nulo e acrescente o
   novo purpose. Reescreva invites_usage_valid como
   use_count >= 0 AND (max_uses IS NULL OR (max_uses > 0 AND use_count <=
   max_uses)), preserve o DEFAULT 1 da coluna e acrescente invites_target_valid
   reimpondo max_uses NOT NULL para o convite de estadia, de modo que
   ilimitado só exista no purpose novo.

   Corrija ConsumeInvite e FinalizeInviteSubmission: hoje comparam
   use_count < max_uses e use_count = max_uses, que viram UNKNOWN com nulo e
   fazem o UPDATE afetar zero linhas, transformando convite ilimitado em
   convite consumido. Essa é falha silenciosa: escreva primeiro o teste que a
   reproduz.

   Parametrize o purpose dentro do MAC e confira o purpose no Verify. A ADR-019
   promete HMAC(key_version, purpose || invite_id) mas o código fixa a
   constante; com dois purposes a promessa deixa de valer sem essa correção.

   O convite reutilizável é rotacionável e revogável; a rotação invalida o
   cartaz anterior. Mantenha o armazenamento por HMAC.

   O token trafega no fragmento da URL (/i#<token>), nunca no caminho. O nginx
   só desliga access log em ^~ /api/v1/invites/, e um prefixo novo gravaria em
   disco, permanentemente, o token de um cartaz de parede. Fragmento não chega
   a servidor, log, WAF ou CDN.

3. Autocadastro pelo link genérico.
   A submissão cria estadia com proveniência self_service e sem membership
   autora, usando o terceiro valor de collection_channel; preserve o nome da
   CHECK de ator existente com três ramos mutuamente exclusivos.

   O canal aberto coleta somente dados generalizados: faixa etária, UF e
   município IBGE, país, papel no grupo e datas. Nome, documento, e-mail e
   telefone são rejeitados neste canal. Não crie identity.visitor_identities
   nesta fase: a ADR-020 vetou a tabela, ela não existe na cadeia executável e
   construir cofre de cifra sem KMS não pertence a esta wave.

   Recuse role='minor' no canal self_service. O formulário é aberto e não
   autenticado; aceitar submissão sobre criança feita por estranho é
   indefensável.

   Exiba consentimento e aviso de privacidade versionados, porque não há
   operador intermediando a coleta.

4. Aprovação pelo estabelecimento.
   Fila por acomodação com aprovar e rejeitar, Idempotency-Key e If-Match.
   Acrescente uma operação própria de aprovação ao mapa allowedOperations de
   accommodation, disponível somente na acomodação ativa; não reaproveite
   update_stay, porque isso daria aprovação a qualquer operador com permissão
   de edição.

   A rejeição exige motivo de uma lista fechada, no precedente de
   questionnaire_change_reason_valid. Não aceite texto livre: audit é
   append-only sem UPDATE nem DELETE, e texto livre ali vira dado pessoal
   permanente.

   Pendência não aprovada expira em 72 horas, mesmo prazo do convite da Fase 2,
   para o produto não passar a ter duas noções de validade. Rejeição e
   expiração eliminam os dados generalizados do autocadastro e preservam apenas
   o fato auditável, sem valor pessoal. Eliminar só na rejeição permitiria
   retenção indefinida por inação. Prove a eliminação com teste que varra o
   banco por information_schema.columns, depois da rejeição e depois da
   expiração.

   Depois de aprovar, a acomodação pode emitir o convite nominal já existente
   da Fase 2 para que a própria pessoa preencha identidade quando a finalidade
   exigir. Não construa um segundo caminho de identidade.

5. Presença e publicação.
   Estadia não aprovada nunca entra em analytics.presence_days nem em qualquer
   agregado público. O filtro tem três pontos obrigatórios, não um:
   - projeção de approval_state em ListPresenceReconciliationStays, sem tocar o
     WHERE, senão presence_days fica órfão de estadia rejeitada;
   - presenceEligible() em analytics_repository.go, choke point compartilhado
     por addPresenceVisitors e incompleteStayCount;
   - predicado em analytics.aggregate_eligible_preferences, sem o qual o gate
     vale para presença e é falso para first_visit_share.
   Ao substituir a função SECURITY DEFINER, reafirme owner, search_path e ACLs,
   senão o dump diverge do esperado pelo teste de migrations.

6. Abuso.
   Reuse platform.rate_limit_buckets e acrescente proof-of-work emitido e
   verificado pela própria API, resolvido em JavaScript no cliente. Não use
   CAPTCHA nem serviço de terceiro: o formulário público não pode depender de
   rede externa e a ADR-019 proíbe terceiro na geração do QR. O desafio não usa
   cookie nem dado do titular.

   Registre gasto de nonce em tabela própria, guardando apenas HMAC do nonce e
   prazo, sem IP, token ou titular. Sem livro de nonces a mesma solução é
   reenviada dentro do TTL e o controle vale zero.

   Com o cartaz, o invite_id é comum a todos, então balde por (invite_id, IP)
   vira balde por bairro: sobrebloqueia hóspede em CGNAT e não toca atacante
   com rotação de IP. Derive dificuldade adaptativa de
   rate_limit_buckets.request_count em vez de endurecer o balde.

   Torne RequestHash com chave. Hoje é sha256 sem chave gravado em duas
   tabelas; com payload mais rico o digest vira oráculo de confirmação por
   palpite sobre dado já eliminado.

Use contracts/openapi.yaml como contrato e atualize-o na mesma mudança. A
migração é 000002; não reabra a baseline 000001, congelada pela ADR-032.
deploy/scripts/test-migrations.sh afirma literalmente o par 000001 e precisa
ser atualizado pelo owner de platform.

Não implemente envio de e-mail, FNRH, cofre de identidade, nem mudança do
dashboard público além do filtro de aprovação. Não acrescente valor ao enum
core.stay_status; a espera de aprovação é proveniência mais carimbo de
aprovação, não um novo estado da máquina de estadia.

O canal aberto coleta sem operador identificado e sem contato com o titular.
Enquanto o gate SELF_SERVICE_LEGAL_BASIS não estiver PASS em
_workspace/cumuru-bootstrap/phase-7/external-gates.env, a fase opera somente
com dados fictícios. Não conclua a fase como apta a dados reais sem esse gate.

O gate de complexidade deste repositório é 5/8, não 10.

Ao terminar, execute todos os gates aplicáveis e apresente PASS/FAIL/UNVERIFIED,
arquivos principais, migrações, riscos e próximo passo.
```

## Prompt 8 — Contexto externo

```text
Leia AGENTS.md e os documentos do blueprint. Confirme que as Fases 1 e 4 estão
verdes. Implemente somente a Fase 8 de
.agents/skills/cumuru-bootstrap/references/phase-matrix.md.

Esta fase não sucede a Fase 7; ela depende das Fases 1 e 4 e invalida qualquer
auditoria 6A anterior, porque muda a fronteira pública.

Leia a ADR-045 antes de qualquer patch. Ela já está aceita e é a lei da fase.
Ela emenda por escrito a ADR-030 (o acesso positivo do papel público passa de
quatro para seis views de public_data) e a ADR-032 (append-only a partir de
000005).

A restrição dura, não negociável: dado externo é contexto e nunca insumo. Ele
não entra em analytics, em public_data.metric_cells, no forecast nem na
cobertura. A estrutura já recusa: analytics.Cell exige SampleSize e
AccommodationCount, e metric_cells_values_valid exige published_value % 10 = 0,
absurdo para temperatura ou altura de maré. As únicas saídas seriam amostra
fictícia ou um ramo de bypass em ProtectCells. Ambas são proibidas.
analytics/policy.go, presence*.go, publication.go e coverage.go ficam
congelados.

Implemente:

1. Schema external e migration 000003, append-only.
   Seis relações: sources, series, observations, fetch_runs, tide_stations e
   tide_harmonics. Unidade e período moram em series, nunca na observação;
   mudar unidade é series_code novo, nunca ALTER in-place. A idempotência de
   escrita é (source_code, series_code, period_start, payload_digest): digest
   igual é no-op, digest diferente insere revision + 1, porque revisão de série
   é fato e ERA5 backfilla dado já publicado. fetch_runs é obrigatória: sem
   ela, indisponível é indistinguível de não rodou, e o card lê o outcome da
   última run, não a ausência de linhas.

2. Direcionalidade por ACL, não por convenção.
   A ingestão corre sob external_runtime, papel novo, login cumuru_external.
   worker_runtime é o papel de reconciliação de analytics e não recebe nenhum
   privilégio em external, nem USAGE. external_runtime não recebe SELECT em
   core, survey, analytics nem public_data. Os dois convivem no mesmo processo
   do worker, em pools distintos. Nenhuma FK atravessa external para core,
   survey, analytics ou public_data.

3. As views públicas da camada.
   public_data.current_external_context e public_data.current_external_sources
   moram em public_data, não em external. A primeira filtra public_exposable; a
   segunda existe porque o Cadastur é creditado sem card (U-7), e uma view com
   forma de card não tem linha onde ele caiba. Ela lê as tabelas base sob os privilégios do dono da
   view, então public_runtime continua sem qualquer privilégio em external e o
   search_path do pool público permanece pg_catalog, public_data.
   publicRuntimeSearchPath não é alterado. Acrescente external a
   application_schemas em store/queries/public.sql apenas do lado negativo: sem
   isso, um grant indevido ao papel público em external não seria detectado.

4. GET /public/context.
   Documento único, security vazio, sem seletor, tag external e
   x-cumuru-feature external-context. Reuse PublicCache, PublicEntityTag e o
   ETag existente com operation context; não crie um segundo algoritmo.
   PublicMetadata fica intocado: schema fechado com consts, e alargar o enum de
   unidade contaminaria a metadata da série protegida. data_mode é por card,
   porque uma página que mistura clima real com presença fictícia sob um rótulo
   global mente nas duas direções. Proveniência é obrigatória também no ramo
   unavailable: fonte, licença e atribuição existem porque a fonte existe, não
   porque o fetch deu certo. attribution_text vem do banco, não é montado em Go.
   Status é published ou unavailable, nunca protected. reason_code é lista
   fechada. Fonte morta é 200 com o card unavailable; 503 só quando o documento
   inteiro não puder ser montado.

5. Ingestão no worker.
   Fonte única: Open-Meteo Forecast e Archive, ponto único do município,
   agregado diário. Egresso só no worker, nunca no caminho de requisição. URL,
   host, path e parâmetros constantes: nenhum byte vindo de requisição HTTP
   entra na URL externa, o que fecha SSRF e vazamento de cadência na mesma
   trava. Somente https, allowlist de host em configuração e não em banco,
   redirect para fora da allowlist recusado, teto de tamanho de resposta,
   timeout próprio e orçamento de lote dedicado, porque DATABASE_TIMEOUT é de
   requisição. Agenda ancorada no calendário civil America/Bahia. User-Agent
   institucional fixo, sem tenant, organização, acomodação, operador ou sujeito
   OIDC. Log de egresso só com host, status e duração; nunca URL com query,
   corpo ou headers. Breaker e métricas por source_code e outcome.

6. Maré.
   O card nasce e permanece unavailable com reason_code
   constants_not_imported. As constantes harmônicas do CHM não têm API e o
   BNDO pode exigir Termo de Compromisso que veda repasse a terceiro; publicar
   predição derivada num painel público é exatamente repasse. É gate de
   direitos, não de dado. Não fabrique constante e não use
   sea_level_height_msl como maré: a própria doc da fonte declara acurácia
   limitada em área costeira. Maré baixa é quando se caminha no recife.

7. Cadastur.
   Entra apenas como fonte creditada e link, sem contagem calculada pela
   plataforma, sem card com valor e sem série de universo. Publicar o universo N
   ao lado da cobertura entregaria não_reportantes = N menos round(r vezes N), e
   numa vila com conjunto enumerável de pousadas isso individualiza quem não
   reporta. public_exposable é coluna com CHECK e ACL, e o Cadastur é o caso
   declarado false.

8. Aba web separada.
   Em apps/web/src/features/analytics, aba própria, sem eixo, escala ou legenda
   compartilhados com o gráfico de presença. Proibida aritmética atravessando a
   fronteira, inclusive no cliente: é ali que a brecha apareceria, com a API
   pura e a UI reconstituindo a quantidade suprimida. A CSP connect-src 'self'
   não é relaxada; logo de fonte é self ou data:. Com value nulo, o DOM não
   contém numeral, não desenha ponto no eixo e não desenha barra de altura zero.

Nenhum gate depende de rede pública: o upstream é stub HTTP local servindo
fixtures gravadas. Corrija as asserções literais que a migration 000003 quebra
em test-migrations.sh e test-local-restore.sh, incluindo a contagem de schemas
remanescentes após o down, que hoje não inclui external e deixaria schema órfão
passar em silêncio.

Critério central de aceite: com o stub parado, as quatro rotas públicas de
analytics mantêm corpo e ETag idênticos, /public/context responde 200 com o
card unavailable, e o DOM do card não contém nenhum numeral.

Toda tarefa que altera código roda make post-task-quality depois dos testes
estreitos e antes de DONE, e o artifact registra comando, exit code e o
marcador exato POST_TASK_QUALITY=PASS.
```
