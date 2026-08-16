package questionnaire

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/google/uuid"
)

const capabilityPurpose = "survey_response"

type Keyring struct {
	CurrentVersion string
	Keys           map[string][]byte
}

type Capability struct {
	ID         uuid.UUID
	KeyVersion string
	LookupHMAC []byte
	Token      string
}

type CapabilityCodec struct {
	keyring Keyring
}

func usableKeyring(keyring Keyring, minimumKeyBytes int) bool {
	if keyring.CurrentVersion == "" || len(keyring.Keys) == 0 {
		return false
	}
	for _, key := range keyring.Keys {
		if len(key) < minimumKeyBytes {
			return false
		}
	}
	_, ok := keyring.Keys[keyring.CurrentVersion]
	return ok
}

func NewCapabilityCodec(keyring Keyring) (*CapabilityCodec, error) {
	if !usableKeyring(keyring, 32) {
		return nil, ErrCapabilityInvalid
	}
	return &CapabilityCodec{keyring: cloneKeyring(keyring)}, nil
}

func (c *CapabilityCodec) Issue(id uuid.UUID) (Capability, error) {
	if id.Version() != 7 {
		return Capability{}, ErrCapabilityInvalid
	}
	return c.reconstruct(id, c.keyring.CurrentVersion)
}

func (c *CapabilityCodec) Reconstruct(id uuid.UUID, version string) (Capability, error) {
	return c.reconstruct(id, version)
}

func (c *CapabilityCodec) Resolve(token string) (Capability, error) {
	identifier, supplied, err := parseCapabilityToken(token)
	if err != nil {
		return Capability{}, ErrCapabilityInvalid
	}
	for version, key := range c.keyring.Keys {
		expected := capabilityMAC(key, identifier)
		if hmac.Equal(expected, supplied) {
			return Capability{
				ID: identifier, KeyVersion: version,
				LookupHMAC: append([]byte(nil), supplied...), Token: token,
			}, nil
		}
	}
	return Capability{}, ErrCapabilityInvalid
}

func (c *CapabilityCodec) reconstruct(id uuid.UUID, version string) (Capability, error) {
	key, ok := c.keyring.Keys[version]
	if !ok || id == uuid.Nil {
		return Capability{}, ErrCapabilityInvalid
	}
	mac := capabilityMAC(key, id)
	token := encodeCapabilityToken(id, mac)
	return Capability{
		ID: id, KeyVersion: version, LookupHMAC: mac, Token: token,
	}, nil
}

func capabilityMAC(key []byte, id uuid.UUID) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(capabilityPurpose))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write(id[:])
	return mac.Sum(nil)
}

func encodeCapabilityToken(id uuid.UUID, mac []byte) string {
	encoding := base64.RawURLEncoding
	return encoding.EncodeToString(id[:]) + "." + encoding.EncodeToString(mac)
}

func splitCapabilityToken(token string) (string, string, error) {
	left, right, ok := strings.Cut(token, ".")
	if !ok || strings.Contains(right, ".") {
		return "", "", errors.New("invalid token segments")
	}
	return left, right, nil
}

// The MAC must round-trip through base64 unchanged; a non-canonical encoding of
// the same bytes would otherwise widen the accepted token set.
func decodeCapabilityMAC(segment string) ([]byte, error) {
	mac, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil || len(mac) != sha256.Size {
		return nil, errors.New("invalid token mac")
	}
	if base64.RawURLEncoding.EncodeToString(mac) != segment {
		return nil, errors.New("non-canonical token mac")
	}
	return mac, nil
}

func decodeCapabilityID(segment string) (uuid.UUID, error) {
	idBytes, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil || len(idBytes) != 16 {
		return uuid.Nil, errors.New("invalid token id")
	}
	return uuid.FromBytes(idBytes)
}

func parseCapabilityToken(token string) (uuid.UUID, []byte, error) {
	left, right, err := splitCapabilityToken(token)
	if err != nil {
		return uuid.Nil, nil, err
	}
	id, err := decodeCapabilityID(left)
	if err != nil {
		return uuid.Nil, nil, err
	}
	mac, err := decodeCapabilityMAC(right)
	if err != nil {
		return uuid.Nil, nil, err
	}
	return id, mac, nil
}

func cloneKeyring(keyring Keyring) Keyring {
	keys := make(map[string][]byte, len(keyring.Keys))
	for version, key := range keyring.Keys {
		keys[version] = append([]byte(nil), key...)
	}
	return Keyring{CurrentVersion: keyring.CurrentVersion, Keys: keys}
}
