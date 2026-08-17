package stay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"regexp"

	"github.com/google/uuid"
)

const inviteCapabilitySize = 16 + sha256.Size

// CapabilityPurpose is the domain separator inside the MAC. The ADR-019 always
// promised HMAC(key_version, purpose ‖ invite_id), but the code pinned the
// constant and Verify checked no purpose at all. With a single purpose that was
// harmless; with three it is what keeps a poster token from being replayed as
// an activation link, and the other way round.
type CapabilityPurpose string

const (
	PurposeStayGroupSubmission           CapabilityPurpose = "stay_group_submission"
	PurposeAccommodationSelfRegistration CapabilityPurpose = "accommodation_self_registration"
	PurposeAccommodationActivation       CapabilityPurpose = "accommodation_activation"
)

var capabilityPurposes = map[CapabilityPurpose]bool{
	PurposeStayGroupSubmission:           true,
	PurposeAccommodationSelfRegistration: true,
	PurposeAccommodationActivation:       true,
}

func (p CapabilityPurpose) Valid() bool {
	return capabilityPurposes[p]
}

var (
	ErrInvalidInviteToken = errors.New("invalid invite token")
	inviteKeyVersion      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
)

type InviteKeyring struct {
	CurrentVersion string
	Keys           map[string][]byte
}

type InviteCodec struct {
	currentVersion string
	keys           map[string][]byte
}

func NewInviteCodec(keyring InviteKeyring) (*InviteCodec, error) {
	if !inviteKeyVersion.MatchString(keyring.CurrentVersion) {
		return nil, ErrInvalidInviteToken
	}
	keys, err := cloneInviteKeys(keyring.Keys)
	if err != nil {
		return nil, err
	}
	if _, ok := keys[keyring.CurrentVersion]; !ok {
		return nil, ErrInvalidInviteToken
	}
	return &InviteCodec{currentVersion: keyring.CurrentVersion, keys: keys}, nil
}

func cloneInviteKeys(source map[string][]byte) (map[string][]byte, error) {
	keys := make(map[string][]byte, len(source))
	for version, key := range source {
		if !inviteKeyVersion.MatchString(version) || len(key) < 32 {
			return nil, ErrInvalidInviteToken
		}
		keys[version] = append([]byte(nil), key...)
	}
	return keys, nil
}

func (c *InviteCodec) Issue(
	purpose CapabilityPurpose,
	inviteID uuid.UUID,
) (string, string, error) {
	token, err := c.Reconstruct(purpose, inviteID, c.currentVersion)
	return token, c.currentVersion, err
}

func (c *InviteCodec) Reconstruct(
	purpose CapabilityPurpose,
	inviteID uuid.UUID,
	version string,
) (string, error) {
	key, ok := c.keys[version]
	if !ok || inviteID == uuid.Nil || !purpose.Valid() {
		return "", ErrInvalidInviteToken
	}
	payload := append([]byte(nil), inviteID[:]...)
	capability := append(payload, inviteMAC(key, purpose, inviteID[:])...)
	return base64.RawURLEncoding.EncodeToString(capability), nil
}

// Verify recomputes the MAC under the purpose the caller expects, so a token
// minted for another purpose fails here rather than resolving to an identifier
// the caller would then look up in the wrong table.
func (c *InviteCodec) Verify(
	purpose CapabilityPurpose,
	token string,
	version string,
) (uuid.UUID, error) {
	key, ok := c.keys[version]
	if !ok || !purpose.Valid() {
		return uuid.Nil, ErrInvalidInviteToken
	}
	payload, err := authenticatedPayload(token, key, purpose)
	if err != nil {
		return uuid.Nil, err
	}
	return inviteIdentifier(payload)
}

func inviteIdentifier(payload []byte) (uuid.UUID, error) {
	inviteID, err := uuid.FromBytes(payload)
	if err != nil || inviteID == uuid.Nil {
		return uuid.Nil, ErrInvalidInviteToken
	}
	return inviteID, nil
}

// The MAC is checked before the payload is interpreted, so a forged token never
// reaches the identifier parser.
func authenticatedPayload(
	token string,
	key []byte,
	purpose CapabilityPurpose,
) ([]byte, error) {
	capability, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(capability) != inviteCapabilitySize {
		return nil, ErrInvalidInviteToken
	}
	payload := capability[:16]
	if !hmac.Equal(capability[16:], inviteMAC(key, purpose, payload)) {
		return nil, ErrInvalidInviteToken
	}
	return payload, nil
}

func (c *InviteCodec) StorageDigest(token, version string) ([]byte, error) {
	key, ok := c.keys[version]
	if !ok {
		return nil, ErrInvalidInviteToken
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("invite-storage\x00"))
	_, _ = mac.Write([]byte(token))
	return mac.Sum(nil), nil
}

func inviteMAC(key []byte, purpose CapabilityPurpose, inviteID []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(purpose))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write(inviteID)
	return mac.Sum(nil)
}
