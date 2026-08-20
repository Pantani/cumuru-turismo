package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/external"
)

type contextReaderStub struct {
	document external.PublicContext
	err      error
	calls    int
}

func (s *contextReaderStub) Context(
	context.Context,
) (external.PublicContext, error) {
	s.calls++
	return s.document, s.err
}

func TestPublicContextReusesTheAnalyticsETagAndCache(t *testing.T) {
	t.Parallel()

	handler, _ := mustNew(t, Dependencies{
		PublicContext: &contextReaderStub{document: publishedDocument()},
	})
	first := getContext(t, handler, "")

	if first.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", first.Code, first.Body.String())
	}
	etag := first.Header().Get("ETag")
	if !strongDocumentETag.MatchString(etag) {
		t.Fatalf("ETag = %q, want the shared strong form", etag)
	}
	if got := first.Header().Get("Cache-Control"); got != publicAnalyticsCache {
		t.Fatalf("Cache-Control = %q, want %q", got, publicAnalyticsCache)
	}

	second := getContext(t, handler, etag)
	if second.Code != http.StatusNotModified {
		t.Fatalf("If-None-Match status = %d, want 304", second.Code)
	}
	if second.Body.Len() != 0 {
		t.Fatalf("304 carried a body: %s", second.Body.String())
	}
}

// The ETag is a distinct identity even for a byte-identical payload, because
// the operation takes part in the digest. A shared identity would let an
// external refresh invalidate the snapshot of the protected series.
func TestPublicContextETagIsDistinctFromTheProtectedDocuments(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"a":1}`)
	if publicDocumentETag("context", "", payload) ==
		publicDocumentETag("methodology", "", payload) {
		t.Fatal("context and methodology share an ETag identity")
	}
}

// A dead source is a card inside a 200. 503 is reserved for the document that
// cannot be assembled at all.
func TestPublicContextServesUnavailableCardsWithTwoHundred(t *testing.T) {
	t.Parallel()

	handler, _ := mustNew(t, Dependencies{
		PublicContext: &contextReaderStub{document: unavailableDocument()},
	})
	response := getContext(t, handler, "")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if !strings.Contains(response.Body.String(), external.StatusUnavailable) {
		t.Fatalf("unavailable card missing: %s", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "protected") {
		t.Fatal("the layer used the word protected")
	}
}

func TestPublicContextAnswers503OnlyWhenTheDocumentIsUnavailable(t *testing.T) {
	t.Parallel()

	handler, _ := mustNew(t, Dependencies{
		PublicContext: &contextReaderStub{err: external.ErrContextUnavailable},
	})
	response := getContext(t, handler, "")

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

// The document has no selector, so any query is rejected rather than ignored.
func TestPublicContextRejectsEverySelector(t *testing.T) {
	t.Parallel()

	reader := &contextReaderStub{document: publishedDocument()}
	handler, _ := mustNew(t, Dependencies{PublicContext: reader})
	for _, query := range []string{
		"?window=recent_30_days", "?source_code=open_meteo_forecast",
		"?url=https://attacker.invalid", "?latitude=1&longitude=2", "?=",
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(
			http.MethodGet, "/api/v1/public/context"+query, nil,
		))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("query %q status = %d, want 400", query, response.Code)
		}
	}
	if reader.calls != 0 {
		t.Fatalf("a rejected request still reached the reader %d times",
			reader.calls)
	}
}

// The route is absent when the reader is, so a disabled layer is a 404 rather
// than a half-configured surface.
func TestPublicContextRouteAbsentWithoutReader(t *testing.T) {
	t.Parallel()

	handler, _ := mustNew(t, Dependencies{})
	response := getContext(t, handler, "")

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}

// Egress belongs to the worker. A request to the panel must not make an
// outbound call, and the request path is given nothing that could: the handler
// package builds no HTTP client of its own.
func TestRequestPathHoldsNoOutboundHTTPClient(t *testing.T) {
	t.Parallel()

	forbidden := []string{
		"http.Client{", "http.DefaultClient", "http.Get(", "http.Post(",
		"http.NewRequest", "net.Dial",
	}
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("package not readable: %v", err)
	}
	for _, name := range sources {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		assertNoOutboundCall(t, name, forbidden)
	}
}

func assertNoOutboundCall(t *testing.T, name string, forbidden []string) {
	t.Helper()
	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("%s not readable: %v", name, err)
	}
	for _, needle := range forbidden {
		if strings.Contains(string(body), needle) {
			t.Fatalf("%s contains %q on the request path", name, needle)
		}
	}
}

// The four protected documents keep their body byte for byte, with or without
// the external layer. The external card never joins them.
func TestProtectedDocumentsGainNoExternalField(t *testing.T) {
	t.Parallel()

	bare, _ := mustNew(t, Dependencies{PublicAnalytics: analyticsReaderStub{}})
	withLayer, _ := mustNew(t, Dependencies{
		PublicAnalytics: analyticsReaderStub{},
		PublicContext:   &contextReaderStub{document: publishedDocument()},
	})
	for _, route := range []string{
		"/api/v1/public/summary",
		"/api/v1/public/presence?window=recent_30_days",
		"/api/v1/public/preferences?period=last_complete_month",
		"/api/v1/public/methodology",
	} {
		before := getRoute(t, bare, route)
		after := getRoute(t, withLayer, route)
		if before.Body.String() != after.Body.String() {
			t.Fatalf("%s changed body with the external layer present", route)
		}
		if before.Header().Get("ETag") != after.Header().Get("ETag") {
			t.Fatalf("%s changed ETag with the external layer present", route)
		}
		assertNoExternalVocabulary(t, route, after.Body.String())
	}
}

func assertNoExternalVocabulary(t *testing.T, route, body string) {
	t.Helper()
	for _, needle := range []string{
		"external", "provenance", "attribution", "license", "card_code",
	} {
		if strings.Contains(strings.ToLower(body), needle) {
			t.Fatalf("%s carries %q", route, needle)
		}
	}
}

func getContext(
	t *testing.T,
	handler http.Handler,
	ifNoneMatch string,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(
		http.MethodGet, "/api/v1/public/context", nil,
	)
	if ifNoneMatch != "" {
		request.Header.Set("If-None-Match", ifNoneMatch)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func getRoute(
	t *testing.T,
	handler http.Handler,
	route string,
) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, route, nil))
	return response
}

func publishedDocument() external.PublicContext {
	return external.PublicContext{
		GeneratedAt:    "2026-08-18T12:00:00Z",
		Layer:          external.LayerCode,
		DisclaimerCode: external.DisclaimerCode,
		Cards: []external.Card{{
			CardCode:   external.CardWeatherDaily,
			Status:     external.StatusPublished,
			DataMode:   "real_source",
			UnitCode:   "celsius",
			Provenance: stubProvenance(),
			Series: []external.SeriesPoint{{
				PeriodStart: "2026-08-17T03:00:00Z",
				PeriodEnd:   "2026-08-18T03:00:00Z",
				Value:       26.4,
			}},
		}},
		Sources: []external.CreditedSource{{
			SourceCode:      external.SourceCadastur,
			Publisher:       "Ministério do Turismo",
			LicenseCode:     "LicenseRef-Cadastur-Termos-de-Uso",
			LicenseURL:      "https://cadastur.turismo.gov.br/",
			AttributionText: "Cadastur, Ministério do Turismo",
			TermsURL:        "https://cadastur.turismo.gov.br/",
		}},
	}
}

func unavailableDocument() external.PublicContext {
	document := publishedDocument()
	document.Cards = []external.Card{{
		CardCode:   external.CardTide,
		Status:     external.StatusUnavailable,
		DataMode:   "real_source",
		Provenance: stubProvenance(),
		ReasonCode: external.ReasonConstantsNotImported,
	}}
	return document
}

func stubProvenance() external.Provenance {
	return external.Provenance{
		SourceCode:      external.SourceOpenMeteoForecast,
		Publisher:       "Open-Meteo",
		LicenseCode:     "CC-BY-4.0",
		LicenseURL:      "https://creativecommons.org/licenses/by/4.0/",
		AttributionText: "Dados meteorológicos por Open-Meteo.com",
		TermsURL:        "https://open-meteo.com/en/terms",
		RetrievedAt:     "2026-08-18T11:00:00Z",
		CoveredPeriod: external.CoveredPeriod{
			Start:        "2026-08-17T03:00:00Z",
			End:          "2026-08-18T03:00:00Z",
			EndExclusive: true,
			TimeZone:     external.PublicTimeZone,
		},
	}
}
