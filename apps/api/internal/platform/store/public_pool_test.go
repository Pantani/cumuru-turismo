package store

import (
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
)

func TestValidPublicSessionFailsClosed(t *testing.T) {
	t.Parallel()

	valid := generated.ValidatePublicRuntimeSessionRow{
		CurrentUserName: "public_runtime",
		SessionUserName: "cumuru_public",
		SearchPath:      publicRuntimeSearchPath,
	}
	if !validPublicSession(valid) {
		t.Fatal("expected the validated public session to be accepted")
	}
	cases := []generated.ValidatePublicRuntimeSessionRow{
		{CurrentUserName: "cumuru_app", SessionUserName: "cumuru_app", SearchPath: publicRuntimeSearchPath},
		{CurrentUserName: "public_runtime", SessionUserName: "", SearchPath: publicRuntimeSearchPath},
		{CurrentUserName: "public_runtime", SessionUserName: "public_runtime", SearchPath: publicRuntimeSearchPath},
		{CurrentUserName: "public_runtime", SessionUserName: "cumuru_public", SearchPath: "public"},
	}
	for _, candidate := range cases {
		if validPublicSession(candidate) {
			t.Fatalf("unsafe session accepted: %#v", candidate)
		}
	}
}
