package questionnaire

import (
	"bytes"
	"testing"
)

func TestTextCipherUsesAuthenticatedContext(t *testing.T) {
	t.Parallel()
	cipher, err := NewTextCipher(Keyring{
		CurrentVersion: "fixture-v1",
		Keys:           map[string][]byte{"fixture-v1": bytes.Repeat([]byte{7}, 32)},
	})
	if err != nil {
		t.Fatal(err)
	}
	value, err := cipher.Encrypt([]byte("fixture livre"), []byte("response|version|question"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(value.Content, []byte("fixture livre")) {
		t.Fatal("plaintext present in ciphertext")
	}
	plain, err := cipher.Decrypt(value, []byte("response|version|question"))
	if err != nil || string(plain) != "fixture livre" {
		t.Fatalf("decrypt failed: %v", err)
	}
	if _, err := cipher.Decrypt(value, []byte("other context")); err == nil {
		t.Fatal("altered associated data accepted")
	}
}
