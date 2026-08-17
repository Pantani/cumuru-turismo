package store

import (
	"context"
	"errors"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrSeedConflict separates a refusal to overwrite curated data from a database
// failure, so the caller can report which row blocked the run.
var ErrSeedConflict = errors.New("seeded row conflicts with existing data")

// SeedAccommodation is one entry of the versioned catalog. The identifier comes
// from the catalog file: the seeder never mints one, so re-running it updates
// the same row instead of creating a second establishment under a new id.
type SeedAccommodation struct {
	ID             uuid.UUID
	Name           string
	Category       string
	CadasturID     *string
	Capacity       *int32
	PublicAreaCode *string
}

type SeedOrganization struct {
	ID             uuid.UUID
	Name           string
	Accommodations []SeedAccommodation
}

// SeedAccount carries an already computed hash; the plain secret never reaches
// this layer.
type SeedAccount struct {
	ID                 uuid.UUID
	Email              string
	DisplayName        string
	PasswordHash       string
	Scopes             []string
	PasswordMustChange bool
}

type SeedRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

func NewSeedRepository(pool *pgxpool.Pool, timeout time.Duration) *SeedRepository {
	return &SeedRepository{pool: pool, timeout: timeout}
}

// AcquireRunLock serializes concurrent seeders on a session advisory lock, so
// two deploys racing on the same database cannot interleave their writes.
func (r *SeedRepository) AcquireRunLock(
	ctx context.Context,
) (func() error, error) {
	lockCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	connection, err := r.pool.Acquire(lockCtx)
	if err != nil {
		return nil, ErrUnavailable
	}
	if err := generated.New(connection).AcquireSeedRunLock(lockCtx); err != nil {
		connection.Release()
		return nil, ErrUnavailable
	}
	return func() error { return r.releaseRunLock(connection) }, nil
}

func (r *SeedRepository) releaseRunLock(connection *pgxpool.Conn) error {
	defer connection.Release()
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	released, err := generated.New(connection).ReleaseSeedRunLock(ctx)
	if err != nil || !released {
		return ErrUnavailable
	}
	return nil
}

// EnsureCatalog writes the organization and its establishments in one
// transaction, so a partially applied catalog never becomes visible.
func (r *SeedRepository) EnsureCatalog(
	ctx context.Context,
	organization SeedOrganization,
) error {
	return r.inTransaction(ctx, func(q *generated.Queries) error {
		if err := upsertSeedOrganization(ctx, q, organization); err != nil {
			return err
		}
		for _, accommodation := range organization.Accommodations {
			err := upsertSeedAccommodation(ctx, q, organization.ID, accommodation)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// EnsureAccount creates the account on the first run and, on later runs, only
// refreshes the display name and scopes. The stored row is read back because the
// insert is a no-op on conflict: a disabled account under the same address is a
// conflict the operator has to resolve, not a row to reuse.
func (r *SeedRepository) EnsureAccount(
	ctx context.Context,
	account SeedAccount,
) (uuid.UUID, error) {
	var stored generated.FindSeedAccountRow
	err := r.inTransaction(ctx, func(q *generated.Queries) error {
		if err := insertSeedAccount(ctx, q, account); err != nil {
			return err
		}
		var findErr error
		stored, findErr = findSeedAccount(ctx, q, account.Email)
		return findErr
	})
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.UUID(stored.ID.Bytes), nil
}

// EnsureMemberships binds the account to every establishment through the same
// membership table the OIDC track uses, so both tracks share one authorization
// path.
func (r *SeedRepository) EnsureMemberships(
	ctx context.Context,
	issuer string,
	accountID uuid.UUID,
	accommodations []SeedAccommodation,
) error {
	return r.inTransaction(ctx, func(q *generated.Queries) error {
		for _, accommodation := range accommodations {
			err := q.UpsertSeedMembership(ctx, generated.UpsertSeedMembershipParams{
				ID:              pgUUID(uuid.New()),
				AccommodationID: pgUUID(accommodation.ID),
				OidcIssuer:      issuer,
				OidcSubject:     accountID.String(),
			})
			if err != nil {
				return ErrUnavailable
			}
		}
		return nil
	})
}

func upsertSeedOrganization(
	ctx context.Context,
	q *generated.Queries,
	organization SeedOrganization,
) error {
	err := q.UpsertSeedOrganization(ctx, generated.UpsertSeedOrganizationParams{
		ID:   pgUUID(organization.ID),
		Name: organization.Name,
	})
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

// The upsert is scoped to the declared organization, so an identifier already
// held by another organization leaves the row untouched. That refusal has to
// reach the operator: the statement itself raises no error, so without the row
// count the run reports success for a catalog entry it never applied, and the
// file and the database diverge without anyone noticing.
func upsertSeedAccommodation(
	ctx context.Context,
	q *generated.Queries,
	organizationID uuid.UUID,
	accommodation SeedAccommodation,
) error {
	applied, err := q.UpsertSeedAccommodation(
		ctx,
		generated.UpsertSeedAccommodationParams{
			ID:             pgUUID(accommodation.ID),
			OrganizationID: pgUUID(organizationID),
			Name:           accommodation.Name,
			Category:       accommodation.Category,
			CadasturID:     accommodation.CadasturID,
			Capacity:       accommodation.Capacity,
			PublicAreaCode: accommodation.PublicAreaCode,
		},
	)
	if err != nil || applied == 0 {
		return ErrSeedConflict
	}
	return nil
}

func insertSeedAccount(
	ctx context.Context,
	q *generated.Queries,
	account SeedAccount,
) error {
	// Semeadura sempre traz credencial; nulo na coluna é a conta que ainda
	// espera ativação por capability (ADR-041), estado que o seed não produz.
	err := q.InsertSeedAccount(ctx, generated.InsertSeedAccountParams{
		ID:                 pgUUID(account.ID),
		Email:              account.Email,
		DisplayName:        account.DisplayName,
		PasswordHash:       &account.PasswordHash,
		Scopes:             account.Scopes,
		PasswordMustChange: account.PasswordMustChange,
	})
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

func findSeedAccount(
	ctx context.Context,
	q *generated.Queries,
	email string,
) (generated.FindSeedAccountRow, error) {
	stored, err := q.FindSeedAccount(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return generated.FindSeedAccountRow{}, ErrSeedConflict
	}
	if err != nil {
		return generated.FindSeedAccountRow{}, ErrUnavailable
	}
	if stored.Status != accountStatusActive {
		return generated.FindSeedAccountRow{}, ErrSeedConflict
	}
	return stored, nil
}

func (r *SeedRepository) inTransaction(
	ctx context.Context,
	work func(*generated.Queries) error,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ErrUnavailable
	}
	if err := work(generated.New(tx)); err != nil {
		rollbackProvisioning(tx, r.timeout)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return ErrUnavailable
	}
	return nil
}
