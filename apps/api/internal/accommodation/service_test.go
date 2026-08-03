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
	createCalls      int
	onboardingCalls  int
	onboardingResult Accommodation
	onboardingReplay bool
	onboardingErr    error
	onboardingInput  CreateCommand
}

func (r *repositoryStub) CreateMembership(context.Context, CreateMembershipCommand) (MembershipCreated, bool, error) {
	r.createCalls++
	return MembershipCreated{}, false, nil
}

func (r *repositoryStub) Create(
	_ context.Context,
	command CreateCommand,
) (Accommodation, bool, error) {
	r.onboardingCalls++
	r.onboardingInput = command
	return r.onboardingResult, r.onboardingReplay, r.onboardingErr
}

func TestServiceCreatesAccommodationWithCanonicalCategory(t *testing.T) {
	t.Parallel()

	want := Accommodation{
		ID:             uuid.MustParse("019f0000-0000-7000-8000-000000000011"),
		OrganizationID: uuid.MustParse("019f0000-0000-7000-8000-000000000012"),
		Name:           "Casa fictícia",
		Category:       CategoryFamilyHosting,
		Status:         StatusActive,
		Version:        1,
	}
	repository := &repositoryStub{onboardingResult: want}
	service := NewService(repository)
	command := CreateCommand{
		Actor:              access.NewPrincipal("https://issuer.invalid", "host", nil),
		Name:               "Casa fictícia",
		Category:           CategoryFamilyHosting,
		Capacity:           6,
		ClientSubmissionID: uuid.MustParse("019f0000-0000-7000-8000-000000000013"),
		IdempotencyKey:     "accommodation-key-1234",
		RequestID:          "request-12345678",
	}

	got, replayed, err := service.Create(context.Background(), command)

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if replayed {
		t.Fatal("Create() replayed = true, want false")
	}
	if got != want {
		t.Fatalf("Create() = %+v, want %+v", got, want)
	}
	if repository.onboardingCalls != 1 ||
		repository.onboardingInput.ClientSubmissionID != command.ClientSubmissionID ||
		repository.onboardingInput.Category != command.Category ||
		repository.onboardingInput.Name != command.Name {
		t.Fatalf("repository input = %+v; calls = %d", repository.onboardingInput, repository.onboardingCalls)
	}
}

func TestServiceRejectsUnsafeAccommodationOnboarding(t *testing.T) {
	t.Parallel()

	valid := CreateCommand{
		Actor:              access.NewPrincipal("https://issuer.invalid", "host", nil),
		Name:               "Hospedagem fictícia",
		Category:           CategorySeasonalRental,
		Capacity:           4,
		ClientSubmissionID: uuid.MustParse("019f0000-0000-7000-8000-000000000013"),
		IdempotencyKey:     "accommodation-key-1234",
		RequestID:          "request-12345678",
	}
	tests := []struct {
		name   string
		mutate func(*CreateCommand)
	}{
		{name: "unclassified input", mutate: func(command *CreateCommand) {
			command.Category = CategoryUnclassified
		}},
		{name: "legacy free category", mutate: func(command *CreateCommand) {
			command.Category = Category("casa-com-cpf-123")
		}},
		{name: "uuid v4 submission", mutate: func(command *CreateCommand) {
			command.ClientSubmissionID = uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
		}},
		{name: "blank name", mutate: func(command *CreateCommand) {
			command.Name = "   "
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			command := valid
			tt.mutate(&command)
			repository := &repositoryStub{}
			_, _, err := NewService(repository).Create(context.Background(), command)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Create() error = %v, want ErrInvalidInput", err)
			}
			if repository.onboardingCalls != 0 {
				t.Fatalf("repository calls = %d, want 0", repository.onboardingCalls)
			}
		})
	}
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
