# ADR-043 — Lista pública de hospedagens

**Status:** aceito para `PROTOTYPE_ONLY`.

**Relacionada:** [ADR-040](ADR-040-autocadastro-generalizado-e-aprovacao.md),
que vetou identidade no canal aberto do visitante;
[ADR-042](ADR-042-pedido-de-convite-da-hospedagem.md), que abriu a primeira
exceção nominal; e [ADR-030](ADR-030-fronteira-publica-snapshots-e-http-da-fase-4.md),
que fixou a fronteira pública de `public_data`.

## Contexto

O hóspede que chega em Cumuruxatiba não tem por onde descobrir quem hospeda. O
cadastro existe, mas só é legível por dentro: `GET /accommodations` filtra por
`core.memberships` e devolve exclusivamente o que a conta alcança. A capa
publica estatística agregada, que responde "quantas pessoas estão na praia" e
nunca "para quem eu ligo".

Duas coisas faltavam para a lista existir. A primeira é o telefone:
`core.accommodations` não tinha nenhum campo de contato — o único telefone do
sistema vivia no pedido de acesso da ADR-042, que é eliminado na recusa e na
expiração, justamente por ser dado de contato sem finalidade permanente.

A segunda é a base legal. Publicar o telefone de uma pousada com CNPJ é uma
coisa; publicar o telefone da casa de família da categoria `family_hosting` é
publicar telefone de pessoa natural. Estar cadastrada no Observatório não é
consentimento para aparecer numa lista aberta, e tratar cadastro como permissão
faria a plataforma publicar contato de quem só quis registrar estadia.

## Decisão

- a publicação é **ato da hospedagem**, não consequência do cadastro:
  `public_listing_enabled` nasce falso, exige telefone e carimba
  `public_listing_consented_at`. Republicar o que já estava publicado não
  reescreve o carimbo; despublicar o elimina;
- **retirar o consentimento tem efeito imediato**. A lista é lida do cadastro
  vivo, na mesma transação que a operadora acabou de escrever — não há snapshot,
  fila nem janela de propagação entre desmarcar e sumir;
- a lista publica **nome, categoria, localidade, capacidade, telefone, WhatsApp
  e site**, e nada além disso. A consulta projeta exatamente essas colunas e
  filtra `public_listing_enabled` e `status = 'active'` **no banco**: a rota é
  aberta, e uma consulta que dependesse do handler para recortar publicaria o
  cadastro inteiro no primeiro erro de quem a chamasse;
- **não passa por `public_data`**, apesar de ser rota aberta. Aquele schema
  guarda release estatístico imutável, protegido por supressão de célula e
  mínimo de acomodações reportantes. A lista é o oposto: nominal de propósito,
  viva por necessidade, e não é amostra de nada. Colocá-la lá faria a política
  de supressão parecer aplicável a um contato consentido, e obrigaria a
  publicação a esperar um snapshot para retirar um consentimento retirado. O
  precedente é o canal aberto da ADR-042, que também lê `core` por
  `app_runtime` sem sessão;
- é a **segunda exceção nominal** à ADR-040, no mesmo limite estreito da
  ADR-042: só se publica quem se descreve a si mesmo. Nenhum dado de hóspede
  entra nesta superfície, em nenhuma forma;
- o telefone é armazenado em **E.164** com constraint no banco, porque vira link
  discável e link de WhatsApp, e nenhum dos dois aceita separador. O site exige
  `https://`;
- o documento é **único e sem cursor**: a lista é municipal, cabe numa resposta
  e é cacheável por inteiro (`max-age=300`, ETag forte). Filtrar por categoria e
  buscar por nome são trabalho do cliente. O teto de leitura é 1000 linhas e a
  leitura **recusa** acima dele, em vez de truncar em silêncio.

## Consequências

- O hóspede passa a ter onde procurar, e a hospedagem passa a ter um motivo
  próprio para manter o cadastro em dia.
- O Observatório publica contato de pessoa natural. É dado pessoal, com base
  legal em consentimento específico e revogável a um clique, e a revogação é
  técnica além de formal: a linha some da lista na mesma transação.
- A lista não é intermediação: não há reserva, disponibilidade, preço nem
  avaliação. Publicar qualquer um dos quatro transformaria o Observatório em
  agência, o que exige decisão de governança que esta ADR não toma.
- O canal continua sem verificação: ninguém confere se o telefone publicado é
  mesmo daquela hospedagem. Quem publica é uma conta ativa vinculada à
  acomodação, o que é mais do que a ADR-042 tem, e ainda assim não é prova.
- `public_data` permanece com exatamente quatro views para `public_runtime`, e
  a validação de ACL do pool público não muda.
