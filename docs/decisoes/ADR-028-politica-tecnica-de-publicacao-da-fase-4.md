# ADR-028 — Política técnica de publicação da Fase 4

**Status:** aceito para `PROTOTYPE_ONLY`.

## Contexto

A Fase 4 precisa publicar presença e preferências sem expor microdados, células
pequenas ou contribuições por hospedagem. O limiar, as dimensões e a política
municipal ainda não foram aprovados. Portanto, esta decisão prova somente um
pipeline técnico com fixtures fictícias; não afirma anonimização jurídica nem
autoriza dados reais.

## Decisão

A política técnica inicial tem versão `prototype-v1` e catálogo fechado:

| Métrica | Unidade | Período público | Dimensão |
| --- | --- | --- | --- |
| `presence` | `person_day` | `recent_30_days`, `next_30_days` | nenhuma |
| `first_visit_share` | `survey_response` | `last_complete_month` | `visit_profile` com categorias `first_visit` e `returning` |

Uma resposta de pesquisa só entra em `first_visit_share` quando a versão e a
pergunta estão explicitamente mapeadas no catálogo, a participação é
`submitted`, o consentimento da finalidade foi concedido, a resposta é
estruturada, `public_aggregation_allowed=true` e a classificação não é
`sensitive|secret`. `analytics_key` isolada nunca autoriza publicação. Texto
livre, prompt, label, capability, IDs e respostas recusadas não atravessam a
fronteira.

O catálogo público inicial aceita uma dimensão por métrica. Não existem
intervalos, filtros, expressões, agrupamentos, drill-downs ou versões de
publicação escolhidos pelo cliente.

### Supressão

- limiar primário: pelo menos 10 contribuições e 3 acomodações participantes;
- limiar maior definido no snapshot histórico da pergunta prevalece;
- supressão ocorre antes do arredondamento;
- uma família aditiva não pode deixar exatamente uma célula protegida
  dedutível: a menor célula ainda publicável também é protegida; em empate,
  vence o código canônico em ordem lexical;
- se não houver segunda célula elegível, o total é protegido;
- o procedimento se repete até não existir família com uma única incógnita;
- a projeção pública usa apenas `published`, `protected` ou `unavailable`;
- `protected` não carrega valor, intervalo, tamanho de amostra ou motivo
  detalhado.

Summary, presença e preferências reutilizam as mesmas células já protegidas.
Nenhum endpoint recalcula um total exato independente.

### Arredondamento

- contagens observadas e centrais usam half-up para múltiplos de 10;
- limite inferior de forecast usa floor e superior usa ceil para múltiplos de
  10;
- percentuais de preferência e cobertura usam half-up para passos de 5 pontos
  percentuais;
- o algoritmo é determinístico pelo valor e pela política, sem seed derivada
  de pessoa, estadia, resposta ou request;
- `lower <= central <= upper` precisa permanecer verdadeiro.

### Payload

O público recebe apenas release corrente, período civil, unidade, `data_mode`,
`updated_at`, `privacy_policy_version`, `methodology_version`, cobertura
protegida, kind/status e valores já liberados. Cobertura pública contém somente
`ratio` arredondada e `status`; numerador, denominador, capacidade e quantidade
de hospedagens ficam internos.

O ordinal interno da publicação, UUIDs, sample size, estabelecimento,
organização, acomodação, pessoa, estadia, pergunta, resposta, operador, texto
livre e dimensões JSON arbitrárias não entram no payload.

## Consequências

- a política real, os valores de limiar e novas métricas continuam
  `BLOCKED` até aprovação institucional;
- differencing é testado sobre todas as superfícies públicas da release;
- adicionar métrica, período, dimensão ou categoria exige nova versão de
  política, ADR e revisão de privacidade;
- dados fictícios precisam ser identificados como
  `data_mode=prototype_fixtures`.
