package store

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/google/uuid"
)

func TestSurveyAADBindsEncryptionKeyVersion(t *testing.T) {
	t.Parallel()
	responseID := uuid.MustParse("019f0000-0000-7000-8000-000000000081")
	versionID := uuid.MustParse("019f0000-0000-7000-8000-000000000082")
	questionID := uuid.MustParse("019f0000-0000-7000-8000-000000000083")
	first := surveyAAD(responseID, versionID, questionID, "text-v1")
	second := surveyAAD(responseID, versionID, questionID, "text-v2")
	if bytes.Equal(first, second) {
		t.Fatal("key version did not change associated data")
	}
	for _, part := range [][]byte{
		[]byte(responseID.String()),
		[]byte(versionID.String()),
		[]byte(questionID.String()),
		[]byte("text-v1"),
	} {
		if !bytes.Contains(first, part) {
			t.Fatalf("associated data misses %q", part)
		}
	}
}

type surveyRateQueriesStub struct {
	generated.Querier
	calls int
}

func (s *surveyRateQueriesStub) IncrementRateLimit(
	context.Context,
	generated.IncrementRateLimitParams,
) (generated.IncrementRateLimitRow, error) {
	s.calls++
	return generated.IncrementRateLimitRow{RequestCount: 1}, nil
}

func TestSurveyRateConnectionUsesExplicitNoPoolUnitFallback(t *testing.T) {
	t.Parallel()

	queries := &surveyRateQueriesStub{}
	keyring := config.KeyringConfig{
		CurrentVersion: "v1",
		Keys: map[string][]byte{
			"v1": []byte("rate-limit-key-material-32-bytes"),
		},
	}
	subject := &Store{
		queries: queries,
		timeout: time.Second,
		core: config.CoreConfig{
			RateLimitKeys:   keyring,
			RateLimitWindow: time.Minute,
		},
		questionnaire: config.QuestionnaireConfig{SurveySubmitRateLimit: 10},
	}
	repository := NewQuestionnaireRepository(subject)

	connection, err := repository.acquireSurveyRateConnection(context.Background())
	if err != nil {
		t.Fatalf("acquireSurveyRateConnection() error = %v", err)
	}
	defer connection.Close()
	if connection.connection != nil || connection.permitHeld {
		t.Fatal("unit fallback unexpectedly acquired a pool connection")
	}
	err = repository.applySurveyRateLimit(
		context.Background(),
		connection.Queries(),
		uuid.MustParse("019f0000-0000-7000-8000-000000000084"),
		"203.0.113.0/24",
		time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC),
	)
	if err != nil || queries.calls != 1 {
		t.Fatalf("unit fallback rate limit calls=%d err=%v", queries.calls, err)
	}
}

func TestQuestionnaireRepositoriesShareStoreSurveyPairPermit(t *testing.T) {
	t.Parallel()

	subject := New(&surveyRateQueriesStub{}, time.Second)
	first := NewQuestionnaireRepository(subject)
	second := NewQuestionnaireRepository(subject)

	if first.store.surveyPairPermit == nil {
		t.Fatal("shared survey pair permit is nil")
	}
	if first.store.surveyPairPermit != second.store.surveyPairPermit {
		t.Fatal("repositories over the same store use distinct permits")
	}
	if cap(first.store.surveyPairPermit) != 1 {
		t.Fatalf("shared survey pair permit capacity = %d", cap(first.store.surveyPairPermit))
	}
}
