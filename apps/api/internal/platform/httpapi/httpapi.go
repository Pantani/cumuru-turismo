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
	"github.com/Pantani/cumuru/apps/api/internal/activation"
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
	Auth                           Authenticator
	LockoutDuration                time.Duration
	Accommodations                 *accommodation.Service
	AccommodationOnboardingEnabled bool
	Stays                          *stay.Service
	SelfServiceEnabled             bool
	Activation                     *activation.Service
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

// New refuses to build a handler it cannot serve correctly. A missing cursor
// keyring used to be swallowed, which left every paginated listing quietly
// unable to issue a next-page cursor instead of failing where an operator would
// notice.
func New(dependencies Dependencies) (http.Handler, http.Handler, error) {
	if dependencies.Logger == nil {
		dependencies.Logger = slog.New(slog.DiscardHandler)
	}
	if dependencies.Registry == nil {
		dependencies.Registry = prometheus.NewRegistry()
	}
	cursor, err := dependencies.resolveCursorCodec()
	if err != nil {
		return nil, nil, err
	}
	dependencies.cursor = cursor
	metrics := newHTTPMetrics(dependencies.Registry)
	mux := http.NewServeMux()
	dependencies.registerPlatformRoutes(mux, metrics)
	dependencies.registerFeatureRoutes(mux, metrics)
	return dependencies.withRequestID(dependencies.recoverPanic(mux)),
		promhttp.HandlerFor(dependencies.Registry, promhttp.HandlerOpts{}),
		nil
}

// The codec is required exactly when a surface that paginates is registered, so
// a handler exposing only health and readiness still builds without a keyring.
func (d Dependencies) resolveCursorCodec() (cursorCodec, error) {
	if !d.paginates() {
		return cursorCodec{}, nil
	}
	return newCursorCodec(d.CursorKeys)
}

func (d Dependencies) paginates() bool {
	return d.Accommodations != nil || d.Stays != nil || d.Questionnaires != nil
}

func (d Dependencies) registerPlatformRoutes(mux *http.ServeMux, metrics *httpMetrics) {
	mux.Handle(
		"GET /api/v1/platform/health",
		d.routeHandler("/api/v1/platform/health", metrics, http.HandlerFunc(d.health)),
	)
	mux.Handle(
		"GET /api/v1/platform/readiness",
		d.routeHandler("/api/v1/platform/readiness", metrics, http.HandlerFunc(d.readiness)),
	)
	mux.Handle(
		"GET /api/v1/platform/build",
		d.routeHandler(
			"/api/v1/platform/build",
			metrics,
			d.requireScope("platform:read", http.HandlerFunc(d.buildInfo)),
		),
	)
}

// Each surface is registered only when its dependency is present, so an absent
// feature returns 404 rather than a misleading error.
func (d Dependencies) registerFeatureRoutes(mux *http.ServeMux, metrics *httpMetrics) {
	if d.Auth != nil {
		d.registerAuthRoutes(mux, metrics)
	}
	if d.Accommodations != nil {
		d.registerAccommodationRoutes(mux, metrics)
	}
	if d.Stays != nil {
		d.registerStayRoutes(mux, metrics)
	}
	if d.Questionnaires != nil {
		d.registerQuestionnaireRoutes(mux, metrics)
	}
	d.registerPhase7Routes(mux, metrics)
	d.registerAnalyticsRoutes(mux, metrics)
}

// The open channel is registered only when the phase is on, so a disabled phase
// answers 404 instead of exposing a half-configured route.
func (d Dependencies) registerPhase7Routes(mux *http.ServeMux, metrics *httpMetrics) {
	if d.Stays != nil && d.SelfServiceEnabled {
		d.registerSelfServiceRoutes(mux, metrics)
	}
	if d.Activation != nil {
		d.registerActivationRoutes(mux, metrics)
	}
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
	return d.requireAnyScope([]string{scope}, next)
}

// Authentication and authorization share one path so the two cannot drift: a
// change to how a credential is rejected applies to every guarded route at once.
func (d Dependencies) requireAnyScope(scopes []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		principal, ok := d.authenticate(writer, request)
		if !ok {
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

// authenticate answers the caller itself on failure, so a missing, malformed or
// rejected credential is always reported as the same 401 and never as a hint
// about which of the three it was.
func (d Dependencies) authenticate(
	writer http.ResponseWriter,
	request *http.Request,
) (access.Principal, bool) {
	token, ok := bearerToken(request.Header.Get("Authorization"))
	if !ok || d.Verifier == nil {
		writeProblem(writer, request, http.StatusUnauthorized, "unauthorized", "Credencial inválida ou ausente")
		return access.Principal{}, false
	}
	principal, err := d.verifyCredential(request, token)
	if err != nil {
		writeProblem(writer, request, http.StatusUnauthorized, "unauthorized", "Credencial inválida ou ausente")
		return access.Principal{}, false
	}
	return principal, true
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
	if d.writeRetryableError(writer, request, err) {
		return
	}
	status, code, title := serviceProblem(err)
	writeProblem(writer, request, status, code, title)
}

type problemMapping struct {
	status int
	code   string
	title  string
	errs   []error
}

// One entry per contract problem type. The order is the resolution order, so a
// more specific mapping always precedes a broader one.
var serviceProblems = []problemMapping{
	{http.StatusForbidden, "forbidden", "Operação não permitida",
		[]error{accommodation.ErrForbidden, stay.ErrForbidden, activation.ErrForbidden}},
	// The refusal of a minor gets its own code so the form can point at the
	// assisted channel. It depends only on the submitted body and never on
	// pre-existing data about a subject, so it is not an oracle (T-02).
	{http.StatusUnprocessableEntity, "minor-not-accepted",
		"Menor de idade só pode ser cadastrado pelo canal assistido",
		[]error{stay.ErrMinorNotAccepted}},
	{http.StatusUnprocessableEntity, "validation-failed", "Dados inválidos",
		[]error{
			accommodation.ErrInvalidInput, stay.ErrInvalidInput,
			questionnaire.ErrInvalidInput, activation.ErrInvalidInput,
		}},
	{http.StatusNotFound, "not-found", "Recurso não encontrado",
		[]error{
			accommodation.ErrNotFound, stay.ErrNotFound, questionnaire.ErrNotFound,
			questionnaire.ErrCapabilityInvalid, activation.ErrNotFound,
		}},
	{http.StatusPreconditionFailed, "precondition-failed", "Versão desatualizada",
		[]error{
			accommodation.ErrPreconditionFailed, stay.ErrPreconditionFailed,
			questionnaire.ErrPreconditionFailed, activation.ErrPreconditionFailed,
		}},
	{http.StatusConflict, "invite-consumed", "Convite já consumido",
		[]error{stay.ErrInviteConsumed}},
	{http.StatusConflict, "conflict", "Conflito de estado",
		[]error{
			accommodation.ErrConflict, stay.ErrConflict, questionnaire.ErrConflict,
			activation.ErrConflict,
		}},
}

func serviceProblem(err error) (int, string, string) {
	for _, mapping := range serviceProblems {
		if matchesAny(err, mapping.errs) {
			return mapping.status, mapping.code, mapping.title
		}
	}
	return http.StatusServiceUnavailable, "dependency-unavailable",
		"Serviço temporariamente indisponível"
}

func matchesAny(err error, candidates []error) bool {
	for _, candidate := range candidates {
		if errors.Is(err, candidate) {
			return true
		}
	}
	return false
}

// The two retryable outcomes carry a Retry-After header, so they are answered
// here instead of through the table.
func (d Dependencies) writeRetryableError(
	writer http.ResponseWriter,
	request *http.Request,
	err error,
) bool {
	var processing *idempotency.ProcessingError
	switch {
	case errors.Is(err, stay.ErrRateLimited), errors.Is(err, questionnaire.ErrRateLimited):
		writer.Header().Set("Retry-After", "60")
		writeProblem(writer, request, http.StatusTooManyRequests, "rate-limited", "Muitas tentativas")
	case errors.As(err, &processing):
		seconds := int64((processing.RetryAfter + time.Second - 1) / time.Second)
		writer.Header().Set("Retry-After", strconv.FormatInt(seconds, 10))
		writeProblem(writer, request, http.StatusConflict, "idempotency-in-progress", "Requisição em processamento")
	default:
		return false
	}
	return true
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
