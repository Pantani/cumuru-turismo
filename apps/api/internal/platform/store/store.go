package store

import (
	"context"
	"errors"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/Pantani/cumuru/apps/api/internal/questionnaire"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUnavailable = errors.New("database unavailable")

type TenantMembership struct {
	MembershipID    string
	Role            string
	AccommodationID string
	OrganizationID  string
}

type Store struct {
	queries          generated.Querier
	timeout          time.Duration
	pool             *pgxpool.Pool
	core             config.CoreConfig
	questionnaire    config.QuestionnaireConfig
	selfService      config.SelfServiceConfig
	auth             config.AuthConfig
	surveyCodec      *questionnaire.CapabilityCodec
	textCipher       *questionnaire.TextCipher
	surveyPairPermit chan struct{}
	now              func() time.Time
}

type Option func(*Store)

func WithCurrentTime(now func() time.Time) Option {
	return func(store *Store) {
		if now != nil {
			store.now = now
		}
	}
}

// WithSelfServiceConfig enables the open self-registration channel and the account
// activation capability. Absent, the surfaces are simply not registered, which
// is a 404 rather than a half-configured route.
func WithSelfServiceConfig(selfService config.SelfServiceConfig) Option {
	return func(store *Store) {
		store.selfService = selfService
	}
}

// WithAuthConfig enables the local e-mail and password track. Without it the
// store stays OIDC-only and Authenticate rejects every attempt.
func WithAuthConfig(auth config.AuthConfig) Option {
	return func(store *Store) {
		store.auth = auth
	}
}

// New builds a store over an arbitrary Querier. It exists for tests that drive
// the store against a stub; the running processes use NewCore or NewQuestionnaire,
// which own a real pool and the feature keyrings.
func New(queries generated.Querier, timeout time.Duration) *Store {
	return &Store{
		queries: queries, timeout: timeout,
		surveyPairPermit: make(chan struct{}, 1),
		now:              time.Now,
	}
}

func NewCore(pool *pgxpool.Pool, timeout time.Duration, core config.CoreConfig) *Store {
	return &Store{
		queries:          generated.New(pool),
		timeout:          timeout,
		pool:             pool,
		core:             core,
		surveyPairPermit: make(chan struct{}, 1),
		now:              time.Now,
	}
}

func NewQuestionnaire(
	pool *pgxpool.Pool,
	timeout time.Duration,
	core config.CoreConfig,
	settings config.QuestionnaireConfig,
	options ...Option,
) (*Store, error) {
	store := NewCore(pool, timeout, core)
	for _, option := range options {
		option(store)
	}
	store.questionnaire = settings
	if !settings.Enabled {
		return store, nil
	}
	codec, err := questionnaire.NewCapabilityCodec(questionnaireKeyring(settings.SurveyKeys))
	if err != nil {
		return nil, err
	}
	textCipher, err := questionnaire.NewTextCipher(questionnaireKeyring(settings.FreeTextKeys))
	if err != nil {
		return nil, err
	}
	store.surveyCodec = codec
	store.textCipher = textCipher
	return store, nil
}

func questionnaireKeyring(config config.KeyringConfig) questionnaire.Keyring {
	return questionnaire.Keyring{
		CurrentVersion: config.CurrentVersion,
		Keys:           config.Keys,
	}
}

func (s *Store) CheckReadiness(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	ready, err := s.queries.CheckReadiness(ctx)
	if err != nil || ready != 1 {
		return ErrUnavailable
	}
	return nil
}

func (s *Store) ResolveTenants(ctx context.Context, principal access.Principal) ([]TenantMembership, error) {
	if principal.Issuer == "" || principal.Subject == "" {
		return nil, errors.New("verified principal is required")
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	rows, err := s.queries.ListActiveTenantMemberships(ctx, generated.ListActiveTenantMembershipsParams{
		OidcIssuer:  principal.Issuer,
		OidcSubject: principal.Subject,
	})
	if err != nil {
		return nil, ErrUnavailable
	}
	return tenantMemberships(rows)
}

func tenantMemberships(
	rows []generated.ListActiveTenantMembershipsRow,
) ([]TenantMembership, error) {
	result := make([]TenantMembership, 0, len(rows))
	for _, row := range rows {
		if !completeTenantRow(row) {
			return nil, ErrUnavailable
		}
		result = append(result, TenantMembership{
			MembershipID:    uuid.UUID(row.MembershipID.Bytes).String(),
			Role:            row.Role,
			AccommodationID: uuid.UUID(row.AccommodationID.Bytes).String(),
			OrganizationID:  uuid.UUID(row.OrganizationID.Bytes).String(),
		})
	}
	return result, nil
}

func completeTenantRow(row generated.ListActiveTenantMembershipsRow) bool {
	return row.MembershipID.Valid &&
		row.AccommodationID.Valid &&
		row.OrganizationID.Valid
}
