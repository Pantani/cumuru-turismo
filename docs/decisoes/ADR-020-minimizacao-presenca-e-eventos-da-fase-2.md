# ADR-020 — Minimização, presença e eventos da Fase 2

**Status:** aceito.

> Nota de baseline: o ADR-032 incorpora as migrations desta fase à
> `000001_initial_schema`; a numeração abaixo permanece como contexto histórico.

## Contexto

O contrato-alvo aceitava nome e contato, mas não há KMS, retenção ou
governança comprovados. O aceite da Fase 2 também menciona recalcular dias e
excluir cancelados da presença, enquanto `analytics.presence_day` pertence à
Fase 4.

## Decisão

Nesta execução `PROTOTYPE_ONLY`:

- `name`, `responsible_contact`, documento, e-mail, telefone,
  `external_reference` e texto livre são removidos/rejeitados;
- não é criada `identity.visitor_identities`;
- visitante contém somente ID interno, `client_id`, papel no grupo, faixa
  etária, país, UF e código de município;
- exatamente um visitante é responsável e todos os valores são fixtures
  fictícios;
- a estadia referencia a membership criadora, nunca duplica OIDC subject em
  texto;
- HMACs de convite, ator e idempotência usam chaves e versões separadas; local
  e teste usam fixtures explícitas, enquanto staging/produção falham sem
  segredo configurado;
- audit e outbox são construídos por DTOs fechados e allowlists, nunca por
  cópia do request ou `map[string]any` livre;
- outbox contém somente aggregate ID, versão e tipo do evento;
- auditoria contém ator pseudônimo, ação, entidade, códigos e nomes de campos,
  nunca valores anteriores/novos.

A Fase 2 implementa uma função pura de presença:

- `draft` e `invited` produzem conjunto vazio;
- `pre_registered` projeta `[planned_arrival_on, planned_departure_on)` como
  presença prevista;
- `checked_in` usa o dia civil do `checked_in_at` em `America/Bahia` até a
  saída planejada como presença observada corrente;
- `checked_out` usa os dias civis entre `checked_in_at` e `checked_out_at` como
  presença observada histórica;
- `cancelled` e `no_show` produzem conjunto vazio;
- alteração de datas ou estado emite evento determinístico
  `stay.presence_recalculation_requested` com ID e versão;
- a Fase 2 não cria nem escreve `analytics.presence_day`, cubos,
  `public_data`, previsão ou dashboard;
- a Fase 4 consultará a estadia pelo ID e materializará/reconciliará presença.

Alerta de sobreposição entre estadias é `N/A` neste protótipo: sem documento,
contato ou pseudônimo estável cross-stay, correlacionar faixa etária e origem
criaria risco de reidentificação e falsos positivos. A Fase 2 deduplica apenas
`client_id` dentro da estadia. Um alerta cross-stay só pode ser introduzido
quando existir identificador pseudônimo com base e proteção aprovadas.

As migrations da fase são:

1. `000005_create_stay_domain`;
2. `000006_create_idempotency_outbox_and_rate_limits`;
3. `000007_apply_phase2_privileges`.

Na decisão original, as migrations `000001`–`000004` permaneceriam intocadas.
O ADR-032 substituiu essa separação antes do primeiro lançamento, sem mudar as
invariantes: `public_runtime` não recebe acesso; `app_runtime` não recebe
acesso a `identity`; auditoria continua append-only.

## Consequências

- O gate de presença é provado no domínio sem antecipar a Fase 4.
- O protótipo não promete tratamento de identidade direta.
- Dados reais, KMS institucional, retenção, menores, backup/restore e
  eliminação continuam `BLOCKED` ou `UNVERIFIED`.
