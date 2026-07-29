# ADR-029 — Presença, cobertura e forecast da Fase 4

**Status:** aceito para `PROTOTYPE_ONLY`.

> Nota de baseline: o ADR-032 incorpora as migrations citadas neste documento
> à `000001_initial_schema`; a numeração permanece como contexto histórico.

## Contexto

O domínio da Fase 2 já conhece `[arrival,departure)`, mas classifica todo o
restante de uma estadia em check-in como observado. A Fase 4 precisa separar
fato observado de permanência futura, definir cobertura e implementar um
baseline reproduzível sem alegar acurácia real.

## Decisão

Toda data civil usa `America/Bahia`. A publicação possui `as_of_on`, obtido do
clock injetado na reconciliação.

### Presença

- `draft`, `invited`, `cancelled` e `no_show`: nenhuma presença;
- `pre_registered`: forecast em `[planned_arrival_on,planned_departure_on)`,
  com peso técnico de comparecimento `0,80`;
- `checked_in`: observed de `date(checked_in_at)` até
  `min(as_of_on + 1 dia, planned_departure_on)`; dias seguintes até a saída
  planejada são forecast com peso `1,00`;
- `checked_out`: observed em
  `[date(checked_in_at),date(checked_out_at))`;
- intervalo vazio/invertido ou instante obrigatório ausente falha fechado.

A materialização interna usa a chave
`(stay_id,visitor_id,presence_on)`, kind, weight e `source_stay_version`.
Cada reconciliação relê o estado atual, substitui atomicamente o conjunto da
estadia e ignora regressão de versão. Repetir a mesma entrada não altera linhas
nem timestamps semânticos.

O worker pode consumir sinais da outbox, mas a reconciliação integral é
autoritativa e cobre eventos ausentes, repetidos e fora de ordem. A primeira
execução e a agenda integral processam todas as estadias elegíveis.

### Cobertura

No protótipo, uma acomodação reportou quando está ativa, tem capacidade
positiva conhecida e possui fonte de presença atualizada nos 30 dias civis que
terminam em `as_of_on`. O denominador interno é a soma das capacidades
positivas conhecidas das acomodações ativas; o numerador é a parcela das que
reportaram.

Denominador zero, menos de três participantes ou fonte insuficiente resulta em
`unavailable`. A projeção pública contém somente a razão arredondada e o
status, nunca contagens, capacidades ou lista. Métricas internas podem exibir
agregados por categoria, sem identificar acomodação.

### Forecast baseline `explainable-baseline-v1`

Para cada dia futuro:

```text
on_books =
  pre_registered × 0,80
  + checked_in_future × 1,00

seasonal_baseline =
  mediana observada das oito ocorrências anteriores do mesmo dia da semana

lead_factor =
  min(dias_de_antecedência, 30) / 30

central =
  on_books + max(seasonal_baseline - on_books, 0) × lead_factor
```

Sem oito semanas completas, o fallback é `on_books`. A faixa provável usa
`85%–115%` do central quando há baseline e `70%–130%` no fallback, antes do
arredondamento protegido. Valores são não negativos.

A metodologia pública expõe as duas faixas como propriedades distintas e
obrigatórias: `forecast_bounds_percent = [85,115]` descreve a faixa nominal
com baseline; `forecast_fallback_bounds_percent = [70,130]` descreve
exclusivamente o fallback `on_books`. A migration append-only `000016`
acrescenta essa distinção à view pública sem alterar a migration `000012`
já aplicada.

O método não divide por cobertura, não chama fonte externa e não usa dado
futuro no histórico. O backtest aplica o mesmo cutoff histórico e registra
erro agregado; acurácia operacional continua `UNVERIFIED`.

### Qualidade

O painel interno mostra somente agregados: cadastros incompletos, saídas
vencidas, acomodações silenciosas, coverage por categoria, freshness,
falhas de agregação e diferença forecast/realizado. Duplicatas cross-stay e
falhas FNRH são `not_available`, nunca zero.

### Orçamento de recomposição do protótipo

O gate local recompõe uma janela de 1.096 dias civis
(`2023-07-29` inclusivo a `2026-07-29` exclusivo) com dez estadias fictícias
por dia e três visitantes por estadia. Cada passagem materializa 10.960 fontes
e 32.880 fatos. Duas passagens sobre a mesma entrada devem produzir o mesmo
digest.

Cada passagem tem orçamento de 30 segundos e 512 MiB de crescimento de heap.
O gate registra volume, duração das duas passagens, maior crescimento de heap,
versão do Go, `GOOS`, `GOARCH`, `GOMAXPROCS`, sistema, CPUs lógicas e memória
do host. Esses limites são hipóteses operacionais do `PROTOTYPE_ONLY`, não
evidência de capacidade produtiva.

O target `phase4-benchmark` executa primeiro `phase4-full-stack`. Assim, uma
recomposição só passa quando o runtime público também preserva exatamente
corpo e `ETag` do último snapshot válido durante falha de publicação. O
benchmark não usa dados reais, não publica resultado e não autoriza deploy.

## Consequências

- o comportamento puro anterior de `checked_in` precisa ser estendido sem
  reclassificar dias futuros como observados;
- a fórmula é deliberadamente simples, explicável e substituível somente após
  backtest, ADR e revisão;
- capacidade, cobertura e acurácia reais continuam `UNVERIFIED`;
- jobs usam clock e agenda injetáveis, timeout de banco e códigos de erro
  allowlisted, sem IDs ou valores de célula em logs.
