package httpapi

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/google/uuid"
)

func TestCursorCodecSignsRejectsTamperAndPreservesRotation(t *testing.T) {
	t.Parallel()

	oldKeys := config.KeyringConfig{
		CurrentVersion: "cursor-v1",
		Keys: map[string][]byte{
			"cursor-v1": []byte("cursor-version-one-key-is-32-bytes"),
		},
	}
	oldCodec, err := newCursorCodec(oldKeys)
	if err != nil {
		t.Fatalf("newCursorCodec() error = %v", err)
	}
	want := pageCursor{
		CreatedAt: time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC),
		ID:        uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
	}
	encoded := oldCodec.encode(want)
	if encoded == nil || strings.Contains(*encoded, want.ID.String()) {
		t.Fatalf("encoded cursor is missing or readable: %v", encoded)
	}
	rotatedCodec, err := newCursorCodec(config.KeyringConfig{
		CurrentVersion: "cursor-v2",
		Keys: map[string][]byte{
			"cursor-v1": oldKeys.Keys["cursor-v1"],
			"cursor-v2": []byte("cursor-version-two-key-is-32-bytes"),
		},
	})
	if err != nil {
		t.Fatalf("newCursorCodec(rotated) error = %v", err)
	}
	got, err := rotatedCodec.decode(*encoded)
	if err != nil || got != want {
		t.Fatalf("decode() = %#v, %v; want %#v", got, err, want)
	}
	assertCursorTamperRejected(t, rotatedCodec, *encoded)
}

func assertCursorTamperRejected(t *testing.T, codec cursorCodec, encoded string) {
	t.Helper()
	last := encoded[len(encoded)-1]
	replacement := byte('A')
	if last == replacement {
		replacement = 'B'
	}
	tampered := encoded[:len(encoded)-1] + string(replacement)
	if _, err := codec.decode(tampered); !errors.Is(err, errInvalidCursor) {
		t.Fatalf("decode(tampered) error = %v", err)
	}
}

func TestCursorCodecRejectsMissingCurrentKey(t *testing.T) {
	t.Parallel()

	_, err := newCursorCodec(config.KeyringConfig{
		CurrentVersion: "cursor-v2",
		Keys: map[string][]byte{
			"cursor-v1": []byte("cursor-version-one-key-is-32-bytes"),
		},
	})
	if !errors.Is(err, errInvalidCursor) {
		t.Fatalf("newCursorCodec() error = %v", err)
	}
}
