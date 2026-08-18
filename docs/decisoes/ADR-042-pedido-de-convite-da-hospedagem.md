# ADR-042 — Pedido de convite da hospedagem

**Status:** aceito para `PROTOTYPE_ONLY`.

**Relacionada:** [ADR-040](ADR-040-autocadastro-generalizado-e-aprovacao.md),
que vetou identidade no canal aberto do visitante;
[ADR-041](ADR-041-ativacao-de-conta-por-capability.md) para a emissão do acesso;
e [ADR-035](ADR-035-participacao-local-sem-cnpj-e-onboarding-prototipo.md) para
o onboarding sem CNPJ.

## Contexto

A capa convida a hospedagem a se cadastrar, e o botão levava para `/acesso`, que
é a tela de login. Quem ainda não tem conta chegava numa porta que só abre por
dentro. O cadastro real só existia por dois caminhos, ambos partindo de quem já
está dentro: a administração cria a acomodação, ou um operador autenticado a
cria para si.

O efeito prático é que a única forma de uma pousada entrar era mandar um e-mail
para alguém, e alguém digitar o cadastro inteiro à mão a partir daquele texto
solto. O dado chegava sem formato, sem validação e sem trilha.

A ADR-040 recusou identidade no canal aberto do visitante por três razões: não
existe cofre de identidade na cadeia executável, um estranho poderia criar
registro nominal falso sobre pessoa real, e o titular nunca seria contatado.
Este canal é aberto pelo mesmo motivo — quem pede ainda não tem conta — então
ele precisa ser confrontado com aquelas três razões, não isentado por analogia.

Ele responde de forma diferente porque o objeto é outro. Quem escreve não é um
terceiro descrevendo alguém: é o responsável descrevendo o próprio
estabelecimento e pedindo acesso a ele. O contato não é coleta acessória, é o
conteúdo do pedido — sem endereço não existe a quem devolver o acesso, e sem
nome da acomodação não existe o que aprovar. Recusar identidade aqui não
protegeria ninguém; apenas tornaria o pedido impossível de atender.

Resta a parte que não se dissolve: o endereço informado **não é verificado**.
Nada impede que alguém escreva o e-mail de outra pessoa.

## Decisão

- o canal público aceita identificação **do solicitante e do estabelecimento**:
  nome da acomodação, categoria, capacidade, município e UF, mais nome, e-mail e
  telefone opcional de quem responde. É a exceção declarada à ADR-040, limitada
  a quem se identifica sobre si mesmo, e não se estende a nenhum outro canal
  aberto;
- o pedido vive em `core.accommodation_access_requests`, **separado de**
  `core.accommodations`. Um pedido não é uma acomodação: nasce de origem anônima
  e pode ser recusado. Mesclar os dois colocaria toda tentativa recusada,
  inclusive as automatizadas, dentro da tabela que o resto do sistema lê como
  cadastro real;
- a aprovação **cria a acomodação e nada mais**. Ela não emite acesso. A
  emissão continua sendo o ato separado da ADR-041, porque aprovar a existência
  de um estabelecimento e entregar credencial a um endereço não verificado são
  duas decisões com consequências diferentes, e juntá-las num clique faria a
  segunda desaparecer atrás da primeira;
- a rejeição exige motivo de **lista fechada**, no precedente da ADR-040. Texto
  livre viraria dado pessoal permanente em tabela append-only;
- **a rejeição elimina os dados de contato na mesma transação** e preserva o
  fato auditável, o motivo e o instante. Quem foi recusado não fica arquivado;
- **o pedido pendente expira em 30 dias** e a purga do contato acontece no ciclo
  de limpeza já existente do worker. O prazo não é o de 72 horas da ADR-040 de
  propósito: lá o relógio corria contra um autocadastro de visitante já dentro
  da pousada, aqui corre contra a caixa de entrada de uma administração
  municipal, que não opera em turnos de três dias. Inação deixa de ser uma forma
  de retenção indefinida em ambos os casos, que é o que o precedente protege;
- **um pedido pendente por endereço**, garantido por índice único parcial. Quem
  reenvia porque não teve resposta recebe conflito, não uma segunda linha na
  fila. Depois de decidido, o mesmo endereço pode pedir de novo;
- a superfície aberta usa o mesmo proof-of-work e o mesmo rate limit por prefixo
  de rede das demais superfícies abertas. Diferente do caso da ADR-040, aqui o
  controle é adequado à ameaça: o abuso que interessa é volume — encher a fila
  de aprovação até ela deixar de ser lida —, e volume é exatamente o que esses
  controles encarecem;
- a fila e a decisão exigem `accommodations:onboard`, o escopo que já governa a
  criação de acomodação. Aprovar um pedido produz o mesmo efeito que criar o
  cadastro à mão, então não pode custar menos permissão do que criar à mão.

## Consequências

- A capa passa a ter um destino honesto: o botão que promete cadastro leva a um
  formulário que grava, e não a um login.
- A administração recebe o pedido com formato, validação e trilha, e decide
  sobre dado estruturado em vez de sobre um e-mail em prosa.
- O sistema passa a guardar dado pessoal vindo de canal aberto pela primeira
  vez. Isso é a mudança de fato desta ADR, e é o que a torna revisável: se a
  base legal do canal aberto for recusada, é este registro que sai.
- **Limitação declarada:** aprovar não prova que quem pediu controla o endereço
  informado. A prova só aparece um passo adiante, quando a administração emite a
  ativação e alguém a conclui — e mesmo ali a ADR-041 já registra que o que se
  prova é posse do link, não do endereço. A administração é quem carrega esse
  julgamento, e a tela precisa dizer isso em vez de sugerir que a aprovação
  autentica alguém.
- Permanece `BLOCKED` para dados reais até `SELF_SERVICE_LEGAL_BASIS` estar
  `PASS`, pela mesma razão da ADR-040, agora com a agravante de o canal ser
  nominal.
