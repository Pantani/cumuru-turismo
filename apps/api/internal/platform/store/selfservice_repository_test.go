package store

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/audit"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/outbox"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/Pantani/cumuru/apps/api/internal/stay"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var posterNow = time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

func posterRow() generated.GetAccommodationInviteForCapabilityRow {
	return generated.GetAccommodationInviteForCapabilityRow{
		ExpiresAt:           pgtype.Timestamptz{Time: posterNow.Add(time.Hour), Valid: true},
		MaxUses:             nil,
		UseCount:            412,
		AccommodationStatus: generated.CoreAccommodationStatus("active"),
	}
}

// A null max_uses means unlimited, which is the whole point of a wall poster.
// Comparing against the pointer without a nil test would declare it consumed on
// the first submission.
func TestUnlimitedPosterStaysUsable(t *testing.T) {
	t.Parallel()

	if !usablePoster(posterRow(), posterNow) {
		t.Fatal("an unlimited poster was read as consumed")
	}
}

func TestPosterRefusesEveryTerminalState(t *testing.T) {
	t.Parallel()

	limit := int32(3)
	tests := map[string]func(*generated.GetAccommodationInviteForCapabilityRow){
		"expired": func(row *generated.GetAccommodationInviteForCapabilityRow) {
			row.ExpiresAt = pgtype.Timestamptz{Time: posterNow, Valid: true}
		},
		"absent expiry": func(row *generated.GetAccommodationInviteForCapabilityRow) {
			row.ExpiresAt = pgtype.Timestamptz{}
		},
		"revoked": func(row *generated.GetAccommodationInviteForCapabilityRow) {
			row.RevokedAt = pgtype.Timestamptz{Time: posterNow, Valid: true}
		},
		// N-06: a poster of a suspended or closed accommodation answers exactly
		// like an unknown one.
		"suspended accommodation": func(row *generated.GetAccommodationInviteForCapabilityRow) {
			row.AccommodationStatus = generated.CoreAccommodationStatus("suspended")
		},
		"closed accommodation": func(row *generated.GetAccommodationInviteForCapabilityRow) {
			row.AccommodationStatus = generated.CoreAccommodationStatus("closed")
		},
		"limited and exhausted": func(row *generated.GetAccommodationInviteForCapabilityRow) {
			row.MaxUses = &limit
			row.UseCount = 3
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			row := posterRow()
			mutate(&row)
			if usablePoster(row, posterNow) {
				t.Fatal("a poster in a terminal state was accepted")
			}
		})
	}
}

func TestLimitedPosterAcceptsUpToItsLimit(t *testing.T) {
	t.Parallel()

	limit := int32(3)
	row := posterRow()
	row.MaxUses = &limit
	row.UseCount = 2
	if !usablePoster(row, posterNow) {
		t.Fatal("a poster below its limit was refused")
	}
}

func activationRow() generated.GetActivationCapabilityRow {
	return generated.GetActivationCapabilityRow{
		ExpiresAt:           pgtype.Timestamptz{Time: posterNow.Add(time.Hour), Valid: true},
		AccommodationStatus: generated.CoreAccommodationStatus("active"),
	}
}

// N-42 and N-43: consumed, revoked and expired all answer the same way, and so
// does a capability whose accommodation is no longer active.
func TestActivationCapabilityIsSingleUseAndRevocable(t *testing.T) {
	t.Parallel()

	if !usableCapability(activationRow(), posterNow) {
		t.Fatal("a live capability was refused")
	}
	tests := map[string]func(*generated.GetActivationCapabilityRow){
		"consumed": func(row *generated.GetActivationCapabilityRow) {
			row.ConsumedAt = pgtype.Timestamptz{Time: posterNow, Valid: true}
		},
		"revoked": func(row *generated.GetActivationCapabilityRow) {
			row.RevokedAt = pgtype.Timestamptz{Time: posterNow, Valid: true}
		},
		"expired": func(row *generated.GetActivationCapabilityRow) {
			row.ExpiresAt = pgtype.Timestamptz{Time: posterNow, Valid: true}
		},
		"suspended accommodation": func(row *generated.GetActivationCapabilityRow) {
			row.AccommodationStatus = generated.CoreAccommodationStatus("suspended")
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			row := activationRow()
			mutate(&row)
			if usableCapability(row, posterNow) {
				t.Fatal("a capability in a terminal state was accepted")
			}
		})
	}
}

// N-25 at the repository: holding the edit permission is not enough, and an
// operator is refused even on an active accommodation.
func TestApprovalNeedsItsOwnOperationAndAManagerRole(t *testing.T) {
	t.Parallel()

	manager := generated.LockStayForCommandRow{
		AccommodationStatus: generated.CoreAccommodationStatus("active"),
		ActorRole:           string(accommodation.RoleManager),
	}
	if err := approvalAllowed(manager); err != nil {
		t.Fatalf("approvalAllowed(manager) = %v", err)
	}
	operator := manager
	operator.ActorRole = string(accommodation.RoleOperator)
	if err := approvalAllowed(operator); err != stay.ErrForbidden {
		t.Fatalf("approvalAllowed(operator) = %v, want ErrForbidden", err)
	}
	// N-26: the operation only exists on an active accommodation.
	for _, status := range []string{"suspended", "closed", "pending_review"} {
		inactive := manager
		inactive.AccommodationStatus = generated.CoreAccommodationStatus(status)
		if err := approvalAllowed(inactive); err != stay.ErrConflict {
			t.Fatalf("approvalAllowed(%s) = %v, want ErrConflict", status, err)
		}
	}
}

// The capability lives in the fragment, so the server-built URL must carry it
// after the hash and never in the path or the query.
func TestFragmentURLKeepsTheTokenOutOfPathAndQuery(t *testing.T) {
	t.Parallel()

	base, err := url.Parse("https://host.invalid/i")
	if err != nil {
		t.Fatal(err)
	}
	const token = "abcDEF-_0123456789"
	built, err := url.Parse(fragmentURL(base, token))
	if err != nil {
		t.Fatalf("fragmentURL() produced %q", fragmentURL(base, token))
	}
	if built.Fragment != token {
		t.Fatalf("fragment = %q, want the token", built.Fragment)
	}
	if built.Path != "/i" || built.RawQuery != "" {
		t.Fatalf("path = %q, query = %q; the token leaked out of the fragment",
			built.Path, built.RawQuery)
	}
	if fragmentURL(nil, token) != "" {
		t.Fatal("a missing base produced a URL")
	}
}

func auditTestStore() *Store {
	keyring := config.KeyringConfig{
		CurrentVersion: "actor-v1",
		Keys:           map[string][]byte{"actor-v1": []byte("actor-key-material-is-at-least-32-bytes")},
	}
	return &Store{core: config.CoreConfig{ActorKeys: keyring}}
}

// The sweep event must validate before it is ever inserted. It failed to, on
// two counts, until ExpirePendingSelfServiceStays started projecting the
// organization: audit.Event demands an actor digest and a non-null
// organization_id for a stay. A failure there aborts the whole transaction and
// the purge never happens, which is the retention hole E-05 describes — so this
// test guards the purge, not just the audit row.
func TestExpirySweepProducesAValidAuditEvent(t *testing.T) {
	t.Parallel()

	spec := expirySweepEvent(
		uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		uuid.MustParse("019f0000-0000-7000-8000-000000000002"),
		7, "019f0000-0000-7000-8000-0000000000ff", posterNow,
	)
	event, err := auditTestStore().newAuditEvent(spec)
	if err != nil {
		t.Fatalf("newAuditEvent() error = %v", err)
	}
	if event.ActorType != audit.ActorSystem {
		t.Fatalf("actor type = %q, want the system", event.ActorType)
	}
	if event.Action != audit.ActionStayApprovalExpired {
		t.Fatalf("action = %q", event.Action)
	}
	if event.OrganizationID == uuid.Nil {
		t.Fatal("the organization did not reach the audit event")
	}
}

// The exact regression: without the organization the event is refused, so the
// projection is load-bearing and not decorative.
func TestExpirySweepEventIsRefusedWithoutAnOrganization(t *testing.T) {
	t.Parallel()

	spec := expirySweepEvent(
		uuid.MustParse("019f0000-0000-7000-8000-000000000001"), uuid.Nil,
		7, "019f0000-0000-7000-8000-0000000000ff", posterNow,
	)
	if _, err := auditTestStore().newAuditEvent(spec); err == nil {
		t.Fatal("an audit event without an organization was accepted")
	}
}

// The sweep has no person behind it, and the record must say so rather than
// borrowing somebody else's identity.
func TestExpirySweepActorIsTheProcessAndNotAPerson(t *testing.T) {
	t.Parallel()

	first := expirySweepEvent(
		uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		uuid.MustParse("019f0000-0000-7000-8000-000000000002"),
		7, "019f0000-0000-7000-8000-0000000000ff", posterNow,
	)
	second := expirySweepEvent(
		uuid.MustParse("019f0000-0000-7000-8000-000000000009"),
		uuid.MustParse("019f0000-0000-7000-8000-000000000002"),
		3, "019f0000-0000-7000-8000-0000000000ff", posterNow,
	)
	if first.actorIssuer != systemActorIssuer || first.actorSubject != expirySweepActor {
		t.Fatalf("actor = %s/%s", first.actorIssuer, first.actorSubject)
	}
	if first.actorSubject != second.actorSubject {
		t.Fatal("two rows of the same sweep reported different actors")
	}
	// One run identifier correlates every row it expired.
	if first.requestID != second.requestID {
		t.Fatal("two rows of the same sweep reported different run identifiers")
	}
}

// The idempotency record stores the body and the repository decodes it back, so
// anything the API type hides from the wire is lost on that round trip. The
// contract forbids `version` in both payloads, and the ETag is built from it —
// which is how a successful self-registration came to answer ETag "0".
//
// The integration test caught this against a real database; this is the cheap
// guard that keeps it caught.
func TestReplayPayloadsCarryTheVersionTheWireOmits(t *testing.T) {
	t.Parallel()

	accepted := stay.SelfRegistrationAccepted{
		SubmissionID: uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		StayID:       uuid.MustParse("019f0000-0000-7000-8000-000000000002"),
		Status:       "accepted", StayStatus: stay.StatusPreRegistered,
		ApprovalState: stay.ApprovalPending, Version: 7,
	}
	restored := decodeSelfRegistrationReplay(t, selfRegistrationReplay(accepted))
	if restored != accepted {
		t.Fatalf("round trip = %#v, want %#v", restored, accepted)
	}

	status := stay.AccommodationInviteStatus{
		InviteID: uuid.MustParse("019f0000-0000-7000-8000-000000000003"),
		UseCount: 3, Version: 4,
	}
	restoredStatus := decodePosterStatusReplay(t, posterStatusReplay(status))
	if restoredStatus.Version != status.Version ||
		restoredStatus.InviteID != status.InviteID {
		t.Fatalf("round trip = %#v, want %#v", restoredStatus, status)
	}
}

// The wire form must stay exactly what the contract allows: no version key, in
// either payload, because both declare additionalProperties false.
func TestApiPayloadsDoNotEmitTheInternalVersion(t *testing.T) {
	t.Parallel()

	payloads := map[string]any{
		"self registration": stay.SelfRegistrationAccepted{Version: 7},
		"poster status":     stay.AccommodationInviteStatus{Version: 4},
	}
	for name, payload := range payloads {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		if _, present := decoded["version"]; present {
			t.Fatalf("%s leaked version to the wire: %s", name, encoded)
		}
	}
}

func decodeSelfRegistrationReplay(t *testing.T, payload any) stay.SelfRegistrationAccepted {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal replay: %v", err)
	}
	var restored stay.SelfRegistrationAccepted
	if err := decodeSelfRegistration(encoded, &restored); err != nil {
		t.Fatalf("decode replay: %v", err)
	}
	return restored
}

func decodePosterStatusReplay(t *testing.T, payload any) stay.AccommodationInviteStatus {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal replay: %v", err)
	}
	var restored stay.AccommodationInviteStatus
	if err := decodePosterStatus(encoded, &restored); err != nil {
		t.Fatalf("decode replay: %v", err)
	}
	return restored
}

// The outbox has a unique index on
// (aggregate_type, aggregate_id, aggregate_version, event_type). Pointing a
// poster event at the accommodation made every rotation reuse the same tuple,
// because issuing a poster does not bump the accommodation version — so the
// second rotation aborted the transaction and left the first poster active,
// which is the opposite of what ADR-039 promises.
//
// The aggregate of a poster event is the poster.
func TestPosterEventsDoNotShareAnOutboxIdentity(t *testing.T) {
	t.Parallel()

	accommodationID := uuid.MustParse("019f0000-0000-7000-8000-000000000001")
	first := posterEventSpec(posterEvent{
		organizationID:  uuid.MustParse("019f0000-0000-7000-8000-000000000002"),
		accommodationID: accommodationID,
		inviteID:        uuid.MustParse("019f0000-0000-7000-8000-00000000000a"),
		requestID:       "request-000000000001",
		action:          audit.ActionAccommodationInvited,
		event:           outbox.EventAccommodationInvited,
	})
	second := first
	second.aggregateID = uuid.MustParse("019f0000-0000-7000-8000-00000000000b")

	if outboxIdentity(t, first) == outboxIdentity(t, second) {
		t.Fatal("two rotations produced the same outbox identity")
	}
	// The audit trail still names the accommodation: that is the entity the
	// operator acted on, and audit has no such uniqueness to trip over.
	if first.entityID != accommodationID {
		t.Fatalf("audit entity = %s, want the accommodation", first.entityID)
	}
}

// Revoking the poster that was just issued must not collide with issuing it:
// same aggregate, same version, different event type.
func TestPosterIssueAndRevokeAreDistinctOutboxIdentities(t *testing.T) {
	t.Parallel()

	base := posterEvent{
		organizationID:  uuid.MustParse("019f0000-0000-7000-8000-000000000002"),
		accommodationID: uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		inviteID:        uuid.MustParse("019f0000-0000-7000-8000-00000000000a"),
		requestID:       "request-000000000001",
		action:          audit.ActionAccommodationInvited,
		event:           outbox.EventAccommodationInvited,
	}
	revoked := base
	revoked.action = audit.ActionAccommodationRevoked
	revoked.event = outbox.EventAccommodationInviteRevoked

	if outboxIdentity(t, posterEventSpec(base)) ==
		outboxIdentity(t, posterEventSpec(revoked)) {
		t.Fatal("issue and revoke share an outbox identity")
	}
}

func outboxIdentity(t *testing.T, spec eventSpec) string {
	t.Helper()
	event, err := newOutboxEvent(spec)
	if err != nil {
		t.Fatalf("newOutboxEvent() error = %v", err)
	}
	return fmt.Sprintf("%s|%s|%d|%s",
		event.AggregateType, event.AggregateID, event.AggregateVersion, event.Type)
}
