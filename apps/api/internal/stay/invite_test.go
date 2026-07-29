package stay_test

import (
	"strings"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
)

func TestInviteCodecIsDeterministicAndVersioned(t *testing.T) {
	t.Parallel()

	codec := versionedInviteCodec(t)
	inviteID := uuid.MustParse("019f0000-0000-7000-8000-000000000010")
	first, version, err := codec.Issue(inviteID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	second, _, _ := codec.Issue(inviteID)
	if first != second || version != "invite-v2" {
		t.Fatalf("Issue() = %q/%s then %q", first, version, second)
	}
	replayed, err := codec.Reconstruct(inviteID, "invite-v2")
	if err != nil || replayed != first {
		t.Fatalf("Reconstruct() = %q, %v", replayed, err)
	}
}

func TestInviteCodecVerifiesAndRejectsTampering(t *testing.T) {
	t.Parallel()

	codec := versionedInviteCodec(t)
	inviteID := uuid.MustParse("019f0000-0000-7000-8000-000000000010")
	token, version, _ := codec.Issue(inviteID)
	gotID, err := codec.Verify(token, version)
	if err != nil || gotID != inviteID {
		t.Fatalf("Verify() = %s, %v", gotID, err)
	}
	tampered := token[:len(token)-1] + differentLastCharacter(token)
	if _, err := codec.Verify(tampered, version); err == nil {
		t.Fatal("Verify(tampered) error = nil")
	}
}

func versionedInviteCodec(t *testing.T) *stay.InviteCodec {
	t.Helper()
	codec, err := stay.NewInviteCodec(stay.InviteKeyring{
		CurrentVersion: "invite-v2",
		Keys: map[string][]byte{
			"invite-v1": []byte("first-invite-key-is-at-least-32-bytes"),
			"invite-v2": []byte("second-invite-key-is-at-least-32-bytes"),
		},
	})
	if err != nil {
		t.Fatalf("NewInviteCodec() error = %v", err)
	}
	return codec
}

func TestInviteCodecProducesOnlyNonReversibleDigestForStorage(t *testing.T) {
	t.Parallel()

	codec, err := stay.NewInviteCodec(stay.InviteKeyring{
		CurrentVersion: "invite-v1",
		Keys: map[string][]byte{
			"invite-v1": []byte("invite-key-material-has-at-least-32-bytes"),
		},
	})
	if err != nil {
		t.Fatalf("NewInviteCodec() error = %v", err)
	}
	token, version, _ := codec.Issue(uuid.MustParse("019f0000-0000-7000-8000-000000000011"))
	digest, err := codec.StorageDigest(token, version)
	if err != nil {
		t.Fatalf("StorageDigest() error = %v", err)
	}
	if len(digest) != 32 || strings.Contains(string(digest), token) {
		t.Fatalf("StorageDigest() returned unsafe value")
	}
}

func differentLastCharacter(value string) string {
	if value[len(value)-1] == 'A' {
		return "B"
	}
	return "A"
}
