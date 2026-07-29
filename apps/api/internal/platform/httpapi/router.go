package httpapi

import (
	"net/http"
	"strings"
)

func (d Dependencies) registerAnalyticsRoutes(mux *http.ServeMux, metrics *httpMetrics) {
	if d.PublicAnalytics != nil {
		d.handlePublicAnalyticsRoute(
			mux, metrics, "GET /api/v1/public/summary", d.publicSummary,
		)
		d.handlePublicAnalyticsRoute(
			mux, metrics, "GET /api/v1/public/presence", d.publicPresence,
		)
		d.handlePublicAnalyticsRoute(
			mux, metrics, "GET /api/v1/public/preferences", d.publicPreferences,
		)
		d.handlePublicAnalyticsRoute(
			mux, metrics, "GET /api/v1/public/methodology", d.publicMethodology,
		)
	}
	if d.AnalyticsQuality != nil {
		d.handleRoute(
			mux, metrics, "GET /api/v1/analytics/quality",
			"analytics:read:internal", d.analyticsQuality,
		)
	}
}

func (d Dependencies) handlePublicAnalyticsRoute(
	mux *http.ServeMux,
	metrics *httpMetrics,
	pattern string,
	handler func(http.ResponseWriter, *http.Request),
) {
	_, route, _ := strings.Cut(pattern, " ")
	mux.Handle(
		pattern,
		d.routeHandler(route, metrics, http.HandlerFunc(handler)),
	)
}
