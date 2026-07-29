# ADR-026 — Capability exclusiva e replay da pesquisa

**Status:** aceito.

## Contexto

O contrato inicial recebia `stay_access_token`, mas esse token não existe. O
convite da Fase 2 tem finalidade exclusiva de cadastro e não pode ser
reutilizado. A pesquisa opcional precisa ser separada da estadia na API sem
perder a prova server-side de elegibilidade.

## Decisão

A pesquisa usa capability própria, purpose-bound a `survey_response`, com
keyring, versão e HMAC distintos de invite, OIDC, ator, cursor, rate limit,
idempotência e cifra.

A capability é emitida atomicamente quando `POST
/stays/{stay_id}/group` ou `POST /invites/{token}/submit` aceita o grupo e há
uma versão ativa do questionário primário `tourism_profile`. A ausência de
questionário ativo não falha o cadastro, não cria capability e não altera
check-in.

O sucesso e seu replay exato devolvem o segredo somente no response header
`Survey-Capability`. O body existente continua sem segredo. CORS permite e
expõe esse header apenas às origens configuradas. O browser o mantém somente em
memória e o envia no mesmo header à submissão. `Authorization` continua
exclusivo de OIDC.

O token é determinístico e reconstruível:

```text
base64url(capability_id) + "." +
base64url(HMAC(key_version, "survey_response" || capability_id))
```

`capability_id` é UUIDv7 gerado com fonte criptográfica. A imprevisibilidade
vem do HMAC-SHA-256 com chave exclusiva. O registro persistido contém
capability ID, HMAC de lookup, key version, purpose, stay ID, version ID,
expiração, revogação e consumo. A projeção idempotente interna guarda somente
capability ID e key version; criação e replay reconstroem o mesmo header byte a
byte com a versão histórica da chave. Token em claro nunca é persistido.

O TTL máximo é 24 horas. O token nunca aparece em URL, query, path, JSON,
IndexedDB, service worker, log, trace, audit, outbox ou métrica.

`POST /survey-responses` exige:

- a capability em `Survey-Capability`;
- `Idempotency-Key`;
- `questionnaire_version_id`;
- `client_submission_id` UUIDv7;
- `participation=submitted|declined`;
- respostas e decisões fechadas de consentimento.

O servidor deriva stay e versão da capability; qualquer stay, accommodation,
visitor, minor ou token no JSON é campo desconhecido. Capability ausente,
inválida, expirada, revogada ou de finalidade divergente recebe `404` uniforme.

Uma submissão efetiva consome a capability. O mesmo token, idempotency key e
request hash reproduz a resposta durante 24 horas sem duplicar linhas. A mesma
key com outro corpo retorna `409`; nova key depois do consumo também retorna
`409`. Corridas bloqueiam a linha e materializam uma única transação.

Existe no máximo uma capability para `(stay_id, questionnaire_version_id)` e
exatamente um resultado efetivo, `submitted` ou `declined`, para o mesmo par.
Índices únicos garantem as duas invariantes. Não há endpoint de reemissão:
nova key não cria outro grant, e mudar recusa efetiva para resposta fica fora
da Fase 3.

A emissão acontece somente na transição bem-sucedida do cadastro para
`pre_registered`; `draft`, `invited`, `checked_in`, `checked_out`, `cancelled`
e `no_show` não possuem fluxo separado de emissão. A mesma transação já
autoriza tenant, membership, accommodation e stay. A stable key não é entrada
do operador.

Rate limit usa HMAC separado da capability e prefixo de rede generalizado,
respeitando a fronteira de proxy confiável do ADR-023. Replay é resolvido antes
de rejeitar versão retirada ou capability já consumida.

## Consequências

- convite e pesquisa não compartilham segredo nem finalidade;
- invite e cadastro assistido entregam a capability no próprio sucesso;
- respostas não aceitam IDs de autoridade enviados pelo cliente;
- recusa e submissão possuem repetição determinística;
- a capability não concede leitura de resposta, edição de questionário,
  check-in ou qualquer outra operação;
- fixtures locais não comprovam secret manager, borda ou operação
  institucional.
