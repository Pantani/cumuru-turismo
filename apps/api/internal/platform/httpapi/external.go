package httpapi

import "net/http"

// The external context layer has an endpoint and a cache identity of its own,
// and merging it into /public/summary, /public/presence, /public/preferences or
// /public/methodology is forbidden by ADR-045 §2. The reason is concrete rather
// than tidy: a shared payload would let an external refresh invalidate the ETag
// of the protected snapshot, and the invalidation cadence would become an
// oracle for the publication time of a release.
//
// The handler reads the layer through the public pool, which sees only the view
// in public_data. No egress happens here: ADR-045 §6 keeps the outbound call in
// the worker so the upstream never observes the panel's cadence.
func (d Dependencies) registerExternalRoutes(
	mux *http.ServeMux,
	metrics *httpMetrics,
) {
	if d.PublicContext == nil {
		return
	}
	d.handlePublicAnalyticsRoute(
		mux, metrics, "GET /api/v1/public/context", d.publicContext,
	)
}

// The document has no selector, like /public/methodology, so any query at all is
// rejected rather than ignored.
//
// The ETag comes from publicDocumentETag with operation "context", the same helper
// and the same algorithm the four protected documents use. A second ETag
// algorithm in one runtime is immediate debt, and the shared writer also brings
// the 304 by If-None-Match and the public cache directives with it.
func (d Dependencies) publicContext(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if !validNoQuery(request) ||
		!validIfNoneMatch(request.Header.Get("If-None-Match")) {
		writePublicBadRequest(writer, request)
		return
	}
	value, err := d.PublicContext.Context(request.Context())
	d.writePublicAnalytics(writer, request, "context", "", value, err)
}
