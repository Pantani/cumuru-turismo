package idempotency

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidKey      = errors.New("invalid idempotency key")
	ErrInvalidIdentity = errors.New("invalid idempotency identity")
	ErrInvalidResponse = errors.New("invalid stored response")
	ErrProcessing      = errors.New("idempotency request processing")
	keyPattern         = regexp.MustCompile(`^[A-Za-z0-9._:-]{16,128}$`)
	versionPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	etagPattern        = regexp.MustCompile(`^"[1-9][0-9]*"$`)
)

type ProcessingError struct {
	RetryAfter time.Duration
}

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

type Digest struct {
	Version string
	Sum     []byte
}

type KeyHasher struct {
	version string
	key     []byte
}

func NewKeyHasher(version string, key []byte) (*KeyHasher, error) {
	if !versionPattern.MatchString(version) || len(key) < 32 {
		return nil, ErrInvalidKey
	}
	return &KeyHasher{version: version, key: append([]byte(nil), key...)}, nil
}

func (h *KeyHasher) Hash(value string) (Digest, error) {
	if !keyPattern.MatchString(value) {
		return Digest{}, ErrInvalidKey
	}
	mac := hmac.New(sha256.New, h.key)
	_, _ = mac.Write([]byte("idempotency-key\x00"))
	_, _ = mac.Write([]byte(value))
	return Digest{Version: h.version, Sum: mac.Sum(nil)}, nil
}

func RequestHash(value any) ([sha256.Size]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(encoded), nil
}

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

type Identity struct {
	Actor      Digest
	Key        Digest
	Operation  Operation
	ResourceID uuid.UUID
}

func (i Identity) Validate() error {
	if !validDigest(i.Actor) || !validDigest(i.Key) {
		return ErrInvalidIdentity
	}
	if !validOperations[i.Operation] || i.ResourceID == uuid.Nil {
		return ErrInvalidIdentity
	}
	return nil
}

func validDigest(value Digest) bool {
	return versionPattern.MatchString(value.Version) && len(value.Sum) == sha256.Size
}

type StoredResponse struct {
	Status      int
	ContentType string
	Location    string
	ETag        string
	ResourceID  uuid.UUID
	Body        []byte
}

func NewStoredResponse(
	status int,
	contentType string,
	location string,
	etag string,
	resourceID uuid.UUID,
	body []byte,
) (StoredResponse, error) {
	if status < 200 || status > 299 || contentType != "application/json" {
		return StoredResponse{}, ErrInvalidResponse
	}
	if etag != "" && !etagPattern.MatchString(etag) {
		return StoredResponse{}, ErrInvalidResponse
	}
	if resourceID == uuid.Nil {
		return StoredResponse{}, ErrInvalidResponse
	}
	return StoredResponse{
		Status: status, ContentType: contentType, Location: location,
		ETag: etag, ResourceID: resourceID, Body: append([]byte(nil), body...),
	}, nil
}
