package idempotency_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/idempotency"
	"github.com/google/uuid"
)

func TestKeyHasherValidatesAndNeverReturnsRawKey(t *testing.T) {
	t.Parallel()

	hasher, err := idempotency.NewKeyHasher(
		"idem-v1",
		[]byte("idempotency-key-material-is-at-least-32-bytes"),
	)
	if err != nil {
		t.Fatalf("NewKeyHasher() error = %v", err)
	}
	const raw = "offline.019f0000-0000-7000-8000-000000000001"
	digest, err := hasher.Hash(raw)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if digest.Version != "idem-v1" || len(digest.Sum) != 32 || bytes.Contains(digest.Sum, []byte(raw)) {
		t.Fatalf("Hash() returned unsafe digest: %#v", digest)
	}
	if _, err := hasher.Hash("short"); !errors.Is(err, idempotency.ErrInvalidKey) {
		t.Fatalf("Hash(short) error = %v", err)
	}
}

func TestRequestHashUsesCanonicalTypedJSON(t *testing.T) {
	t.Parallel()

	type request struct {
		AccommodationID string         `json:"accommodation_id"`
		Labels          map[string]int `json:"labels"`
	}
	first := request{
		AccommodationID: "019f0000-0000-7000-8000-000000000001",
		Labels:          map[string]int{"b": 2, "a": 1},
	}
	second := request{
		AccommodationID: first.AccommodationID,
		Labels:          map[string]int{"a": 1, "b": 2},
	}
	firstHash, err := idempotency.RequestHash(first)
	if err != nil {
		t.Fatalf("RequestHash(first) error = %v", err)
	}
	secondHash, _ := idempotency.RequestHash(second)
	if firstHash != secondHash {
		t.Fatal("semantic-equivalent typed requests produced different hashes")
	}
}

func TestIdentitySeparatesOperationResourceAndActor(t *testing.T) {
	t.Parallel()

	resourceID := uuid.MustParse("019f0000-0000-7000-8000-000000000021")
	identity := idempotency.Identity{
		Actor:      idempotency.Digest{Version: "actor-v1", Sum: bytes.Repeat([]byte{1}, 32)},
		Key:        idempotency.Digest{Version: "idem-v1", Sum: bytes.Repeat([]byte{2}, 32)},
		Operation:  idempotency.OperationCreateStay,
		ResourceID: resourceID,
	}
	if err := identity.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	identity.Operation = idempotency.Operation("future-operation")
	if err := identity.Validate(); !errors.Is(err, idempotency.ErrInvalidIdentity) {
		t.Fatalf("Validate(future operation) error = %v", err)
	}
}

func TestPhase3OperationsAreExplicitlyAllowed(t *testing.T) {
	t.Parallel()
	operations := []idempotency.Operation{
		idempotency.OperationCreateQuestionnaire,
		idempotency.OperationCloneQuestionnaire,
		idempotency.OperationSubmitQuestionnaireReview,
		idempotency.OperationRequestQuestionnaireChanges,
		idempotency.OperationApproveQuestionnaire,
		idempotency.OperationPublishQuestionnaire,
		idempotency.OperationRetireQuestionnaire,
		idempotency.OperationSubmitSurveyResponse,
	}
	for _, operation := range operations {
		identity := idempotency.Identity{
			Actor:     idempotency.Digest{Version: "actor-v1", Sum: bytes.Repeat([]byte{1}, 32)},
			Key:       idempotency.Digest{Version: "idem-v1", Sum: bytes.Repeat([]byte{2}, 32)},
			Operation: operation, ResourceID: uuid.New(),
		}
		if err := identity.Validate(); err != nil {
			t.Fatalf("operation %s rejected: %v", operation, err)
		}
	}
}

func TestAccommodationOnboardingOperationIsExplicitlyAllowed(t *testing.T) {
	t.Parallel()
	identity := idempotency.Identity{
		Actor:      idempotency.Digest{Version: "actor-v1", Sum: bytes.Repeat([]byte{1}, 32)},
		Key:        idempotency.Digest{Version: "idem-v1", Sum: bytes.Repeat([]byte{2}, 32)},
		Operation:  idempotency.OperationCreateAccommodation,
		ResourceID: uuid.MustParse("019f0000-0000-7000-8000-000000000021"),
	}
	if err := identity.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestStoredResponseAllowsOnlyReplayHeadersAndCopiesBody(t *testing.T) {
	t.Parallel()

	body := []byte(`{"id":"019f0000-0000-7000-8000-000000000021"}`)
	response, err := idempotency.NewStoredResponse(
		201,
		"application/json",
		"/api/v1/stays/019f0000-0000-7000-8000-000000000021",
		`"1"`,
		uuid.MustParse("019f0000-0000-7000-8000-000000000021"),
		body,
	)
	if err != nil {
		t.Fatalf("NewStoredResponse() error = %v", err)
	}
	body[0] = 'x'
	if response.Body[0] == 'x' {
		t.Fatal("NewStoredResponse() retained caller body storage")
	}
	if _, err := idempotency.NewStoredResponse(500, "application/json", "", "", uuid.Nil, nil); err == nil {
		t.Fatal("NewStoredResponse(500) error = nil")
	}
}

func TestProcessingErrorCarriesRetryAfter(t *testing.T) {
	t.Parallel()

	err := idempotency.NewProcessingError(3 * time.Second)
	var processing *idempotency.ProcessingError
	if !errors.Is(err, idempotency.ErrProcessing) || !errors.As(err, &processing) {
		t.Fatalf("NewProcessingError() = %v, want typed ErrProcessing", err)
	}
	if processing.RetryAfter != 3*time.Second {
		t.Fatalf("RetryAfter = %s", processing.RetryAfter)
	}
}
