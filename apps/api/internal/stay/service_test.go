package stay

import (
	"context"
	"errors"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/google/uuid"
)

type serviceRepositoryStub struct {
	Repository
	createCalls int
}

func (r *serviceRepositoryStub) Create(context.Context, CreateCommand) (MutationResult, bool, error) {
	r.createCalls++
	return MutationResult{}, false, nil
}

func TestServiceRejectsInvalidStayDatesBeforeRepository(t *testing.T) {
	t.Parallel()

	repository := &serviceRepositoryStub{}
	service := NewService(repository)
	_, _, err := service.Create(context.Background(), CreateCommand{
		Actor:              access.NewPrincipal("https://issuer.invalid", "operator", nil),
		AccommodationID:    uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		ClientSubmissionID: uuid.MustParse("019f0000-0000-7000-8000-000000000002"),
		PlannedArrivalOn:   "2026-08-10",
		PlannedDepartureOn: "2026-08-10",
		ExpectedGuestCount: 1,
		IdempotencyKey:     "create-stay-key-1234",
		RequestID:          "request-12345678",
	})

	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Create() error = %v, want ErrInvalidInput", err)
	}
	if repository.createCalls != 0 {
		t.Fatalf("repository calls = %d, want 0", repository.createCalls)
	}
}

func TestServiceRejectsGroupWithoutResponsible(t *testing.T) {
	t.Parallel()

	service := NewService(&serviceRepositoryStub{})
	_, _, err := service.SubmitAssistedGroup(context.Background(), GroupCommand{
		Actor:                access.NewPrincipal("https://issuer.invalid", "operator", nil),
		StayID:               uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		ClientSubmissionID:   uuid.MustParse("019f0000-0000-7000-8000-000000000002"),
		PrivacyNoticeVersion: "v1",
		Visitors: []Visitor{{
			ClientID:         "019f0000-0000-7000-8000-000000000003",
			Role:             VisitorCompanion,
			AgeBand:          Age25To34,
			ResidenceCountry: "AR",
		}},
		ExpectedVersion: 1,
		IdempotencyKey:  "group-submit-key-1234",
		RequestID:       "request-12345678",
	})

	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("SubmitAssistedGroup() error = %v, want ErrInvalidInput", err)
	}
}
