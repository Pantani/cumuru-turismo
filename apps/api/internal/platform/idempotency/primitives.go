// Package idempotency holds the vocabulary a replayable mutation is described
// with: which operation is being retried, how a request body is reduced to a
// comparable hash, and how a still-running attempt is reported to the caller.
//
// Keying and response storage deliberately live in the store instead: the store
// probes every key version under one transaction, which the caller cannot do
// from here without reproducing the transaction itself.
package idempotency

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"
)

var ErrProcessing = errors.New("idempotency request processing")

// ProcessingError reports that an identical request is still in flight, and how
// long the caller should wait before retrying it.
type ProcessingError struct {
	RetryAfter time.Duration
}

// NewProcessingError floors the delay at one second: a Retry-After of zero would
// invite an immediate retry against an attempt that is still running.
func NewProcessingError(retryAfter time.Duration) error {
	if retryAfter < time.Second {
		retryAfter = time.Second
	}
	return &ProcessingError{RetryAfter: retryAfter}
}

func (e *ProcessingError) Error() string {
	return ErrProcessing.Error()
}

func (e *ProcessingError) Unwrap() error {
	return ErrProcessing
}

// RequestHash reduces a typed request to the value a replay is compared against.
// Encoding a struct rather than free-form input is what makes the comparison
// stable: encoding/json orders struct fields by declaration and map keys by sort
// order, so two semantically equal requests always hash alike.
func RequestHash(value any) ([sha256.Size]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(encoded), nil
}

// Operation names the mutation a key replays. It is part of the stored identity,
// so a key reused across two different operations never collides.
type Operation string

const (
	OperationCreateAccommodation         Operation = "createAccommodation"
	OperationCreateMembership            Operation = "createAccommodationMembership"
	OperationCreateStay                  Operation = "createStay"
	OperationSubmitAssistedGroup         Operation = "submitAssistedStayGroup"
	OperationCreateInvite                Operation = "createStayInvite"
	OperationCheckIn                     Operation = "checkInStay"
	OperationCheckOut                    Operation = "checkOutStay"
	OperationCancel                      Operation = "cancelStay"
	OperationNoShow                      Operation = "markStayNoShow"
	OperationSubmitInviteGroup           Operation = "submitInviteGroup"
	OperationCreateQuestionnaire         Operation = "createQuestionnaire"
	OperationCloneQuestionnaire          Operation = "cloneQuestionnaireVersion"
	OperationSubmitQuestionnaireReview   Operation = "submitQuestionnaireVersionReview"
	OperationRequestQuestionnaireChanges Operation = "requestQuestionnaireVersionChanges"
	OperationApproveQuestionnaire        Operation = "approveQuestionnaireVersion"
	OperationPublishQuestionnaire        Operation = "publishQuestionnaireVersion"
	OperationRetireQuestionnaire         Operation = "retireQuestionnaireVersion"
	OperationSubmitSurveyResponse        Operation = "submitSurveyResponse"
)

// The allow-list is explicit rather than derived, so adding a constant without
// deciding that it may be replayed does not silently make it replayable.
var validOperations = map[Operation]bool{
	OperationCreateAccommodation:         true,
	OperationCreateMembership:            true,
	OperationCreateStay:                  true,
	OperationSubmitAssistedGroup:         true,
	OperationCreateInvite:                true,
	OperationCheckIn:                     true,
	OperationCheckOut:                    true,
	OperationCancel:                      true,
	OperationNoShow:                      true,
	OperationSubmitInviteGroup:           true,
	OperationCreateQuestionnaire:         true,
	OperationCloneQuestionnaire:          true,
	OperationSubmitQuestionnaireReview:   true,
	OperationRequestQuestionnaireChanges: true,
	OperationApproveQuestionnaire:        true,
	OperationPublishQuestionnaire:        true,
	OperationRetireQuestionnaire:         true,
	OperationSubmitSurveyResponse:        true,
}

// Valid reports whether the operation may be recorded against an idempotency
// key. The store checks it before claiming one, so an unregistered operation
// fails the mutation instead of persisting a row nothing can ever replay.
func (o Operation) Valid() bool {
	return validOperations[o]
}
