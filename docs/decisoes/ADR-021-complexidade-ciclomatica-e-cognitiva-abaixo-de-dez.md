# ADR-021 — Complexidade ciclomática e cognitiva abaixo de dez

**Status:** aceito.

## Contexto

Toda implementação do Cumuru precisa permanecer simples de revisar, testar e
manter. A exigência adicional da Fase 2 determina que todo código tenha
complexidade ciclomática e cognitiva estritamente abaixo de 10.

## Decisão

O limite por função é `9`. Qualquer função com valor `10` ou superior falha o
gate, inclusive em código de aplicação, testes e código gerado pertencente ao
projeto.

Para Go:

- `gocyclo` mede complexidade ciclomática;
- `gocognit` mede complexidade cognitiva;
- ambas as ferramentas são instaladas em versão fixada pelo `Makefile`;
- a análise cobre `apps/api`, incluindo comandos, módulos internos e testes;
- `gocognit` é sempre chamado com `-test`, pois sem essa flag a própria
  ferramenta exclui arquivos `_test.go`; `gocyclo` recebe o diretório sem
  expressão de ignore.

Para TypeScript, TSX e JavaScript:

- a regra core `complexity` do Oxlint mede complexidade ciclomática;
- `sonarjs/cognitive-complexity`, carregada como plugin JavaScript pelo Oxlint,
  mede complexidade cognitiva;
- a análise cobre todo arquivo TypeScript, TSX e JavaScript próprio em
  `apps/web`, incluindo configuração, testes e o cliente gerado; somente
  dependências instaladas e artefatos de build ficam fora.

`make complexity` executa as quatro verificações e faz parte do gate local e da
CI. Um teste do próprio gate usa fixtures temporárias com complexidade 10,
inclusive em `_test.go`, para provar que as ferramentas falham no limiar. O
baseline existente também precisa passar antes de promover a fase.

Não são permitidos comentários de ignore, disable inline, allowlists por
arquivo, aumento do limite ou exclusão de código próprio para obter `PASS`.
Quando código gerado exceder o limite, a correção deve ocorrer no contrato,
gerador, template ou desenho que o produz; o arquivo gerado não será editado à
mão.

Arquivos declarativos, SQL e shell não têm função equivalente mensurável por
essas quatro ferramentas. Eles continuam sujeitos aos linters específicos,
testes e revisão, sem alegação indevida de que receberam uma métrica que não se
aplica.

## Consequências

- writers devem decompor funções antes que alcancem o limite;
- testes de complexidade são obrigatórios em cada onda e no QA final;
- uma violação é `FAIL`, não `UNVERIFIED`;
- ferramenta indisponível ou instalação não reproduzível é `UNVERIFIED` e
  impede `PASS`;
- suppressions ou exceções futuras exigem decisão explícita, nova evidência e
  aprovação do usuário.
