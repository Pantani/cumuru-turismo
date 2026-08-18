import type { Messages } from "./messages-pt";

/** Espanhol. O tipo `Messages` recusa a compilação se faltar qualquer chave. */
export const messagesEs: Messages = {
  // ---------------------------------------------------------------- Navegação
  "app.nav.public": "Panel público",
  "app.nav.registration": "Registro",
  "app.nav.selfRegistration": "Autorregistro",
  "app.nav.activation": "Activación",
  "app.nav.survey": "Encuesta",
  "app.nav.workspace": "Área del alojamiento",
  "app.nav.questionnaires": "Cuestionarios",
  "app.nav.quality": "Calidad",
  "app.nav.aria": "Navegación principal",
  "app.skipLink": "Ir al contenido",
  "app.brand.name": "Observatorio Turístico",
  "app.brand.place": "Cumuruxatiba · Prado, Bahía",
  "app.signOut": "Salir",
  "app.documentTitle": "{page} · Observatorio Turístico de Cumuruxatiba",
  "app.routeAnnounce": "Página actual: {title}",
  "app.routeLoading": "Cargando página…",
  "app.footer.note":
    "Prototipo técnico con datos ficticios. El uso real depende de los gates de gobernanza del Municipio de Prado.",
  "app.locale.aria": "Idioma del sitio",
  "app.locale.pt": "PT",
  "app.locale.en": "EN",
  "app.locale.es": "ES",
  "app.locale.ptName": "Portugués",
  "app.locale.enName": "Inglés",
  "app.locale.esName": "Español",

  "app.title.public": "Panel público del turismo",
  "app.title.registration": "Registro de visitantes",
  "app.title.selfRegistration": "Autorregistro por cartel",
  "app.title.activation": "Activación de la cuenta",
  "app.title.survey": "Encuesta al visitante",
  "app.title.workspace": "Área del alojamiento",
  "app.title.questionnaires": "Cuestionarios",
  "app.title.quality": "Calidad de los datos",
  "app.title.notFound": "Página no encontrada",

  // ------------------------------------------------------- Landing: seções
  "landing.sections.aria": "Secciones de esta página",
  "landing.nav.numbers": "Números",
  "landing.nav.how": "Cómo funciona",
  "landing.nav.hosts": "Anfitriones",
  "landing.nav.commerce": "Comercio",
  "landing.nav.privacy": "Privacidad",
  "landing.nav.about": "Acerca de",
  "landing.nav.register": "Registrar alojamiento",

  // ------------------------------------------------------------ Landing: hero
  "landing.hero.kicker": "Turismo en números",
  "landing.hero.titleLead": "El turismo de nuestra playa,",
  "landing.hero.titleAccent": "por fin en números.",
  "landing.hero.lead":
    "Posadas y casas de alquiler registran las estancias en un solo lugar. El Observatorio devuelve a la comunidad indicadores públicos de presencia y previsión — sin exponer a ningún huésped.",
  "landing.hero.primary": "Registrar mi alojamiento",
  "landing.hero.secondary": "Ver los números de hoy",
  "landing.hero.todayLabel": "Hoy en el pueblo",
  "landing.hero.todayUnit": "personas-día",
  "landing.hero.todayHint": "Gente que duerme aquí, semana a semana.",
  "landing.hero.todayPending": "Cargando",
  "landing.hero.image":
    "Foto del acantilado y la playa de Cumuruxatiba (vertical)",

  // --------------------------------------------------------- Landing: ticker
  "landing.ticker.aria": "Resumen en movimiento de los indicadores de hoy",
  "landing.ticker.today": "Presencia hoy {value}",
  "landing.ticker.peak": "Pico previsto {value}",
  "landing.ticker.coverage": "{coverage}",
  "landing.ticker.prototype": "Datos ficticios de prototipo",
  "landing.ticker.pending": "Cargando indicadores públicos",

  // ---------------------------------------------------- Landing: como funciona
  "landing.how.index": "02",
  "landing.how.kicker": "Del check-in al indicador",
  "landing.how.title": "Cuatro pasos, dos minutos por estancia.",
  "landing.how.lead":
    "Funciona en el móvil, tolera internet débil y no pide nada que usted no pregunte ya en la recepción.",
  "landing.how.step1.title": "Registre el alojamiento",
  "landing.how.step1.body":
    "Nombre, identificación fiscal, tipo y capacidad aproximada. Posada formal o casa familiar — ambas entran.",
  "landing.how.step2.title": "Abra la estancia",
  "landing.how.step2.body":
    "Llegada, salida y número de personas. Solo eso ya alimenta el indicador de presencia del pueblo.",
  "landing.how.step3.title": "El huésped completa o usted aprueba",
  "landing.how.step3.body":
    "Un código QR en el mostrador lleva al visitante a su propio registro. Si no quiere, la recepción lo completa y aprueba.",
  "landing.how.step4.title": "El pueblo recibe el número",
  "landing.how.step4.body":
    "Todo entra agregado, redondeado y protegido en el panel público. Nadie ve su ocupación por separado.",
  "landing.how.image.square":
    "Plaza del centro de Cumuruxatiba, con el comercio y la posada del pueblo",
  "landing.how.image.gate":
    "Pórtico de entrada de Cumuruxatiba en el camino de acceso",
  "landing.how.image.street": "Calle del pueblo bajo los cocoteros",

  // -------------------------------------------------------- Landing: anfitriões
  "landing.hosts.index": "03",
  "landing.hosts.kicker": "Para quien hospeda",
  "landing.hosts.title":
    "Su casa también es turismo — y ahora aparece en la cuenta.",
  "landing.hosts.lead":
    "No es fiscalización. Es la primera vez que Cumuruxatiba puede decir, con número, cuánta gente duerme aquí cada semana del año.",
  "landing.hosts.benefit1.title": "Sepa cuándo se llena la casa",
  "landing.hosts.benefit1.body":
    "Previsión de 30 días con rango probable para definir tarifas, equipo y compras.",
  "landing.hosts.benefit2.title":
    "Con identificación personal o de empresa, da igual",
  "landing.hosts.benefit2.body":
    "Quien alquila una casa participa igual que una posada registrada. Sin licencia turística obligatoria.",
  "landing.hosts.benefit3.title": "Su ocupación nunca se expone",
  "landing.hosts.benefit3.body":
    "Ningún indicador público se abre por establecimiento. Solo existe el total del pueblo.",
  "landing.hosts.benefit4.title": "Funciona con internet mala",
  "landing.hosts.benefit4.body":
    "El registro queda guardado en el dispositivo y se envía solo cuando vuelve la señal.",
  "landing.hosts.quote":
    "Antes adivinaba la temporada alta por el movimiento de la calle. Hoy miro la previsión de treinta días y sé cuándo llamar más gente a la cocina.",
  "landing.hosts.quoteCaption":
    "Testimonio de ejemplo — sustituya por un anfitrión real del pueblo.",
  "landing.hosts.image":
    "Hamaca colgada en la galería de un alojamiento del pueblo",

  // ---------------------------------------------------------- Landing: cadastro
  "landing.register.title": "Registre su alojamiento",
  "landing.register.body":
    "Toma cinco minutos. Recibe el acceso al área del alojamiento y el código QR del mostrador el mismo día.",
  "landing.register.action": "Ir al área del alojamiento",
  "landing.register.footnote": "Sin costo. Puede salir cuando quiera.",

  // ---------------------------------------------------------- Landing: comércio
  "landing.commerce.index": "04",
  "landing.commerce.kicker": "Para el comercio y las asociaciones",
  "landing.commerce.title":
    "Planifique la temporada con el número, no con el rumor.",
  "landing.commerce.lead":
    "Restaurantes, mercados, paseos en barco y transporte usan la misma previsión pública que los alojamientos. Todos ven la misma curva.",
  "landing.commerce.item1.title": "Compras e inventario",
  "landing.commerce.item1.body":
    "Saber que vienen mil personas-día en el feriado cambia el pedido al proveedor de la semana anterior.",
  "landing.commerce.item2.title": "Equipo temporal",
  "landing.commerce.item2.body":
    "El rango probable indica cuándo contratar refuerzo y cuándo cuidar la caja.",
  "landing.commerce.item3.title": "Servicios del pueblo",
  "landing.commerce.item3.body":
    "Agua, recolección de basura y transporte se dimensionan por presencia estimada, no por conjetura.",
  "landing.commerce.item4.title": "Argumento ante el poder público",
  "landing.commerce.item4.body":
    "Una serie histórica sustenta una solicitud de inversión mejor que cualquier impresión personal.",

  // -------------------------------------------------------------- Landing: mapa
  "landing.place.index": "05",
  "landing.place.kicker": "Dónde estamos",
  "landing.place.title":
    "Cumuruxatiba, distrito de Prado, litoral sur de Bahía.",
  "landing.place.lead":
    "Entre el Monte Pascoal y la desembocadura del río Jucuruçu, una franja de playa protegida por acantilados que aún recibe al visitante por el boca a boca.",
  "landing.place.municipality": "Municipio",
  "landing.place.municipalityValue": "Prado, Bahía",
  "landing.place.timezone": "Huso horario",
  "landing.place.beach": "Franja de playa",
  "landing.place.beachValue": "Cerca de 17 km",
  "landing.place.season": "Temporada alta",
  "landing.place.seasonValue": "Diciembre a febrero",
  "landing.place.image":
    "Praia do Centro en marea baja, con los acantilados de la Costa das Baleias al fondo",

  // ------------------------------------------------------- Landing: privacidade
  "landing.privacy.index": "06",
  "landing.privacy.kicker": "Privacidad por construcción",
  "landing.privacy.titleLead": "El comercio recibe la curva.",
  "landing.privacy.titleAccent": "Nadie recibe al huésped.",
  "landing.privacy.lead":
    "La separación entre el dato de la estancia y el indicador público es estructural, no una promesa de uso.",
  "landing.privacy.item1.title": "El panel nunca lee respuestas brutas",
  "landing.privacy.item1.body":
    "El sitio público consulta solo celdas ya agregadas y redondeadas. No existe camino del gráfico a una persona.",
  "landing.privacy.item2.title": "Supresión automática de celda pequeña",
  "landing.privacy.item2.body":
    "Por debajo del umbral de contribuciones o de alojamientos, el valor no se publica y ningún sustituto aparece en su lugar.",
  "landing.privacy.item3.title": "La encuesta es siempre opcional",
  "landing.privacy.item3.body":
    "Rechazar la encuesta de perfil no impide el check-in ni cambia la atención. El consentimiento se registra y es revocable.",
  "landing.privacy.item4.title": "Derechos del titular según la LGPD",
  "landing.privacy.item4.body":
    "Acceso, corrección y eliminación del propio registro, con retención configurada y traza de auditoría de cada operación.",
  "landing.privacy.prototypeTitle": "Entorno de prototipo",
  "landing.privacy.prototypeBody":
    "Esta demostración usa solo datos ficticios. No sustituye estadística oficial ni censo, no autoriza operación real y ningún registro municipal se vuelve obligatorio sin fundamento jurídico formal de la Alcaldía de Prado.",

  // -------------------------------------------------------- Landing: sobre e FAQ
  "landing.about.index": "07",
  "landing.about.kicker": "Acerca de y gobernanza",
  "landing.about.title":
    "Un observatorio del pueblo, con la regla escrita antes del dato.",
  "landing.about.body1":
    "Cumuruxatiba es distrito del Municipio de Prado. Cualquier obligatoriedad de registro depende del patrocinio formal de la Alcaldía o de una entidad con competencia delegada — hasta entonces, la participación es voluntaria.",
  "landing.about.body2":
    "Las preguntas publicadas son inmutables: un cambio genera nueva versión. Toda escritura es idempotente y ninguna integración externa fallida pierde el registro local.",
  "landing.about.guideCityHall": "Guía del Observatorio para la Alcaldía",
  "landing.about.guideCityHallMeta": "PDF · presentación y capacitación",
  "landing.about.guideFnrh": "Guía para generar la clave FNRH",
  "landing.about.guideFnrhMeta": "PDF · para alojamientos elegibles",

  "landing.faq.title": "Preguntas frecuentes",
  "landing.faq.q1.question":
    "¿Necesito identificación de empresa para participar?",
  "landing.faq.q1.answer":
    "No. Puede registrarse con identificación personal, como persona física que alquila una casa, o de empresa si tiene posada. El registro en el Observatorio no comprueba regularidad ni licencia.",
  "landing.faq.q2.question": "¿Mis competidores verán cuántos huéspedes tengo?",
  "landing.faq.q2.answer":
    "No. Ningún indicador se abre por establecimiento. El panel publica solo el total del pueblo, redondeado y con supresión cuando pocos alojamientos contribuyen.",
  "landing.faq.q3.question": "¿Esto sustituye la FNRH?",
  "landing.faq.q3.answer":
    "No. La integración con la FNRH Digital es una vía separada y opcional, con la clave oficial de cada establecimiento. El Observatorio no emite ni comparte esa credencial.",
  "landing.faq.q4.question":
    "¿El huésped está obligado a responder la encuesta?",
  "landing.faq.q4.answer":
    "Nunca. La encuesta de perfil es voluntaria y separada del registro de la estancia. Rechazarla no afecta el check-in.",
  "landing.faq.q5.question": "¿Y si se cae internet en medio del registro?",
  "landing.faq.q5.answer":
    "El registro queda guardado en el dispositivo y se envía cuando vuelve la conexión. Reenviar el mismo registro no genera estancia duplicada.",
  "landing.faq.q6.question": "¿Cuánto cuesta?",
  "landing.faq.q6.answer":
    "Nada para el alojamiento. El Observatorio es una iniciativa de interés público del pueblo y puede terminar su participación cuando quiera.",

  // ----------------------------------------------------------- Landing: contato
  "landing.contact.title": "¿Quiere entenderlo mejor antes de entrar?",
  "landing.contact.lead":
    "El equipo pasa por los alojamientos para explicar en persona y ayudar con el primer registro. Escriba al equipo o pida una visita.",
  "landing.contact.write": "Escribir al equipo",
  "landing.contact.visit": "Pedir una visita",
  "landing.contact.email": "Correo",
  "landing.contact.inPerson": "Atención presencial",
  "landing.contact.inPersonValue":
    "Martes y jueves, de 9 a 12, en la sede de la asociación de vecinos",
  "landing.contact.dpo": "Encargado de datos",
  "landing.contact.mark": "OBSERVATORIO TURÍSTICO · CUMURUXATIBA",

  "landing.license.ccBy20": "CC BY 2.0",
  "landing.license.ccBySa30": "CC BY-SA 3.0",
  "landing.license.ccBySa40": "CC BY-SA 4.0",
  "landing.license.publicDomain": "Dominio público",
  "landing.photoCredit": "Foto: {author} · {license} · Wikimedia Commons",

  // -------------------------------------------------------- Painel: envelope
  "analytics.index": "01",
  "analytics.kicker": "Publicación protegida",
  "analytics.title": "Indicadores públicos",
  "analytics.lead":
    "Tendencias agregadas para planificar temporada, equipo y stock. Sin microdatos, sin identificación de huéspedes y sin desglose por establecimiento.",
  "analytics.prototypeBadge": "Datos ficticios de prototipo",
  "analytics.loading": "Actualizando indicadores públicos…",
  "analytics.error": "No fue posible cargar los indicadores públicos.",
  "analytics.retry": "Intentar de nuevo",

  "analytics.metadata.aria": "Contexto de los indicadores",
  "analytics.metadata.updated": "Actualización",
  "analytics.metadata.coverage": "Cobertura",
  "analytics.metadata.unit": "Unidad",
  "analytics.metadata.mode": "Modo de datos",
  "analytics.coverage.published": "Cobertura estimada: {ratio}%",
  "analytics.coverage.protected":
    "Cobertura protegida por la política de publicación",
  "analytics.coverage.unavailable": "Cobertura no disponible",
  "analytics.unit.personDay": "Personas-día",
  "analytics.unit.surveyAnswer": "Respuestas de encuesta",
  "analytics.unit.inline": "personas-día",

  // ------------------------------------------------------- Painel: cartões
  "analytics.summary.aria": "Resumen de la presencia",
  "analytics.summary.today": "Presencia de hoy",
  "analytics.summary.todayHint":
    "Personas presentes hoy, contadas entre la llegada y la salida en el huso America/Bahia. Una persona cuenta una vez por día.",
  "analytics.summary.peak": "Pico previsto en los próximos 30 días",
  "analytics.summary.peakHint":
    "Mayor estimación central de la línea base explicable. Planifique por el rango probable, nunca por el número central aislado.",
  "analytics.kind.observed": "● Observado",
  "analytics.kind.forecast": "◇ Previsto",
  "analytics.state.protected": "Dato protegido",
  "analytics.state.unavailable": "Dato no disponible",
  "analytics.value.observed": "{value} personas-día",
  "analytics.value.central": "Estimación central: {value} personas-día",
  "analytics.value.band": "Rango probable: {lower} a {upper}",

  // -------------------------------------------------------- Painel: série
  "analytics.presence.kicker": "Serie en personas-día",
  "analytics.presence.title": "Presencia a lo largo del tiempo",
  "analytics.window.label": "Ventana de presencia",
  "analytics.window.recent": "Últimos 30 días",
  "analytics.window.next": "Próximos 30 días",
  "analytics.window.combined": "Últimos 30 y próximos 30 días",
  "analytics.window.scope.recent": "últimos 30 días",
  "analytics.window.scope.next": "próximos 30 días",
  "analytics.window.scope.combined": "últimos 30 días observados",

  "analytics.legend.aria": "Leyenda de la serie",
  "analytics.legend.observed": "Observado",
  "analytics.legend.forecast": "Previsto, con rango probable",
  "analytics.legend.gap": "Protegido o no disponible, sin valor sustituto",
  "analytics.legend.average": "Media de referencia",
  "analytics.legend.trend": "Media móvil de {days} días",
  "analytics.legend.weekend": "Fin de semana",

  "analytics.stats.aria": "Estadísticas de los {scope}",
  "analytics.tile.average": "Promedio diario",
  "analytics.tile.averageHint":
    "Promedio de los {count} días con valor publicado. La línea discontinua del gráfico marca ese nivel.",
  "analytics.tile.peak": "Día más lleno",
  "analytics.tile.peakHint":
    "Mayor valor publicado de la ventana y el día en que ocurrió.",
  "analytics.tile.trough": "Día más vacío",
  "analytics.tile.troughHint":
    "Menor valor publicado de la ventana. Compare con el día más lleno para dimensionar la variación.",
  "analytics.tile.trend": "Tendencia",
  "analytics.tile.trendHint":
    "Promedio de los {count} últimos días publicados frente a los {count} anteriores. Los días protegidos se saltan, nunca se cuentan como cero.",
  "analytics.tile.trendNone":
    "Se necesitan al menos dos días publicados para comparar períodos.",
  "analytics.tile.total": "Total acumulado",
  "analytics.tile.totalHint":
    "Suma de los días publicados. Los días protegidos quedan fuera, así que el total es un piso, no un censo.",
  "analytics.tile.published": "Días publicados",
  "analytics.tile.publishedValue": "{published} de {days}",
  "analytics.withheld.none":
    "Todos los días de la ventana tienen valor publicado.",
  "analytics.withheld.one":
    "1 día quedó sin valor por protección estadística o ausencia de dato. Ningún valor sustituto se muestra.",
  "analytics.withheld.other":
    "{count} días quedaron sin valor por protección estadística o ausencia de dato. Ningún valor sustituto se muestra.",

  "analytics.empty": "—",

  // ------------------------------------------------------ Painel: dia da semana
  "analytics.weekday.title": "Ritmo de la semana",
  "analytics.weekday.hint":
    "Promedio por día de la semana sobre los días publicados de la ventana observada. Muestra el patrón semanal que la serie diaria esconde; con pocos días publicados, una sola fecha puede responder por todo el día de la semana.",
  "analytics.weekday.aria": "Promedio por día de la semana",
  "analytics.weekday.none": "sin día publicado",
  "analytics.weekday.days.one": " · 1 día",
  "analytics.weekday.days.other": " · {count} días",

  // ---------------------------------------------------------- Painel: tabela
  "analytics.table.aria": "Presencia observada y prevista",
  "analytics.table.date": "Fecha",
  "analytics.table.kind": "Tipo",
  "analytics.table.result": "Resultado",
  "analytics.table.delta": "Frente al promedio",
  "analytics.details": "Ver la serie día a día",

  // ------------------------------------------------------ Painel: preferências
  "analytics.preferences.kicker": "Encuesta voluntaria",
  "analytics.preferences.title": "Perfil de visita agregado",
  "analytics.preferences.periodLabel": "Período de las preferencias",
  "analytics.preferences.lastCompleteMonth": "Último mes completo",
  "analytics.preferences.lead":
    "Unidad: respuestas de encuesta estructuradas y consentidas. Una respuesta de grupo no se multiplica por la cantidad de visitantes.",
  "analytics.preferences.firstVisit": "Primera visita",
  "analytics.preferences.returning": "Visitante recurrente",
  "analytics.preferences.share": "{percent}% de las respuestas elegibles",

  // ------------------------------------------------------- Painel: metodologia
  "analytics.methodology.kicker": "Cómo interpretar",
  "analytics.methodology.title": "Metodología y limitaciones",
  "analytics.methodology.observed": "Presencia observada",
  "analytics.methodology.observedBody":
    "Cada persona contribuye en el intervalo civil entre llegada y salida, en America/Bahia. La salida no cuenta como nuevo día de presencia.",
  "analytics.methodology.forecast": "Presencia prevista",
  "analytics.methodology.forecastBody":
    "La línea base explicable combina reservas conocidas e histórico estacional. El rango normal usa límites de {low}% a {high}%. Sin histórico elegible suficiente, la línea base usa un fallback más amplio, de {fallbackLow}% a {fallbackHigh}%. El contrato público no identifica qué rango se aplicó a cada punto; por eso la interfaz no atribuye ese estado a valores individuales.",
  "analytics.methodology.protection": "Protección estadística",
  "analytics.methodology.protectionBody":
    "Las celdas por debajo de {threshold} contribuciones o de {accommodations} acomodaciones están protegidas. Hay supresión complementaria y redondeo en base {rounding}.",
  "analytics.methodology.limits": "Limitaciones",
  "analytics.methodology.limitsBody":
    "La cobertura es parcial y no representa un censo. Los datos son ficticios de prototipo; la exactitud operacional, la política municipal y el uso con datos reales siguen sin verificarse.",

  // ---------------------------------------------------------- Painel: gráfico
  "analytics.chart.empty":
    "Ningún día de la ventana tiene valor publicable; toda la serie está protegida o no disponible.",
  "analytics.chart.summary":
    "Serie de {days} días en personas-día. {published} días con valor publicado y {withheld} protegidos o no disponibles, dibujados como hueco en la base del gráfico.",
  "analytics.chart.average": "media {value}",
  "analytics.chart.forecastFrom": "previsión a partir de aquí",
  "analytics.chart.readoutIdle":
    "Apunte a un día del gráfico o use las flechas ← → del teclado para leer cada día. Inicio y Fin van al primero y al último día.",
  "analytics.chart.readoutDay": "{day}. {lines}.",
  "analytics.chart.caption":
    "Escala hasta {bound} personas-día. El fin de semana aparece con fondo sombreado, la línea discontinua marca la media de la ventana, la línea continua es la media móvil de {days} días y los días protegidos aparecen como hueco, sin valor sustituto. La media móvil se interrumpe donde faltan valores.",

  "analytics.slot.observed": "Observado: {value} personas-día",
  "analytics.slot.forecast": "Previsto: {value} personas-día",
  "analytics.slot.band": "Rango probable: {lower} a {upper} personas-día",
  "analytics.slot.protected":
    "Protegido por la política de publicación, sin valor sustituto",
  "analytics.slot.unavailable": "Sin dato disponible para este día",
  "analytics.delta.same": "al mismo nivel que la media de referencia",
  "analytics.delta.above": "{percent}% por encima de la media de referencia",
  "analytics.delta.below": "{percent}% por debajo de la media de referencia",
  "analytics.trend.value":
    "{sign}{percent}% frente a los {size} días anteriores",

  // ------------------------------------------------------- Autorregistro por cartel
  "selfService.posterRequired.title": "Cartel necesario",
  "selfService.posterRequired.body":
    "Lea el código QR expuesto por el alojamiento. El token vive en el fragmento de la dirección, nunca se envía al servidor y se borra de la barra de direcciones antes de que aparezca esta página.",
  "selfService.eyebrow": "Cartel del alojamiento",
  "selfService.pageTitle": "Autorregistro por cartel",
  "selfService.validating": "Validando el cartel…",
  "selfService.pending.title": "Cartel en validación",
  "selfService.retry": "Intentar de nuevo",
  "selfService.verifyingDevice": "Verificando este dispositivo…",
  "selfService.formTitle": "Confirme los datos de la estadía",
  "selfService.submit": "Enviar autorregistro",
  "selfService.submitting": "Enviando…",

  "selfService.privacy.title": "Aviso de privacidad",
  "selfService.privacy.introBefore": "Está iniciando un registro para",
  "selfService.privacy.introAfter":
    ". Este formulario es abierto y no pide identificación: recopilamos solo rango de edad, papel en el grupo, residencia y fechas previstas.",
  "selfService.privacy.noIdentity":
    "No indique nombre, documento, correo electrónico, teléfono, dirección ni ninguna observación personal. Estos datos no se aceptan aquí.",
  "selfService.privacy.needsApproval":
    "El alojamiento debe aprobar el registro. Sin aprobación, nada entra en estadística ni en el panel público.",
  "selfService.privacy.expiry":
    "Si el alojamiento rechaza, o si nadie decide en 72 horas, los datos enviados se eliminan y solo queda el registro auditable de la decisión.",
  "selfService.privacy.assistedAlternative":
    "Si prefiere no usar este formulario, pida al alojamiento que registre la estadía con usted, por el canal asistido.",
  "selfService.privacy.versionLabel": "Versión del aviso:",

  "selfService.window.legend": "Período previsto",
  "selfService.window.arrival": "Fecha de llegada",
  "selfService.window.departure": "Fecha de salida",

  "selfService.completion.title": "Autorregistro enviado",
  "selfService.completion.body":
    "El alojamiento debe aprobar este registro. Si nadie decide en 72 horas, la solicitud expira y los datos enviados se eliminan. Nada entra en estadística ni en el panel público antes de la aprobación.",
  "selfService.completion.continueSurvey": "Responder la encuesta voluntaria",

  "selfService.error.forbidden":
    "Este cartel no está aceptando registros ahora.",
  "selfService.error.notFound":
    "Este cartel ya no es válido. Pida uno nuevo al alojamiento.",
  "selfService.error.conflict":
    "El aviso de privacidad cambió desde que se imprimió el cartel. Pida un cartel actualizado al alojamiento.",
  "selfService.error.unprocessable":
    "Algunos datos no se aceptan en este formulario abierto. Revíselos e intente de nuevo.",
  "selfService.error.rateLimited":
    "Ya hubo demasiados envíos desde esta red hace poco.",
  "selfService.error.proofOfWorkAborted":
    "La verificación fue interrumpida. Envíe de nuevo cuando quiera.",
  "selfService.error.offline":
    "Sin conexión ahora. Lo que completó quedó guardado, cifrado, en este dispositivo.",

  // -------------------------------------------------------- Copia de error de huésped
  "guestCopy.unexpectedFailure":
    "No pudimos comunicarnos con el servicio ahora. Intente de nuevo en unos instantes.",
  "guestCopy.idempotencyInProgress":
    "Ya recibimos este envío y lo estamos concluyendo.",
  "guestCopy.retrySeconds.one": "Intente de nuevo en 1 segundo.",
  "guestCopy.retrySeconds.other": "Intente de nuevo en {seconds} segundos.",

  // ------------------------------------------------------------- Editor de visitantes
  "visitor.role.responsible": "Responsable",
  "visitor.role.companion": "Acompañante",
  "visitor.role.minor": "Menor",
  "visitor.ageBand.0_5": "0 a 5",
  "visitor.ageBand.6_11": "6 a 11",
  "visitor.ageBand.12_17": "12 a 17",
  "visitor.ageBand.18_24": "18 a 24",
  "visitor.ageBand.25_34": "25 a 34",
  "visitor.ageBand.35_44": "35 a 44",
  "visitor.ageBand.45_59": "45 a 59",
  "visitor.ageBand.60_plus": "60 o más",
  "visitor.legend": "Visitante {number}",
  "visitor.roleLabel": "Papel del visitante {number}",
  "visitor.ageBandLabel": "Rango de edad del visitante {number}",
  "visitor.residenceState": "Estado de residencia del visitante {number}",
  "visitor.residenceCity": "Municipio IBGE del visitante {number}",
  "visitor.residenceCountry": "País de residencia del visitante {number}",
  "visitor.remove": "Eliminar visitante {number}",
  "visitor.add": "Añadir visitante",
};
