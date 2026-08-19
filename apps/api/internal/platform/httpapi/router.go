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
	// O comparativo da hospedagem usa o escopo que já governa a leitura da
	// própria acomodação (GET /accommodations/{id}). Ele expõe menos que a
	// listagem de estadias que esse mesmo escopo libera: agregado próprio ao
	// lado do documento público, sem nenhuma linha de hóspede.
	if d.OwnPerformance != nil {
		d.handleRoute(
			mux, metrics,
			"GET /api/v1/accommodations/{accommodation_id}/performance",
			"stays:read:own", d.accommodationPerformance,
		)
	}
	if d.AnalyticsQuality != nil {
		d.handleRoute(
			mux, metrics, "GET /api/v1/analytics/quality",
			"analytics:read:internal", d.analyticsQuality,
		)
	}
	if d.AnalyticsFunnel != nil {
		d.handleRoute(
			mux, metrics, "GET /api/v1/analytics/funnel",
			"analytics:read:internal", d.analyticsFunnel,
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
