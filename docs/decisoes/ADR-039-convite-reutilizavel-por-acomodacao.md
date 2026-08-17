# ADR-039 — Convite reutilizável por acomodação

**Status:** aceito para `PROTOTYPE_ONLY`.

**Supersede parcialmente:** [ADR-019](ADR-019-convites-qr-e-rascunho-offline.md),
na parte em que "código curto e QR público reutilizável ficam fora da Fase 2 até
existir segunda prova aprovada". Todo o restante da ADR-019 permanece válido:
armazenamento por HMAC, ausência de token em log e trace, `404` uniforme para
token ausente, incorreto, expirado ou revogado, e rascunho local em IndexedDB.

## Contexto

A ADR-019 adiou o QR público reutilizável por um motivo específico e correto:
um cartaz de parede é uma credencial *bearer* permanente, e não havia segunda
prova definida. A Fase 7 supre essa lacuna com proof-of-work verificado pela
própria API e com aprovação humana obrigatória antes de qualquer efeito
estatístico.

Ao estender `core.invites` para um segundo `purpose`, três defeitos do desenho
atual passaram a importar.

O primeiro é o ciclo de vida. `ConsumeInvite` filtra `use_count < max_uses` e
`FinalizeInviteSubmission` filtra `use_count = max_uses`. Com `max_uses` nulo
representando uso ilimitado, ambas as comparações avaliam `UNKNOWN`, o `UPDATE`
afeta zero linhas e o convite ilimitado passa a se comportar como convite já
consumido. É falha silenciosa: qualquer suíte que exercite apenas convite
limitado continua verde.

O segundo é o MAC. A ADR-019 promete
`HMAC(key_version, purpose || invite_id_bytes)`, mas o código fixa a constante
`invitePurpose` dentro do MAC e o `Verify` não confere purpose algum. Com um
único purpose isso era inócuo. Com dois, a promessa da ADR deixa de valer.

O terceiro é o transporte. O nginx desliga access log apenas em
`^~ /api/v1/invites/`. Um prefixo novo gravaria em disco, permanentemente, o
token de um cartaz que fica exposto na parede por meses.

## Decisão

- `core.invites` ganha `accommodation_id`; `stay_id` passa a aceitar nulo e o
  novo `purpose` identifica o convite reutilizável;
- `invites_usage_valid` é reescrito como
  `use_count >= 0 AND (max_uses IS NULL OR (max_uses > 0 AND use_count <= max_uses))`,
  de modo que o termo comparativo nunca seja avaliado como `UNKNOWN`;
- `invites_target_valid` reimpõe `max_uses NOT NULL` para o convite de estadia;
  "ilimitado" existe somente no purpose novo;
- o `DEFAULT 1` da coluna é preservado, porque `CreateStayInvite` não lista a
  coluna e depende dele;
- `ConsumeInvite` e `FinalizeInviteSubmission` passam a operar sobre convite
  ilimitado, e o teste que reproduz a falha silenciosa é escrito antes da
  correção;
- o `purpose` entra no MAC e é conferido no `Verify`, cumprindo a promessa
  original da ADR-019;
- o token trafega no **fragmento** da URL (`/i#<token>`), nunca no caminho.
  Fragmento não é enviado ao servidor e portanto não alcança log, WAF nem CDN;
- do fragmento até a API, o token viaja no cabeçalho
  `X-Cumuru-Invite-Token`. Cabeçalho customizado força preflight CORS, a mesma
  propriedade que `Idempotency-Key` já explora para impedir form POST simples,
  e mantém o token fora da linha de requisição que servidores registram;
- o convite reutilizável é rotacionável e revogável, e a rotação invalida o
  cartaz anterior.

## Consequências

- O QR de parede passa a ser possível sem abrir mão do armazenamento por HMAC.
- A correção do MAC é uma mudança de segurança que vale para os dois purposes.
- O token no fragmento exige que o cliente leia `location.hash` e o envie por
  cabeçalho ou corpo; o servidor nunca o recebe pela linha de requisição.
- Um cartaz fotografado continua utilizável por quem o fotografou. A defesa não
  é o token: é a aprovação humana e a minimização do que o canal aceita.
