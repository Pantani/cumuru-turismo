/**
 * Dicionário canônico do Observatório. Este arquivo é a fonte da verdade: o
 * tipo `Messages` deriva dele, então `messages-en.ts` e `messages-es.ts` só
 * compilam com exatamente as mesmas chaves. Chave faltando vira erro de tipo,
 * nunca texto ausente em produção.
 *
 * Interpolação usa `{nome}`; ver `interpolate` em `translate.ts`.
 */
export const messagesPt = {
  // ---------------------------------------------------------------- Navegação
  "app.nav.public": "Painel público",
  "app.nav.registration": "Registro",
  "app.nav.selfRegistration": "Autocadastro",
  "app.nav.activation": "Ativação",
  "app.nav.survey": "Pesquisa",
  "app.nav.workspace": "Área da hospedagem",
  "app.nav.questionnaires": "Questionários",
  "app.nav.quality": "Qualidade",
  "app.nav.aria": "Navegação principal",
  "app.skipLink": "Ir para o conteúdo",
  "app.brand.name": "Observatório Turístico",
  "app.brand.place": "Cumuruxatiba · Prado, Bahia",
  "app.signOut": "Sair",
  "app.documentTitle": "{page} · Observatório Turístico de Cumuruxatiba",
  "app.routeAnnounce": "Página atual: {title}",
  "app.routeLoading": "Carregando página…",
  "app.footer.note":
    "Protótipo técnico com dados fictícios. Uso real depende dos gates de governança do Município de Prado.",
  "app.locale.aria": "Idioma do site",
  "app.locale.pt": "PT",
  "app.locale.en": "EN",
  "app.locale.es": "ES",
  "app.locale.ptName": "Português",
  "app.locale.enName": "Inglês",
  "app.locale.esName": "Espanhol",

  "app.title.public": "Painel público do turismo",
  "app.title.registration": "Registro de visitantes",
  "app.title.selfRegistration": "Autocadastro pelo cartaz",
  "app.title.activation": "Ativação da conta",
  "app.title.survey": "Pesquisa com o visitante",
  "app.title.workspace": "Área da hospedagem",
  "app.title.questionnaires": "Questionários",
  "app.title.quality": "Qualidade dos dados",
  "app.title.notFound": "Página não encontrada",

  // ------------------------------------------------------- Landing: seções
  "landing.sections.aria": "Seções desta página",
  "landing.nav.numbers": "Números",
  "landing.nav.how": "Como funciona",
  "landing.nav.hosts": "Anfitriões",
  "landing.nav.commerce": "Comércio",
  "landing.nav.privacy": "Privacidade",
  "landing.nav.about": "Sobre",
  "landing.nav.register": "Cadastrar hospedagem",

  // ------------------------------------------------------------ Landing: hero
  "landing.hero.kicker": "Turismo em números",
  "landing.hero.titleLead": "O turismo da nossa praia,",
  "landing.hero.titleAccent": "finalmente em números.",
  "landing.hero.lead":
    "Pousadas e casas de aluguel registram as estadias em um só lugar. O Observatório devolve à comunidade indicadores públicos de presença e previsão — sem expor nenhum hóspede.",
  "landing.hero.primary": "Cadastrar minha hospedagem",
  "landing.hero.secondary": "Ver os números de hoje",
  "landing.hero.todayLabel": "Hoje na vila",
  "landing.hero.todayUnit": "pessoas-dia",
  "landing.hero.todayHint": "Gente que dorme aqui, semana a semana.",
  "landing.hero.todayPending": "Carregando",
  "landing.hero.image": "Foto da falésia e da praia de Cumuruxatiba (vertical)",

  // --------------------------------------------------------- Landing: ticker
  "landing.ticker.aria": "Resumo em movimento dos indicadores de hoje",
  "landing.ticker.today": "Presença hoje {value}",
  "landing.ticker.peak": "Pico previsto {value}",
  "landing.ticker.coverage": "{coverage}",
  "landing.ticker.prototype": "Dados fictícios de protótipo",
  "landing.ticker.pending": "Carregando indicadores públicos",

  // ---------------------------------------------------- Landing: como funciona
  "landing.how.index": "02",
  "landing.how.kicker": "Do check-in ao indicador",
  "landing.how.title": "Quatro passos, dois minutos por estadia.",
  "landing.how.lead":
    "Funciona no celular, aguenta internet fraca e não pede nada que você já não pergunte na recepção.",
  "landing.how.step1.title": "Cadastre a hospedagem",
  "landing.how.step1.body":
    "Nome do local, tipo de hospedagem e quantas pessoas cabem. Não pedimos CPF, CNPJ nem Cadastur. Pousada formal ou casa de família — os dois entram.",
  "landing.how.step2.title": "Abra a estadia",
  "landing.how.step2.body":
    "Chegada, saída e número de pessoas. Só isso já alimenta o indicador de presença da vila.",
  "landing.how.step3.title": "O hóspede preenche e você aprova",
  "landing.how.step3.body":
    "O QR code do balcão abre um formulário sem nome, documento, e-mail ou telefone: só faixa de idade, de onde a pessoa vem e as datas. A estadia chega pendente e só entra na conta da vila depois que você aprova.",
  "landing.how.step4.title": "A vila recebe o número",
  "landing.how.step4.body":
    "Tudo entra agregado, arredondado e protegido no painel público. Ninguém vê a sua ocupação separada.",
  "landing.how.image.square":
    "Praça do centro de Cumuruxatiba, com o comércio e a pousada da vila",
  "landing.how.image.gate":
    "Pórtico de entrada de Cumuruxatiba na estrada de acesso",
  "landing.how.image.street": "Rua da vila sob os coqueiros",

  // -------------------------------------------------------- Landing: anfitriões
  "landing.hosts.index": "03",
  "landing.hosts.kicker": "Para quem hospeda",
  "landing.hosts.title":
    "Sua casa também é turismo — e agora aparece na conta.",
  "landing.hosts.lead":
    "Não é fiscalização. É a primeira vez que Cumuruxatiba consegue dizer, com número, quanta gente dorme aqui em cada semana do ano.",
  "landing.hosts.benefit1.title": "Saiba quando encher a casa",
  "landing.hosts.benefit1.body":
    "Previsão de 30 dias com faixa provável para definir diária, equipe e compras.",
  "landing.hosts.benefit2.title": "Sem CNPJ, sem Cadastur, sem documento",
  "landing.hosts.benefit2.body":
    "Quem aluga uma casa participa igual a quem tem pousada registrada. O cadastro não pede documento nenhum.",
  "landing.hosts.benefit3.title": "Sua ocupação nunca é exposta",
  "landing.hosts.benefit3.body":
    "Nenhum indicador público é aberto por estabelecimento. Só existe o total da vila.",
  "landing.hosts.benefit4.title": "Funciona com internet ruim",
  "landing.hosts.benefit4.body":
    "O registro fica salvo no aparelho e sobe sozinho quando o sinal volta.",
  "landing.hosts.image": "Rede armada na varanda de uma hospedagem da vila",

  // ---------------------------------------------------------- Landing: cadastro
  "landing.register.title": "Cadastre sua hospedagem",
  "landing.register.body":
    "A equipe faz o cadastro com você, por e-mail ou na visita, e envia um link de ativação para você escolher a senha. O QR code do balcão fica dentro da área da hospedagem.",
  "landing.register.action": "Falar com a equipe",
  "landing.register.signIn": "Já tem acesso? Entrar na área da hospedagem",
  "landing.register.footnote": "Sem custo. Você pode sair quando quiser.",

  // ---------------------------------------------------------- Landing: comércio
  "landing.commerce.index": "04",
  "landing.commerce.kicker": "Para o comércio e as associações",
  "landing.commerce.title":
    "Planeje a temporada com o número, não com o boato.",
  "landing.commerce.lead":
    "Restaurantes, mercados, passeios de barco e transporte usam a mesma previsão pública que as hospedagens. Todo mundo enxerga a mesma curva.",
  "landing.commerce.item1.title": "Compras e estoque",
  "landing.commerce.item1.body":
    "Saber que vêm mil pessoas-dia no feriado muda o pedido do fornecedor da semana anterior.",
  "landing.commerce.item2.title": "Equipe temporária",
  "landing.commerce.item2.body":
    "A faixa provável indica quando contratar reforço e quando segurar o caixa.",
  "landing.commerce.item3.title": "Serviços da vila",
  "landing.commerce.item3.body":
    "Água, coleta de lixo e transporte passam a ser dimensionados por presença estimada, não por chute.",
  "landing.commerce.item4.title": "Argumento junto ao poder público",
  "landing.commerce.item4.body":
    "Uma série histórica sustenta pedido de investimento melhor do que qualquer impressão pessoal.",

  // -------------------------------------------------------------- Landing: mapa
  "landing.place.index": "05",
  "landing.place.kicker": "Onde estamos",
  "landing.place.title":
    "Cumuruxatiba, distrito de Prado, litoral sul da Bahia.",
  "landing.place.lead":
    "Entre o Monte Pascoal e a foz do Rio Jucuruçu, uma faixa de praia protegida por falésias que ainda recebe o visitante pelo boca a boca.",
  "landing.place.municipality": "Município",
  "landing.place.municipalityValue": "Prado, Bahia",
  "landing.place.timezone": "Fuso",
  "landing.place.beach": "Faixa de praia",
  "landing.place.beachValue": "Cerca de 17 km",
  "landing.place.season": "Alta temporada",
  "landing.place.seasonValue": "Dezembro a fevereiro",
  "landing.place.image":
    "Praia do Centro na maré baixa, com as falésias da Costa das Baleias ao fundo",

  // ------------------------------------------------------- Landing: privacidade
  "landing.privacy.index": "06",
  "landing.privacy.kicker": "Privacidade por construção",
  "landing.privacy.titleLead": "O comércio recebe a curva.",
  "landing.privacy.titleAccent": "Ninguém recebe o hóspede.",
  "landing.privacy.lead":
    "A separação entre o dado da estadia e o indicador público é estrutural, não uma promessa de uso.",
  "landing.privacy.item1.title": "O painel nunca lê resposta bruta",
  "landing.privacy.item1.body":
    "O site público consulta apenas células já agregadas e arredondadas. Não existe caminho do gráfico até uma pessoa.",
  "landing.privacy.item2.title": "Supressão automática de célula pequena",
  "landing.privacy.item2.body":
    "Abaixo do limiar de contribuições ou de hospedagens, o valor não é publicado e nenhum substituto aparece no lugar.",
  "landing.privacy.item3.title": "Pesquisa é sempre opcional",
  "landing.privacy.item3.body":
    "Recusar a pesquisa de perfil não impede o check-in nem muda o atendimento. O consentimento fica registrado e pode ser revogado pelo encarregado de dados.",
  "landing.privacy.item4.title": "Direitos do titular pela LGPD",
  "landing.privacy.item4.body":
    "Acesso, correção e exclusão do próprio registro são atendidos pelo encarregado de dados, no e-mail do fim desta página. Cada operação sobre o registro fica na trilha de auditoria.",
  "landing.privacy.prototypeTitle": "Ambiente de protótipo",
  "landing.privacy.prototypeBody":
    "Esta demonstração usa somente dados fictícios. Não substitui estatística oficial nem censo, não autoriza operação real e nenhum cadastro municipal se torna obrigatório sem fundamento jurídico formal da Prefeitura de Prado.",

  // -------------------------------------------------------- Landing: sobre e FAQ
  "landing.about.index": "07",
  "landing.about.kicker": "Sobre e governança",
  "landing.about.title":
    "Um observatório da vila, com regra escrita antes do dado.",
  "landing.about.body1":
    "Cumuruxatiba é distrito do Município de Prado. Qualquer obrigatoriedade de cadastro depende de patrocínio formal da Prefeitura ou de entidade com competência delegada — até lá, a participação é voluntária.",
  "landing.about.body2":
    "Perguntas publicadas são imutáveis: mudança gera nova versão. Toda escrita é idempotente e nenhuma integração externa que falhe faz o registro local se perder.",
  "landing.about.guideCityHall": "Guia do Observatório para a Prefeitura",
  "landing.about.guideCityHallMeta": "PDF · apresentação e treinamento",
  "landing.about.guideFnrh": "Guia para gerar a chave FNRH",
  "landing.about.guideFnrhMeta": "PDF · para hospedagens elegíveis",

  "landing.faq.title": "Perguntas frequentes",
  "landing.faq.q1.question": "Preciso ter CNPJ para participar?",
  "landing.faq.q1.answer":
    "Não. Também não pedimos CPF nem Cadastur: o cadastro é feito com o nome do local, o tipo e a capacidade. Participar do Observatório não comprova regularidade nem licenciamento.",
  "landing.faq.q2.question":
    "Meus concorrentes vão ver quantos hóspedes eu tenho?",
  "landing.faq.q2.answer":
    "Não. Nenhum indicador é aberto por estabelecimento. O painel publica apenas o total da vila, arredondado e com supressão quando poucas hospedagens contribuem.",
  "landing.faq.q3.question": "Isso substitui a FNRH?",
  "landing.faq.q3.answer":
    "Não. A integração com a FNRH Digital é uma trilha separada e opcional, com a chave oficial de cada estabelecimento. O Observatório não emite nem compartilha essa credencial.",
  "landing.faq.q4.question": "O hóspede é obrigado a responder a pesquisa?",
  "landing.faq.q4.answer":
    "Nunca. A pesquisa de perfil é voluntária e separada do registro da estadia. Recusar não afeta o check-in.",
  "landing.faq.q5.question": "E se a internet cair no meio do registro?",
  "landing.faq.q5.answer":
    "O registro fica guardado no aparelho e é enviado quando a conexão volta. Reenvio do mesmo registro não gera estadia duplicada.",
  "landing.faq.q6.question": "Quanto custa?",
  "landing.faq.q6.answer":
    "Nada para a hospedagem. O Observatório é uma iniciativa de interesse público da vila e você pode encerrar a participação quando quiser.",

  // ----------------------------------------------------------- Landing: contato
  "landing.contact.title": "Quer entender melhor antes de entrar?",
  "landing.contact.lead":
    "A equipe passa nas hospedagens para explicar pessoalmente e ajudar no primeiro cadastro. Escreva para a equipe ou peça uma visita.",
  "landing.contact.write": "Escrever para a equipe",
  "landing.contact.visit": "Pedir uma visita",
  "landing.contact.email": "E-mail",
  "landing.contact.dpo": "Encarregado de dados",
  "landing.contact.mark": "OBSERVATÓRIO TURÍSTICO · CUMURUXATIBA",

  "landing.license.ccBy20": "CC BY 2.0",
  "landing.license.ccBySa30": "CC BY-SA 3.0",
  "landing.license.ccBySa40": "CC BY-SA 4.0",
  "landing.license.publicDomain": "Domínio público",
  "landing.photoCredit": "Foto: {author} · {license} · Wikimedia Commons",
  "landing.photoSource":
    "Abrir a página da foto de {author} no Wikimedia Commons",

  // -------------------------------------------------------- Painel: envelope
  "analytics.index": "01",
  "analytics.kicker": "Publicação protegida",
  "analytics.title": "Indicadores públicos",
  "analytics.lead":
    "Tendências agregadas para planejar temporada, equipe e estoque. Sem microdados, sem identificação de hóspedes e sem recorte por estabelecimento.",
  "analytics.prototypeBadge": "Dados fictícios de protótipo",
  "analytics.loading": "Atualizando indicadores públicos…",
  "analytics.error": "Não foi possível carregar os indicadores públicos.",
  "analytics.retry": "Tentar novamente",

  "analytics.metadata.aria": "Contexto dos indicadores",
  "analytics.metadata.updated": "Atualização",
  "analytics.metadata.coverage": "Cobertura",
  "analytics.metadata.unit": "Unidade",
  "analytics.metadata.mode": "Modo dos dados",
  "analytics.coverage.published": "Cobertura estimada: {ratio}%",
  "analytics.coverage.protected":
    "Cobertura protegida pela política de publicação",
  "analytics.coverage.unavailable": "Cobertura indisponível",
  "analytics.unit.personDay": "Pessoas-dia",
  "analytics.unit.surveyAnswer": "Respostas de pesquisa",
  "analytics.unit.inline": "pessoas-dia",

  // ------------------------------------------------------- Painel: cartões
  "analytics.summary.aria": "Resumo da presença",
  "analytics.summary.today": "Presença de hoje",
  "analytics.summary.todayHint":
    "Pessoas presentes hoje, contadas no intervalo entre chegada e saída, no fuso America/Bahia. Uma pessoa conta uma vez por dia.",
  "analytics.summary.peak": "Pico previsto nos próximos 30 dias",
  "analytics.summary.peakHint":
    "Maior estimativa central do baseline explicável. Planeje pela faixa provável, nunca pelo número central isolado.",
  "analytics.kind.observed": "● Observado",
  "analytics.kind.forecast": "◇ Previsto",
  "analytics.state.protected": "Dado protegido",
  "analytics.state.unavailable": "Dado indisponível",
  "analytics.value.observed": "{value} pessoas-dia",
  "analytics.value.central": "Estimativa central: {value} pessoas-dia",
  "analytics.value.band": "Faixa provável: {lower} a {upper}",
  "analytics.percent.plain": "{percent}%",
  "analytics.percent.signed": "{sign}{percent}%",
  "analytics.summary.forecastTotal": "Total previsto em 30 dias",
  "analytics.summary.forecastTotalHint":
    "Soma das estimativas centrais dos {count} dias previstos publicados, em pessoas-dia. É o volume do mês, não o de um dia.",
  "analytics.summary.forecastTotalNone":
    "Nenhum dia do horizonte tem previsão publicada, então não há volume a somar.",

  // -------------------------------------------------------- Painel: série
  "analytics.presence.kicker": "Série em pessoas-dia",
  "analytics.presence.title": "Presença ao longo do tempo",
  "analytics.updating": "Atualizando a janela…",
  "analytics.history.label": "Histórico",
  "analytics.history.recent30": "30 dias",
  "analytics.history.recent90": "90 dias",
  "analytics.history.recent365": "1 ano",
  "analytics.history.recent730": "2 anos",
  "analytics.history.month": "Mês",
  "analytics.history.monthLabel": "Mês consultado",
  "analytics.history.monthPrevious": "Mês anterior",
  "analytics.history.monthNext": "Mês seguinte",
  "analytics.view.label": "Mostrar",
  "analytics.view.observed": "Observado",
  "analytics.view.combined": "Observado e previsto",
  "analytics.view.forecast": "Só previsão",
  "analytics.window.scope.recent": "dias observados da janela",
  "analytics.window.scope.next": "próximos 30 dias",
  "analytics.window.scope.combined": "dias observados da janela",

  "analytics.legend.aria": "Legenda da série",
  "analytics.legend.observed": "Observado",
  "analytics.legend.forecast": "Previsto, com faixa provável",
  "analytics.legend.gap": "Protegido ou indisponível, sem valor substituto",
  "analytics.legend.average": "Média de referência",
  "analytics.legend.trend": "Média móvel de {days} dias",
  "analytics.legend.weekend": "Fim de semana",

  "analytics.stats.aria": "Estatísticas dos {scope}",
  "analytics.stats.rhythmAria": "Ritmo e alcance dos {scope}",
  "analytics.tile.average": "Média diária",
  "analytics.tile.averageHint":
    "Média dos {count} dias com valor publicado. A linha tracejada do gráfico marca esse nível.",
  "analytics.tile.peak": "Dia mais cheio",
  "analytics.tile.peakHint":
    "Maior valor publicado da janela e o dia em que ocorreu.",
  "analytics.tile.trough": "Dia mais vazio",
  "analytics.tile.troughHint":
    "Menor valor publicado da janela. Compare com o dia mais cheio para dimensionar a variação.",
  "analytics.tile.trend": "Tendência",
  "analytics.tile.trendHint":
    "Média dos {count} últimos dias publicados ante os {count} anteriores. Dias protegidos são pulados, nunca contados como zero.",
  "analytics.tile.trendNone":
    "São necessários pelo menos dois dias publicados para comparar períodos.",
  "analytics.tile.median": "Dia comum",
  "analytics.tile.medianHint":
    "Mediana dos dias publicados: metade dos dias ficou acima, metade abaixo. Um feriado isolado desloca a média, não a mediana.",
  "analytics.tile.weekend": "Fim de semana",
  "analytics.tile.weekendHint":
    "Quanto o sábado e o domingo pesam acima ou abaixo dos dias úteis da janela. É o número que dimensiona escala de equipe e compra de estoque.",
  "analytics.tile.weekendNone":
    "É preciso ao menos um dia publicado em cada lado para comparar fim de semana com dia útil.",
  "analytics.tile.variation": "Variabilidade",
  "analytics.tile.variationHint":
    "Desvio-padrão como percentual da média. Perto de 20% a demanda é previsível; acima de 60% planejar pela média engana.",
  "analytics.tile.variationNone":
    "São necessários pelo menos dois dias publicados para medir a variação.",
  "analytics.tile.total": "Total acumulado",
  "analytics.tile.totalHint":
    "Soma dos dias publicados. Dias protegidos ficam de fora, então o total é um piso, não um censo.",
  "analytics.tile.totalPartialHint":
    "Soma parcial: {count} dias da janela ficaram sem valor publicado e não entram no total. O número é um piso, não um censo.",
  "analytics.tile.published": "Dias publicados",
  "analytics.tile.publishedValue": "{published} de {days}",
  "analytics.withheld.none": "Todos os dias da janela têm valor publicado.",
  "analytics.withheld.one":
    "1 dia ficou sem valor por proteção estatística ou ausência de dado. Nenhum valor substituto é exibido.",
  "analytics.withheld.other":
    "{count} dias ficaram sem valor por proteção estatística ou ausência de dado. Nenhum valor substituto é exibido.",

  "analytics.empty": "—",

  // ------------------------------------------------------ Painel: dia da semana
  "analytics.weekday.title": "Ritmo da semana",
  "analytics.weekday.hint":
    "Média por dia da semana sobre os dias publicados da janela observada. Mostra o padrão semanal que a série dia a dia esconde; com poucos dias publicados, uma única data pode responder por todo o dia da semana.",
  "analytics.weekday.aria": "Média por dia da semana",
  "analytics.weekday.none": "sem dia publicado",
  "analytics.weekday.days.one": " · 1 dia",
  "analytics.weekday.days.other": " · {count} dias",

  // ---------------------------------------------------------- Painel: tabela
  "analytics.table.aria": "Presença observada e prevista",
  "analytics.table.date": "Data",
  "analytics.table.kind": "Tipo",
  "analytics.table.result": "Resultado",
  "analytics.table.delta": "Ante a média",
  "analytics.details": "Ver a série dia a dia",

  // ------------------------------------------------------ Painel: preferências
  "analytics.preferences.kicker": "Pesquisa voluntária",
  "analytics.preferences.title": "Perfil de visita agregado",
  "analytics.preferences.periodLabel": "Período das preferências",
  "analytics.preferences.lastCompleteMonth": "Último mês completo",
  "analytics.preferences.lead":
    "Unidade: respostas de pesquisa estruturadas e consentidas. Uma resposta de grupo não é multiplicada pela quantidade de visitantes.",
  "analytics.preferences.firstVisit": "Primeira visita",
  "analytics.preferences.returning": "Visitante recorrente",
  "analytics.preferences.share": "{percent}% das respostas elegíveis",

  // ------------------------------------------------------- Painel: metodologia
  "analytics.methodology.kicker": "Como interpretar",
  "analytics.methodology.title": "Metodologia e limitações",
  "analytics.methodology.observed": "Presença observada",
  "analytics.methodology.observedBody":
    "Cada pessoa contribui no intervalo civil entre chegada e saída, em America/Bahia. A saída não conta como novo dia de presença.",
  "analytics.methodology.forecast": "Presença prevista",
  "analytics.methodology.forecastBody":
    "O baseline explicável combina reservas já conhecidas e histórico sazonal. A faixa normal usa limites de {low}% a {high}%. Sem histórico elegível suficiente, o baseline usa fallback mais amplo, de {fallbackLow}% a {fallbackHigh}%. O contrato público não identifica qual faixa foi aplicada a cada ponto; por isso a interface não atribui esse estado a valores individuais.",
  "analytics.methodology.protection": "Proteção estatística",
  "analytics.methodology.protectionBody":
    "Células abaixo de {threshold} contribuições ou de {accommodations} acomodações são protegidas. Há supressão complementar e arredondamento em base {rounding}.",
  "analytics.methodology.limits": "Limitações",
  "analytics.methodology.limitsBody":
    "A cobertura é parcial e não representa um censo. Os dados são fictícios de protótipo; acurácia operacional, política municipal e uso com dados reais permanecem não verificados.",

  // ---------------------------------------------------------- Painel: gráfico
  "analytics.chart.empty":
    "Nenhum dia da janela tem valor publicável; toda a série está protegida ou indisponível.",
  "analytics.chart.summary":
    "Série de {days} dias em pessoas-dia. {published} dias com valor publicado e {withheld} protegidos ou indisponíveis, exibidos como falha na base do gráfico.",
  "analytics.chart.average": "média {value}",
  "analytics.chart.forecastFrom": "previsão a partir daqui",
  "analytics.chart.readoutIdle":
    "Aponte um dia no gráfico ou use as setas ← → do teclado para ler cada dia. Home e End vão ao primeiro e ao último dia.",
  "analytics.chart.readoutDay": "{day}. {lines}.",
  "analytics.chart.caption":
    "Escala até {bound} pessoas-dia. Fim de semana aparece com fundo sombreado, a linha tracejada marca a média da janela, a linha cheia é a média móvel de {days} dias e dias protegidos aparecem como falha, sem valor substituto. A média móvel interrompe onde faltam valores.",

  "analytics.slot.observed": "Observado: {value} pessoas-dia",
  "analytics.slot.forecast": "Previsto: {value} pessoas-dia",
  "analytics.slot.band": "Faixa provável: {lower} a {upper} pessoas-dia",
  "analytics.slot.protected":
    "Protegido pela política de publicação, sem valor substituto",
  "analytics.slot.unavailable": "Sem dado disponível para este dia",
  "analytics.delta.same": "no mesmo nível da média de referência",
  "analytics.delta.above": "{percent}% acima da média de referência",
  "analytics.delta.below": "{percent}% abaixo da média de referência",
  "analytics.trend.value": "{sign}{percent}% ante os {size} dias anteriores",

  // -------------------------------------------------- Autocadastro pelo cartaz
  "selfService.posterRequired.title": "Cartaz necessário",
  "selfService.posterRequired.body":
    "Leia o código QR exposto pela hospedagem. O token fica no fragmento do endereço, nunca é enviado ao servidor e é apagado da barra de endereço antes desta página aparecer.",
  "selfService.eyebrow": "Cartaz da hospedagem",
  "selfService.pageTitle": "Autocadastro pelo cartaz",
  "selfService.validating": "Validando o cartaz…",
  "selfService.pending.title": "Cartaz em validação",
  "selfService.retry": "Tentar de novo",
  "selfService.verifyingDevice": "Verificando este dispositivo…",
  "selfService.formTitle": "Confirme os dados da estadia",
  "selfService.submit": "Enviar autocadastro",
  "selfService.submitting": "Enviando…",

  "selfService.privacy.title": "Aviso de privacidade",
  "selfService.privacy.introBefore": "Você está iniciando um cadastro para",
  "selfService.privacy.introAfter":
    ". Este formulário é aberto e não pede identificação: coletamos apenas faixa etária, papel no grupo, residência e datas previstas.",
  "selfService.privacy.noIdentity":
    "Não informe nome, documento, e-mail, telefone, endereço ou qualquer observação pessoal. Estes dados não são aceitos aqui.",
  "selfService.privacy.needsApproval":
    "A hospedagem precisa aprovar o cadastro. Sem aprovação, nada entra em estatística nem no painel público.",
  "selfService.privacy.expiry":
    "Se a hospedagem recusar, ou se ninguém decidir em 72 horas, os dados enviados são eliminados e resta apenas o registro auditável da decisão.",
  "selfService.privacy.assistedAlternative":
    "Se preferir não usar este formulário, peça à hospedagem para registrar a estadia com você, pelo canal assistido.",
  "selfService.privacy.versionLabel": "Versão do aviso:",

  "selfService.window.legend": "Período previsto",
  "selfService.window.arrival": "Data de chegada",
  "selfService.window.departure": "Data de saída",

  "selfService.completion.title": "Autocadastro enviado",
  "selfService.completion.body":
    "A hospedagem precisa aprovar este cadastro. Se ninguém decidir em 72 horas, o pedido expira e os dados enviados são eliminados. Nada entra em estatística ou no painel público antes da aprovação.",
  "selfService.completion.continueSurvey": "Responder pesquisa voluntária",

  "selfService.error.forbidden":
    "Este cartaz não está aceitando cadastros agora.",
  "selfService.error.notFound":
    "Este cartaz não é mais válido. Peça um novo à hospedagem.",
  "selfService.error.conflict":
    "O aviso de privacidade mudou desde que o cartaz foi impresso. Peça um cartaz atualizado à hospedagem.",
  "selfService.error.unprocessable":
    "Alguns dados não são aceitos neste formulário aberto. Revise e tente de novo.",
  "selfService.error.rateLimited":
    "Já houve envios demais desta rede agora há pouco.",
  "selfService.error.proofOfWorkAborted":
    "A verificação foi interrompida. Envie novamente quando quiser.",
  "selfService.error.offline":
    "Sem conexão agora. O que você preencheu ficou guardado, cifrado, neste dispositivo.",

  // ------------------------------------------------ Cópia de erro para hóspede
  "guestCopy.unexpectedFailure":
    "Não conseguimos falar com o serviço agora. Tente de novo em alguns instantes.",
  "guestCopy.idempotencyInProgress":
    "Já recebemos este envio e estamos concluindo.",
  "guestCopy.retrySeconds.one": "Tente novamente em 1 segundo.",
  "guestCopy.retrySeconds.other": "Tente novamente em {seconds} segundos.",

  // -------------------------------------------------------- Editor de visitantes
  "visitor.role.responsible": "Responsável",
  "visitor.role.companion": "Acompanhante",
  "visitor.role.minor": "Menor",
  "visitor.ageBand.0_5": "0 a 5",
  "visitor.ageBand.6_11": "6 a 11",
  "visitor.ageBand.12_17": "12 a 17",
  "visitor.ageBand.18_24": "18 a 24",
  "visitor.ageBand.25_34": "25 a 34",
  "visitor.ageBand.35_44": "35 a 44",
  "visitor.ageBand.45_59": "45 a 59",
  "visitor.ageBand.60_plus": "60 ou mais",
  "visitor.legend": "Visitante {number}",
  "visitor.roleLabel": "Papel do visitante {number}",
  "visitor.ageBandLabel": "Faixa etária do visitante {number}",
  "visitor.residenceState": "UF de residência do visitante {number}",
  "visitor.residenceCity": "Município IBGE do visitante {number}",
  "visitor.residenceCountry": "País de residência do visitante {number}",
  "visitor.remove": "Remover visitante {number}",
  "visitor.add": "Adicionar visitante",
} as const;

export type MessageKey = keyof typeof messagesPt;
export type Messages = Record<MessageKey, string>;
