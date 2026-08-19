package httpapi

import (
	"errors"
	"net/http"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
)

// O painel da hospedagem é dado de um único inquilino: nunca entra em cache
// compartilhado, e por isso não carrega ETag como os documentos públicos.
const ownPerformanceCache = "no-store"

func (d Dependencies) accommodationPerformance(
	writer http.ResponseWriter,
	request *http.Request,
) {
	id, ok := pathUUID(writer, request, "accommodation_id")
	if !ok {
		return
	}
	slice, ok := observedPresenceSlice(request)
	if !ok {
		writeInvalidAnalyticsRequest(writer, request)
		return
	}
	value, err := d.OwnPerformance.Performance(
		request.Context(),
		analytics.OwnPerformanceQuery{
			Actor: requestPrincipal(request), AccommodationID: id, Slice: slice,
		},
	)
	if err != nil {
		writeOwnPerformanceError(writer, request, err)
		return
	}
	writer.Header().Set("Cache-Control", ownPerformanceCache)
	writeJSON(writer, http.StatusOK, value)
}

// A previsão não entra no comparativo: ela é publicada como intervalo e a
// hospedagem não tem série própria equivalente para pôr ao lado.
func observedPresenceSlice(request *http.Request) (analytics.PresenceSlice, bool) {
	slice, ok := presenceSlice(request)
	if !ok || slice.Selector != analytics.PresenceObservedSelector {
		return analytics.PresenceSlice{}, false
	}
	return slice, true
}

func writeOwnPerformanceError(
	writer http.ResponseWriter,
	request *http.Request,
	err error,
) {
	switch {
	case errors.Is(err, analytics.ErrInvalidOwnPerformance):
		writeInvalidAnalyticsRequest(writer, request)
	case errors.Is(err, analytics.ErrOwnPerformanceNotFound):
		writeProblem(
			writer, request, http.StatusNotFound,
			"not-found", "Recurso não encontrado",
		)
	default:
		writeAnalyticsUnavailable(writer, request)
	}
}
