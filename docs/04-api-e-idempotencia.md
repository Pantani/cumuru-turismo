# API, concorrência e idempotência

## Convenções

- Base: `/api/v1`.
- JSON em `snake_case`.
- Datas: `YYYY-MM-DD`.
- Instantes: RFC 3339 em UTC.
- IDs: UUIDv7.
- Paginação por cursor opaco.
- Erros: `application/problem+json`.
- Request ID recebido ou gerado em `X-Request-ID`.
- Contrato canônico em `contracts/openapi.yaml`.

## Autorização

Escopos iniciais:

```text
stays:write
stays:read:own
accommodations:manage
questionnaires:manage
questionnaires:approve
analytics:read:internal
privacy:manage
audit:read
```

Além do escopo, toda consulta autenticada é limitada à organização e à
acomodação do ator.

## Escritas

`Idempotency-Key` é obrigatório em:

- criação de estadia;
- envio de grupo;
- check-in/check-out;
- publicação de questionário;
- solicitação de integração;
- solicitação de direito do titular.

Implementação:

1. Normalizar método, rota, ator e chave.
2. Calcular SHA-256 do corpo canônico.
3. Inserir registro `PROCESSING` com restrição única.
4. Se a chave existir com outro hash, responder `409`.
5. Se estiver processando, responder `409` com `Retry-After`.
6. Se concluída, reproduzir status e corpo originais.
7. Gravar resposta na mesma transação do efeito quando possível.
8. Expirar somente após a maior janela segura de repetição.

Não use a chave fornecida pelo cliente como identificador primário da entidade.

## Concorrência

Entidades mutáveis possuem `version bigint` e `updated_at`.

```http
GET /api/v1/stays/{id}
ETag: "7"

PATCH /api/v1/stays/{id}
If-Match: "7"
```

Se outra atualização avançou a versão, responder `412 Precondition Failed`.

## Operações principais

### Público

```text
GET  /public/summary
GET  /public/presence
GET  /public/preferences
GET  /public/methodology
```

Somente combinações pré-aprovadas. Nenhuma consulta SQL dinâmica baseada em
filtros arbitrários.

### Registro por convite

```text
GET  /invites/{token}
PUT  /invites/{token}/group
POST /invites/{token}/submit
POST /invites/{token}/survey-responses
```

Tokens são aleatórios, de uso limitado, guardados como hash e revogáveis.

### Hospedagem

```text
POST  /stays
GET   /stays
GET   /stays/{id}
PATCH /stays/{id}
POST  /stays/{id}/invite
POST  /stays/{id}/check-in
POST  /stays/{id}/check-out
POST  /stays/{id}/cancel
```

### Questionários

```text
GET|POST /questionnaires
GET      /questionnaires/{questionnaire_id}
GET|POST /questionnaires/{questionnaire_id}/versions
GET|PUT  /questionnaire-versions/{version_id}
POST     /questionnaire-versions/{version_id}/submit-review
POST     /questionnaire-versions/{version_id}/request-changes
POST     /questionnaire-versions/{version_id}/approve
POST     /questionnaire-versions/{version_id}/publish
POST     /questionnaire-versions/{version_id}/retire
GET      /questionnaires/{stable_key}/active
POST     /survey-responses
```

`active` significa exatamente uma versão `published` por questionário. A
pesquisa primária usa `tourism_profile`. Não existe endpoint de reemissão: os
dois POSTs de submissão de grupo devolvem opcionalmente
`Survey-Capability` ao chegar a `pre_registered`, e o replay exato reconstrói
o mesmo header.

`POST /survey-responses` recebe a capability somente nesse header. O JSON não
aceita stay, accommodation, visitor, minor nem token. `declined` leva listas
vazias; `submitted` exige uma decisão exata por requisito da versão.

### Privacidade

```text
POST /privacy/requests
GET  /privacy/requests/{id}
POST /privacy/requests/{id}/verify
POST /privacy/requests/{id}/complete
```

## Erro padrão

```json
{
  "type": "https://turismo.prado.ba.gov.br/problems/idempotency-conflict",
  "title": "Chave de idempotência reutilizada",
  "status": 409,
  "detail": "A chave já foi usada com outro conteúdo.",
  "instance": "/api/v1/stays",
  "request_id": "019f..."
}
```

Não incluir stack trace, SQL, token ou conteúdo pessoal.

## Rate limiting

- por IP generalizado nos endpoints públicos;
- por token/acomodação nos endpoints autenticados;
- por convite nos formulários;
- limites mais rígidos para login, busca de identidade e exportações;
- resposta `429` com `Retry-After`.

O rate limiter distribuído pode começar em PostgreSQL para operações sensíveis e
ser movido para serviço dedicado quando houver necessidade medida.

## Compatibilidade

- adições opcionais são compatíveis;
- remoção ou mudança semântica exige `/v2`;
- deprecações têm prazo, telemetria de uso e documentação;
- adaptador FNRH possui versão independente da API municipal.
