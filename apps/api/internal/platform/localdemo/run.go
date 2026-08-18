package localdemo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/database"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/Pantani/cumuru/apps/api/internal/questionnaire"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DATABASE_TIMEOUT is shaped for a request; two years of fixtures are a batch.
// The full reconciliation that closes the seed, and the wait of a second seeder
// on the run lock, both outlast a request by design — borrowing the request
// budget would abort the publication halfway and leave the cover without a
// series to read.
const seedBatchTimeout = 10 * time.Minute

type servicesAt func(time.Time) fixtureServices

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
			return provisioner.AcquireRunLock(ctx, seedBatchTimeout)
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
	if err := ensureFoundationAndAccount(ctx, provisioner, foundation); err != nil {
		return err
	}
	now := time.Now().UTC()
	fixtures, err := loadJourneys(ctx, cfg, pools, location, provisioner, now)
	if err != nil {
		return err
	}
	if err := publishAnalytics(ctx, pools.worker, cfg, civilDay(now, location)); err != nil {
		return fmt.Errorf("local demo analytics publication failed: %w", err)
	}
	return reportSeed(output, foundation, fixtures)
}

// The catalogue must exist before any stay journey runs, since a submission
// needs a published questionnaire to issue its survey capability.
func loadJourneys(
	ctx context.Context,
	cfg config.Config,
	pools localDemoPools,
	location *time.Location,
	provisioner *store.LocalDemoRepository,
	now time.Time,
) ([]stayFixture, error) {
	factory, err := fixtureServiceFactory(pools.application, cfg, now)
	if err != nil {
		return nil, fmt.Errorf("local demo fixture services failed: %w", err)
	}
	if err := ensureCatalog(ctx, factory(now), provisioner); err != nil {
		return nil, err
	}
	fixtures := stayFixtures(now, location)
	if err := loadStayFixtures(ctx, factory, provisioner, fixtures); err != nil {
		return nil, err
	}
	return fixtures, nil
}

func reportSeed(
	output io.Writer,
	foundation store.LocalDemoFoundation,
	fixtures []stayFixture,
) error {
	_, err := fmt.Fprintf(
		output,
		"LOCAL_DEMO_SEED=PASS %s source=go\n",
		fixtureSummary(foundation, fixtures),
	)
	if err != nil {
		return fmt.Errorf("local demo summary failed: %w", err)
	}
	return nil
}

func ensureFoundationAndAccount(
	ctx context.Context,
	provisioner *store.LocalDemoRepository,
	foundation store.LocalDemoFoundation,
) error {
	if err := provisioner.EnsureFoundation(ctx, foundation); err != nil {
		return fmt.Errorf("local demo foundation failed: %w", err)
	}
	account, err := accountFixture(os.LookupEnv)
	if err != nil {
		return err
	}
	if err := provisioner.EnsureAccount(ctx, foundation, account); err != nil {
		return fmt.Errorf("local demo account failed: %w", err)
	}
	return nil
}

func ensureCatalog(
	ctx context.Context,
	services fixtureServices,
	provisioner *store.LocalDemoRepository,
) error {
	if err := ensureQuestionnaire(ctx, services.questionnaires); err != nil {
		return fmt.Errorf("local demo questionnaire failed: %w", err)
	}
	if err := provisioner.EnsureMappings(ctx, mappingFixtures()); err != nil {
		return fmt.Errorf("local demo mappings failed: %w", err)
	}
	return nil
}

func fixtureSummary(
	foundation store.LocalDemoFoundation,
	fixtures []stayFixture,
) string {
	responses := 0
	visitors := int32(0)
	for _, fixture := range fixtures {
		visitors += fixture.guestCount
		if fixture.responseCategory != "" {
			responses++
		}
	}
	return fmt.Sprintf(
		"organizations=1 accommodations=%d stays=%d visitors=%d responses=%d",
		len(foundation.Accommodations),
		len(fixtures),
		visitors,
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
		services := factory(fixture.clock)
		if serviceErr := loadStayFixture(
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
	initialTime time.Time,
) (servicesAt, error) {
	var currentTime atomic.Pointer[time.Time]
	currentTime.Store(&initialTime)
	platformStore, err := store.NewQuestionnaire(
		pool,
		cfg.DatabaseTimeout,
		cfg.Core,
		cfg.Questionnaire,
		store.WithCurrentTime(func() time.Time { return *currentTime.Load() }),
	)
	if err != nil {
		return nil, err
	}
	stayRepository, err := store.NewStayRepository(platformStore)
	if err != nil {
		return nil, err
	}
	services := fixtureServices{
		stays: stay.NewService(stayRepository),
		questionnaires: questionnaire.NewService(
			store.NewQuestionnaireRepository(platformStore),
		),
	}
	// The run lock serializes fixture journeys; atomic storage also keeps the
	// callback race-safe if a future caller observes the clock concurrently.
	return func(now time.Time) fixtureServices {
		currentTime.Store(&now)
		return services
	}, nil
}

func publishAnalytics(
	ctx context.Context,
	pool *pgxpool.Pool,
	cfg config.Config,
	asOf time.Time,
) error {
	platformStore, err := store.NewQuestionnaire(
		pool,
		seedBatchTimeout,
		cfg.Core,
		cfg.Questionnaire,
	)
	if err != nil {
		return err
	}
	repository := store.NewAnalyticsRepository(platformStore, cfg.Analytics)
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
