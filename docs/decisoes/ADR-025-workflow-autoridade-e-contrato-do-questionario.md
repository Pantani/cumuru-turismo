# ADR-025 — Workflow, autoridade e contrato do questionário

**Status:** aceito.

## Contexto

A Fase 3 precisa permitir edição, revisão de privacidade, publicação e retirada
sem criar autoridade municipal inexistente. O contrato anterior expunha apenas
uma leitura ativa ambígua e uma submissão sem workflow administrativo.

Versões publicadas precisam preservar a interpretação histórica. Ao mesmo
tempo, o editor e o revisor não podem compartilhar a mesma autoridade, e o
protótipo não pode converter scopes locais em evidência institucional.

## Decisão

O catálogo de questionários é global no MVP e identificado por `stable_key`.
Não são aceitos `organization_id`, `accommodation_id` ou claims de tenant no
conteúdo do questionário.

O workflow fechado de uma versão é:

```text
draft -> privacy_review -> approved -> published -> retired
             |
             +-> draft, por solicitação de mudanças
```

Somente `draft` pode ser editada. Enviar para revisão exige definição completa
de classificação, finalidade, retenção, tipo, opções, consentimentos e lógica
condicional. Aprovar, publicar ou retirar não altera conteúdo.

Os scopes técnicos são:

- `questionnaires:manage`: criar catálogo, criar/editar versão, clonar e enviar
  para revisão;
- `questionnaires:approve`: solicitar mudanças, aprovar, publicar e retirar.

Editor e revisor usam subjects distintos no fake OIDC de `local|test`.
Autoaprovação é recusada comparando os HMACs versionados do último editor e do
revisor. IdP, MFA, atribuição institucional e segregação reais continuam
`UNVERIFIED`.

Operações materiais usam `Idempotency-Key`. Alteração de draft e transições
usam `If-Match`; toda leitura mutável devolve `ETag`. Erros seguem
`application/problem+json`, respostas privadas usam `Cache-Control: no-store`
e todos os endpoints devolvem `X-Request-ID`.

A superfície da Fase 3 é:

- `GET|POST /questionnaires`;
- `GET /questionnaires/{questionnaire_id}`;
- `GET|POST /questionnaires/{questionnaire_id}/versions`;
- `GET|PUT /questionnaire-versions/{version_id}`;
- `POST /questionnaire-versions/{version_id}/submit-review`;
- `POST /questionnaire-versions/{version_id}/request-changes`;
- `POST /questionnaire-versions/{version_id}/approve`;
- `POST /questionnaire-versions/{version_id}/publish`;
- `POST /questionnaire-versions/{version_id}/retire`;
- `GET /questionnaires/{stable_key}/active`.

A rota pública por `stable_key` elimina a semântica ambígua de “um questionário
ativo global”. Pode existir no máximo uma versão `published` por questionário.
A publicação de outra versão retira atomicamente a anterior.
Não há agendamento temporal na Fase 3: `active` significa exatamente
`status=published`. `valid_from` e `valid_until` ficam fora do contrato, schema
e migrations desta fase.

Clonar cria novos IDs para versão, perguntas, opções e requisitos de
consentimento, preserva `stable_key` e significado, e começa em `draft`.
Respostas sempre referenciam o snapshot histórico, nunca a versão mais recente.
Retirada impede novas submissões, mas um replay idempotente aceito antes da
retirada continua reproduzível durante a janela de replay.

A projeção pública `PublishedQuestionnaire` é allowlisted e contém somente ID e
número da versão, stable key, título, descrição pública, perguntas, opções,
requisitos de consentimento e DSL necessária à renderização. HMACs de
editor/reviewer, retenção interna, metadata de audit, timestamps internos e
controles administrativos não são expostos.

`POST /privacy/requests` é reclassificado para fase futura. Ele coleta contato
direto, depende de verificação de identidade, retenção e operação do canal do
titular, e não faz parte do Prompt 3.

## Consequências

- não existe edição silenciosa de conteúdo publicado;
- múltiplos questionários possuem leitura ativa determinística;
- o fake local prova somente o controle técnico de scopes;
- contrato, migrations, cliente gerado e testes mudam juntos;
- a futura operação de direitos do titular permanece bloqueada até possuir
  governança e fluxo próprios.
