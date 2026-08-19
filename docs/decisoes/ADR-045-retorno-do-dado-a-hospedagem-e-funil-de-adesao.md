# ADR-045 — Retorno do dado à hospedagem, ocupação e funil de adesão

**Status:** aceito para `PROTOTYPE_ONLY`.

**Relacionada:** [ADR-028](ADR-028-politica-tecnica-de-publicacao-da-fase-4.md),
que fixou limiar, supressão complementar e arredondamento da publicação;
[ADR-029](ADR-029-presenca-cobertura-e-forecast-da-fase-4.md) para presença e
cobertura; [ADR-030](ADR-030-fronteira-publica-snapshots-e-http-da-fase-4.md)
para a fronteira pública; e
[ADR-020](ADR-020-minimizacao-presenca-e-eventos-da-fase-2.md) para minimização.

## Contexto

A área da hospedagem só pedia. Quadro de estadias, cadastro e fila de hóspedes —
nenhum retorno do dado que ela digita. A hospedagem é o único ator do sistema
que entrega matéria-prima e não recebe nada, e essa assimetria é a explicação
mais provável para adesão baixa.

Devolver o dado parece trivial: é dado dela cruzado com agregado já público.
Não é, e as três decisões abaixo existem porque a leitura muda de natureza
quando o leitor conhece o próprio número.

Ao mesmo tempo, não havia como saber **onde** a adesão se perde. O painel
`/qualidade` mede qualidade do dado registrado, não conversão do fluxo que
produz o registro.

## Decisão 1 — o comparativo tem limiar próprio, mais alto que o público

A política pública exige dez na amostra e **três** estabelecimentos por célula
(`analytics/policy.go`). Para o público isso basta: ele lê o agregado sem saber
qual parcela é de quem.

A hospedagem lê a mesma célula sabendo exatamente o próprio número. Para ela,
`outros = total - meu` é uma subtração que só ela consegue fazer. Com o mínimo
de três, subtrair-se deixa dois — e numa vila onde todos se conhecem, dois é
quase um.

- o comparativo exige **cinco** estabelecimentos reportando na janela, não três,
  para que sobrem quatro após a subtração;
- o comparativo fecha quando a capacidade própria passa de **25%** do
  denominador reportante: acima disso o agregado é espelho do próprio dado, não
  comparação, e a subtração fica afiada;
- o denominador **entra na decisão e não sai dela**. A resposta carrega apenas o
  veredito e um motivo; "somos sete com 210 leitos" já é informação sobre
  terceiros;
- o lado da vila sai como **índice**, não valor absoluto. O absoluto continua
  público em `/public/presence`; o que se recusa é entregá-lo já composto ao
  lado do número exato de quem lê;
- um dia suprimido interrompe a linha em vez de ser interpolado: ligar os
  vizinhos desenharia um valor que a publicação recusou.

Nenhuma família de célula nova é publicada. O lado da vila é a publicação
corrente, lida pela mesma role restrita que serve a capa, com a mesma versão de
metodologia e privacidade.

## Decisão 2 — a ocupação da vila não vira célula pública

A taxa de ocupação é o indicador mais pedido e não existia. Publicá-la como
família nova, porém, equivale a publicar a capacidade da vila.

Presença já é pública em múltiplos de 10 e a cobertura em múltiplos de 5. Com
ocupação publicada, `capacidade ≈ presença / ocupação`.

O arredondamento **não** torna a capacidade indeterminável, e também não permite
determiná-la exatamente: como ele é uma função determinística da capacidade
verdadeira, capacidades vizinhas produzem publicações idênticas — presença 50 com
ocupação 50% é compatível com 100 e com 101 leitos, e nenhuma quantidade de dias
separa as duas. O que 730 dias fazem é **estreitar o intervalo**: cada dia
adicional descarta candidatas que seriam compatíveis com aquele dia isolado, até
sobrar uma faixa curta em vez de um número.

Uma faixa curta basta para o problema. Depois de uma admissão, a diferença entre
duas publicações consecutivas estreita do mesmo modo a capacidade do
estabelecimento que entrou — 120 e 121 permanecem indistinguíveis, 120 e 200 não.
Atributo de um estabelecimento identificável, ainda que por intervalo, é dado
individualizado de estabelecimento, que a Portaria MTur nº 41/2025 veda divulgar
publicamente ([`docs/06`](../06-legal-e-governanca.md)).

O limite de inferência é o que sustenta a decisão, não a exatidão: publicar a
ocupação trocaria "ninguém sabe a capacidade de ninguém" por "qualquer leitor
estreita a capacidade de cada estabelecimento a uma faixa de poucos leitos".

Decisão:

- a ocupação existe **apenas atrás de autenticação**, no comparativo da própria
  hospedagem;
- é **um número por janela**, não série diária: reduz de centenas para unidades
  as observações disponíveis para promediar;
- a taxa da vila sai em banda de **cinco pontos** e obedece aos mesmos guardas
  da Decisão 1; a taxa própria é exata, porque é dado dela;
- taxa acima de 100% **não é aparada**: significa capacidade declarada menor que
  a operação real, e aparar trocaria um defeito de cadastro por um número
  plausível.

Esta decisão é reversível. Publicar a família exigiria decisão explícita sobre o
oráculo de capacidade descrito acima, e ela não foi tomada aqui.

## Decisão 3 — o funil mede sem instrumentar

Medir abandono por etapa sugere telemetria: evento por passo, tempo por tela,
identificador de sessão. Isso abriria coleta comportamental sobre pessoas num
formulário que já coleta dado pessoal, e num canal aberto cuja base legal ainda
não existe.

O funil não faz nada disso. Ele conta estados que o registro já guarda: convite
emitido, usado e expirado (`core.invites`); capability de pesquisa emitida,
consumida e vencida (`survey.capabilities`); autocadastro iniciado, pendente e
decidido (`core.stays`). Sem evento por pessoa, sem canal novo, sem base legal
nova.

- a latência é medida entre carimbos que já existiam — emissão e submissão — e
  não por relógio de cliente;
- a mediana só é publicada a partir de **dez** conclusões na janela. Abaixo
  disso ela deixa de descrever um comportamento e passa a descrever uma pessoa:
  com uma submissão, a "mediana" é o tempo exato daquele hóspede;
- recusa explícita da pesquisa é contada como **resposta**, não abandono.
  Misturá-las esconderia justamente o que o questionário precisa saber sobre si;
- é diagnóstico interno, escopo `analytics:read:internal`, e não entra em
  publicação alguma.

## Consequências

- a hospedagem passa a ter motivo próprio para reportar, e o piloto tem como
  medir se isso é verdade;
- o comparativo fecha para hospedagem grande em vila pequena. É o custo aceito:
  o caso em que a comparação seria mais útil é o mesmo em que ela seria mais
  reveladora;
- nenhuma migration e nenhum escopo novo. O comparativo usa `stays:read:own`,
  que já libera a listagem de estadias — mais sensível, porque traz linha de
  hóspede;
- a ocupação fica indisponível ao público e ao comércio. Se um dia for
  necessária lá fora, o caminho é uma ADR que decida sobre a capacidade
  derivável, não um endpoint;
- o piso de dez na mediana torna o funil pouco informativo em vila pequena nos
  primeiros meses. Preferimos ausência a um número que descreve uma pessoa.

## Alternativas descartadas

- **reusar o limiar público de três no comparativo:** ignora que o leitor
  autenticado sabe subtrair a si mesmo, que é a diferença que motiva esta ADR;
- **publicar ocupação em banda larga para blindar a derivação:** a banda encarece
  a estimativa mas não impede a média de convergir ao longo de 730 dias;
- **telemetria de cliente no formulário:** daria tempo por tela, que é menos útil
  que a conversão por etapa, ao custo de coleta comportamental sem base legal;
- **materializar o funil no snapshot de publicação:** implicaria que ele flui
  para publicação. É diagnóstico, e a separação precisa ser estrutural.
