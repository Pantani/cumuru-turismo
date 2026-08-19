package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/accessrequest"
	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/activation"
	"github.com/Pantani/cumuru/apps/api/internal/analytics"
	"github.com/Pantani/cumuru/apps/api/internal/directory"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/database"
	"github.com/Pantani/cumuru/apps/api/internal/platform/httpapi"
	"github.com/Pantani/cumuru/apps/api/internal/platform/logging"
	"github.com/Pantani/cumuru/apps/api/internal/platform/server"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/Pantani/cumuru/apps/api/internal/platform/telemetry"
	"github.com/Pantani/cumuru/apps/api/internal/questionnaire"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/prometheus/client_golang/prometheus"
)

type Build struct {
	Version  string
	Revision string
	BuiltAt  time.Time
}

func RunAPI(ctx context.Context, cfg config.Config, build Build, output io.Writer) error {
	if err := build.validate(); err != nil {
		return err
	}
	logger := logging.New(output, cfg.LogLevel)
	tracing, err := telemetry.New(ctx, cfg.Telemetry, cfg.ServiceName, build.Version)
	if err != nil {
		return err
	}
	defer shutdownTelemetry(tracing, cfg.ShutdownTimeout, logger)
	return runAPIWithTelemetry(ctx, cfg, build, logger, tracing)
}

func runAPIWithTelemetry(
	ctx context.Context,
	cfg config.Config,
	build Build,
	logger *slog.Logger,
	tracing *telemetry.Provider,
) error {
	platformStore, accommodationService, stayService, questionnaireService,
		publicAnalytics, analyticsQuality, closeDatabase, err := openServices(ctx, cfg)
	if err != nil {
		return err
	}
	defer closeDatabase()

	verifier, err := verifierFor(ctx, cfg, platformStore)
	if err != nil {
		return err
	}
	publicHandler, operationsHandler, err := apiHandlers(
		cfg, build, logger, tracing, verifier,
		platformStore, accommodationService, stayService, questionnaireService,
		publicAnalytics, analyticsQuality,
	)
	if err != nil {
		return err
	}
	publicListener, operationsListener, err := apiListeners(cfg)
	if err != nil {
		return err
	}
	return serveBoth(
		ctx, cfg, logger,
		listenerHandler{publicListener, publicHandler},
		listenerHandler{operationsListener, operationsHandler},
	)
}

// The handlers are assembled before any listener is opened, so a dependency the
// API cannot serve correctly — an activation repository that fails to build, or
// a cursor keyring a paginated surface cannot use — stops the process before it
// starts accepting connections.
func apiHandlers(
	cfg config.Config,
	build Build,
	logger *slog.Logger,
	tracing *telemetry.Provider,
	verifier access.Verifier,
	platformStore *store.Store,
	accommodationService *accommodation.Service,
	stayService *stay.Service,
	questionnaireService *questionnaire.Service,
	publicAnalytics analytics.PublicReader,
	analyticsQuality analytics.QualityReader,
) (http.Handler, http.Handler, error) {
	activations, err := activationService(platformStore, cfg)
	if err != nil {
		return nil, nil, errors.New("activation repository initialization failed")
	}
	accessRequests, err := accessRequestService(platformStore, cfg)
	if err != nil {
		return nil, nil, errors.New("access request repository initialization failed")
	}
	// A lista pública de hospedagens não tem chave própria nem pool próprio: é
	// leitura do mesmo cadastro, recortada na consulta, e por isso sobe junto
	// com a acomodação em vez de atrás de um flag que poderia deixar o hóspede
	// sem lista numa stack que tem hospedagem publicada.
	publicDirectory := store.NewAccommodationDirectoryRepository(platformStore)
	return httpapi.New(apiDependencies(
		cfg, build, logger, tracing, verifier,
		platformStore, accommodationService, stayService, questionnaireService,
		publicAnalytics, publicDirectory, analyticsQuality, activations, accessRequests,
	))
}

type listenerHandler struct {
	listener net.Listener
	handler  http.Handler
}

// Either listener stopping takes the other down with it: a half-serving API is
// worse than one that exits and is restarted.
func serveBoth(
	ctx context.Context,
	cfg config.Config,
	logger *slog.Logger,
	public listenerHandler,
	operations listenerHandler,
) error {
	runContext, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan error, 2)
	for _, target := range []listenerHandler{public, operations} {
		go func() {
			results <- server.Serve(
				runContext,
				target.listener,
				configuredHTTPServer(target.handler, cfg),
				cfg.ShutdownTimeout,
				logger,
			)
		}()
	}
	logger.Info("api started", "component", "api")
	first := <-results
	cancel()
	second := <-results
	if first != nil || second != nil {
		return errors.New("HTTP server stopped unexpectedly")
	}
	return nil
}

func apiDependencies(
	cfg config.Config,
	build Build,
	logger *slog.Logger,
	tracing *telemetry.Provider,
	verifier access.Verifier,
	platformStore *store.Store,
	accommodationService *accommodation.Service,
	stayService *stay.Service,
	questionnaireService *questionnaire.Service,
	publicAnalytics analytics.PublicReader,
	publicDirectory directory.PublicReader,
	analyticsQuality analytics.QualityReader,
	activationService *activation.Service,
	accessRequestService *accessrequest.Service,
) httpapi.Dependencies {
	return httpapi.Dependencies{
		Readiness:                      platformStore,
		Verifier:                       verifier,
		Auth:                           localAuthenticator(cfg, platformStore),
		LockoutDuration:                cfg.Auth.LockoutDuration,
		Accommodations:                 accommodationService,
		AccommodationOnboardingEnabled: cfg.Core.AccommodationOnboardingEnabled,
		Stays:                          stayService,
		SelfServiceEnabled:             cfg.SelfService.Enabled,
		AccessRequests:                 accessRequestService,
		Activation:                     activationService,
		Questionnaires:                 questionnaireService,
		PublicAnalytics:                publicAnalytics,
		PublicDirectory:                publicDirectory,
		AnalyticsQuality:               analyticsQuality,
		CORSAllowedOrigins:             cfg.Core.CORSAllowedOrigins,
		TrustedProxyCIDRs:              cfg.TrustedProxyCIDRs,
		CursorKeys:                     cfg.Core.CursorKeys,
		Logger:                         logger,
		Registry:                       prometheus.NewRegistry(),
		Tracer:                         tracing.Tracer("cumuru-api-http"),
		Build: httpapi.BuildInfo{
			Version:  build.Version,
			Revision: build.Revision,
			BuiltAt:  build.BuiltAt,
		},
	}
}

// The public and operations listeners are opened together so a partially bound
// process never starts serving.
func apiListeners(cfg config.Config) (net.Listener, net.Listener, error) {
	publicListener, err := net.Listen("tcp", cfg.HTTPAddress)
	if err != nil {
		return nil, nil, errors.New("public HTTP listener failed")
	}
	operationsListener, err := net.Listen("tcp", cfg.OperationsAddress)
	if err != nil {
		_ = publicListener.Close()
		return nil, nil, errors.New("operations HTTP listener failed")
	}
	return publicListener, operationsListener, nil
}

func services(
	platformStore *store.Store,
) (*accommodation.Service, *stay.Service, *questionnaire.Service, error) {
	accommodationService := accommodation.NewService(store.NewAccommodationRepository(platformStore))
	stayRepository, err := store.NewStayRepository(platformStore)
	if err != nil {
		return nil, nil, nil, err
	}
	questionnaireService := questionnaire.NewService(
		store.NewQuestionnaireRepository(platformStore),
	)
	return accommodationService, stay.NewService(stayRepository), questionnaireService, nil
}

// activationService is nil when the feature is off, which is what keeps the two
// public activation routes unregistered instead of half configured.
func activationService(
	platformStore *store.Store,
	cfg config.Config,
) (*activation.Service, error) {
	if !cfg.SelfService.Enabled {
		return nil, nil
	}
	repository, err := store.NewActivationRepository(platformStore)
	if err != nil {
		return nil, err
	}
	return activation.NewService(repository), nil
}

// accessRequestService follows the same rule: nil when the feature is off, and
// the five routes simply do not exist instead of existing without a usable
// proof-of-work key.
func accessRequestService(
	platformStore *store.Store,
	cfg config.Config,
) (*accessrequest.Service, error) {
	if !cfg.SelfService.Enabled {
		return nil, nil
	}
	repository, err := store.NewAccessRequestRepository(platformStore)
	if err != nil {
		return nil, err
	}
	return accessrequest.NewService(repository), nil
}

func openServices(
	ctx context.Context,
	cfg config.Config,
) (
	*store.Store,
	*accommodation.Service,
	*stay.Service,
	*questionnaire.Service,
	analytics.PublicReader,
	analytics.QualityReader,
	func(),
	error,
) {
	pool, err := database.Open(ctx, cfg.DatabaseURL, cfg.DatabaseTimeout)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	platformStore, err := store.NewQuestionnaire(
		pool, cfg.DatabaseTimeout, cfg.Core, cfg.Questionnaire,
		store.WithAuthConfig(cfg.Auth),
		store.WithSelfServiceConfig(cfg.SelfService),
	)
	if err != nil {
		pool.Close()
		return nil, nil, nil, nil, nil, nil, nil,
			errors.New("questionnaire repository initialization failed")
	}
	accommodationService, stayService, questionnaireService, err := services(platformStore)
	if err != nil {
		pool.Close()
		return nil, nil, nil, nil, nil, nil, nil,
			errors.New("repository initialization failed")
	}
	publicAnalytics, analyticsQuality, closePublic, err := openAnalyticsServices(
		ctx, cfg, platformStore,
	)
	if err != nil {
		pool.Close()
		return nil, nil, nil, nil, nil, nil, nil, err
	}
	closeDatabases := func() {
		closePublic()
		pool.Close()
	}
	return platformStore, accommodationService, stayService, questionnaireService,
		publicAnalytics, analyticsQuality, closeDatabases, nil
}

func openAnalyticsServices(
	ctx context.Context,
	cfg config.Config,
	platformStore *store.Store,
) (analytics.PublicReader, analytics.QualityReader, func(), error) {
	if !cfg.Analytics.Enabled {
		return nil, nil, func() {}, nil
	}
	publicPool, err := store.OpenPublicPool(
		ctx,
		cfg.Analytics.PublicDatabaseURL,
		cfg.Analytics.PublicDatabaseTimeout,
	)
	if err != nil {
		return nil, nil, nil, errors.New("public analytics repository initialization failed")
	}
	publicRepository := store.NewPublicAnalyticsRepository(
		publicPool,
		cfg.Analytics.PublicDatabaseTimeout,
	)
	qualityRepository := store.NewAnalyticsRepository(platformStore, cfg.Analytics)
	return publicRepository, qualityRepository, publicPool.Close, nil
}

// verifierFor chains the local session verifier ahead of the OIDC one. The two
// tracks coexist: a prefixed session token never reaches OIDC discovery, and an
// OIDC token never touches the session store.
func verifierFor(
	ctx context.Context,
	cfg config.Config,
	platformStore *store.Store,
) (access.Verifier, error) {
	federated, err := federatedVerifier(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if !cfg.Auth.Enabled {
		return federated, nil
	}
	session, err := access.NewSessionVerifier(platformStore)
	if err != nil {
		return nil, err
	}
	return access.NewChainVerifier(session, federated)
}

func federatedVerifier(ctx context.Context, cfg config.Config) (access.Verifier, error) {
	if cfg.OIDC.Mode == config.OIDCModeFake {
		return access.NewDevelopmentFake(string(cfg.Environment), cfg.OIDC.Issuer)
	}
	discoveryContext, cancel := context.WithTimeout(ctx, cfg.OIDC.HTTPTimeout)
	defer cancel()
	return access.NewOIDCVerifier(discoveryContext, access.OIDCOptions{
		Issuer:   cfg.OIDC.Issuer,
		Audience: cfg.OIDC.Audience,
		HTTPClient: &http.Client{
			Timeout: cfg.OIDC.HTTPTimeout,
		},
	})
}

func localAuthenticator(cfg config.Config, platformStore *store.Store) httpapi.Authenticator {
	if !cfg.Auth.Enabled {
		return nil
	}
	return platformStore
}

func configuredHTTPServer(handler http.Handler, cfg config.Config) *http.Server {
	return &http.Server{
		Handler:           handler,
		MaxHeaderBytes:    1 << 20,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}

func shutdownTelemetry(provider *telemetry.Provider, timeout time.Duration, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := provider.Shutdown(ctx); err != nil {
		logger.Error("telemetry shutdown failed", "error_code", "telemetry_shutdown_failed")
	}
}
