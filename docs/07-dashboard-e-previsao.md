# Dashboard, anonimização e previsão

## Princípio

O dashboard responde “qual a tendência da cidade?” e nunca “quem está onde?”.

## Indicadores

### Presença

- pessoas estimadas presentes hoje;
- pessoas esperadas por dia nos próximos 30 dias;
- chegadas e saídas;
- pernoites;
- permanência média;
- tamanho médio de grupo.

### Perfil agregado

- macrorregião e UF de origem;
- faixas etárias amplas;
- primeira visita ou retorno;
- tipo de grupo;
- motivo da viagem;
- modal de transporte;
- tipo de hospedagem em categorias amplas.

### Preferências

- praia, gastronomia, descanso, cultura, natureza e passeios;
- tranquilidade, música ao vivo, programação cultural ou festa;
- estilos musicais em categorias;
- faixas de gasto;
- horários de consumo;
- serviços procurados.

## Metodologia de presença

Uma pessoa com estadia válida contribui para cada dia no intervalo:

```text
[data de entrada, data de saída)
```

Estados incluídos:

- `PRE_REGISTERED` na previsão, ponderado pela taxa de comparecimento;
- `CHECKED_IN` na estimativa corrente;
- `CHECKED_OUT` no histórico.

Excluídos:

- `DRAFT`;
- `CANCELLED`;
- `NO_SHOW`;
- duplicatas confirmadas.

## Cobertura

O sistema não conhece automaticamente todo turista. Cada gráfico exibe:

- estabelecimentos ativos que reportaram;
- capacidade estimada coberta;
- data da última atualização;
- se o valor é observado ou previsto;
- intervalo de incerteza.

Nunca apresentar estimativa parcial como censo.

## Previsão inicial

Começar com modelo explicável:

```text
presença esperada =
reservas confirmadas
× taxa histórica de comparecimento por período
+ projeção de reservas ainda não feitas
```

Variáveis:

- antecedência da reserva;
- feriados e eventos;
- dia da semana;
- temporada;
- histórico de cancelamento/no-show;
- ocupação declarada;
- cobertura do sistema.

Implementação sugerida:

1. baseline sazonal por média móvel robusta;
2. curvas de antecedência por temporada;
3. quantis P10, P50 e P90;
4. backtest mensal;
5. modelo mais complexo apenas se superar o baseline.

Publicar intervalo:

```text
Estimativa central: 4.100
Faixa provável: 3.700–4.500
Cobertura da capacidade conhecida: 64%
```

## Política de publicação

### Total municipal/distrital

- total por dia pode ser publicado arredondado para dezenas;
- futuro sempre em intervalo;
- exigir número mínimo de hospedagens participantes;
- não revelar contribuição por estabelecimento.

### Dimensões

- célula com menos de 10 participantes é suprimida;
- aplicar supressão complementar quando o total permitir dedução;
- cidade de origem vira UF ou macrorregião em amostras pequenas;
- idade usa faixas amplas;
- intervalos livres não podem ser arbitrariamente pequenos;
- cruzamento público limitado a no máximo duas dimensões aprovadas.

O valor 10 é configuração inicial, não garantia matemática de anonimização. O
encarregado e o responsável estatístico devem ajustar o limiar ao risco local.

## Proteção contra consultas por diferença

- front-end escolhe filtros de catálogo, não envia expressões livres;
- API retorna cubos pré-computados;
- resultados usam versão de publicação;
- arredondamento é estável dentro da versão;
- intervalos e dimensões raras são coalescidos;
- telemetria detecta varredura sistemática.

## Atualização

- agregação incremental: até 15 minutos;
- reconciliação integral: madrugada;
- previsão: após reconciliação e em mudanças relevantes;
- métricas públicas mantêm última versão válida se o job falhar;
- painel mostra “dados atualizados em”.

## Qualidade

Painel interno deve mostrar:

- cadastros incompletos;
- saídas previstas vencidas sem check-out;
- duplicatas suspeitas;
- hospedagens silenciosas;
- falhas FNRH;
- variação anormal;
- cobertura por categoria;
- diferença entre previsão e realizado.

## API pública

Não fornecer microdados no MVP. Endpoints públicos retornam:

```json
{
  "metric": "expected_presence",
  "period": {"start": "2026-12-28", "end": "2027-01-04"},
  "series": [
    {
      "date": "2026-12-28",
      "lower": 3700,
      "central": 4100,
      "upper": 4500
    }
  ],
  "coverage": 0.64,
  "privacy_policy_version": "1",
  "updated_at": "2026-12-20T12:00:00Z"
}
```
