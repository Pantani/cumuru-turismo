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
	first, version, err := codec.Issue(stay.PurposeStayGroupSubmission, inviteID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	second, _, _ := codec.Issue(stay.PurposeStayGroupSubmission, inviteID)
	if first != second || version != "invite-v2" {
		t.Fatalf("Issue() = %q/%s then %q", first, version, second)
	}
	replayed, err := codec.Reconstruct(stay.PurposeStayGroupSubmission, inviteID, "invite-v2")
	if err != nil || replayed != first {
		t.Fatalf("Reconstruct() = %q, %v", replayed, err)
	}
}

func TestInviteCodecVerifiesAndRejectsTampering(t *testing.T) {
	t.Parallel()

	codec := versionedInviteCodec(t)
	inviteID := uuid.MustParse("019f0000-0000-7000-8000-000000000010")
	token, version, _ := codec.Issue(stay.PurposeStayGroupSubmission, inviteID)
	gotID, err := codec.Verify(stay.PurposeStayGroupSubmission, token, version)
	if err != nil || gotID != inviteID {
		t.Fatalf("Verify() = %s, %v", gotID, err)
	}
	tampered := token[:len(token)-1] + differentLastCharacter(token)
	if _, err := codec.Verify(stay.PurposeStayGroupSubmission, tampered, version); err == nil {
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
	token, version, _ := codec.Issue(stay.PurposeStayGroupSubmission, uuid.MustParse("019f0000-0000-7000-8000-000000000011"))
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

// The purpose is inside the MAC, so a poster token cannot be replayed as an
// activation link even though both carry the same 16-byte identifier. This is
// the promise ADR-019 made and the code did not keep until now.
func TestInviteCodecSeparatesPurposes(t *testing.T) {
	t.Parallel()

	codec := versionedInviteCodec(t)
	inviteID := uuid.MustParse("019f0000-0000-7000-8000-000000000010")
	poster, version, err := codec.Issue(stay.PurposeAccommodationSelfRegistration, inviteID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	stayToken, _, _ := codec.Issue(stay.PurposeStayGroupSubmission, inviteID)
	if poster == stayToken {
		t.Fatal("two purposes minted the same token for the same identifier")
	}
	if _, err := codec.Verify(stay.PurposeStayGroupSubmission, poster, version); err == nil {
		t.Fatal("a poster token verified under the stay purpose")
	}
	if _, err := codec.Verify(
		stay.PurposeAccommodationActivation, poster, version,
	); err == nil {
		t.Fatal("a poster token verified under the activation purpose")
	}
	resolved, err := codec.Verify(stay.PurposeAccommodationSelfRegistration, poster, version)
	if err != nil || resolved != inviteID {
		t.Fatalf("Verify() = %s, %v", resolved, err)
	}
}

func TestInviteCodecRefusesAnUnknownPurpose(t *testing.T) {
	t.Parallel()

	codec := versionedInviteCodec(t)
	inviteID := uuid.MustParse("019f0000-0000-7000-8000-000000000010")
	if _, _, err := codec.Issue("survey_capability", inviteID); err == nil {
		t.Fatal("Issue() accepted a purpose outside the closed list")
	}
	token, version, _ := codec.Issue(stay.PurposeStayGroupSubmission, inviteID)
	if _, err := codec.Verify("", token, version); err == nil {
		t.Fatal("Verify() accepted an empty purpose")
	}
}

// The Fase 2 token must not change shape: an invite issued before this phase
// still has to reconstruct byte for byte, or every idempotent replay of
// createStayInvite would start returning a token the poster no longer matches.
func TestStayPurposeKeepsTheHistoricalMAC(t *testing.T) {
	t.Parallel()

	codec := versionedInviteCodec(t)
	inviteID := uuid.MustParse("019f0000-0000-7000-8000-000000000010")
	token, _, err := codec.Issue(stay.PurposeStayGroupSubmission, inviteID)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	// Computed outside this package as
	// base64url(id ‖ HMAC-SHA256(invite-v2 key, "stay_group_submission" ‖ 0x00 ‖ id)),
	// which is the encoding the constant-purpose code produced before the MAC
	// was parameterized.
	const historical = "AZ8AAAAAcACAAAAAAAAAEBSLnee77_cGfDsUsbTitOzUjZSg4SGTRQ9UCzhPtqs5"
	if token != historical {
		t.Fatalf("Issue() = %q, want the pre-phase-7 encoding %q", token, historical)
	}
}
