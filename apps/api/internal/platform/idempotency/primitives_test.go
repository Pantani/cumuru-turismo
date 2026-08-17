package idempotency_test

import (
	"errors"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/idempotency"
)

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

// A request that cannot be encoded has no comparable hash, so it must fail
// rather than collapse to the hash of an empty document.
func TestRequestHashRejectsUnencodableRequest(t *testing.T) {
	t.Parallel()

	if _, err := idempotency.RequestHash(make(chan int)); err == nil {
		t.Fatal("RequestHash(chan) error = nil, want a failure")
	}
}

func TestReplayableOperationsAreExplicitlyAllowed(t *testing.T) {
	t.Parallel()

	operations := []idempotency.Operation{
		idempotency.OperationCreateAccommodation,
		idempotency.OperationCreateMembership,
		idempotency.OperationCreateStay,
		idempotency.OperationSubmitAssistedGroup,
		idempotency.OperationCreateInvite,
		idempotency.OperationCheckIn,
		idempotency.OperationCheckOut,
		idempotency.OperationCancel,
		idempotency.OperationNoShow,
		idempotency.OperationSubmitInviteGroup,
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
		if !operation.Valid() {
			t.Fatalf("operation %s rejected", operation)
		}
	}
}

func TestUnregisteredOperationIsNotReplayable(t *testing.T) {
	t.Parallel()

	for _, operation := range []idempotency.Operation{"", "future-operation", "CreateStay"} {
		if operation.Valid() {
			t.Fatalf("operation %q accepted", operation)
		}
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

// A Retry-After below one second would invite an immediate retry against an
// attempt that is still running.
func TestProcessingErrorFloorsRetryAfterAtOneSecond(t *testing.T) {
	t.Parallel()

	for _, given := range []time.Duration{-time.Hour, 0, time.Millisecond} {
		var processing *idempotency.ProcessingError
		if !errors.As(idempotency.NewProcessingError(given), &processing) {
			t.Fatalf("NewProcessingError(%s) is not a *ProcessingError", given)
		}
		if processing.RetryAfter != time.Second {
			t.Fatalf("RetryAfter = %s, want 1s", processing.RetryAfter)
		}
	}
}
