package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/analytics"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/idempotency"
	"github.com/Pantani/cumuru/apps/api/internal/questionnaire"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	problemBase              = "https://turismo.prado.ba.gov.br/problems/"
	maxBufferedResponseBytes = 1 << 20
)

var safeRequestID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)
var errResponseTooLarge = errors.New("response exceeds bounded buffer")

type ReadinessChecker interface {
	CheckReadiness(context.Context) error
}

type BuildInfo struct {
	Version  string
	Revision string
	BuiltAt  time.Time
}

type Dependencies struct {
	Readiness                      ReadinessChecker
	Verifier                       access.Verifier
	Accommodations                 *accommodation.Service
	AccommodationOnboardingEnabled bool
	Stays                          *stay.Service
	Questionnaires                 *questionnaire.Service
	PublicAnalytics                analytics.PublicReader
	AnalyticsQuality               analytics.QualityReader
	CORSAllowedOrigins             []string
	TrustedProxyCIDRs              []netip.Prefix
	CursorKeys                     config.KeyringConfig
	Logger                         *slog.Logger
	Registry                       *prometheus.Registry
	Tracer                         trace.Tracer
	Build                          BuildInfo
	cursor                         cursorCodec
}

type problem struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

type buildResponse struct {
	Version  string `json:"version"`
	Revision string `json:"revision"`
	BuiltAt  string `json:"built_at"`
}

type statusResponse struct {
	Status string `json:"status"`
}

type contextKey uint8

const (
	requestIDKey contextKey = iota
	principalKey
)

func New(dependencies Dependencies) (http.Handler, http.Handler) {
	if dependencies.Logger == nil {
		dependencies.Logger = slog.New(slog.DiscardHandler)
	}
	if dependencies.Registry == nil {
		dependencies.Registry = prometheus.NewRegistry()
	}
	dependencies.cursor, _ = newCursorCodec(dependencies.CursorKeys)
	metrics := newHTTPMetrics(dependencies.Registry)
	mux := http.NewServeMux()
	mux.Handle(
		"GET /api/v1/platform/health",
		dependencies.routeHandler(
			"/api/v1/platform/health", metrics, http.HandlerFunc(dependencies.health),
		),
	)
	mux.Handle(
		"GET /api/v1/platform/readiness",
		dependencies.routeHandler(
			"/api/v1/platform/readiness", metrics, http.HandlerFunc(dependencies.readiness),
		),
	)
	mux.Handle(
		"GET /api/v1/platform/build",
		dependencies.routeHandler(
			"/api/v1/platform/build",
			metrics,
			dependencies.requireScope("platform:read", http.HandlerFunc(dependencies.buildInfo)),
		),
	)
	if dependencies.Accommodations != nil {
		dependencies.registerAccommodationRoutes(mux, metrics)
	}
	if dependencies.Stays != nil {
		dependencies.registerStayRoutes(mux, metrics)
	}
	if dependencies.Questionnaires != nil {
		dependencies.registerQuestionnaireRoutes(mux, metrics)
	}
	dependencies.registerAnalyticsRoutes(mux, metrics)
	return dependencies.withRequestID(dependencies.recoverPanic(mux)),
		promhttp.HandlerFor(dependencies.Registry, promhttp.HandlerOpts{})
}

func (d Dependencies) health(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, statusResponse{Status: "ok"})
}

func (d Dependencies) readiness(writer http.ResponseWriter, request *http.Request) {
	if d.Readiness == nil || d.Readiness.CheckReadiness(request.Context()) != nil {
		writeProblem(writer, request, http.StatusServiceUnavailable, "dependency-unavailable", "Serviço temporariamente indisponível")
		return
	}
	writeJSON(writer, http.StatusOK, statusResponse{Status: "ready"})
}

func (d Dependencies) buildInfo(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, buildResponse{
		Version:  d.Build.Version,
		Revision: d.Build.Revision,
		BuiltAt:  d.Build.BuiltAt.UTC().Format(time.RFC3339),
	})
}

func (d Dependencies) requireScope(scope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		token, ok := bearerToken(request.Header.Get("Authorization"))
		if !ok || d.Verifier == nil {
			writeProblem(writer, request, http.StatusUnauthorized, "unauthorized", "Credencial inválida ou ausente")
			return
		}
		principal, err := d.verifyCredential(request, token)
		if err != nil {
			writeProblem(writer, request, http.StatusUnauthorized, "unauthorized", "Credencial inválida ou ausente")
			return
		}
		if !principal.HasScope(scope) {
			writeProblem(writer, request, http.StatusForbidden, "forbidden", "Escopo insuficiente")
			return
		}
		ctx := context.WithValue(request.Context(), principalKey, principal)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func (d Dependencies) requireAnyScope(scopes []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		token, ok := bearerToken(request.Header.Get("Authorization"))
		if !ok || d.Verifier == nil {
			writeProblem(writer, request, http.StatusUnauthorized, "unauthorized", "Credencial inválida ou ausente")
			return
		}
		principal, err := d.verifyCredential(request, token)
		if err != nil {
			writeProblem(writer, request, http.StatusUnauthorized, "unauthorized", "Credencial inválida ou ausente")
			return
		}
		if !hasAnyScope(principal, scopes) {
			writeProblem(writer, request, http.StatusForbidden, "forbidden", "Escopo insuficiente")
			return
		}
		ctx := context.WithValue(request.Context(), principalKey, principal)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func (d Dependencies) verifyCredential(
	request *http.Request,
	token string,
) (access.Principal, error) {
	fixtureVerifier, fixture := d.Verifier.(access.FixtureCredentialVerifier)
	if fixture && fixtureVerifier.IsFixtureCredential(token) {
		address, err := clientAddress(request, d.TrustedProxyCIDRs)
		if err != nil || !address.IsLoopback() {
			return access.Principal{}, access.ErrInvalidToken
		}
	}
	return d.Verifier.Verify(request.Context(), token)
}

func hasAnyScope(principal access.Principal, scopes []string) bool {
	for _, scope := range scopes {
		if principal.HasScope(scope) {
			return true
		}
	}
	return false
}

func (d Dependencies) withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID := request.Header.Get("X-Request-ID")
		if !safeRequestID.MatchString(requestID) {
			id, err := uuid.NewV7()
			if err != nil {
				id = uuid.New()
			}
			requestID = id.String()
		}
		writer.Header().Set("X-Request-ID", requestID)
		writer.Header().Set("Cache-Control", "no-store")
		ctx := context.WithValue(request.Context(), requestIDKey, requestID)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func (d Dependencies) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() {
			if recover() != nil {
				d.Logger.Error(
					"request failed",
					"error_code", "request_panic_last_resort",
					"request_id", requestID(request),
				)
				resetToSafeHeaders(writer.Header())
				writeProblem(writer, request, http.StatusInternalServerError, "internal-error", "Erro interno")
			}
		}()
		next.ServeHTTP(writer, request)
	})
}

func (d Dependencies) recoverBuffered(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		buffered := newBoundedResponseWriter(writer.Header())
		panicked := false
		func() {
			defer func() {
				panicked = recover() != nil
			}()
			next.ServeHTTP(buffered, request)
		}()
		switch {
		case panicked:
			d.Logger.Error(
				"request failed",
				"error_code", "request_panic",
				"request_id", requestID(request),
			)
			writeCleanInternalProblem(writer, request)
		case buffered.overflow:
			d.Logger.Error(
				"request failed",
				"error_code", "response_buffer_overflow",
				"request_id", requestID(request),
			)
			writeCleanInternalProblem(writer, request)
		default:
			buffered.commit(writer)
		}
	})
}

func writeCleanInternalProblem(writer http.ResponseWriter, request *http.Request) {
	resetToSafeHeaders(writer.Header())
	writeProblem(
		writer,
		request,
		http.StatusInternalServerError,
		"internal-error",
		"Erro interno",
	)
}

func resetToSafeHeaders(header http.Header) {
	requestID := header.Get("X-Request-ID")
	cacheControl := header.Get("Cache-Control")
	clear(header)
	if safeRequestID.MatchString(requestID) {
		header.Set("X-Request-ID", requestID)
	}
	if cacheControl == "no-store" {
		header.Set("Cache-Control", cacheControl)
	}
}

func (d Dependencies) observe(route string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		ctx := request.Context()
		var span trace.Span
		if d.Tracer != nil {
			ctx, span = d.Tracer.Start(ctx, route,
				trace.WithAttributes(
					attribute.String("http.request.method", request.Method),
					attribute.String("http.route", route),
				),
			)
			defer span.End()
		}
		recorder := &statusWriter{ResponseWriter: writer, status: http.StatusOK}
		next.ServeHTTP(recorder, request.WithContext(ctx))
		d.Logger.Info("http request",
			"request_id", requestID(request),
			"method", request.Method,
			"route", route,
			"status", recorder.status,
			"duration_ms", time.Since(started).Milliseconds(),
		)
	})
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeProblem(writer http.ResponseWriter, request *http.Request, status int, code, title string) {
	writer.Header().Set("Content-Type", "application/problem+json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(problem{
		Type:      problemBase + code,
		Title:     title,
		Status:    status,
		RequestID: requestID(request),
	})
}

func requestID(request *http.Request) string {
	value, _ := request.Context().Value(requestIDKey).(string)
	return value
}

func requestPrincipal(request *http.Request) access.Principal {
	value, _ := request.Context().Value(principalKey).(access.Principal)
	return value
}

func (d Dependencies) handleRoute(
	mux *http.ServeMux,
	metrics *httpMetrics,
	pattern string,
	scope string,
	handler func(http.ResponseWriter, *http.Request),
) {
	_, route, _ := strings.Cut(pattern, " ")
	protected := d.requireScope(scope, http.HandlerFunc(handler))
	mux.Handle(pattern, d.routeHandler(route, metrics, protected))
}

func (d Dependencies) handleAnyScopeRoute(
	mux *http.ServeMux,
	metrics *httpMetrics,
	pattern string,
	scopes []string,
	handler func(http.ResponseWriter, *http.Request),
) {
	_, route, _ := strings.Cut(pattern, " ")
	protected := d.requireAnyScope(scopes, http.HandlerFunc(handler))
	mux.Handle(pattern, d.routeHandler(route, metrics, protected))
}

func (d Dependencies) routeHandler(
	route string,
	metrics *httpMetrics,
	next http.Handler,
) http.Handler {
	return metrics.instrument(
		route,
		d.observe(route, d.recoverBuffered(next)),
	)
}

func mutationTarget(
	writer http.ResponseWriter,
	request *http.Request,
	pathName string,
) (uuid.UUID, int64, bool) {
	id, ok := pathUUID(writer, request, pathName)
	if !ok {
		return uuid.Nil, 0, false
	}
	version, err := parseIfMatch(request.Header.Get("If-Match"))
	if errors.Is(err, errIfMatchRequired) {
		writeProblem(writer, request, http.StatusPreconditionRequired, "precondition-required", "If-Match obrigatório")
		return uuid.Nil, 0, false
	}
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "invalid-request", "Requisição inválida")
		return uuid.Nil, 0, false
	}
	return id, version, true
}

func writeMutationSuccess(
	writer http.ResponseWriter,
	status int,
	version int64,
	replayed bool,
	value any,
) {
	writer.Header().Set("ETag", etag(version))
	writer.Header().Set("Idempotency-Replayed", strconv.FormatBool(replayed))
	writeJSON(writer, status, value)
}

func (d Dependencies) writeServiceError(
	writer http.ResponseWriter,
	request *http.Request,
	err error,
) {
	var processing *idempotency.ProcessingError
	switch {
	case errors.Is(err, accommodation.ErrForbidden):
		writeProblem(writer, request, http.StatusForbidden, "forbidden", "Operação não permitida")
	case errors.Is(err, accommodation.ErrInvalidInput),
		errors.Is(err, stay.ErrInvalidInput),
		errors.Is(err, questionnaire.ErrInvalidInput):
		writeProblem(writer, request, http.StatusUnprocessableEntity, "validation-failed", "Dados inválidos")
	case errors.Is(err, accommodation.ErrNotFound),
		errors.Is(err, stay.ErrNotFound),
		errors.Is(err, questionnaire.ErrNotFound),
		errors.Is(err, questionnaire.ErrCapabilityInvalid):
		writeProblem(writer, request, http.StatusNotFound, "not-found", "Recurso não encontrado")
	case errors.Is(err, accommodation.ErrPreconditionFailed),
		errors.Is(err, stay.ErrPreconditionFailed),
		errors.Is(err, questionnaire.ErrPreconditionFailed):
		writeProblem(writer, request, http.StatusPreconditionFailed, "precondition-failed", "Versão desatualizada")
	case errors.Is(err, stay.ErrRateLimited), errors.Is(err, questionnaire.ErrRateLimited):
		writer.Header().Set("Retry-After", "60")
		writeProblem(writer, request, http.StatusTooManyRequests, "rate-limited", "Muitas tentativas")
	case errors.As(err, &processing):
		seconds := int64((processing.RetryAfter + time.Second - 1) / time.Second)
		writer.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
		writeProblem(writer, request, http.StatusConflict, "idempotency-in-progress", "Requisição em processamento")
	case errors.Is(err, stay.ErrInviteConsumed):
		writeProblem(writer, request, http.StatusConflict, "invite-consumed", "Convite já consumido")
	case errors.Is(err, accommodation.ErrConflict),
		errors.Is(err, stay.ErrConflict),
		errors.Is(err, questionnaire.ErrConflict):
		writeProblem(writer, request, http.StatusConflict, "conflict", "Conflito de estado")
	default:
		writeProblem(writer, request, http.StatusServiceUnavailable, "dependency-unavailable", "Serviço temporariamente indisponível")
	}
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(content []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(content)
}

type boundedResponseWriter struct {
	header      http.Header
	body        bytes.Buffer
	status      int
	wroteHeader bool
	overflow    bool
}

func newBoundedResponseWriter(initial http.Header) *boundedResponseWriter {
	return &boundedResponseWriter{
		header: initial.Clone(),
		status: http.StatusOK,
	}
}

func (w *boundedResponseWriter) Header() http.Header {
	return w.header
}

func (w *boundedResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = status
}

func (w *boundedResponseWriter) Write(content []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.overflow || len(content) > maxBufferedResponseBytes-w.body.Len() {
		w.overflow = true
		w.body.Reset()
		return 0, errResponseTooLarge
	}
	return w.body.Write(content)
}

func (w *boundedResponseWriter) commit(writer http.ResponseWriter) {
	target := writer.Header()
	clear(target)
	for name, values := range w.header {
		target[name] = append([]string(nil), values...)
	}
	writer.WriteHeader(w.status)
	if w.status != http.StatusNoContent && w.status != http.StatusNotModified {
		_, _ = writer.Write(w.body.Bytes())
	}
}

type httpMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func newHTTPMetrics(registry *prometheus.Registry) *httpMetrics {
	metrics := &httpMetrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "cumuru",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total de requisições HTTP por rota normalizada.",
		}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "cumuru",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "Duração das requisições HTTP por rota normalizada.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "route", "status"}),
	}
	registry.MustRegister(metrics.requests, metrics.duration)
	return metrics
}

func (m *httpMetrics) instrument(route string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		recorder := &statusWriter{ResponseWriter: writer, status: http.StatusOK}
		next.ServeHTTP(recorder, request)
		status := strconv.Itoa(recorder.status)
		m.requests.WithLabelValues(request.Method, route, status).Inc()
		m.duration.WithLabelValues(request.Method, route, status).Observe(time.Since(started).Seconds())
	})
}
