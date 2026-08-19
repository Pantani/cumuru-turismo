package calendarfeed

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"io"
)

const keyBytes = 32

// Keyring mirrors the shape every other capability domain uses. It is its own
// ring on purpose: rotating or leaking the ring that protects feed URLs must
// never reach invites, cursors or the blinded document (ADR-038's reasoning,
// applied here).
type Keyring struct {
	CurrentVersion string
	Keys           map[string][]byte
}

// SealedURL is the feed address at rest. The address is a bearer secret —
// whoever holds it reads the whole listing calendar — so it is stored sealed
// and never returned by the API (ADR-043).
type SealedURL struct {
	Ciphertext []byte
	Nonce      []byte
	KeyVersion string
}

// Fingerprint answers one question and no other: has this accommodation already
// registered this feed? Comparison is all we need, so the address itself has no
// reason to be stored in a form that also answers "which listing is it".
type Fingerprint struct {
	Digest     []byte
	KeyVersion string
}

// URLSealer holds both rings because sealing and fingerprinting always happen
// together, and a caller holding only one of them could store a feed that can
// never be deduplicated or one that can never be read back.
type URLSealer struct {
	aeads          map[string]cipher.AEAD
	currentVersion string
	fingerprint    Keyring
}

func NewURLSealer(sealing, fingerprint Keyring) (*URLSealer, error) {
	aeads, err := buildAEADs(sealing)
	if err != nil {
		return nil, err
	}
	if !usableKeyring(fingerprint) {
		return nil, ErrInvalidInput
	}
	return &URLSealer{
		aeads:          aeads,
		currentVersion: sealing.CurrentVersion,
		fingerprint:    fingerprint,
	}, nil
}

// Seal binds the ciphertext to the accommodation through the associated data:
// a row moved to another accommodation stops decrypting, so a feed cannot be
// silently reassigned by writing to the foreign key alone.
func (s *URLSealer) Seal(url string, associatedData []byte) (SealedURL, error) {
	aead, ok := s.aeads[s.currentVersion]
	if !ok || url == "" {
		return SealedURL{}, ErrInvalidInput
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return SealedURL{}, ErrUnavailable
	}
	return SealedURL{
		Ciphertext: aead.Seal(nil, nonce, []byte(url), associatedData),
		Nonce:      nonce,
		KeyVersion: s.currentVersion,
	}, nil
}

func (s *URLSealer) Open(sealed SealedURL, associatedData []byte) (string, error) {
	aead, ok := s.aeads[sealed.KeyVersion]
	if !ok || len(sealed.Nonce) != aead.NonceSize() {
		return "", ErrInvalidInput
	}
	plaintext, err := aead.Open(nil, sealed.Nonce, sealed.Ciphertext, associatedData)
	if err != nil {
		return "", ErrInvalidInput
	}
	return string(plaintext), nil
}

func (s *URLSealer) Fingerprint(url string) (Fingerprint, error) {
	key, ok := s.fingerprint.Keys[s.fingerprint.CurrentVersion]
	if !ok || url == "" {
		return Fingerprint{}, ErrInvalidInput
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(url))
	return Fingerprint{
		Digest:     mac.Sum(nil),
		KeyVersion: s.fingerprint.CurrentVersion,
	}, nil
}

// FingerprintUID blinds the reservation identifier the origin platform assigned.
// It is a business identifier belonging to somebody else's system; under HMAC it
// still does the only job we need, which is recognizing that today's event is
// yesterday's event.
func (s *URLSealer) FingerprintUID(feedID, uid string) (Fingerprint, error) {
	if uid == "" || feedID == "" {
		return Fingerprint{}, ErrInvalidInput
	}
	return s.Fingerprint(feedID + "\x00" + uid)
}

func buildAEADs(keyring Keyring) (map[string]cipher.AEAD, error) {
	if !usableKeyring(keyring) {
		return nil, ErrInvalidInput
	}
	aeads := make(map[string]cipher.AEAD, len(keyring.Keys))
	for version, key := range keyring.Keys {
		aead, err := newAEAD(key)
		if err != nil {
			return nil, err
		}
		aeads[version] = aead
	}
	return aeads, nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != keyBytes {
		return nil, ErrInvalidInput
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrInvalidInput
	}
	return cipher.NewGCM(block)
}

func usableKeyring(keyring Keyring) bool {
	if keyring.CurrentVersion == "" || len(keyring.Keys) == 0 {
		return false
	}
	for _, key := range keyring.Keys {
		if len(key) < keyBytes {
			return false
		}
	}
	_, ok := keyring.Keys[keyring.CurrentVersion]
	return ok
}
