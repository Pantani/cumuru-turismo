package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrLocalDemoConflict = errors.New("reserved local demo fixture conflicts with existing data")

type LocalDemoAccommodation struct {
	ID             uuid.UUID
	Name           string
	Category       string
	CadasturID     *string
	Capacity       int32
	PublicAreaCode string
}

type LocalDemoFoundation struct {
	OrganizationID   uuid.UUID
	OrganizationName string
	OIDCIssuer       string
	OIDCSubject      string
	Accommodations   []LocalDemoAccommodation
}

type LocalDemoMetricMapping struct {
	PrivacyPolicyVersion   string
	MetricCode             string
	QuestionnaireVersionID uuid.UUID
	QuestionID             uuid.UUID
	SourceValue            string
	CategoryCode           string
}

type LocalDemoRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

func NewLocalDemoRepository(
	pool *pgxpool.Pool,
	timeout time.Duration,
) *LocalDemoRepository {
	return &LocalDemoRepository{pool: pool, timeout: timeout}
}

func (r *LocalDemoRepository) AcquireRunLock(
	ctx context.Context,
) (func() error, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	connection, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, ErrUnavailable
	}
	if err := generated.New(connection).AcquireLocalDemoRunLock(ctx); err != nil {
		connection.Release()
		return nil, ErrUnavailable
	}
	return func() error {
		defer connection.Release()
		releaseCtx, releaseCancel := context.WithTimeout(
			context.Background(),
			r.timeout,
		)
		defer releaseCancel()
		released, err := generated.New(connection).ReleaseLocalDemoRunLock(
			releaseCtx,
		)
		if err != nil || !released {
			return ErrUnavailable
		}
		return nil
	}, nil
}

func (r *LocalDemoRepository) EnsureFoundation(
	ctx context.Context,
	fixture LocalDemoFoundation,
) error {
	return r.inTransaction(ctx, func(q *generated.Queries) error {
		if err := ensureLocalOrganization(ctx, q, fixture); err != nil {
			return err
		}
		for index, accommodation := range fixture.Accommodations {
			if err := ensureLocalAccommodation(
				ctx,
				q,
				fixture,
				accommodation,
				index,
			); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *LocalDemoRepository) EnsureMappings(
	ctx context.Context,
	mappings []LocalDemoMetricMapping,
) error {
	return r.inTransaction(ctx, func(q *generated.Queries) error {
		for _, mapping := range mappings {
			if err := ensureLocalMetricMapping(ctx, q, mapping); err != nil {
				return err
			}
		}
		return nil
	})
}

func ensureLocalMetricMapping(
	ctx context.Context,
	q *generated.Queries,
	mapping LocalDemoMetricMapping,
) error {
	params := generated.InsertLocalDemoMetricMappingParams{
		PrivacyPolicyVersion:   mapping.PrivacyPolicyVersion,
		MetricCode:             mapping.MetricCode,
		QuestionnaireVersionID: pgUUID(mapping.QuestionnaireVersionID),
		QuestionID:             pgUUID(mapping.QuestionID),
		SourceValue:            mapping.SourceValue,
		CategoryCode:           mapping.CategoryCode,
	}
	if err := q.InsertLocalDemoMetricMapping(ctx, params); err != nil {
		return ErrUnavailable
	}
	category, err := q.GetLocalDemoMetricMapping(
		ctx,
		generated.GetLocalDemoMetricMappingParams{
			PrivacyPolicyVersion:   mapping.PrivacyPolicyVersion,
			MetricCode:             mapping.MetricCode,
			QuestionnaireVersionID: pgUUID(mapping.QuestionnaireVersionID),
			QuestionID:             pgUUID(mapping.QuestionID),
			SourceValue:            mapping.SourceValue,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrLocalDemoConflict
	}
	if err != nil {
		return ErrUnavailable
	}
	if category != mapping.CategoryCode {
		return ErrLocalDemoConflict
	}
	return nil
}

func (r *LocalDemoRepository) FindStay(
	ctx context.Context,
	accommodationID uuid.UUID,
	clientSubmissionID uuid.UUID,
) (uuid.UUID, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	id, err := generated.New(r.pool).FindLocalDemoStay(
		ctx,
		generated.FindLocalDemoStayParams{
			AccommodationID:    pgUUID(accommodationID),
			ClientSubmissionID: pgUUID(clientSubmissionID),
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, ErrUnavailable
	}
	return idFromPG(id), true, nil
}

func (r *LocalDemoRepository) HasGroupSubmission(
	ctx context.Context,
	stayID uuid.UUID,
) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	found, err := generated.New(r.pool).HasLocalDemoGroupSubmission(
		ctx,
		pgUUID(stayID),
	)
	if err != nil {
		return false, ErrUnavailable
	}
	return found, nil
}

func (r *LocalDemoRepository) HasSurveyResponse(
	ctx context.Context,
	stayID uuid.UUID,
	questionnaireVersionID uuid.UUID,
	clientSubmissionID uuid.UUID,
) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	found, err := generated.New(r.pool).HasLocalDemoSurveyResponse(
		ctx,
		generated.HasLocalDemoSurveyResponseParams{
			StayID:                 pgUUID(stayID),
			QuestionnaireVersionID: pgUUID(questionnaireVersionID),
			ClientSubmissionID:     pgUUID(clientSubmissionID),
		},
	)
	if err != nil {
		return false, ErrUnavailable
	}
	return found, nil
}

func (r *LocalDemoRepository) inTransaction(
	ctx context.Context,
	work func(*generated.Queries) error,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ErrUnavailable
	}
	queries := generated.New(tx)
	if err := work(queries); err != nil {
		rollbackLocalDemo(tx, r.timeout)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return ErrUnavailable
	}
	return nil
}

func rollbackLocalDemo(tx pgx.Tx, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_ = tx.Rollback(ctx)
}

func ensureLocalOrganization(
	ctx context.Context,
	q *generated.Queries,
	fixture LocalDemoFoundation,
) error {
	if err := q.InsertLocalDemoOrganization(
		ctx,
		generated.InsertLocalDemoOrganizationParams{
			ID:   pgUUID(fixture.OrganizationID),
			Name: fixture.OrganizationName,
		},
	); err != nil {
		return ErrUnavailable
	}
	name, err := q.GetLocalDemoOrganization(ctx, pgUUID(fixture.OrganizationID))
	return classifyLocalOrganizationResult(name, err, fixture.OrganizationName)
}

func classifyLocalOrganizationResult(name string, err error, expectedName string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrLocalDemoConflict
	}
	if err != nil {
		return ErrUnavailable
	}
	if name != expectedName {
		return ErrLocalDemoConflict
	}
	return nil
}

func ensureLocalAccommodation(
	ctx context.Context,
	q *generated.Queries,
	fixture LocalDemoFoundation,
	accommodation LocalDemoAccommodation,
	index int,
) error {
	if err := insertLocalAccommodation(
		ctx,
		q,
		fixture.OrganizationID,
		accommodation,
	); err != nil {
		return err
	}
	if err := verifyLocalAccommodation(
		ctx,
		q,
		fixture.OrganizationID,
		accommodation,
	); err != nil {
		return err
	}
	return ensureLocalMembership(
		ctx,
		q,
		fixture,
		accommodation.ID,
		index,
	)
}

func insertLocalAccommodation(
	ctx context.Context,
	q *generated.Queries,
	organizationID uuid.UUID,
	accommodation LocalDemoAccommodation,
) error {
	if err := q.InsertLocalDemoAccommodation(
		ctx,
		generated.InsertLocalDemoAccommodationParams{
			ID:             pgUUID(accommodation.ID),
			OrganizationID: pgUUID(organizationID),
			Name:           accommodation.Name,
			Category:       accommodation.Category,
			CadasturID:     accommodation.CadasturID,
			Capacity:       &accommodation.Capacity,
			PublicAreaCode: &accommodation.PublicAreaCode,
		},
	); err != nil {
		return ErrUnavailable
	}
	return nil
}

func verifyLocalAccommodation(
	ctx context.Context,
	q *generated.Queries,
	organizationID uuid.UUID,
	accommodation LocalDemoAccommodation,
) error {
	row, err := q.GetLocalDemoAccommodation(ctx, pgUUID(accommodation.ID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrLocalDemoConflict
	}
	if err != nil {
		return ErrUnavailable
	}
	if !localAccommodationMatches(row, organizationID, accommodation) {
		return ErrLocalDemoConflict
	}
	return nil
}

func localAccommodationMatches(
	row generated.GetLocalDemoAccommodationRow,
	organizationID uuid.UUID,
	accommodation LocalDemoAccommodation,
) bool {
	if row.Capacity == nil || row.PublicAreaCode == nil {
		return false
	}
	if !optionalStringsEqual(row.CadasturID, accommodation.CadasturID) {
		return false
	}
	actual := struct {
		OrganizationID uuid.UUID
		Name           string
		Category       string
		Status         string
		Capacity       int32
		PublicAreaCode string
	}{
		OrganizationID: idFromPG(row.OrganizationID),
		Name:           row.Name,
		Category:       row.Category,
		Status:         string(row.Status),
		Capacity:       *row.Capacity,
		PublicAreaCode: *row.PublicAreaCode,
	}
	expected := struct {
		OrganizationID uuid.UUID
		Name           string
		Category       string
		Status         string
		Capacity       int32
		PublicAreaCode string
	}{
		OrganizationID: organizationID,
		Name:           accommodation.Name,
		Category:       accommodation.Category,
		Status:         "active",
		Capacity:       accommodation.Capacity,
		PublicAreaCode: accommodation.PublicAreaCode,
	}
	return actual == expected
}

func optionalStringsEqual(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func ensureLocalMembership(
	ctx context.Context,
	q *generated.Queries,
	fixture LocalDemoFoundation,
	accommodationID uuid.UUID,
	index int,
) error {
	membershipID := localMembershipID(index)
	if err := q.InsertLocalDemoMembership(
		ctx,
		generated.InsertLocalDemoMembershipParams{
			ID:              pgUUID(membershipID),
			AccommodationID: pgUUID(accommodationID),
			OidcIssuer:      fixture.OIDCIssuer,
			OidcSubject:     fixture.OIDCSubject,
		},
	); err != nil {
		return ErrUnavailable
	}
	membership, err := q.GetLocalDemoMembership(ctx, pgUUID(membershipID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrLocalDemoConflict
	}
	if err != nil {
		return ErrUnavailable
	}
	if !localMembershipMatches(
		membership,
		accommodationID,
		fixture.OIDCIssuer,
		fixture.OIDCSubject,
	) {
		return ErrLocalDemoConflict
	}
	return nil
}

func localMembershipMatches(
	row generated.GetLocalDemoMembershipRow,
	accommodationID uuid.UUID,
	oidcIssuer string,
	oidcSubject string,
) bool {
	actual := struct {
		AccommodationID uuid.UUID
		OIDCIssuer      string
		OIDCSubject     string
		Role            string
		Active          bool
	}{
		AccommodationID: idFromPG(row.AccommodationID),
		OIDCIssuer:      row.OidcIssuer,
		OIDCSubject:     row.OidcSubject,
		Role:            string(row.Role),
		Active:          row.Active,
	}
	expected := struct {
		AccommodationID uuid.UUID
		OIDCIssuer      string
		OIDCSubject     string
		Role            string
		Active          bool
	}{
		AccommodationID: accommodationID,
		OIDCIssuer:      oidcIssuer,
		OIDCSubject:     oidcSubject,
		Role:            "manager",
		Active:          true,
	}
	return actual == expected
}

func localMembershipID(index int) uuid.UUID {
	return uuid.MustParse(
		fmt.Sprintf("019fae12-0000-7000-8000-%012x", index+1),
	)
}
