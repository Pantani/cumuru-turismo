package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/Pantani/cumuru/apps/api/internal/directory"
)

// Cinco minutos, o mesmo dos documentos públicos de analytics. A lista muda
// quando uma hospedagem publica ou despublica, o que é raro; o hóspede que
// recarrega a página não deve custar uma consulta cada vez.
const publicDirectoryCache = "public, max-age=300, stale-if-error=86400"

// A lista é aberta e sem token, como o pedido de convite: quem procura onde se
// hospedar ainda não tem conta e nunca vai ter. Passa pelo mesmo CORS das
// outras rotas abertas.
func (d Dependencies) registerDirectoryRoutes(mux *http.ServeMux, metrics *httpMetrics) {
	d.handleInviteRoute(
		mux, metrics, "GET /api/v1/public/accommodations", d.publicAccommodations,
	)
	d.handleInviteRoute(
		mux, metrics, "OPTIONS /api/v1/public/accommodations", emptyHandler,
	)
}

func (d Dependencies) publicAccommodations(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if !validNoQuery(request) || !validIfNoneMatch(request.Header.Get("If-None-Match")) {
		writePublicBadRequest(writer, request)
		return
	}
	listing, err := d.PublicDirectory.List(request.Context())
	if err != nil {
		writePublicUnavailable(writer, request)
		return
	}
	writeDirectoryListing(writer, request, listing)
}

func writeDirectoryListing(
	writer http.ResponseWriter,
	request *http.Request,
	listing directory.Listing,
) {
	payload, err := json.Marshal(listing)
	if err != nil {
		writePublicUnavailable(writer, request)
		return
	}
	tag := publicDocumentETag("accommodations", "", payload)
	writer.Header().Set("Cache-Control", publicDirectoryCache)
	writer.Header().Set("ETag", tag)
	if request.Header.Get("If-None-Match") == tag {
		writer.WriteHeader(http.StatusNotModified)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(payload)
}
