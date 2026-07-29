package stay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"regexp"

	"github.com/google/uuid"
)

const (
	invitePurpose        = "stay_group_submission"
	inviteCapabilitySize = 16 + sha256.Size
)

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
	keys := make(map[string][]byte, len(keyring.Keys))
	for version, key := range keyring.Keys {
		if !inviteKeyVersion.MatchString(version) || len(key) < 32 {
			return nil, ErrInvalidInviteToken
		}
		keys[version] = append([]byte(nil), key...)
	}
	if _, ok := keys[keyring.CurrentVersion]; !ok {
		return nil, ErrInvalidInviteToken
	}
	return &InviteCodec{currentVersion: keyring.CurrentVersion, keys: keys}, nil
}

func (c *InviteCodec) Issue(inviteID uuid.UUID) (string, string, error) {
	token, err := c.Reconstruct(inviteID, c.currentVersion)
	return token, c.currentVersion, err
}

func (c *InviteCodec) Reconstruct(inviteID uuid.UUID, version string) (string, error) {
	key, ok := c.keys[version]
	if !ok || inviteID == uuid.Nil {
		return "", ErrInvalidInviteToken
	}
	payload := append([]byte(nil), inviteID[:]...)
	capability := append(payload, inviteMAC(key, payload)...)
	return base64.RawURLEncoding.EncodeToString(capability), nil
}

func (c *InviteCodec) Verify(token, version string) (uuid.UUID, error) {
	key, ok := c.keys[version]
	if !ok {
		return uuid.Nil, ErrInvalidInviteToken
	}
	capability, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(capability) != inviteCapabilitySize {
		return uuid.Nil, ErrInvalidInviteToken
	}
	payload := capability[:16]
	if !hmac.Equal(capability[16:], inviteMAC(key, payload)) {
		return uuid.Nil, ErrInvalidInviteToken
	}
	inviteID, err := uuid.FromBytes(payload)
	if err != nil || inviteID == uuid.Nil {
		return uuid.Nil, ErrInvalidInviteToken
	}
	return inviteID, nil
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

func inviteMAC(key, inviteID []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(invitePurpose))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write(inviteID)
	return mac.Sum(nil)
}
