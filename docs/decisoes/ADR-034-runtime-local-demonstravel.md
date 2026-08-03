# ADR-034 — Runtime local demonstrável

**Status:** aceito.

## Contexto

A stack criada por `make up` aplicava migrations e iniciava PostgreSQL, API,
worker e web, mas não criava nenhuma fixture de negócio. O catálogo técnico da
Fase 4 existia sem questionário ou mappings; por isso o worker falhava fechado
antes da primeira publicação e os quatro endpoints públicos retornavam `503`.

O fake OIDC do backend já era restrito a `local|test`, porém o frontend nunca
iniciava uma sessão em memória. As rotas de registro e pesquisa também eram
expostas na navegação sem as capabilities que só são obtidas na etapa
anterior. O smoke aceitava uma lista vazia de acomodações e não consultava a
superfície pública, produzindo sucesso para uma aplicação inutilizável.

Não é correto inserir dados de demonstração na migration canônica, pois a mesma
baseline atende ambientes não locais. Também não é suficiente usar
`docker-entrypoint-initdb.d`, que só executa na criação do volume.

## Decisão

O runtime local usa um overlay Compose explícito com um bootstrap
`PROTOTYPE_ONLY`, executado depois de `migrate` e antes da API e do worker. O
bootstrap:

- é um comando Go compilado junto com a API, e não um script SQL;
- declara as fixtures como tipos e valores Go e atravessa serviços de domínio
  para questionário, estadias, grupos, transições e respostas;
- usa uma fronteira de provisionamento local, implementada pelo repositório,
  somente para a organização, acomodações, memberships e mappings que não têm
  caso de uso público de criação;
- grava somente IDs reservados e dados declaradamente fictícios;
- é transacional no provisionamento, idempotente nos casos de uso e não
  destrutivo para linhas fora do namespace reservado;
- reconcilia datas civis das próprias fixtures para manter as janelas
  demonstráveis em `America/Bahia`;
- cria o questionário publicado e os mappings necessários à primeira
  publicação, além de dados suficientes para atravessar as proteções
  estatísticas;
- não executa `psql`, não monta arquivo `.sql` de fixtures e não pertence às
  migrations, ao schema lógico ou ao OpenAPI.

O build web do overlay local recebe um sinal de modo demo e o token público do
verificador fake já existente. Esse identificador público e não institucional
existe somente no bundle da variante local; o build padrão o rejeita. O
`AuthSessionProvider` só inicia a sessão quando sinal, identificador e host
loopback concordam. O estado autenticado permanece em memória e não existe
formulário de login, conta local, cookie, refresh token, fallback OIDC,
`localStorage`, `sessionStorage` ou IndexedDB. Recarregar a variante local
recria a sessão fictícia por decisão de usabilidade, sem transformá-la em
credencial válida fora de `local|test`. A interface mostra permanentemente os
marcadores “Sessão fictícia local” e `PROTOTYPE_ONLY` enquanto essa sessão
estiver ativa. Builds sem o sinal e o identificador, servidos em host não
loopback ou com configuração parcial continuam fail-closed, e o bundle de
produção não contém a fixture.

A navegação mostra registro e pesquisa somente quando as capabilities
correspondentes existem. A jornada do operador compartilha a acomodação e a
estadia selecionadas entre os passos, oferece abertura local do convite sem
renderizar a capability e conduz explicitamente do registro concluído para a
pesquisa.

`make up` só é considerado pronto quando a publicação pública responde. O
smoke exige os quatro endpoints públicos, uma acomodação acessível ao operador
e shapes públicos permitidos; uma lista vazia ou qualquer `4xx/5xx` falha.

## Complemento de convergência

A mesma implementação canônica da fixture é usada pelo runtime local, pelo
teste PostgreSQL de idempotência e pelo full-stack da Fase 4. O full-stack pode
adicionar somente canários próprios depois do bootstrap; não mantém uma segunda
definição SQL de questionário ou mappings.

O bootstrap aceita `APP_ENV=local|test`, sempre com `OIDC_MODE=fake` e
`LOCAL_DEMO_ENABLED=true`. Cada transação de provisionamento adquire um
advisory lock. Inserts de organização, acomodação, membership e mapping não
atualizam conflitos: após o insert, o repositório compara a linha completa com
a definição esperada e falha fechado diante de qualquer colisão.

O código de retenção canônico da fixture é
`prototype_aggregate_only`, compatível com a validação de códigos do domínio. A
grafia `prototype-aggregate-only`, produzida pelo antigo seed SQL isolado, é
reconhecida somente ao comparar uma versão de questionário já publicada;
nenhuma outra divergência é tolerada e conteúdo publicado nunca é reescrito.

As cohorts históricas recebem identidade por mês civil, pois as preferências
usam o último mês completo. As três estadias correntes mantêm identidade
estável e têm apenas o período planejado reconciliado pelo caso de uso de
atualização, que registra auditoria e outbox. Assim, repetição no mesmo período
é idempotente, o rollover não reutiliza uma chave com outro payload e dados
fora do namespace reservado permanecem intocados.

## Consequências

- um banco local limpo ou persistente converge para uma demonstração útil sem
  apagar dados criados pelo usuário;
- o dataset de demonstração fica sujeito às mesmas validações, transições,
  idempotência, auditoria e outbox dos casos de uso exercitados;
- a mudança do dia civil pode atualizar apenas as fixtures reservadas e gerar
  uma nova publicação;
- testes full-stack canônicos continuam isolados do seed local porque não usam
  o overlay;
- a separação entre OIDC real e fake permanece aplicada no backend e no build;
- capabilities continuam necessárias e não podem ser obtidas abrindo rotas
  fora de ordem;
- o ambiente local não representa dados oficiais, produção, homologação ou
  autorização municipal;
- FNRH, OIDC institucional, dados reais, deploy, release e piloto permanecem
  bloqueados.
