package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
)

const publicAnalyticsCache = "public, max-age=300, stale-if-error=86400"

var strongAnalyticsETag = regexp.MustCompile(`^"sha256-[0-9a-f]{64}"$`)

func (d Dependencies) publicSummary(writer http.ResponseWriter, request *http.Request) {
	if !validNoQuery(request) || !validIfNoneMatch(request.Header.Get("If-None-Match")) {
		writeInvalidAnalyticsRequest(writer, request)
		return
	}
	value, err := d.PublicAnalytics.Summary(request.Context())
	d.writePublicAnalytics(writer, request, "summary", "", value, err)
}

func (d Dependencies) publicPresence(writer http.ResponseWriter, request *http.Request) {
	window, ok := analyticsSelector(
		request, "window", "recent_30_days", "next_30_days",
	)
	if !ok || !validIfNoneMatch(request.Header.Get("If-None-Match")) {
		writeInvalidAnalyticsRequest(writer, request)
		return
	}
	value, err := d.PublicAnalytics.Presence(request.Context(), window)
	d.writePublicAnalytics(writer, request, "presence", window, value, err)
}

func (d Dependencies) publicPreferences(writer http.ResponseWriter, request *http.Request) {
	period, ok := analyticsSelector(request, "period", "last_complete_month")
	if !ok || !validIfNoneMatch(request.Header.Get("If-None-Match")) {
		writeInvalidAnalyticsRequest(writer, request)
		return
	}
	value, err := d.PublicAnalytics.Preferences(request.Context(), period)
	d.writePublicAnalytics(writer, request, "preferences", period, value, err)
}

func (d Dependencies) publicMethodology(writer http.ResponseWriter, request *http.Request) {
	if !validNoQuery(request) || !validIfNoneMatch(request.Header.Get("If-None-Match")) {
		writeInvalidAnalyticsRequest(writer, request)
		return
	}
	value, err := d.PublicAnalytics.Methodology(request.Context())
	d.writePublicAnalytics(writer, request, "methodology", "", value, err)
}

func (d Dependencies) analyticsQuality(writer http.ResponseWriter, request *http.Request) {
	window, ok := analyticsSelector(request, "window", "last_30_days")
	if !ok {
		writeInvalidAnalyticsRequest(writer, request)
		return
	}
	value, err := d.AnalyticsQuality.Quality(request.Context(), window)
	if err != nil {
		writeAnalyticsUnavailable(writer, request)
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
		writeAnalyticsUnavailable(writer, request)
		return
	}
	payload, err := json.Marshal(value)
	if err != nil {
		writeAnalyticsUnavailable(writer, request)
		return
	}
	etag := analyticsETag(operation, selector, payload)
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

func analyticsETag(operation, selector string, payload []byte) string {
	source := make([]byte, 0, len(operation)+len(selector)+len(payload)+2)
	source = append(source, operation...)
	source = append(source, '\n')
	source = append(source, selector...)
	source = append(source, '\n')
	source = append(source, payload...)
	sum := sha256.Sum256(source)
	return `"sha256-` + hex.EncodeToString(sum[:]) + `"`
}

// The public endpoints accept exactly one selector and nothing else, so an
// unexpected parameter is rejected rather than ignored.
func soleQueryValue(request *http.Request, name string) (string, bool) {
	query := request.URL.Query()
	if len(query) != 1 {
		return "", false
	}
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

func validNoQuery(request *http.Request) bool {
	return len(request.URL.Query()) == 0
}

func validIfNoneMatch(value string) bool {
	return value == "" || strongAnalyticsETag.MatchString(value)
}

func writeInvalidAnalyticsRequest(writer http.ResponseWriter, request *http.Request) {
	writeProblem(writer, request, http.StatusBadRequest, "invalid-request", "Requisição inválida")
}

func writeAnalyticsUnavailable(writer http.ResponseWriter, request *http.Request) {
	writeProblem(
		writer, request, http.StatusServiceUnavailable,
		"dependency-unavailable", "Serviço temporariamente indisponível",
	)
}
