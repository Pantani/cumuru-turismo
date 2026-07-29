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

func NewCapabilityCodec(keyring Keyring) (*CapabilityCodec, error) {
	if keyring.CurrentVersion == "" || len(keyring.Keys) == 0 {
		return nil, ErrCapabilityInvalid
	}
	for _, key := range keyring.Keys {
		if len(key) < 32 {
			return nil, ErrCapabilityInvalid
		}
	}
	if _, ok := keyring.Keys[keyring.CurrentVersion]; !ok {
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

func parseCapabilityToken(token string) (uuid.UUID, []byte, error) {
	left, right, ok := strings.Cut(token, ".")
	if !ok || strings.Contains(right, ".") {
		return uuid.Nil, nil, errors.New("invalid token segments")
	}
	idBytes, err := base64.RawURLEncoding.DecodeString(left)
	if err != nil || len(idBytes) != 16 {
		return uuid.Nil, nil, errors.New("invalid token id")
	}
	mac, err := base64.RawURLEncoding.DecodeString(right)
	if err != nil || len(mac) != sha256.Size {
		return uuid.Nil, nil, errors.New("invalid token mac")
	}
	if base64.RawURLEncoding.EncodeToString(mac) != right {
		return uuid.Nil, nil, errors.New("non-canonical token mac")
	}
	id, err := uuid.FromBytes(idBytes)
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
