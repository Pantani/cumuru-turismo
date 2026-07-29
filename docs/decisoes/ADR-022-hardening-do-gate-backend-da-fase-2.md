# ADR-022 — Hardening do gate backend da Fase 2

**Status:** aceito.

## Contexto

O QA incremental da onda backend encontrou divergências que os testes sob papel
administrativo não revelavam: o runtime tentava executar cleanup reservado ao
worker; inserts de audit/outbox usavam `RETURNING` sem privilégio de leitura;
replays de convite eram validados depois da mudança de estado; políticas de
acomodação não alcançavam todas as mutações; e algumas respostas idempotentes
duplicavam dados operacionais além do necessário.

Também faltavam prova do runtime PostgreSQL com o papel `cumuru_app`, cursor
assinado, identidade correta do rate limit, snapshot consistente do grupo,
distinção entre `null` e campo ausente e sinalização de reserva idempotente em
processamento.

## Decisão

### Runtime e privilégios

- o fluxo HTTP nunca executa `DeleteExpiredIdempotencyKeys`; cleanup expirado é
  responsabilidade exclusiva do worker;
- `InsertAuditEvent` e `InsertOutboxEvent` são comandos sem `RETURNING`; os IDs
  são gerados pela aplicação antes do insert;
- os grants mínimos das migrations permanecem; não se amplia o runtime para
  mascarar queries incompatíveis;
- testes PostgreSQL usam duas conexões: uma administrativa somente para
  migrations/fixtures e outra obrigatoriamente autenticada como `cumuru_app`
  para repositories e transações da aplicação.

### Replay e minimização

- a resolução de capability aceita o estado `pre_registered` somente para
  localizar um replay potencial de convite consumido;
- replay idêntico consulta idempotência e reproduz o sucesso;
- nova chave para convite consumido retorna `409 invite-consumed`;
- respostas persistidas de mutações de estadia usam a projeção fechada
  `StayMutationResult`, com somente `id`, `status` e `version`;
- `POST /stays` e os comandos check-in, check-out, cancel e no-show retornam
  `StayMutationResult`; detalhes permanecem disponíveis no `GET`;
- `submission_id` externo é o `client_submission_id`; o ID interno do registro
  não é exposto nem persistido como resposta.

### Estado e concorrência

- acomodação `closed` é somente leitura, inclusive memberships;
- `PATCH` de estadia é permitido somente quando a acomodação está `active`;
- `suspended` continua permitindo apenas leitura, check-out, cancelamento e
  no-show; criação, edição geral, grupo, convite e check-in são negados;
- autorização e estado permanecem no SQL; o repository não converte role ou
  estado inválido em falso `412`;
- leitura do grupo usa transação `REPEATABLE READ READ ONLY`, garantindo que
  payload e ETag pertençam ao mesmo snapshot;
- reserva idempotente ainda `processing` produz erro tipado `409` com
  `Retry-After`; hash divergente continua `409` sem esse header.

### Entradas HTTP

- cursor de paginação contém versão de chave e HMAC-SHA256, com keyring próprio
  e separado dos keyrings de invite, actor, idempotency e rate limit;
- a identidade do rate limit é o HMAC de finalidade sobre capability derivada e
  prefixo de IP generalizado (`/24` para IPv4 e `/56` para IPv6); token e IP
  brutos não são persistidos;
- DTOs de PATCH distinguem campo ausente de `null`; `null` em propriedade não
  nullable retorna `400`;
- UUID, CORS e indisponibilidade declarados pela implementação precisam constar
  no OpenAPI da operação correspondente;
- o fake OIDC continua restrito a local/test, mas sua fixture recebe os scopes
  mínimos da Fase 2 para permitir smoke autenticado; isso não prova IdP
  institucional.

## Consequências

- OpenAPI, cliente TypeScript, queries sqlc e código gerado mudam por seus
  owners exclusivos antes dos consumidores;
- integração sob papel administrador não promove grants ou runtime a `PASS`;
- respostas idempotentes deixam de duplicar datas e ocorrências por 30 dias;
- o quinto keyring operacional é exclusivamente de cursor e não pode reutilizar
  material de outra finalidade;
- todo código novo ou refatorado permanece com complexidade ciclomática e
  cognitiva máxima 9, sem suppression.
