package accommodation

import (
	"context"
	"errors"
	"strings"
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
	updateCalls      int
	updateInput      UpdateCommand
	updateResult     Accommodation
}

func (r *repositoryStub) Update(
	_ context.Context,
	command UpdateCommand,
) (Accommodation, error) {
	r.updateCalls++
	r.updateInput = command
	return r.updateResult, nil
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
		Actor: access.NewPrincipal(
			"https://issuer.invalid",
			"host",
			[]string{"accommodations:onboard"},
		),
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
	if repository.onboardingCalls != 1 {
		t.Fatalf("repository input = %+v; calls = %d", repository.onboardingInput, repository.onboardingCalls)
	}
	assertCreateCommandEqual(t, repository.onboardingInput, command)
}

func assertCreateCommandEqual(t *testing.T, got, want CreateCommand) {
	t.Helper()
	if comparableCreateCommand(got) != comparableCreateCommand(want) ||
		got.Actor.HasScope("accommodations:onboard") !=
			want.Actor.HasScope("accommodations:onboard") {
		t.Fatalf("repository input = %+v, want %+v", got, want)
	}
}

type createCommandComparison struct {
	actorIssuer        string
	actorSubject       string
	name               string
	category           Category
	capacity           int32
	clientSubmissionID uuid.UUID
	idempotencyKey     string
	requestID          string
}

func comparableCreateCommand(command CreateCommand) createCommandComparison {
	return createCommandComparison{
		actorIssuer:        command.Actor.Issuer,
		actorSubject:       command.Actor.Subject,
		name:               command.Name,
		category:           command.Category,
		capacity:           command.Capacity,
		clientSubmissionID: command.ClientSubmissionID,
		idempotencyKey:     command.IdempotencyKey,
		requestID:          command.RequestID,
	}
}

func TestServiceCountsAccommodationNameLimitInUnicodeCharacters(t *testing.T) {
	t.Parallel()

	repository := &repositoryStub{}
	command := validCreateCommand()
	command.Name = strings.Repeat("á", 200)
	_, _, err := NewService(repository).Create(context.Background(), command)
	if err != nil {
		t.Fatalf("Create() Unicode boundary error = %v", err)
	}
	if repository.onboardingCalls != 1 {
		t.Fatalf("repository calls = %d, want 1", repository.onboardingCalls)
	}

	command.Name += "á"
	_, _, err = NewService(repository).Create(context.Background(), command)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Create() over Unicode boundary error = %v, want ErrInvalidInput", err)
	}
}

func TestServiceCountsAccommodationPatchLimitInUnicodeCharacters(t *testing.T) {
	t.Parallel()

	repository := &repositoryStub{}
	command := validUpdateCommand()
	command.Patch = UpdatePatch{SetName: true, Name: strings.Repeat("á", 200)}
	if _, err := NewService(repository).Update(context.Background(), command); err != nil {
		t.Fatalf("Update() Unicode name boundary error = %v", err)
	}
	if repository.updateCalls != 1 {
		t.Fatalf("repository calls = %d, want 1", repository.updateCalls)
	}

	command.Patch.Name += "á"
	if _, err := NewService(repository).Update(context.Background(), command); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Update() over Unicode name boundary error = %v, want ErrInvalidInput", err)
	}
}

func TestServiceCountsNullablePublicAreaLimitInUnicodeCharacters(t *testing.T) {
	t.Parallel()

	repository := &repositoryStub{}
	command := validUpdateCommand()
	publicArea := strings.Repeat("ç", 100)
	command.Patch = UpdatePatch{SetPublicAreaCode: true, PublicAreaCode: &publicArea}
	if _, err := NewService(repository).Update(context.Background(), command); err != nil {
		t.Fatalf("Update() Unicode public area boundary error = %v", err)
	}

	publicArea += "ç"
	if _, err := NewService(repository).Update(context.Background(), command); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Update() over Unicode public area boundary error = %v, want ErrInvalidInput", err)
	}
}

func validCreateCommand() CreateCommand {
	return CreateCommand{
		Actor:              access.NewPrincipal("https://issuer.invalid", "host", nil),
		Name:               "Hospedagem fictícia",
		Category:           CategorySeasonalRental,
		Capacity:           4,
		ClientSubmissionID: uuid.MustParse("019f0000-0000-7000-8000-000000000013"),
		IdempotencyKey:     "accommodation-key-1234",
		RequestID:          "request-12345678",
	}
}

func validUpdateCommand() UpdateCommand {
	return UpdateCommand{
		Actor:           access.NewPrincipal("https://issuer.invalid", "manager", nil),
		AccommodationID: uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		ExpectedVersion: 1,
		RequestID:       "request-12345678",
	}
}

func TestServiceRejectsUnsafeAccommodationOnboarding(t *testing.T) {
	t.Parallel()

	valid := validCreateCommand()
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

// N-25: approval is its own operation. An operator holding update_stay must not
// inherit it, and neither must a suspended or closed accommodation.
func TestApprovalIsAnOperationOfItsOwnAndOnlyWhenActive(t *testing.T) {
	t.Parallel()

	if OperationApproveStay == OperationUpdateStay {
		t.Fatal("approval reuses the edit permission")
	}
	allowed := map[Status]bool{
		StatusActive:        true,
		StatusPendingReview: false,
		StatusSuspended:     false,
		StatusClosed:        false,
	}
	for status, want := range allowed {
		got := status.Allows(OperationApproveStay)
		if got != want {
			t.Fatalf("%s allows approve_stay = %t, want %t", status, got, want)
		}
		if issue := status.Allows(OperationIssueActivation); issue != want {
			t.Fatalf("%s allows issue_activation = %t, want %t", status, issue, want)
		}
	}
	// The edit permission must stay available where approval is not, otherwise
	// the test would pass for the wrong reason.
	if !StatusActive.Allows(OperationUpdateStay) {
		t.Fatal("update_stay disappeared from the active accommodation")
	}
}
