package store

import (
	"bytes"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/idempotency"
	"github.com/Pantani/cumuru/apps/api/internal/questionnaire"
	"github.com/google/uuid"
)

func TestQuestionnaireIdempotencyHashesIgnoreTransportMetadata(t *testing.T) {
	t.Parallel()
	actor := access.NewPrincipal("https://issuer.invalid", "editor", nil)
	first := questionnaire.CreateCommand{
		Actor:     actor,
		ID:        uuid.MustParse("019f0000-0000-7000-8000-000000000071"),
		VersionID: uuid.MustParse("019f0000-0000-7000-8000-000000000072"),
		StableKey: "tourism_profile", Name: "Perfil", Title: "Pesquisa",
		PrivacyNoticeVersion: "notice-v1", IdempotencyKey: "aaaaaaaaaaaaaaaa",
		RequestID: "request-one",
	}
	second := first
	second.RequestID = "request-two"
	assertSameHash(t, createQuestionnaireHash(first), createQuestionnaireHash(second))

	cloneFirst := questionnaire.CloneCommand{
		Actor: actor, QuestionnaireID: first.ID,
		SourceVersionID: first.VersionID,
		NewVersionID:    uuid.MustParse("019f0000-0000-7000-8000-000000000073"),
		IdempotencyKey:  "bbbbbbbbbbbbbbbb", RequestID: "request-one",
	}
	cloneSecond := cloneFirst
	cloneSecond.RequestID = "request-two"
	assertSameHash(t, cloneQuestionnaireHash(cloneFirst), cloneQuestionnaireHash(cloneSecond))

	transitionFirst := questionnaire.TransitionCommand{
		Actor: actor, VersionID: first.VersionID, ExpectedVersion: 1,
		Transition:     questionnaire.TransitionSubmitReview,
		IdempotencyKey: "cccccccccccccccc", RequestID: "request-one",
	}
	transitionSecond := transitionFirst
	transitionSecond.RequestID = "request-two"
	assertSameHash(
		t,
		transitionQuestionnaireHash(transitionFirst),
		transitionQuestionnaireHash(transitionSecond),
	)
}

func TestReviewerIdentityRemainsDistinctAcrossKeyRotation(t *testing.T) {
	t.Parallel()
	store := &Store{phase2: config.Phase2Config{
		ActorKeys: config.KeyringConfig{
			CurrentVersion: "actor-v2",
			Keys: map[string][]byte{
				"actor-v1": bytes.Repeat([]byte{1}, 32),
				"actor-v2": bytes.Repeat([]byte{2}, 32),
			},
		},
	}}
	editor := access.NewPrincipal("https://issuer.invalid", "same-user", nil)
	other := access.NewPrincipal("https://issuer.invalid", "other-user", nil)
	oldDigest := digests(store.phase2.ActorKeys, "actor", actorValue(editor.Issuer, editor.Subject))[1]
	version := questionnaire.Version{
		LastEditorHMAC: oldDigest.sum, LastEditorKeyVersion: oldDigest.version,
	}
	if reviewerDistinctFromEditor(store, version, editor) {
		t.Fatal("same editor became distinct after key rotation")
	}
	if !reviewerDistinctFromEditor(store, version, other) {
		t.Fatal("different reviewer was rejected")
	}
	version.LastEditorKeyVersion = "removed-key"
	if reviewerDistinctFromEditor(store, version, other) {
		t.Fatal("missing historical key did not fail closed")
	}
}

func assertSameHash(t *testing.T, left, right any) {
	t.Helper()
	leftHash, err := idempotency.RequestHash(left)
	if err != nil {
		t.Fatal(err)
	}
	rightHash, err := idempotency.RequestHash(right)
	if err != nil {
		t.Fatal(err)
	}
	if leftHash != rightHash {
		t.Fatal("transport-only change modified request hash")
	}
}
