# ADR-036 — Complexidade ciclomática ≤ 5 e cognitiva ≤ 8

**Status:** aceito. Substitui o limite único de `9` do
[ADR-021](ADR-021-complexidade-ciclomatica-e-cognitiva-abaixo-de-dez.md).

## Contexto

O limite anterior era `9` para as duas métricas, em todo o código próprio.
Na prática ele permitia funções longas com muitas responsabilidades: uma
validação com oito condições encadeadas ou um handler com sete verificações
sequenciais passava o gate sem revelar que fazia várias coisas.

A revisão de produto exigiu um limite mais estrito: ciclomática `5` e cognitiva
`8`.

## Decisão

O limite passa a ser diferente por métrica e por natureza do código.

| Superfície | Ciclomática | Cognitiva |
| --- | --- | --- |
| Go de aplicação e código gerado | 5 | 8 |
| Go de teste | 9 | 9 |
| Web (fonte, config, gerado e teste) | 5 | 8 |

`make complexity` mede as quatro faixas e falha fechado em qualquer uma.
Nenhuma suppression inline é aceita; o scanner de suppressions continua ativo.

### Por que o Go de teste fica em 9

Um teste orientado a tabela com seis ou sete asserções sequenciais tem
ciclomática 7 e permanece legível: a falha e a evidência ficam no mesmo lugar.
Quebrar essas asserções em helpers para atingir `5` custa exatamente a
localidade que torna a falha diagnosticável, e o benefício de manutenção que o
limite busca não se aplica a código sem lógica de negócio.

O web não recebe a mesma exceção porque suas suítes já satisfazem `5/8` sem
perda de clareza: o `oxlint` cobre `src`, `public`, `vite.config.ts` e os
próprios testes na mesma faixa da aplicação.

## Consequências

- 138 funções Go e 30 do web foram decompostas em unidades nomeadas; a
  decomposição virou regra nova em vez de dívida registrada;
- validações longas viraram listas de predicados nomeados, e vários `switch`
  viraram tabelas de despacho — o que também fez uma métrica ou operador novo
  falhar fechado em vez de cair num `default`;
- o gate distingue faixas, então o `make complexity` passa a reportar qual
  faixa falhou;
- o ADR-021 permanece como registro histórico do limite anterior.
