package httpapi

import (
	"net/http"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
)

// mustNew builds the handlers and fails the test if New rejects the
// dependencies, so a construction error surfaces at its own line instead of as a
// nil-handler panic further down.
func mustNew(t *testing.T, dependencies Dependencies) (http.Handler, http.Handler) {
	t.Helper()
	public, operations, err := New(dependencies)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return public, operations
}

// testCursorKeys satisfies the codec for the surfaces that paginate. The value
// is fixture-only and never leaves the test binary.
func testCursorKeys() config.KeyringConfig {
	return config.KeyringConfig{
		CurrentVersion: "cursor-v1",
		Keys: map[string][]byte{
			"cursor-v1": []byte("cursor-test-key-is-at-least-32-bytes"),
		},
	}
}

// A paginated surface without a usable cursor keyring used to build a handler
// that answered every listing without a next-page cursor. It must refuse to
// build instead, so the misconfiguration stops the process at startup.
func TestNewRejectsPaginatedSurfaceWithoutCursorKeyring(t *testing.T) {
	t.Parallel()

	if _, _, err := New(Dependencies{Stays: stay.NewService(nil)}); err == nil {
		t.Fatal("New() error = nil, want a rejected cursor keyring")
	}
}

// A handler exposing only platform routes paginates nothing, so it still builds
// without a keyring.
func TestNewAcceptsUnpaginatedSurfaceWithoutCursorKeyring(t *testing.T) {
	t.Parallel()

	if _, _, err := New(Dependencies{}); err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
}
