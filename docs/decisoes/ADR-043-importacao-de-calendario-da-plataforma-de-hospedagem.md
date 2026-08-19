# ADR-043 — Importação de calendário da plataforma de hospedagem

**Status:** aceito para `PROTOTYPE_ONLY`.

**Relacionada:** [ADR-020](ADR-020-minimizacao-presenca-e-eventos-da-fase-2.md),
que recusou o cofre de identidade;
[ADR-040](ADR-040-autocadastro-generalizado-e-aprovacao.md) para o limite do que
um canal sem o titular pode coletar; e
[ADR-029](ADR-029-presenca-cobertura-e-forecast-da-fase-4.md) para o efeito de
uma estadia sobre a presença publicada.

## Contexto

A hospedagem que vende pelo Booking.com já digitou chegada e saída lá. Digitar
de novo no Observatório é trabalho puro, e trabalho repetido é a primeira coisa
que se abandona quando a temporada aperta. Reduzir esse atrito é o objetivo.

O caminho óbvio — consumir a API do Booking.com — não está disponível. O
Connectivity API exige ser Connectivity Partner homologado: conformidade PCI e
PII, software que gerencie preço, disponibilidade, reservas e conteúdo em tempo
real, e carga de no mínimo um ano de tarifas por propriedade. Isso é a descrição
de um channel manager. O Observatório não distribui inventário, não toca em
tarifa e não quer assumir responsabilidade sobre a receita de ninguém. A porta
existe e está fechada para quem nós somos.

O que existe sem parceria é o `.ics` que o próprio anfitrião exporta do extranet.
Ele tem três limitações que não são contornáveis e por isso definem o desenho:

1. **não traz identidade.** O Airbnb removeu nome, e-mail e detalhes da reserva
   do iCal em dezembro de 2019, e o Booking.com exporta status de quarto. Isso
   aqui é uma vantagem, não um defeito: é exatamente a fronteira que a ADR-020
   desenhou;
2. **não traz número de hóspedes.** O `.ics` diz que o quarto está ocupado de
   uma data a outra, não por quantas pessoas;
3. **não distingue reserva de bloqueio manual com confiabilidade.** Manutenção,
   uso do dono e reserva real chegam pelo mesmo canal. O Airbnb permite separar
   pelo `SUMMARY`; o Booking.com nem sempre.

A terceira é a que tem consequência real. `analytics.presence_days` conta pessoa
por dia, e um bloqueio de manutenção importado como estadia infla o indicador
público. Trocar subregistro por sobrecontagem seria piorar o dado enquanto
parece melhorá-lo.

## Decisão

- **importar o calendário, não a reserva.** O feed produz linha em
  `core.calendar_reservations`, que é intenção de estadia observada de fora, e
  não `core.stays`. Uma tabela guarda o que a plataforma disse; a outra guarda o
  que a hospedagem afirma. Mesclar as duas colocaria bloqueio de manutenção
  dentro do registro que o `analytics` lê como presença;
- **a estadia só nasce por confirmação humana.** A hospedagem abre a reserva
  importada com as datas já preenchidas, informa o número de hóspedes e
  confirma; aí o sistema cria a `core.stays` pelo mesmo caminho de sempre, com
  os mesmos invariantes, a mesma idempotência e a mesma trilha. Não existe
  importação que vire presença sozinha. É a única resposta honesta à limitação 3,
  e ela também resolve a 2 sem inventar um número;
- **nada de identidade entra por este canal**, mesmo que uma plataforma passe a
  oferecer. O canal não tem o titular do outro lado, que é a mesma razão pela
  qual a ADR-040 recusou identidade no autocadastro. Se o `.ics` trouxer nome no
  `SUMMARY`, ele é descartado na borda, não guardado e depois filtrado;
- **o `UID` do evento é guardado sob HMAC**, com versão de chave, e nunca em
  claro. Ele é o identificador da reserva na plataforma de origem: em claro
  seria um dado de negócio de terceiro guardado sem necessidade. Sob HMAC ele
  ainda serve à única coisa de que precisamos — reconhecer que o evento de hoje
  é o mesmo de ontem;
- **a URL do feed é segredo e fica cifrada em repouso**, com o keyring
  versionado e AES-GCM já usados no texto livre da pesquisa. Quem tem o link lê
  o calendário do anúncio inteiro. Ela não aparece em log, não volta em resposta
  de API e não é exibida depois de salva: a tela mostra o provedor, o rótulo e a
  data da última sincronização;
- **o feed pertence à acomodação e é gerido por quem já opera as estadias**
  (`stays:write`). Cadastrar um feed é declarar de onde vêm as datas daquela
  hospedagem, o que é o mesmo ato de quem registra estadia à mão, e não custa
  permissão diferente;
- **falha de sincronização é visível e nunca bloqueia**. Feed inalcançável,
  resposta que não é iCal ou evento malformado registram o motivo no próprio
  feed e param naquele feed. O registro manual continua sendo o caminho
  principal, e o importador é conveniência sobre ele;
- **remover o feed não apaga o que já foi confirmado.** As estadias criadas a
  partir dele são fato da hospedagem, não do feed. O que some é a origem.

## Consequências

- A hospedagem que vende pelo Booking deixa de digitar duas datas por reserva e
  passa a confirmar uma linha já preenchida. O ganho é real e é pequeno, e
  descrevê-lo como pequeno é parte da decisão: quem esperava que a integração
  eliminasse a jornada vai continuar convidando o hóspede na mão.
- O sistema passa a fazer requisição de saída para host informado por usuário,
  o que é superfície nova. O importador aceita apenas `https`, resolve com
  timeout, limita o tamanho da resposta e não segue redirecionamento que troque
  de host, volte para `http` ou reintroduza credencial no endereço — o `.ics`
  legítimo não precisa de nenhuma dessas coisas.
- A recusa de endereço interno acontece **duas vezes, e a que vale é a
  segunda**. Na hora de salvar, só dá para julgar o que foi digitado, e um nome
  não é um endereço: `pousada.invalid` pode resolver para `127.0.0.1`, e pode
  resolver diferente na segunda consulta. Por isso a checagem que decide roda no
  momento de discar, depois da resolução, uma vez por conexão, sobre o endereço
  que será realmente contatado. A validação do formulário continua existindo
  para recusar cedo o literal óbvio, não porque baste.
- **Limitação declarada:** o `.ics` é assinatura de disponibilidade, não de
  verdade. Reserva cancelada some do feed sem dizer que foi cancelada, e o
  sistema trata desaparecimento como retirada da fila, nunca como cancelamento
  de estadia já confirmada. Corrigir estadia continua sendo ato da hospedagem.
- **Limitação declarada:** nada aqui prova que o feed cadastrado pertence à
  acomodação que o cadastrou. A URL é colada por quem está autenticado naquela
  hospedagem, e é isso que se sabe.
- O desenho vale para qualquer origem que fale iCal, mas a entrega é só
  Booking.com. O provedor é campo, não abstração especulativa: quando entrar o
  segundo, ele é uma linha na lista, e a decisão de qual entra continua sendo
  sobre quantas hospedagens daqui usam aquilo, não sobre elegância.
- Permanece `PROTOTYPE_ONLY`. A cifra da URL depende de chave em variável de
  ambiente, e o gate de KMS continua aberto: até ele fechar, só URL fictícia.
