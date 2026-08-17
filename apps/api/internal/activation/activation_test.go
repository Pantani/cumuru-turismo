package activation_test

import (
	"strings"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/activation"
)

// The activated account mirrors the accommodation operator and carries the
// phase's own approval scope. Without stays:approve the whole flow is hollow:
// the accommodation that received the link could not approve its own queue, and
// approval would stay with the principal that self-provisioned.
//
// platform:read and accommodations:onboard stay out — they belong to other
// roles, and the accommodation already exists when the capability is issued.
func TestActivatedAccountCarriesTheOperatorScopeSet(t *testing.T) {
	t.Parallel()

	granted := strings.Join(activation.Scopes(), " ")
	want := "accommodations:manage stays:read:own stays:write stays:approve"
	if granted != want {
		t.Fatalf("Scopes() = %q, want %q", granted, want)
	}
	for _, refused := range []string{"platform:read", "accommodations:onboard"} {
		if strings.Contains(granted, refused) {
			t.Fatalf("%s was granted to an activated account", refused)
		}
	}
}

// The slice is package state, so a caller that appended to it would widen every
// future account.
func TestActivationScopesAreNotSharedState(t *testing.T) {
	t.Parallel()

	first := activation.Scopes()
	first[0] = "tampered"
	if activation.Scopes()[0] != "accommodations:manage" {
		t.Fatal("mutating one account's scopes changed the next account's scopes")
	}
}
