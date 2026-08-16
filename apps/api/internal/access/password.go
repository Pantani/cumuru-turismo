package access

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// ErrInvalidPassword separates a rejected secret from an operational failure so
// the caller can answer with a uniform 401 without swallowing real errors.
var ErrInvalidPassword = errors.New("invalid password")

var errMalformedHash = errors.New("malformed password hash")

const (
	// MinPasswordLength mirrors the contract and the NIST SP 800-63B guidance
	// of favouring length over composition rules.
	MinPasswordLength = 12
	// MaxPasswordLength bounds the Argon2id input so a large body cannot turn a
	// login attempt into a denial of service.
	MaxPasswordLength = 256

	argonPHCPrefix  = "$argon2id$"
	argonVersionTag = "v=19"
	argonSegments   = 6
)

// PasswordHasher holds the Argon2id cost. The defaults follow the OWASP
// Password Storage Cheat Sheet configuration of 19 MiB, two iterations and one
// lane.
type PasswordHasher struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

type argonParams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

func NewPasswordHasher() PasswordHasher {
	return PasswordHasher{
		memory:      19456,
		iterations:  2,
		parallelism: 1,
		saltLength:  16,
		keyLength:   32,
	}
}

// Hash returns the PHC encoding of the secret, salt included, so a rotation of
// the cost parameters never invalidates the stored credentials.
func (h PasswordHasher) Hash(password string) (string, error) {
	if err := ValidatePasswordLength(password); err != nil {
		return "", err
	}
	salt := make([]byte, h.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", errors.New("password salt generation failed")
	}
	key := argon2.IDKey(
		[]byte(password), salt, h.iterations, h.memory, h.parallelism, h.keyLength,
	)
	return encodeArgonHash(
		argonParams{h.memory, h.iterations, h.parallelism}, salt, key,
	), nil
}

// VerifyPassword recomputes the digest with the parameters carried by the
// stored encoding and compares in constant time.
func VerifyPassword(encoded, password string) error {
	if err := ValidatePasswordLength(password); err != nil {
		return err
	}
	params, salt, expected, err := decodeArgonHash(encoded)
	if err != nil {
		return err
	}
	candidate := argon2.IDKey(
		[]byte(password),
		salt,
		params.iterations,
		params.memory,
		params.parallelism,
		uint32(len(expected)),
	)
	if subtle.ConstantTimeCompare(candidate, expected) != 1 {
		return ErrInvalidPassword
	}
	return nil
}

func ValidatePasswordLength(password string) error {
	if len(password) < MinPasswordLength || len(password) > MaxPasswordLength {
		return ErrInvalidPassword
	}
	return nil
}

func encodeArgonHash(params argonParams, salt, key []byte) string {
	return fmt.Sprintf(
		"%s%s$m=%d,t=%d,p=%d$%s$%s",
		argonPHCPrefix,
		argonVersionTag,
		params.memory,
		params.iterations,
		params.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
}

func decodeArgonHash(encoded string) (argonParams, []byte, []byte, error) {
	segments, err := splitArgonHash(encoded)
	if err != nil {
		return argonParams{}, nil, nil, err
	}
	params, err := parseArgonParams(segments[3])
	if err != nil {
		return argonParams{}, nil, nil, err
	}
	salt, hash, err := decodeArgonPayload(segments[4], segments[5])
	if err != nil {
		return argonParams{}, nil, nil, err
	}
	return params, salt, hash, nil
}

func splitArgonHash(encoded string) ([]string, error) {
	segments := strings.Split(encoded, "$")
	if len(segments) != argonSegments {
		return nil, errMalformedHash
	}
	if segments[1] != "argon2id" || segments[2] != argonVersionTag {
		return nil, errMalformedHash
	}
	return segments, nil
}

func (p argonParams) usable() bool {
	return p.memory != 0 && p.iterations != 0 && p.parallelism != 0
}

func parseArgonParams(segment string) (argonParams, error) {
	var params argonParams
	read, err := fmt.Sscanf(
		segment, "m=%d,t=%d,p=%d",
		&params.memory, &params.iterations, &params.parallelism,
	)
	if err != nil || read != 3 || !params.usable() {
		return argonParams{}, errMalformedHash
	}
	return params, nil
}

func decodeArgonPayload(saltSegment, hashSegment string) ([]byte, []byte, error) {
	salt, err := base64.RawStdEncoding.DecodeString(saltSegment)
	if err != nil || len(salt) == 0 {
		return nil, nil, errMalformedHash
	}
	hash, err := base64.RawStdEncoding.DecodeString(hashSegment)
	if err != nil || len(hash) == 0 {
		return nil, nil, errMalformedHash
	}
	return salt, hash, nil
}
