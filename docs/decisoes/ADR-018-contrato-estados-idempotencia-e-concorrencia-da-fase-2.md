# ADR-018 — Contrato, estados, idempotência e concorrência da Fase 2

**Status:** aceito.

## Contexto

O contrato-alvo possuía apenas parte das jornadas da Fase 2. Faltavam
acomodações, memberships, grupo assistido, cancelamento e no-show; ETags e
precondições também não estavam uniformes. O blueprint exige replay exato,
concorrência otimista, isolamento e atomicidade com outbox e auditoria.

## Decisão

O OpenAPI da Fase 2 será publicado como versão `0.3.0` e materializará:

- `GET/PATCH /accommodations/{id}` e `GET /accommodations`;
- `GET/POST /accommodations/{id}/memberships` e
  `PATCH /accommodations/{id}/memberships/{membership_id}`;
- `POST/GET /stays`, `GET/PATCH /stays/{stay_id}`;
- `GET/POST /stays/{stay_id}/group`, sendo o `POST` o cadastro assistido;
- `POST /stays/{stay_id}/invite`;
- `POST /stays/{stay_id}/check-in`;
- `POST /stays/{stay_id}/check-out`;
- `POST /stays/{stay_id}/cancel`;
- `POST /stays/{stay_id}/no-show`;
- `GET /invites/{token}` e `POST /invites/{token}/submit`.

Entidades mutáveis usam ETag forte canônico `"N"`. `If-Match` ausente retorna
`428`, malformado retorna `400` e obsoleto retorna `412`. Comandos que
transicionam uma estadia exigem `If-Match` e `Idempotency-Key`.

Todo `POST` de efeito material:

1. aceita chave de 16–128 caracteres da allowlist documentada;
2. persiste somente HMAC versionado da chave;
3. calcula SHA-256 do DTO validado e canônico;
4. separa ator, método, operação e recurso na identidade da chave;
5. reproduz status, corpo, `Location` e `ETag` em replay idêntico;
6. retorna `409` para a mesma chave com conteúdo diferente;
7. retorna `409` com `Retry-After` quando a reserva concorrente não puder ser
   resolvida dentro do timeout curto;
8. conclui negócio, idempotência, audit e outbox na mesma transação.

`POST /accommodations/{id}/memberships` responde com
`MembershipCreated`, uma projeção mínima sem `oidc_issuer` nem
`oidc_subject`: `id`, `accommodation_id`, `role`, `active`, `version`,
`created_at` e `updated_at`. O registro de idempotência nunca persiste nem
reproduz issuer ou subject. Esses campos podem aparecer somente na listagem
autenticada de memberships para manager, que não possui replay persistido.

A janela técnica inicial de replay é de 30 dias, configurável. Ela não é prazo
jurídico de retenção. Records expirados continuam protegidos pelas unicidades
de negócio e pelo consumo do convite.

`POST /stays/{stay_id}/invite` segue o replay exato sem persistir o segredo:

- o token é derivado deterministicamente do invite ID, da finalidade e da chave
  HMAC versionada, conforme ADR-019;
- o idempotency record persiste somente `resource_id`, status e headers
  permitidos;
- o corpo/URL é reconstruído no replay com a versão histórica da chave;
- versões antigas permanecem no keyring ao menos pela maior janela entre
  convite e idempotência;
- a URL aparece somente na criação e em seus replays idempotentes exatos, nunca
  em uma leitura posterior.

O request JSON é estrito: um único valor, sem campos desconhecidos e sem chaves
duplicadas. Respostas usam `application/problem+json`, `X-Request-ID` e
`Cache-Control: no-store`. Recursos de outro tenant são indistinguíveis de
inexistentes (`404`).

A máquina de estadia é:

```text
create                                      -> draft
draft --invite----------------------------> invited
invited --reissue invite------------------> invited
draft|invited --submit assisted/invite----> pre_registered
pre_registered --check-in-----------------> checked_in
checked_in --check-out--------------------> checked_out
draft|invited|pre_registered --cancel-----> cancelled
checked_in --manager correction cancel----> cancelled
invited|pre_registered --no-show----------> no_show
```

Check-in exige visitante válido. Check-out não pode anteceder check-in.
Cancelamento após check-in exige `manager`, `correction=true` e código de
motivo fechado. No-show só é permitido a partir do dia civil de chegada em
`America/Bahia`. Estados terminais são imutáveis nesta fase.

`POST /stays/{stay_id}/group` é o cadastro assistido autenticado:

- exige `stays:write`, membership `operator|manager`, `If-Match` e
  `Idempotency-Key`;
- usa o mesmo payload minimizado de submissão do convite:
  `client_submission_id`, `privacy_notice_version` e `visitors`;
- a proveniência `assisted` e a membership do operador são derivadas no
  servidor, nunca aceitas do body;
- aceita `draft|invited`, revoga convites ativos e termina em
  `pre_registered`;
- responde `200` com `submission_id`, `status=accepted`,
  `stay_status=pre_registered`, novo ETag e indicador de replay.

Código de revisão do hóspede é `N/A` neste protótipo: não há identidade direta,
contato nem endpoint de correção de grupo. Implementá-lo sem canal e finalidade
aprovados criaria uma capability sem jornada segura.

## Consequências

- OpenAPI, cliente TypeScript, handlers, testes e changelog mudam juntos.
- Replays não incrementam versão nem duplicam audit/outbox.
- Nenhuma resposta persistida para replay duplica identificadores OIDC.
- Nenhum endpoint das Fases 3–5 recebe handler por existir no contrato-alvo.
- A implementação precisa de PostgreSQL real para provar lock, replay,
  rollback e concorrência; mock não promove esses gates a `PASS`.
