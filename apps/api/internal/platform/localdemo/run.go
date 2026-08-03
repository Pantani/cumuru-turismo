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
		return errors.New("local demo timezone unavailable")
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
	release, err := provisioner.AcquireRunLock(ctx)
	if err != nil {
		return errors.New("local demo lock failed")
	}
	result := loadFixturesLocked(
		ctx,
		cfg,
		pools,
		location,
		output,
		provisioner,
	)
	releaseErr := release()
	if result != nil {
		return result
	}
	if releaseErr != nil {
		return errors.New("local demo unlock failed")
	}
	return nil
}

func loadFixturesLocked(
	ctx context.Context,
	cfg config.Config,
	pools localDemoPools,
	location *time.Location,
	output io.Writer,
	provisioner *store.LocalDemoRepository,
) error {
	if err := provisioner.EnsureFoundation(ctx, foundationFixture()); err != nil {
		return errors.New("local demo foundation failed")
	}
	now := time.Now().UTC()
	factory := fixtureServiceFactory(pools.application, cfg)
	currentServices, err := factory(now)
	if err != nil {
		return err
	}
	if err := ensureQuestionnaire(ctx, currentServices.questionnaires); err != nil {
		return errors.New("local demo questionnaire failed")
	}
	if err := provisioner.EnsureMappings(ctx, mappingFixtures()); err != nil {
		return errors.New("local demo mappings failed")
	}
	if err := loadStayFixtures(
		ctx,
		factory,
		provisioner,
		stayFixtures(now, location),
	); err != nil {
		return err
	}
	if err := publishAnalytics(
		ctx,
		pools.worker,
		cfg,
		civilDay(now, location),
	); err != nil {
		return errors.New("local demo analytics publication failed")
	}
	_, err = fmt.Fprintln(
		output,
		"LOCAL_DEMO_SEED=PASS organizations=1 accommodations=4 responses=20 source=go",
	)
	return err
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
