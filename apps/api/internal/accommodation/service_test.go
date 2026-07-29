package accommodation

import (
	"context"
	"errors"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/google/uuid"
)

type repositoryStub struct {
	Repository
	createCalls int
}

func (r *repositoryStub) CreateMembership(context.Context, CreateMembershipCommand) (MembershipCreated, bool, error) {
	r.createCalls++
	return MembershipCreated{}, false, nil
}

func TestServiceRejectsInvalidMembershipBeforeRepository(t *testing.T) {
	t.Parallel()

	repository := &repositoryStub{}
	service := NewService(repository)
	_, _, err := service.CreateMembership(context.Background(), CreateMembershipCommand{
		Actor:           access.NewPrincipal("https://issuer.invalid", "manager", nil),
		AccommodationID: uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		TargetIssuer:    "https://issuer.invalid",
		TargetSubject:   "operator",
		Role:            Role("owner"),
		IdempotencyKey:  "membership-key-1234",
		RequestID:       "request-12345678",
	})

	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreateMembership() error = %v, want ErrInvalidInput", err)
	}
	if repository.createCalls != 0 {
		t.Fatalf("repository calls = %d, want 0", repository.createCalls)
	}
}

func TestServiceRejectsEmptyAccommodationPatch(t *testing.T) {
	t.Parallel()

	service := NewService(&repositoryStub{})
	_, err := service.Update(context.Background(), UpdateCommand{
		Actor:           access.NewPrincipal("https://issuer.invalid", "manager", nil),
		AccommodationID: uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		ExpectedVersion: 1,
		RequestID:       "request-12345678",
	})

	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Update() error = %v, want ErrInvalidInput", err)
	}
}
