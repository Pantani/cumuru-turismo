package questionnaire

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

type Ciphertext struct {
	Content    []byte
	Nonce      []byte
	KeyVersion string
}

type TextCipher struct {
	currentVersion string
	aeads          map[string]cipher.AEAD
}

func (c *TextCipher) CurrentVersion() string {
	if c == nil {
		return ""
	}
	return c.currentVersion
}

func NewTextCipher(keyring Keyring) (*TextCipher, error) {
	if keyring.CurrentVersion == "" || len(keyring.Keys) == 0 {
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
	if _, ok := aeads[keyring.CurrentVersion]; !ok {
		return nil, ErrInvalidInput
	}
	return &TextCipher{currentVersion: keyring.CurrentVersion, aeads: aeads}, nil
}

func (c *TextCipher) Encrypt(plaintext, associatedData []byte) (Ciphertext, error) {
	aead, ok := c.aeads[c.currentVersion]
	if !ok || len(plaintext) == 0 {
		return Ciphertext{}, ErrInvalidInput
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return Ciphertext{}, ErrUnavailable
	}
	content := aead.Seal(nil, nonce, plaintext, associatedData)
	return Ciphertext{Content: content, Nonce: nonce, KeyVersion: c.currentVersion}, nil
}

func (c *TextCipher) Decrypt(value Ciphertext, associatedData []byte) ([]byte, error) {
	aead, ok := c.aeads[value.KeyVersion]
	if !ok || len(value.Nonce) != aead.NonceSize() {
		return nil, ErrInvalidInput
	}
	plaintext, err := aead.Open(nil, value.Nonce, value.Content, associatedData)
	if err != nil {
		return nil, ErrInvalidInput
	}
	return plaintext, nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("AES-GCM-256 key required")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
