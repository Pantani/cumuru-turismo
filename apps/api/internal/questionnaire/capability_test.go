package questionnaire

import (
	"bytes"
	"testing"

	"github.com/google/uuid"
)

func TestCapabilityRoundTripAndHistoricalReplay(t *testing.T) {
	t.Parallel()
	id := mustV7(t)
	codec, err := NewCapabilityCodec(Keyring{
		CurrentVersion: "v2",
		Keys: map[string][]byte{
			"v1": bytes.Repeat([]byte{1}, 32),
			"v2": bytes.Repeat([]byte{2}, 32),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	issued, err := codec.Issue(id)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := codec.Resolve(issued.Token)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := codec.Reconstruct(resolved.ID, resolved.KeyVersion)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Token != issued.Token || !bytes.Equal(resolved.LookupHMAC, issued.LookupHMAC) {
		t.Fatal("capability replay changed")
	}
}

func TestCapabilityRejectsTampering(t *testing.T) {
	t.Parallel()
	codec, err := NewCapabilityCodec(Keyring{
		CurrentVersion: "v1",
		Keys:           map[string][]byte{"v1": bytes.Repeat([]byte{3}, 32)},
	})
	if err != nil {
		t.Fatal(err)
	}
	issued, err := codec.Issue(mustV7(t))
	if err != nil {
		t.Fatal(err)
	}
	identifier, mac, err := parseCapabilityToken(issued.Token)
	if err != nil {
		t.Fatal(err)
	}
	mac[0] ^= 1
	token := encodeCapabilityToken(identifier, mac)
	if _, err := codec.Resolve(token); err == nil {
		t.Fatal("tampered capability accepted")
	}
}

func mustV7(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
