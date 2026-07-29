package application

import (
	"strings"
	"testing"
	"time"
)

func TestParseBuildAcceptsReproducibleMetadata(t *testing.T) {
	t.Parallel()

	const builtAt = "2026-07-28T00:00:00Z"
	got, err := ParseBuild(
		"0.2.0",
		"source-9c521fed28d7e090cad55609e59e1a4aa2e2225ba7d4d27a96835917b32f5e58",
		builtAt,
	)
	if err != nil {
		t.Fatalf("ParseBuild() error = %v", err)
	}
	if got.Version != "0.2.0" {
		t.Fatalf("Version = %q, want 0.2.0", got.Version)
	}
	if got.Revision != "source-9c521fed28d7e090cad55609e59e1a4aa2e2225ba7d4d27a96835917b32f5e58" {
		t.Fatalf("Revision = %q", got.Revision)
	}
	if got.BuiltAt.Location() != time.UTC || got.BuiltAt.Format(time.RFC3339) != builtAt {
		t.Fatalf("BuiltAt = %s in %s, want %s in UTC", got.BuiltAt, got.BuiltAt.Location(), builtAt)
	}
}

func TestParseBuildRejectsUntruthfulMetadataWithoutLeakingIt(t *testing.T) {
	t.Parallel()

	const secretSentinel = "private-build-sentinel"
	tests := []struct {
		name     string
		version  string
		revision string
		builtAt  string
		field    string
	}{
		{
			name:     "missing version",
			revision: "source-abc123",
			builtAt:  "2026-07-28T00:00:00Z",
			field:    "VERSION",
		},
		{
			name:     "unknown version",
			version:  "unknown",
			revision: "source-abc123",
			builtAt:  "2026-07-28T00:00:00Z",
			field:    "VERSION",
		},
		{
			name:     "unsupported version characters",
			version:  "0.2.0/" + secretSentinel,
			revision: "source-abc123",
			builtAt:  "2026-07-28T00:00:00Z",
			field:    "VERSION",
		},
		{
			name:    "missing revision",
			version: "0.2.0",
			builtAt: "2026-07-28T00:00:00Z",
			field:   "REVISION",
		},
		{
			name:     "unknown revision",
			version:  "0.2.0",
			revision: "unknown",
			builtAt:  "2026-07-28T00:00:00Z",
			field:    "REVISION",
		},
		{
			name:     "unsupported revision characters",
			version:  "0.2.0",
			revision: "source/" + secretSentinel,
			builtAt:  "2026-07-28T00:00:00Z",
			field:    "REVISION",
		},
		{
			name:     "missing build time",
			version:  "0.2.0",
			revision: "source-abc123",
			field:    "BUILT_AT",
		},
		{
			name:     "unknown build time",
			version:  "0.2.0",
			revision: "source-abc123",
			builtAt:  "unknown",
			field:    "BUILT_AT",
		},
		{
			name:     "epoch substitution is forbidden",
			version:  "0.2.0",
			revision: "source-abc123",
			builtAt:  "1970-01-01T00:00:00Z",
			field:    "BUILT_AT",
		},
		{
			name:     "offset is not canonical UTC",
			version:  "0.2.0",
			revision: "source-abc123",
			builtAt:  "2026-07-27T21:00:00-03:00",
			field:    "BUILT_AT",
		},
		{
			name:     "fractional seconds are not reproducible format",
			version:  "0.2.0",
			revision: "source-abc123",
			builtAt:  "2026-07-28T00:00:00.000Z",
			field:    "BUILT_AT",
		},
		{
			name:     "invalid calendar date",
			version:  "0.2.0",
			revision: "source-abc123",
			builtAt:  "2026-02-30T00:00:00Z",
			field:    "BUILT_AT",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertRejectedBuild(t, tt.version, tt.revision, tt.builtAt, tt.field, secretSentinel)
		})
	}
}

func assertRejectedBuild(t *testing.T, version, revision, builtAt, field, sentinel string) {
	t.Helper()
	_, err := ParseBuild(version, revision, builtAt)
	if err == nil || !strings.Contains(err.Error(), field) {
		t.Fatalf("ParseBuild() error = %v, want sanitized field %s", err, field)
	}
	values := []string{sentinel, version, revision, builtAt}
	for _, value := range values {
		if value != "" && strings.Contains(err.Error(), value) {
			t.Fatalf("ParseBuild() leaked metadata value: %v", err)
		}
	}
}

func TestBuildValidationRejectsProgrammaticBypass(t *testing.T) {
	t.Parallel()

	tests := []Build{
		{Version: "0.2.0", Revision: "source-abc123", BuiltAt: time.Time{}},
		{Version: "0.2.0", Revision: "unknown", BuiltAt: time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)},
		{
			Version:  "0.2.0",
			Revision: "source-abc123",
			BuiltAt:  time.Date(2026, 7, 28, 0, 0, 0, 1, time.UTC),
		},
	}
	for _, build := range tests {
		if err := build.validate(); err == nil {
			t.Fatalf("validate(%+v) error = nil", build)
		}
	}
}
