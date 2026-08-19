package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
)

const publicAnalyticsCache = "public, max-age=300, stale-if-error=86400"

func (d Dependencies) publicSummary(writer http.ResponseWriter, request *http.Request) {
	if !validNoQuery(request) || !validIfNoneMatch(request.Header.Get("If-None-Match")) {
		writePublicBadRequest(writer, request)
		return
	}
	value, err := d.PublicAnalytics.Summary(request.Context())
	d.writePublicAnalytics(writer, request, "summary", "", value, err)
}

func (d Dependencies) publicPresence(writer http.ResponseWriter, request *http.Request) {
	slice, ok := presenceSlice(request)
	if !ok || !validIfNoneMatch(request.Header.Get("If-None-Match")) {
		writePublicBadRequest(writer, request)
		return
	}
	value, err := d.PublicAnalytics.Presence(request.Context(), slice)
	d.writePublicAnalytics(writer, request, "presence", slice.CacheKey(), value, err)
}

// A presença é o único documento com dois seletores, e o segundo só existe para
// uma das janelas. O par é resolvido junto para que `month` fora de
// `window=month` seja recusado em vez de ignorado.
func presenceSlice(request *http.Request) (analytics.PresenceSlice, bool) {
	window, month, ok := presenceSelectors(request.URL.Query())
	if !ok {
		return analytics.PresenceSlice{}, false
	}
	return analytics.ResolvePresenceWindow(window, month)
}

// Nada além de `window` e do `month` que a acompanha: um parâmetro inesperado é
// recusado em vez de ignorado. Enviado vazio, `month` continua sendo um seletor
// enviado — `?window=recent_30_days&month=` nomeia o mesmo par inconsistente que
// a forma preenchida, e tratá-lo como ausente o deixaria passar.
func presenceSelectors(query url.Values) (string, string, bool) {
	window, ok := singleQueryValue(query, "window")
	if !ok || len(query) > 2 {
		return "", "", false
	}
	if len(query) == 1 {
		return window, "", true
	}
	return presenceMonthSelector(query, window)
}

// O segundo seletor, quando existe, é `month` e só `month`.
func presenceMonthSelector(
	query url.Values,
	window string,
) (string, string, bool) {
	month, ok := singleQueryValue(query, "month")
	if !ok || month == "" {
		return "", "", false
	}
	return window, month, true
}

func (d Dependencies) publicPreferences(writer http.ResponseWriter, request *http.Request) {
	period, ok := analyticsSelector(request, "period", "last_complete_month")
	if !ok || !validIfNoneMatch(request.Header.Get("If-None-Match")) {
		writePublicBadRequest(writer, request)
		return
	}
	value, err := d.PublicAnalytics.Preferences(request.Context(), period)
	d.writePublicAnalytics(writer, request, "preferences", period, value, err)
}

func (d Dependencies) publicMethodology(writer http.ResponseWriter, request *http.Request) {
	if !validNoQuery(request) || !validIfNoneMatch(request.Header.Get("If-None-Match")) {
		writePublicBadRequest(writer, request)
		return
	}
	value, err := d.PublicAnalytics.Methodology(request.Context())
	d.writePublicAnalytics(writer, request, "methodology", "", value, err)
}

func (d Dependencies) analyticsQuality(writer http.ResponseWriter, request *http.Request) {
	window, ok := analyticsSelector(request, "window", "last_30_days")
	if !ok {
		writePublicBadRequest(writer, request)
		return
	}
	value, err := d.AnalyticsQuality.Quality(request.Context(), window)
	if err != nil {
		writePublicUnavailable(writer, request)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writeJSON(writer, http.StatusOK, value)
}

func (d Dependencies) writePublicAnalytics(
	writer http.ResponseWriter,
	request *http.Request,
	operation, selector string,
	value any,
	err error,
) {
	if err != nil {
		writePublicUnavailable(writer, request)
		return
	}
	payload, err := json.Marshal(value)
	if err != nil {
		writePublicUnavailable(writer, request)
		return
	}
	etag := publicDocumentETag(operation, selector, payload)
	writer.Header().Set("Cache-Control", publicAnalyticsCache)
	writer.Header().Set("ETag", etag)
	if request.Header.Get("If-None-Match") == etag {
		writer.WriteHeader(http.StatusNotModified)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(payload)
}

// The public endpoints accept exactly one selector and nothing else, so an
// unexpected parameter is rejected rather than ignored.
func soleQueryValue(request *http.Request, name string) (string, bool) {
	query := request.URL.Query()
	if len(query) != 1 {
		return "", false
	}
	return singleQueryValue(query, name)
}

// Um seletor repetido é ambíguo, não uma lista: só a ocorrência única vale.
func singleQueryValue(query url.Values, name string) (string, bool) {
	values, exists := query[name]
	if !exists || len(values) != 1 {
		return "", false
	}
	return values[0], true
}

func analyticsSelector(request *http.Request, name string, allowed ...string) (string, bool) {
	value, ok := soleQueryValue(request, name)
	if !ok {
		return "", false
	}
	if slices.Contains(allowed, value) {
		return value, true
	}
	return "", false
}
