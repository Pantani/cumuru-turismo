// Package idempotency holds the vocabulary a replayable mutation is described
// with: which operation is being retried, how a request body is reduced to a
// comparable hash, and how a still-running attempt is reported to the caller.
//
// Key derivation and response storage deliberately live in the store instead:
// the store probes every key version under one transaction, which the caller
// cannot do from here without reproducing the transaction itself.
package idempotency

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrInvalidKey = errors.New("invalid idempotency key")
	ErrProcessing = errors.New("idempotency request processing")
)

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

// RequestHash keys the digest of the request body. The key is not optional and
// there is no unkeyed variant: the same digest is persisted in
// platform.idempotency_records and core.group_submissions, and an unkeyed
// SHA-256 over a body would let anyone holding the dump confirm a guess about
// data the rejection or the expiry already erased. The erasure has to survive
// the digest, otherwise it is fiction (ADR-040).
//
// Encoding a struct rather than free-form input is what makes the comparison
// stable: encoding/json orders struct fields by declaration and map keys by sort
// order, so two semantically equal requests always hash alike.
func RequestHash(key []byte, value any) ([sha256.Size]byte, error) {
	if len(key) < 32 {
		return [sha256.Size]byte{}, ErrInvalidKey
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("idempotency-request\x00"))
	_, _ = mac.Write(encoded)
	var digest [sha256.Size]byte
	copy(digest[:], mac.Sum(nil))
	return digest, nil
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
	OperationCreateAccommodationInvite   Operation = "createAccommodationInvite"
	OperationRevokeAccommodationInvite   Operation = "revokeAccommodationInvite"
	OperationSubmitSelfRegistration      Operation = "submitAccommodationSelfRegistration"
	OperationApproveStay                 Operation = "approveStay"
	OperationRejectStay                  Operation = "rejectStay"
	OperationCreateActivation            Operation = "createAccommodationActivation"
	OperationCreateAccessRequest         Operation = "createAccommodationAccessRequest"
	OperationApproveAccessRequest        Operation = "approveAccommodationAccessRequest"
	OperationRejectAccessRequest         Operation = "rejectAccommodationAccessRequest"
	OperationCreateCalendarFeed          Operation = "createCalendarFeed"
	OperationRemoveCalendarFeed          Operation = "removeCalendarFeed"
	OperationConfirmCalendarReservation  Operation = "confirmCalendarReservation"
	OperationDismissCalendarReservation  Operation = "dismissCalendarReservation"
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
	OperationCreateAccommodationInvite:   true,
	OperationRevokeAccommodationInvite:   true,
	OperationSubmitSelfRegistration:      true,
	OperationApproveStay:                 true,
	OperationRejectStay:                  true,
	OperationCreateActivation:            true,
	OperationCreateAccessRequest:         true,
	OperationApproveAccessRequest:        true,
	OperationRejectAccessRequest:         true,
	OperationCreateCalendarFeed:          true,
	OperationRemoveCalendarFeed:          true,
	OperationConfirmCalendarReservation:  true,
	OperationDismissCalendarReservation:  true,
}

// Valid reports whether the operation may be recorded against an idempotency
// key. The store checks it before claiming one, so an unregistered operation
// fails the mutation instead of persisting a row nothing can ever replay.
func (o Operation) Valid() bool {
	return validOperations[o]
}
