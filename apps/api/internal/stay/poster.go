package stay

import (
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/google/uuid"
)

// AccommodationInviteCommand issues or rotates the reusable poster. Rotation is
// not a key-version bump on the same row: it revokes the active invite and
// mints a new invite_id, therefore a new token. Bumping only the key version
// would leave the old poster working for as long as the historical key stayed
// in the ring, because the verifier reads the version from the stored row —
// the rotation would be silently ineffective (T-05, N-14).
type AccommodationInviteCommand struct {
	Actor                access.Principal
	AccommodationID      uuid.UUID
	PrivacyNoticeVersion string
	// MaxUses nil means unlimited, which is the whole point of the poster and
	// exists only in this purpose: invites_target_valid keeps the stay invite
	// with a mandatory limit.
	MaxUses         *int32
	ExpectedVersion int64
	IdempotencyKey  string
	RequestID       string
}

type AccommodationInviteRevokeCommand struct {
	Actor           access.Principal
	AccommodationID uuid.UUID
	ExpectedVersion int64
	IdempotencyKey  string
	RequestID       string
}

// AccommodationInviteCreated is the only place the URL ever appears, alongside
// the exact idempotent replay. Nothing reads it back later.
type AccommodationInviteCreated struct {
	InviteID  uuid.UUID `json:"invite_id"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
	MaxUses   *int32    `json:"max_uses"`
	UseCount  int32     `json:"use_count"`
	Version   int64     `json:"-"`
}

// AccommodationInviteStatus carries no token and no URL: the capability is not
// reconstructible from a read (ADR-019).
type AccommodationInviteStatus struct {
	InviteID  uuid.UUID  `json:"invite_id"`
	ExpiresAt time.Time  `json:"expires_at"`
	MaxUses   *int32     `json:"max_uses"`
	UseCount  int32      `json:"use_count"`
	RevokedAt *time.Time `json:"revoked_at"`
	Version   int64      `json:"-"`
}

// ProofOfWorkChallenge travels with the poster context because that route
// already owns a rate limit bucket, and the bucket counter is the source of the
// adaptive difficulty. A separate issuing endpoint would add a scope no ADR
// asks for and a second thing to rate limit.
type ProofOfWorkChallenge struct {
	Algorithm      string    `json:"algorithm"`
	Challenge      string    `json:"challenge"`
	DifficultyBits int       `json:"difficulty_bits"`
	ExpiresAt      time.Time `json:"expires_at"`
}

// AccommodationInviteContext is the minimum the open form needs. There is no
// accommodation identifier, no operator and no counter: a poster on a wall is
// readable by anyone, so its context must disclose nothing an outsider does not
// already see printed on it.
type AccommodationInviteContext struct {
	AccommodationName    string               `json:"accommodation_name"`
	PrivacyNoticeVersion string               `json:"privacy_notice_version"`
	ProofOfWork          ProofOfWorkChallenge `json:"proof_of_work"`
}

// SelfRegistrationAccepted echoes identifiers and states, never a submitted
// value. The body is stored verbatim in platform.idempotency_records for the
// replay, so anything echoed here would survive the purge (E-02).
type SelfRegistrationAccepted struct {
	SubmissionID  uuid.UUID     `json:"submission_id"`
	StayID        uuid.UUID     `json:"stay_id"`
	Status        string        `json:"status"`
	StayStatus    Status        `json:"stay_status"`
	ApprovalState ApprovalState `json:"approval_state"`
	Version       int64         `json:"-"`
}

func ValidateAccommodationInvite(command AccommodationInviteCommand) error {
	if !validPosterTarget(command) || !validMaxUses(command.MaxUses) {
		return ErrInvalidInput
	}
	return validPosterMeta(command.ExpectedVersion, command.IdempotencyKey, command.RequestID)
}

func validPosterTarget(command AccommodationInviteCommand) bool {
	return validActor(command.Actor) &&
		command.AccommodationID != uuid.Nil &&
		validNoticeVersion(command.PrivacyNoticeVersion)
}

func validPosterMeta(expectedVersion int64, idempotencyKey, requestID string) error {
	if expectedVersion < 1 || !validMutationMeta(idempotencyKey, requestID) {
		return ErrInvalidInput
	}
	return nil
}

// A limit of zero would make the poster born consumed, and the CHECK refuses it
// anyway; refusing here keeps the answer a 422 instead of a 500.
func validMaxUses(value *int32) bool {
	return value == nil || (*value >= 1 && *value <= 100000)
}

func ValidateAccommodationInviteRevoke(command AccommodationInviteRevokeCommand) error {
	if !validActor(command.Actor) || command.AccommodationID == uuid.Nil {
		return ErrInvalidInput
	}
	return validPosterMeta(command.ExpectedVersion, command.IdempotencyKey, command.RequestID)
}
