import type { Messages } from "./messages-pt";

/** Inglês. O tipo `Messages` recusa a compilação se faltar qualquer chave. */
export const messagesEn: Messages = {
  // ---------------------------------------------------------------- Navegação
  "app.nav.public": "Public dashboard",
  "app.nav.registration": "Registration",
  "app.nav.selfRegistration": "Self-registration",
  "app.nav.activation": "Activation",
  "app.nav.survey": "Survey",
  "app.nav.workspace": "Lodging area",
  "app.nav.questionnaires": "Questionnaires",
  "app.nav.quality": "Quality",
  "app.nav.aria": "Main navigation",
  "app.skipLink": "Skip to content",
  "app.brand.name": "Tourism Observatory",
  "app.brand.place": "Cumuruxatiba · Prado, Bahia",
  "app.signOut": "Sign out",
  "app.documentTitle": "{page} · Tourism Observatory of Cumuruxatiba",
  "app.routeAnnounce": "Current page: {title}",
  "app.routeLoading": "Loading page…",
  "app.footer.note":
    "Technical prototype with fictional data. Real use depends on the governance gates of the Municipality of Prado.",
  "app.locale.aria": "Site language",
  "app.locale.pt": "PT",
  "app.locale.en": "EN",
  "app.locale.es": "ES",
  "app.locale.ptName": "Portuguese",
  "app.locale.enName": "English",
  "app.locale.esName": "Spanish",

  "app.title.public": "Public tourism dashboard",
  "app.title.registration": "Visitor registration",
  "app.title.selfRegistration": "Self-registration by poster",
  "app.title.activation": "Account activation",
  "app.title.survey": "Visitor survey",
  "app.title.workspace": "Lodging area",
  "app.title.questionnaires": "Questionnaires",
  "app.title.quality": "Data quality",
  "app.title.notFound": "Page not found",

  // ------------------------------------------------------- Landing: seções
  "landing.sections.aria": "Sections on this page",
  "landing.nav.numbers": "Numbers",
  "landing.nav.how": "How it works",
  "landing.nav.hosts": "Hosts",
  "landing.nav.commerce": "Business",
  "landing.nav.privacy": "Privacy",
  "landing.nav.about": "About",
  "landing.nav.register": "Register your lodging",

  // ------------------------------------------------------------ Landing: hero
  "landing.hero.kicker": "Tourism in numbers",
  "landing.hero.titleLead": "Our beach's tourism,",
  "landing.hero.titleAccent": "finally in numbers.",
  "landing.hero.lead":
    "Inns and rental houses record stays in one place. The Observatory gives the community public presence and forecast indicators — without exposing a single guest.",
  "landing.hero.primary": "Register my lodging",
  "landing.hero.secondary": "See today's numbers",
  "landing.hero.todayLabel": "Today in the village",
  "landing.hero.todayUnit": "person-days",
  "landing.hero.todayHint": "People sleeping here, week by week.",
  "landing.hero.todayPending": "Loading",
  "landing.hero.image":
    "Photo of the cliff and beach of Cumuruxatiba (vertical)",

  // --------------------------------------------------------- Landing: ticker
  "landing.ticker.aria": "Scrolling summary of today's indicators",
  "landing.ticker.today": "Presence today {value}",
  "landing.ticker.peak": "Forecast peak {value}",
  "landing.ticker.coverage": "{coverage}",
  "landing.ticker.prototype": "Fictional prototype data",
  "landing.ticker.pending": "Loading public indicators",

  // ---------------------------------------------------- Landing: como funciona
  "landing.how.index": "02",
  "landing.how.kicker": "From check-in to indicator",
  "landing.how.title": "Four steps, two minutes per stay.",
  "landing.how.lead":
    "Works on a phone, tolerates weak internet and asks nothing you don't already ask at the front desk.",
  "landing.how.step1.title": "Register the lodging",
  "landing.how.step1.body":
    "Name of the place, type of lodging and how many people fit. We ask for no tax ID and no tourism licence. A licensed inn or a family house — both are welcome.",
  "landing.how.step2.title": "Open the stay",
  "landing.how.step2.body":
    "Arrival, departure and number of people. That alone already feeds the village presence indicator.",
  "landing.how.step3.title": "The guest fills it in and you approve",
  "landing.how.step3.body":
    "The QR code at the desk opens a form with no name, document, email or phone: only age band, where the person comes from and the dates. The stay arrives pending and only counts for the village once you approve it.",
  "landing.how.step4.title": "The village gets the number",
  "landing.how.step4.body":
    "Everything enters the public dashboard aggregated, rounded and protected. Nobody sees your occupancy on its own.",
  "landing.how.image.desk": "Inn front desk with the QR code on the counter",
  "landing.how.image.phone": "Guest filling in the form on a phone",
  "landing.how.image.street": "Village street on a busy day",

  // -------------------------------------------------------- Landing: anfitriões
  "landing.hosts.index": "03",
  "landing.hosts.kicker": "For those who host",
  "landing.hosts.title": "Your house is tourism too — and now it counts.",
  "landing.hosts.lead":
    "This is not enforcement. It is the first time Cumuruxatiba can say, with a number, how many people sleep here each week of the year.",
  "landing.hosts.benefit1.title": "Know when the house fills up",
  "landing.hosts.benefit1.body":
    "A 30-day forecast with a likely range to set rates, staff and purchases.",
  "landing.hosts.benefit2.title": "No tax ID, no licence, no document",
  "landing.hosts.benefit2.body":
    "Someone renting out a house takes part just like a registered inn. Registration asks for no document at all.",
  "landing.hosts.benefit3.title": "Your occupancy is never exposed",
  "landing.hosts.benefit3.body":
    "No public indicator is broken down by property. Only the village total exists.",
  "landing.hosts.benefit4.title": "Works on bad internet",
  "landing.hosts.benefit4.body":
    "The record stays on the device and uploads itself when the signal returns.",
  "landing.hosts.image": "Portrait of a host or an inn in the village",

  // ---------------------------------------------------------- Landing: cadastro
  "landing.register.title": "Register your lodging",
  "landing.register.body":
    "The team sets the registration up with you, by email or on a visit, and sends an activation link for you to choose a password. The front-desk QR code lives inside the lodging area.",
  "landing.register.action": "Talk to the team",
  "landing.register.signIn": "Already have access? Go to the lodging area",
  "landing.register.footnote": "No cost. You can leave whenever you like.",

  // ---------------------------------------------------------- Landing: comércio
  "landing.commerce.index": "04",
  "landing.commerce.kicker": "For business and associations",
  "landing.commerce.title": "Plan the season with the number, not the rumour.",
  "landing.commerce.lead":
    "Restaurants, markets, boat trips and transport use the same public forecast as the lodgings. Everyone sees the same curve.",
  "landing.commerce.item1.title": "Purchasing and stock",
  "landing.commerce.item1.body":
    "Knowing a thousand person-days are coming on the holiday changes the supplier order the week before.",
  "landing.commerce.item2.title": "Temporary staff",
  "landing.commerce.item2.body":
    "The likely range shows when to hire extra hands and when to hold the cash.",
  "landing.commerce.item3.title": "Village services",
  "landing.commerce.item3.body":
    "Water, waste collection and transport get sized by estimated presence instead of guesswork.",
  "landing.commerce.item4.title": "A case for public authorities",
  "landing.commerce.item4.body":
    "A historical series supports an investment request better than any personal impression.",

  // -------------------------------------------------------------- Landing: mapa
  "landing.place.index": "05",
  "landing.place.kicker": "Where we are",
  "landing.place.title":
    "Cumuruxatiba, a district of Prado, southern coast of Bahia.",
  "landing.place.lead":
    "Between Monte Pascoal and the mouth of the Jucuruçu river, a stretch of beach sheltered by cliffs that still welcomes visitors by word of mouth.",
  "landing.place.municipality": "Municipality",
  "landing.place.municipalityValue": "Prado, Bahia",
  "landing.place.timezone": "Time zone",
  "landing.place.beach": "Beach stretch",
  "landing.place.beachValue": "About 17 km",
  "landing.place.season": "High season",
  "landing.place.seasonValue": "December to February",
  "landing.place.image": "Map of Cumuruxatiba village with the lodgings",

  // ------------------------------------------------------- Landing: privacidade
  "landing.privacy.index": "06",
  "landing.privacy.kicker": "Privacy by construction",
  "landing.privacy.titleLead": "Business gets the curve.",
  "landing.privacy.titleAccent": "Nobody gets the guest.",
  "landing.privacy.lead":
    "The separation between stay data and the public indicator is structural, not a promise of good behaviour.",
  "landing.privacy.item1.title": "The dashboard never reads raw answers",
  "landing.privacy.item1.body":
    "The public site queries only pre-aggregated, rounded cells. There is no path from the chart to a person.",
  "landing.privacy.item2.title": "Automatic suppression of small cells",
  "landing.privacy.item2.body":
    "Below the contribution or lodging threshold, the value is not published, and no substitute appears in its place.",
  "landing.privacy.item3.title": "The survey is always optional",
  "landing.privacy.item3.body":
    "Declining the profile survey does not block check-in or change service. Consent is recorded and can be withdrawn through the data protection officer.",
  "landing.privacy.item4.title": "Data subject rights under the LGPD",
  "landing.privacy.item4.body":
    "Access, correction and deletion of one's own record are handled by the data protection officer, at the address at the foot of this page. Every operation on a record stays in the audit trail.",
  "landing.privacy.prototypeTitle": "Prototype environment",
  "landing.privacy.prototypeBody":
    "This demo uses fictional data only. It does not replace official statistics or a census, it does not authorise real operation, and no municipal registration becomes mandatory without formal legal grounding from the Prado City Hall.",

  // -------------------------------------------------------- Landing: sobre e FAQ
  "landing.about.index": "07",
  "landing.about.kicker": "About and governance",
  "landing.about.title":
    "An observatory of the village, with the rules written before the data.",
  "landing.about.body1":
    "Cumuruxatiba is a district of the Municipality of Prado. Any mandatory registration depends on formal sponsorship by the City Hall or a delegated body — until then, participation is voluntary.",
  "landing.about.body2":
    "Published questions are immutable: a change creates a new version. Every write is idempotent, and no failing external integration loses the local record.",
  "landing.about.guideCityHall": "Observatory guide for the City Hall",
  "landing.about.guideCityHallMeta": "PDF · presentation and training",
  "landing.about.guideFnrh": "Guide to generating the FNRH key",
  "landing.about.guideFnrhMeta": "PDF · for eligible lodgings",

  "landing.faq.title": "Frequently asked questions",
  "landing.faq.q1.question": "Do I need a company tax ID to take part?",
  "landing.faq.q1.answer":
    "No. We ask for no tax ID and no tourism licence either: registration takes the name of the place, its type and its capacity. Registering with the Observatory does not prove compliance or licensing.",
  "landing.faq.q2.question": "Will my competitors see how many guests I have?",
  "landing.faq.q2.answer":
    "No. No indicator is broken down by property. The dashboard publishes only the village total, rounded and suppressed when few lodgings contribute.",
  "landing.faq.q3.question": "Does this replace the FNRH?",
  "landing.faq.q3.answer":
    "No. Integration with FNRH Digital is a separate, optional track using each property's own official key. The Observatory neither issues it nor shares it.",
  "landing.faq.q4.question": "Is the guest required to answer the survey?",
  "landing.faq.q4.answer":
    "Never. The profile survey is voluntary and separate from the stay record. Declining does not affect check-in.",
  "landing.faq.q5.question": "What if the internet drops mid-registration?",
  "landing.faq.q5.answer":
    "The record is kept on the device and sent when the connection returns. Resending the same record does not create a duplicate stay.",
  "landing.faq.q6.question": "What does it cost?",
  "landing.faq.q6.answer":
    "Nothing for the lodging. The Observatory is a public-interest initiative of the village and you can end your participation whenever you want.",

  // ----------------------------------------------------------- Landing: contato
  "landing.contact.title": "Want to understand it better before joining?",
  "landing.contact.lead":
    "The team visits lodgings to explain in person and help with the first registration. Email the team or ask for a visit.",
  "landing.contact.write": "Email the team",
  "landing.contact.visit": "Ask for a visit",
  "landing.contact.email": "Email",
  "landing.contact.dpo": "Data protection officer",
  "landing.contact.mark": "TOURISM OBSERVATORY · CUMURUXATIBA",

  "landing.imagePending": "Reserved space for a photo",
  "landing.photoCredit": "Photo: {author} · {license} · Wikimedia Commons",

  // -------------------------------------------------------- Painel: envelope
  "analytics.index": "01",
  "analytics.kicker": "Protected publication",
  "analytics.title": "Public indicators",
  "analytics.lead":
    "Aggregated trends for planning season, staff and stock. No microdata, no guest identification and no per-property breakdown.",
  "analytics.prototypeBadge": "Fictional prototype data",
  "analytics.loading": "Updating public indicators…",
  "analytics.error": "The public indicators could not be loaded.",
  "analytics.retry": "Try again",

  "analytics.metadata.aria": "Indicator context",
  "analytics.metadata.updated": "Updated",
  "analytics.metadata.coverage": "Coverage",
  "analytics.metadata.unit": "Unit",
  "analytics.metadata.mode": "Data mode",
  "analytics.coverage.published": "Estimated coverage: {ratio}%",
  "analytics.coverage.protected": "Coverage protected by the publication policy",
  "analytics.coverage.unavailable": "Coverage unavailable",
  "analytics.unit.personDay": "Person-days",
  "analytics.unit.surveyAnswer": "Survey answers",
  "analytics.unit.inline": "person-days",

  // ------------------------------------------------------- Painel: cartões
  "analytics.summary.aria": "Presence summary",
  "analytics.summary.today": "Today's presence",
  "analytics.summary.todayHint":
    "People present today, counted between arrival and departure in the America/Bahia time zone. One person counts once per day.",
  "analytics.summary.peak": "Forecast peak in the next 30 days",
  "analytics.summary.peakHint":
    "Highest central estimate of the explainable baseline. Plan by the likely range, never by the central number alone.",
  "analytics.kind.observed": "● Observed",
  "analytics.kind.forecast": "◇ Forecast",
  "analytics.state.protected": "Protected data",
  "analytics.state.unavailable": "Data unavailable",
  "analytics.value.observed": "{value} person-days",
  "analytics.value.central": "Central estimate: {value} person-days",
  "analytics.value.band": "Likely range: {lower} to {upper}",

  // -------------------------------------------------------- Painel: série
  "analytics.presence.kicker": "Series in person-days",
  "analytics.presence.title": "Presence over time",
  "analytics.window.label": "Presence window",
  "analytics.window.recent": "Last 30 days",
  "analytics.window.next": "Next 30 days",
  "analytics.window.combined": "Last 30 and next 30 days",
  "analytics.window.scope.recent": "last 30 days",
  "analytics.window.scope.next": "next 30 days",
  "analytics.window.scope.combined": "last 30 observed days",

  "analytics.legend.aria": "Series legend",
  "analytics.legend.observed": "Observed",
  "analytics.legend.forecast": "Forecast, with likely range",
  "analytics.legend.gap": "Protected or unavailable, no substitute value",
  "analytics.legend.average": "Reference average",
  "analytics.legend.trend": "{days}-day moving average",
  "analytics.legend.weekend": "Weekend",

  "analytics.stats.aria": "Statistics for the {scope}",
  "analytics.tile.average": "Daily average",
  "analytics.tile.averageHint":
    "Average of the {count} days with a published value. The dashed line on the chart marks that level.",
  "analytics.tile.peak": "Busiest day",
  "analytics.tile.peakHint":
    "Highest published value in the window and the day it happened.",
  "analytics.tile.trough": "Emptiest day",
  "analytics.tile.troughHint":
    "Lowest published value in the window. Compare with the busiest day to size the swing.",
  "analytics.tile.trend": "Trend",
  "analytics.tile.trendHint":
    "Average of the last {count} published days against the {count} before them. Protected days are skipped, never counted as zero.",
  "analytics.tile.trendNone":
    "At least two published days are needed to compare periods.",
  "analytics.tile.total": "Running total",
  "analytics.tile.totalHint":
    "Sum of published days. Protected days are left out, so the total is a floor, not a census.",
  "analytics.tile.published": "Published days",
  "analytics.tile.publishedValue": "{published} of {days}",
  "analytics.withheld.none": "Every day in the window has a published value.",
  "analytics.withheld.one":
    "1 day has no value due to statistical protection or missing data. No substitute value is shown.",
  "analytics.withheld.other":
    "{count} days have no value due to statistical protection or missing data. No substitute value is shown.",

  "analytics.empty": "—",

  // ------------------------------------------------------ Painel: dia da semana
  "analytics.weekday.title": "Rhythm of the week",
  "analytics.weekday.hint":
    "Average by weekday over the published days of the observed window. It shows the weekly pattern the daily series hides; with few published days, a single date can account for a whole weekday.",
  "analytics.weekday.aria": "Average by weekday",
  "analytics.weekday.none": "no published day",
  "analytics.weekday.days.one": " · 1 day",
  "analytics.weekday.days.other": " · {count} days",

  // ---------------------------------------------------------- Painel: tabela
  "analytics.table.aria": "Observed and forecast presence",
  "analytics.table.date": "Date",
  "analytics.table.kind": "Type",
  "analytics.table.result": "Result",
  "analytics.table.delta": "Vs. average",
  "analytics.details": "See the day-by-day series",

  // ------------------------------------------------------ Painel: preferências
  "analytics.preferences.kicker": "Voluntary survey",
  "analytics.preferences.title": "Aggregated visit profile",
  "analytics.preferences.periodLabel": "Preferences period",
  "analytics.preferences.lastCompleteMonth": "Last complete month",
  "analytics.preferences.lead":
    "Unit: structured, consented survey answers. A group answer is not multiplied by the number of visitors.",
  "analytics.preferences.firstVisit": "First visit",
  "analytics.preferences.returning": "Returning visitor",
  "analytics.preferences.share": "{percent}% of eligible answers",

  // ------------------------------------------------------- Painel: metodologia
  "analytics.methodology.kicker": "How to read it",
  "analytics.methodology.title": "Methodology and limits",
  "analytics.methodology.observed": "Observed presence",
  "analytics.methodology.observedBody":
    "Each person counts over the civil interval between arrival and departure, in America/Bahia. Departure day is not a new day of presence.",
  "analytics.methodology.forecast": "Forecast presence",
  "analytics.methodology.forecastBody":
    "The explainable baseline combines known bookings and seasonal history. The normal range uses bounds of {low}% to {high}%. Without enough eligible history, the baseline uses a wider fallback, from {fallbackLow}% to {fallbackHigh}%. The public contract does not say which range applied to each point, so the interface does not attribute that state to individual values.",
  "analytics.methodology.protection": "Statistical protection",
  "analytics.methodology.protectionBody":
    "Cells below {threshold} contributions or {accommodations} accommodations are protected. Complementary suppression applies, and values are rounded to base {rounding}.",
  "analytics.methodology.limits": "Limits",
  "analytics.methodology.limitsBody":
    "Coverage is partial and is not a census. The data is fictional prototype data; operational accuracy, municipal policy and real-data use remain unverified.",

  // ---------------------------------------------------------- Painel: gráfico
  "analytics.chart.empty":
    "No day in the window has a publishable value; the whole series is protected or unavailable.",
  "analytics.chart.summary":
    "Series of {days} days in person-days. {published} days with a published value and {withheld} protected or unavailable, drawn as a gap at the base of the chart.",
  "analytics.chart.average": "average {value}",
  "analytics.chart.forecastFrom": "forecast from here",
  "analytics.chart.readoutIdle":
    "Point at a day on the chart or use the ← → arrow keys to read each day. Home and End jump to the first and last day.",
  "analytics.chart.readoutDay": "{day}. {lines}.",
  "analytics.chart.caption":
    "Scale up to {bound} person-days. Weekends have a shaded background, the dashed line marks the window average, the solid line is the {days}-day moving average, and protected days appear as a gap with no substitute value. The moving average breaks where values are missing.",

  "analytics.slot.observed": "Observed: {value} person-days",
  "analytics.slot.forecast": "Forecast: {value} person-days",
  "analytics.slot.band": "Likely range: {lower} to {upper} person-days",
  "analytics.slot.protected":
    "Protected by the publication policy, no substitute value",
  "analytics.slot.unavailable": "No data available for this day",
  "analytics.delta.same": "at the same level as the reference average",
  "analytics.delta.above": "{percent}% above the reference average",
  "analytics.delta.below": "{percent}% below the reference average",
  "analytics.trend.value": "{sign}{percent}% against the previous {size} days",

  // ------------------------------------------------------ Poster self-registration
  "selfService.posterRequired.title": "Poster required",
  "selfService.posterRequired.body":
    "Scan the QR code displayed by the accommodation. The token lives in the address fragment, is never sent to the server, and is erased from the address bar before this page appears.",
  "selfService.eyebrow": "Accommodation poster",
  "selfService.pageTitle": "Self-registration by poster",
  "selfService.validating": "Validating the poster…",
  "selfService.pending.title": "Poster being validated",
  "selfService.retry": "Try again",
  "selfService.verifyingDevice": "Verifying this device…",
  "selfService.formTitle": "Confirm the stay details",
  "selfService.submit": "Submit self-registration",
  "selfService.submitting": "Submitting…",

  "selfService.privacy.title": "Privacy notice",
  "selfService.privacy.introBefore": "You are starting a registration for",
  "selfService.privacy.introAfter":
    ". This form is open and does not ask for identification: we only collect age range, role in the group, residence, and planned dates.",
  "selfService.privacy.noIdentity":
    "Do not enter name, document, e-mail, phone, address, or any personal remark. This data is not accepted here.",
  "selfService.privacy.needsApproval":
    "The accommodation must approve the registration. Without approval, nothing enters statistics or the public dashboard.",
  "selfService.privacy.expiry":
    "If the accommodation declines, or if no one decides within 72 hours, the data sent is deleted and only the auditable record of the decision remains.",
  "selfService.privacy.assistedAlternative":
    "If you would rather not use this form, ask the accommodation to register the stay with you, through the assisted channel.",
  "selfService.privacy.versionLabel": "Notice version:",

  "selfService.window.legend": "Planned window",
  "selfService.window.arrival": "Arrival date",
  "selfService.window.departure": "Departure date",

  "selfService.completion.title": "Self-registration submitted",
  "selfService.completion.body":
    "The accommodation must approve this registration. If no one decides within 72 hours, the request expires and the data sent is deleted. Nothing enters statistics or the public dashboard before approval.",
  "selfService.completion.continueSurvey": "Answer the voluntary survey",

  "selfService.error.forbidden": "This poster is not accepting registrations right now.",
  "selfService.error.notFound":
    "This poster is no longer valid. Ask the accommodation for a new one.",
  "selfService.error.conflict":
    "The privacy notice changed since the poster was printed. Ask the accommodation for an updated poster.",
  "selfService.error.unprocessable":
    "Some data is not accepted in this open form. Review it and try again.",
  "selfService.error.rateLimited": "Too many submissions from this network just now.",
  "selfService.error.proofOfWorkAborted":
    "The verification was interrupted. Submit again whenever you like.",
  "selfService.error.offline":
    "No connection right now. What you filled in was kept, encrypted, on this device.",

  // ------------------------------------------------------------- Guest error copy
  "guestCopy.unexpectedFailure":
    "We could not reach the service right now. Try again in a few moments.",
  "guestCopy.idempotencyInProgress": "We already received this submission and are finishing it up.",
  "guestCopy.retrySeconds.one": "Try again in 1 second.",
  "guestCopy.retrySeconds.other": "Try again in {seconds} seconds.",

  // ------------------------------------------------------------------ Visitor editor
  "visitor.role.responsible": "Responsible",
  "visitor.role.companion": "Companion",
  "visitor.role.minor": "Minor",
  "visitor.ageBand.0_5": "0 to 5",
  "visitor.ageBand.6_11": "6 to 11",
  "visitor.ageBand.12_17": "12 to 17",
  "visitor.ageBand.18_24": "18 to 24",
  "visitor.ageBand.25_34": "25 to 34",
  "visitor.ageBand.35_44": "35 to 44",
  "visitor.ageBand.45_59": "45 to 59",
  "visitor.ageBand.60_plus": "60 or more",
  "visitor.legend": "Visitor {number}",
  "visitor.roleLabel": "Role of visitor {number}",
  "visitor.ageBandLabel": "Age range of visitor {number}",
  "visitor.residenceState": "State of residence of visitor {number}",
  "visitor.residenceCity": "IBGE municipality of visitor {number}",
  "visitor.residenceCountry": "Country of residence of visitor {number}",
  "visitor.remove": "Remove visitor {number}",
  "visitor.add": "Add visitor",
};
