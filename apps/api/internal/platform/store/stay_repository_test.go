package store

import (
	"bytes"
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type presenceEventQueries struct {
	generated.Querier
	params generated.InsertOutboxEventParams
}

func (f *presenceEventQueries) InsertOutboxEvent(
	_ context.Context,
	params generated.InsertOutboxEventParams,
) error {
	f.params = params
	return nil
}

func TestCapabilityInviteIDRejectsMalformedToken(t *testing.T) {
	t.Parallel()

	if _, err := capabilityInviteID("not-a-capability"); err == nil {
		t.Fatal("capabilityInviteID() error = nil")
	}
}

func TestPresenceRecalculationEventContainsNoPayload(t *testing.T) {
	t.Parallel()

	queries := &presenceEventQueries{}
	stayID := uuid.MustParse("019f0000-0000-7000-8000-000000000001")
	if err := insertPresenceEvent(context.Background(), queries, stayID, 4); err != nil {
		t.Fatalf("insertPresenceEvent() error = %v", err)
	}
	if queries.params.AggregateID.Bytes != [16]byte(stayID) {
		t.Fatalf("aggregate ID = %x", queries.params.AggregateID.Bytes)
	}
	if queries.params.AggregateVersion != 4 {
		t.Fatalf("aggregate version = %d", queries.params.AggregateVersion)
	}
	if queries.params.EventType != "stay.presence_recalculation_requested" {
		t.Fatalf("event type = %q", queries.params.EventType)
	}
}

func TestCapabilityInviteIDExtractsUUIDWithoutTrustingMAC(t *testing.T) {
	t.Parallel()

	id := uuid.MustParse("019f0000-0000-7000-8000-000000000001")
	value := append(append([]byte(nil), id[:]...), make([]byte, 32)...)
	token := base64.RawURLEncoding.EncodeToString(value)

	got, err := capabilityInviteID(token)
	if err != nil || got != id {
		t.Fatalf("capabilityInviteID() = %s, %v", got, err)
	}
}

func TestConsumedCapabilityIsAllowedOnlyForReplayLookup(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	row := generated.GetInviteForCapabilityRow{
		ExpiresAt:           pgtype.Timestamptz{Time: now.Add(time.Hour), Valid: true},
		UseCount:            1,
		MaxUses:             1,
		StayStatus:          generated.CoreStayStatus(stay.StatusInvited),
		AccommodationStatus: generated.CoreAccommodationStatus("active"),
	}

	if validCapabilityRow(row, now, false) {
		t.Fatal("consumed capability accepted as a new request")
	}
	if !validCapabilityRow(row, now, true) {
		t.Fatal("consumed capability rejected before idempotency replay lookup")
	}
}

func TestPreRegisteredCapabilityIsAllowedOnlyForConsumedReplayLookup(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	row := generated.GetInviteForCapabilityRow{
		ExpiresAt:           pgtype.Timestamptz{Time: now.Add(time.Hour), Valid: true},
		UseCount:            1,
		MaxUses:             1,
		StayStatus:          generated.CoreStayStatus(stay.StatusPreRegistered),
		AccommodationStatus: generated.CoreAccommodationStatus("active"),
	}
	if validCapabilityRow(row, now, false) {
		t.Fatal("pre-registered capability accepted as a new request")
	}
	if !validCapabilityRow(row, now, true) {
		t.Fatal("pre-registered consumed capability rejected for replay lookup")
	}
	row.UseCount = 0
	if validCapabilityRow(row, now, true) {
		t.Fatal("pre-registered unconsumed capability accepted")
	}
}

func TestRateLimitDigestCombinesCapabilityAndGeneralizedIP(t *testing.T) {
	t.Parallel()

	key := []byte("rate-limit-test-key-is-at-least-32-bytes")
	firstToken := testCapabilityToken("019f0000-0000-7000-8000-000000000001")
	secondToken := testCapabilityToken("019f0000-0000-7000-8000-000000000002")
	first := rateLimitDigest(key, "invite_context", firstToken, "203.0.113.0/24")
	same := rateLimitDigest(key, "invite_context", firstToken, "203.0.113.0/24")
	differentCapability := rateLimitDigest(key, "invite_context", secondToken, "203.0.113.0/24")
	differentPrefix := rateLimitDigest(key, "invite_context", firstToken, "203.0.114.0/24")
	if !bytes.Equal(first, same) ||
		bytes.Equal(first, differentCapability) ||
		bytes.Equal(first, differentPrefix) {
		t.Fatal("rate limit digest does not bind capability and generalized prefix")
	}
	if bytes.Contains(first, []byte(firstToken)) || bytes.Contains(first, []byte("203.0.113.0/24")) {
		t.Fatal("rate limit digest contains raw input")
	}
}

func TestMutationProjectionAndExternalSubmissionIDAreMinimal(t *testing.T) {
	t.Parallel()

	stayID := uuid.MustParse("019f0000-0000-7000-8000-000000000001")
	clientID := uuid.MustParse("019f0000-0000-7000-8000-000000000002")
	mutation := mutationResult(stay.Record{
		ID: stayID, Status: stay.StatusPreRegistered, Version: 4,
		PlannedArrivalOn: "2026-08-01", ExpectedGuestCount: 2,
	})
	if mutation.ID != stayID || mutation.Status != stay.StatusPreRegistered || mutation.Version != 4 {
		t.Fatalf("mutationResult() = %#v", mutation)
	}
	accepted := acceptedSubmission(clientID, 4)
	if accepted.SubmissionID != clientID || accepted.Version != 4 {
		t.Fatalf("acceptedSubmission() = %#v", accepted)
	}
}

func testCapabilityToken(value string) string {
	id := uuid.MustParse(value)
	content := append(append([]byte(nil), id[:]...), make([]byte, 32)...)
	return base64.RawURLEncoding.EncodeToString(content)
}
