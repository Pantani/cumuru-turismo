package stay_test

import (
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/stay"
)

// The unset state must not be countable and must not be valid. Anything else
// would let a caller that forgot to read the column publish a stay nobody
// approved, which is exactly the silent regression this feature guards against.
func TestUnsetApprovalIsNeitherValidNorCountable(t *testing.T) {
	t.Parallel()

	if stay.ApprovalUnset.Valid() || stay.ApprovalUnset.Countable() {
		t.Fatal("the zero approval state passed as a decided one")
	}
}

func TestOnlyAssistedAndApprovedStaysCount(t *testing.T) {
	t.Parallel()

	countable := map[stay.ApprovalState]bool{
		stay.ApprovalNotRequired: true,
		stay.ApprovalApproved:    true,
		stay.ApprovalPending:     false,
		stay.ApprovalRejected:    false,
		stay.ApprovalExpired:     false,
	}
	for state, want := range countable {
		if state.Countable() != want {
			t.Fatalf("%s countable = %t, want %t", state, state.Countable(), want)
		}
		if !state.Valid() {
			t.Fatalf("%s reported as an unknown state", state)
		}
	}
}

// NULL in the column is the assisted stay, not an undecided one.
func TestNullApprovalColumnMapsToNotRequired(t *testing.T) {
	t.Parallel()

	if stay.ApprovalStateFromColumn(nil) != stay.ApprovalNotRequired {
		t.Fatal("a null approval column did not map to not_required")
	}
	pending := "pending"
	if stay.ApprovalStateFromColumn(&pending) != stay.ApprovalPending {
		t.Fatal("a pending approval column did not map to pending")
	}
	if stay.ApprovalStateColumn(stay.ApprovalNotRequired) != nil {
		t.Fatal("an assisted stay wrote a non-null approval column")
	}
	if column := stay.ApprovalStateColumn(stay.ApprovalPending); column == nil ||
		*column != "pending" {
		t.Fatal("a pending state did not write its column value")
	}
}

func TestRejectionReasonIsAClosedList(t *testing.T) {
	t.Parallel()

	valid := []stay.RejectionReason{
		stay.RejectionIdentityNotVerified, stay.RejectionNotAGuest,
		stay.RejectionDuplicate, stay.RejectionDataIncorrect, stay.RejectionOther,
	}
	for _, reason := range valid {
		if !reason.Valid() {
			t.Fatalf("%s rejected by the closed list", reason)
		}
	}
	refused := []stay.RejectionReason{
		"", "CPF falso", "other ", "identity_not_verified\n",
	}
	for _, reason := range refused {
		if reason.Valid() {
			t.Fatalf("free text %q accepted as a rejection reason", reason)
		}
	}
}

func TestProvenanceIsAClosedList(t *testing.T) {
	t.Parallel()

	if !stay.ProvenanceAssisted.Valid() || !stay.ProvenanceSelfService.Valid() {
		t.Fatal("a known provenance was rejected")
	}
	if stay.Provenance("").Valid() || stay.Provenance("operator").Valid() {
		t.Fatal("an unknown provenance was accepted")
	}
}

// The same 72 hours as the core invite: the product must not grow a second
// notion of validity.
func TestPendingApprovalSharesTheInviteLifetime(t *testing.T) {
	t.Parallel()

	if stay.PendingApprovalTTL != 72*time.Hour {
		t.Fatalf("PendingApprovalTTL = %s, want 72h", stay.PendingApprovalTTL)
	}
}
