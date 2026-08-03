package localdemo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/database"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/Pantani/cumuru/apps/api/internal/questionnaire"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/jackc/pgx/v5/pgxpool"
)

type servicesAt func(time.Time) (fixtureServices, error)

type analyticsPublisher interface {
	Reconcile(
		context.Context,
		analytics.ReconciliationKind,
		stay.CivilDate,
	) (bool, error)
	BuildAndPublish(context.Context, stay.CivilDate) (int64, bool, error)
}

type localDemoPools struct {
	application  *pgxpool.Pool
	worker       *pgxpool.Pool
	provisioning *pgxpool.Pool
}

func Run(
	ctx context.Context,
	cfg config.LocalDemoConfig,
	output io.Writer,
) error {
	location, err := time.LoadLocation("America/Bahia")
	if err != nil {
		return fmt.Errorf("local demo timezone unavailable: %w", err)
	}
	pools, err := openLocalDemoPools(ctx, cfg)
	if err != nil {
		return err
	}
	defer pools.close()
	return loadFixtures(ctx, cfg.Application, pools, location, output)
}

func openLocalDemoPools(
	ctx context.Context,
	cfg config.LocalDemoConfig,
) (localDemoPools, error) {
	var pools localDemoPools
	var err error
	pools.application, err = database.Open(
		ctx,
		cfg.Application.DatabaseURL,
		cfg.Application.DatabaseTimeout,
	)
	if err != nil {
		return localDemoPools{}, err
	}
	pools.worker, err = database.Open(
		ctx,
		cfg.WorkerDatabaseURL,
		cfg.Application.DatabaseTimeout,
	)
	if err != nil {
		pools.close()
		return localDemoPools{}, err
	}
	pools.provisioning, err = database.Open(
		ctx,
		cfg.ProvisioningDatabaseURL,
		cfg.Application.DatabaseTimeout,
	)
	if err != nil {
		pools.close()
		return localDemoPools{}, err
	}
	return pools, nil
}

func loadFixtures(
	ctx context.Context,
	cfg config.Config,
	pools localDemoPools,
	location *time.Location,
	output io.Writer,
) error {
	provisioner := store.NewLocalDemoRepository(
		pools.provisioning,
		cfg.DatabaseTimeout,
	)
	return executeWithRunLock(
		func() (func() error, error) {
			return provisioner.AcquireRunLock(ctx)
		},
		func() error {
			return loadFixturesLocked(
				ctx,
				cfg,
				pools,
				location,
				output,
				provisioner,
			)
		},
	)
}

func executeWithRunLock(
	acquire func() (func() error, error),
	work func() error,
) error {
	release, err := acquire()
	if err != nil {
		return fmt.Errorf("local demo lock failed: %w", err)
	}
	workErr := work()
	releaseErr := release()
	if releaseErr != nil {
		releaseErr = fmt.Errorf("local demo unlock failed: %w", releaseErr)
	}
	return errors.Join(workErr, releaseErr)
}

func loadFixturesLocked(
	ctx context.Context,
	cfg config.Config,
	pools localDemoPools,
	location *time.Location,
	output io.Writer,
	provisioner *store.LocalDemoRepository,
) error {
	foundation := foundationFixture()
	if err := provisioner.EnsureFoundation(ctx, foundation); err != nil {
		return fmt.Errorf("local demo foundation failed: %w", err)
	}
	now := time.Now().UTC()
	factory := fixtureServiceFactory(pools.application, cfg)
	currentServices, err := factory(now)
	if err != nil {
		return err
	}
	if err := ensureQuestionnaire(ctx, currentServices.questionnaires); err != nil {
		return fmt.Errorf("local demo questionnaire failed: %w", err)
	}
	if err := provisioner.EnsureMappings(ctx, mappingFixtures()); err != nil {
		return fmt.Errorf("local demo mappings failed: %w", err)
	}
	fixtures := stayFixtures(now, location)
	if err := loadStayFixtures(
		ctx,
		factory,
		provisioner,
		fixtures,
	); err != nil {
		return err
	}
	if err := publishAnalytics(
		ctx,
		pools.worker,
		cfg,
		civilDay(now, location),
	); err != nil {
		return fmt.Errorf("local demo analytics publication failed: %w", err)
	}
	_, err = fmt.Fprintf(
		output,
		"LOCAL_DEMO_SEED=PASS %s source=go\n",
		fixtureSummary(foundation, fixtures),
	)
	if err != nil {
		return fmt.Errorf("local demo summary failed: %w", err)
	}
	return nil
}

func fixtureSummary(
	foundation store.LocalDemoFoundation,
	fixtures []stayFixture,
) string {
	responses := 0
	for _, fixture := range fixtures {
		if fixture.responseCategory != "" {
			responses++
		}
	}
	return fmt.Sprintf(
		"organizations=1 accommodations=%d responses=%d",
		len(foundation.Accommodations),
		responses,
	)
}

func loadStayFixtures(
	ctx context.Context,
	factory servicesAt,
	provisioner *store.LocalDemoRepository,
	fixtures []stayFixture,
) error {
	for _, fixture := range fixtures {
		services, serviceErr := factory(fixture.clock)
		if serviceErr != nil {
			return serviceErr
		}
		if serviceErr = loadStayFixture(
			ctx,
			services,
			provisioner,
			fixture,
		); serviceErr != nil {
			return fmt.Errorf(
				"local demo stay journey failed: %s: %w",
				fixture.key,
				serviceErr,
			)
		}
	}
	return nil
}

func (p localDemoPools) close() {
	if p.provisioning != nil {
		p.provisioning.Close()
	}
	if p.worker != nil {
		p.worker.Close()
	}
	if p.application != nil {
		p.application.Close()
	}
}

func fixtureServiceFactory(
	pool *pgxpool.Pool,
	cfg config.Config,
) servicesAt {
	return func(now time.Time) (fixtureServices, error) {
		platformStore, err := store.NewPhase3(
			pool,
			cfg.DatabaseTimeout,
			cfg.Phase2,
			cfg.Phase3,
			store.WithCurrentTime(func() time.Time { return now }),
		)
		if err != nil {
			return fixtureServices{}, err
		}
		stayRepository, err := store.NewStayRepository(platformStore)
		if err != nil {
			return fixtureServices{}, err
		}
		return fixtureServices{
			stays: stay.NewService(stayRepository),
			questionnaires: questionnaire.NewService(
				store.NewQuestionnaireRepository(platformStore),
			),
		}, nil
	}
}

func publishAnalytics(
	ctx context.Context,
	pool *pgxpool.Pool,
	cfg config.Config,
	asOf time.Time,
) error {
	platformStore, err := store.NewPhase3(
		pool,
		cfg.DatabaseTimeout,
		cfg.Phase2,
		cfg.Phase3,
	)
	if err != nil {
		return err
	}
	repository := store.NewAnalyticsRepository(platformStore, cfg.Phase4)
	civil, err := stay.ParseCivilDate(civilDate(asOf))
	if err != nil {
		return err
	}
	return reconcileAndPublish(ctx, repository, civil)
}

func reconcileAndPublish(
	ctx context.Context,
	repository analyticsPublisher,
	asOf stay.CivilDate,
) error {
	if _, err := repository.Reconcile(
		ctx,
		analytics.ReconciliationFull,
		asOf,
	); err != nil {
		return err
	}
	_, _, err := repository.BuildAndPublish(ctx, asOf)
	return err
}
