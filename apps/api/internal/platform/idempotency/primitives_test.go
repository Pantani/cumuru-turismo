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
	key := []byte("request-hash-key-material-is-at-least-32-bytes")
	firstHash, err := idempotency.RequestHash(key, first)
	if err != nil {
		t.Fatalf("RequestHash(first) error = %v", err)
	}
	secondHash, _ := idempotency.RequestHash(key, second)
	if firstHash != secondHash {
		t.Fatal("semantic-equivalent typed requests produced different hashes")
	}
}

// Without a key the digest is an oracle: whoever holds the dump recomputes it
// from a candidate body and confirms a guess about data the rejection already
// erased. There is no unkeyed variant to fall back to, and a short key is
// refused rather than silently accepted.
func TestRequestHashRefusesToDigestWithoutAKey(t *testing.T) {
	t.Parallel()

	body := struct {
		Value string `json:"value"`
	}{"canary"}
	for _, key := range [][]byte{nil, {}, []byte("too-short")} {
		if _, err := idempotency.RequestHash(key, body); !errors.Is(err, idempotency.ErrInvalidKey) {
			t.Fatalf("RequestHash(%d-byte key) error = %v, want ErrInvalidKey", len(key), err)
		}
	}
}

// Two keys over the same body must not agree, otherwise the key would not be
// contributing anything.
func TestRequestHashBindsTheKey(t *testing.T) {
	t.Parallel()

	body := struct {
		Value string `json:"value"`
	}{"canary"}
	first, err := idempotency.RequestHash([]byte("first-request-hash-key-is-at-least-32-bytes"), body)
	if err != nil {
		t.Fatalf("RequestHash() error = %v", err)
	}
	second, err := idempotency.RequestHash([]byte("second-request-hash-key-is-at-least-32-bytes"), body)
	if err != nil {
		t.Fatalf("RequestHash() error = %v", err)
	}
	if first == second {
		t.Fatal("the key did not change the digest")
	}
}

// A request that cannot be encoded has no comparable hash, so it must fail
// rather than collapse to the hash of an empty document.
func TestRequestHashRejectsUnencodableRequest(t *testing.T) {
	t.Parallel()

	key := []byte("request-hash-key-material-is-at-least-32-bytes")
	if _, err := idempotency.RequestHash(key, make(chan int)); err == nil {
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
