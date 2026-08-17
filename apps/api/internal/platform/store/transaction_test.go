package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/idempotency"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestDigestsUseCurrentVersionFirstAndPreserveHistory(t *testing.T) {
	t.Parallel()

	keyring := config.KeyringConfig{
		CurrentVersion: "v2",
		Keys: map[string][]byte{
			"v1": []byte("11111111111111111111111111111111"),
			"v2": []byte("22222222222222222222222222222222"),
		},
	}
	values := digests(keyring, "purpose", "value")

	if len(values) != 2 || values[0].version != "v2" || values[1].version != "v1" {
		t.Fatalf("digests = %#v", values)
	}
	if string(values[0].sum) == string(values[1].sum) {
		t.Fatal("rotated keys produced the same digest")
	}
}

func TestExistingIdempotencyRejectsChangedRequest(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	resourceID := uuid.MustParse("019f0000-0000-7000-8000-000000000001")
	row := generated.PlatformIdempotencyRecord{
		RequestHash:        []byte("first"),
		State:              "completed",
		ResponseStatus:     int32Pointer(200),
		ResponseResourceID: pgtype.UUID{Bytes: [16]byte(resourceID), Valid: true},
		ExpiresAt:          pgtype.Timestamptz{Time: now.Add(time.Hour), Valid: true},
	}

	_, err := existingIdempotency(row, []byte("second"), now)
	if !errors.Is(err, errIdempotencyConflict) {
		t.Fatalf("existingIdempotency() error = %v", err)
	}
}

func TestExistingIdempotencyProcessingIsTyped(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	row := generated.PlatformIdempotencyRecord{
		RequestHash: []byte("same"),
		State:       "processing",
		ExpiresAt:   pgtype.Timestamptz{Time: now.Add(time.Hour), Valid: true},
	}
	_, err := existingIdempotency(row, []byte("same"), now)
	var processing *idempotency.ProcessingError
	if !errors.As(err, &processing) || processing.RetryAfter < time.Second {
		t.Fatalf("existingIdempotency() error = %v", err)
	}
}

type idempotencyQueriesStub struct {
	generated.Querier
	row         generated.PlatformIdempotencyRecord
	deleteCalls int
}

func (f *idempotencyQueriesStub) LockIdempotencyKey(
	context.Context,
	generated.LockIdempotencyKeyParams,
) (generated.PlatformIdempotencyRecord, error) {
	return f.row, nil
}

func (f *idempotencyQueriesStub) CleanupExpiredOperationalRecords(
	context.Context,
	generated.CleanupExpiredOperationalRecordsParams,
) (generated.CleanupExpiredOperationalRecordsRow, error) {
	f.deleteCalls++
	return generated.CleanupExpiredOperationalRecordsRow{}, nil
}

func TestRunIdempotentNeverExecutesWorkerCleanup(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	request := struct {
		Value string `json:"value"`
	}{"same"}
	keyring := config.KeyringConfig{
		CurrentVersion: "v1",
		Keys:           map[string][]byte{"v1": []byte("key-material-is-at-least-32-bytes")},
	}
	requestHash, err := idempotency.RequestHash(keyring.Keys["v1"], request)
	if err != nil {
		t.Fatalf("RequestHash() error = %v", err)
	}
	queries := &idempotencyQueriesStub{row: generated.PlatformIdempotencyRecord{
		RequestHash: requestHash[:],
		State:       "processing",
		ExpiresAt:   pgtype.Timestamptz{Time: now.Add(time.Hour), Valid: true},
	}}
	subject := &Store{core: config.CoreConfig{
		ActorKeys: keyring, IdempotencyKeys: keyring,
	}}
	_, err = subject.runIdempotent(
		context.Background(),
		queries,
		idempotencySpec{
			actorValue: "actor", operation: idempotency.OperationCreateStay,
			resourceID: uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
			key:        "idempotency-key-1234", request: request, now: now,
		},
		func() (storedMutation, error) {
			t.Fatal("mutation work executed for processing reservation")
			return storedMutation{}, nil
		},
	)
	if !errors.Is(err, idempotency.ErrProcessing) {
		t.Fatalf("runIdempotent() error = %v", err)
	}
	if queries.deleteCalls != 0 {
		t.Fatalf("DeleteExpiredIdempotencyKeys calls = %d", queries.deleteCalls)
	}
}

func TestReadOnlyTransactionOptionsUseRepeatableRead(t *testing.T) {
	t.Parallel()

	options := readOnlyTransactionOptions()
	if options.IsoLevel != pgx.RepeatableRead || options.AccessMode != pgx.ReadOnly {
		t.Fatalf("readOnlyTransactionOptions() = %#v", options)
	}
}

func int32Pointer(value int32) *int32 {
	return &value
}
