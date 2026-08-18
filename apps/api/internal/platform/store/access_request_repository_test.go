package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/accessrequest"
	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var accessRequestNow = time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)

const accessRequestID = "019f0000-0000-7000-8000-0000000000a1"

type accessRequestQueriesStub struct {
	generated.Querier
	lockRow    generated.LockAccommodationAccessRequestForDecisionRow
	lockErr    error
	spendRows  int64
	spendErr   error
	spendCalls int
}

func (s *accessRequestQueriesStub) LockAccommodationAccessRequestForDecision(
	context.Context,
	pgtype.UUID,
) (generated.LockAccommodationAccessRequestForDecisionRow, error) {
	return s.lockRow, s.lockErr
}

func (s *accessRequestQueriesStub) SpendProofOfWorkChallenge(
	_ context.Context,
	_ generated.SpendProofOfWorkChallengeParams,
) (int64, error) {
	s.spendCalls++
	return s.spendRows, s.spendErr
}

func accessRequestKeyring(version string) config.KeyringConfig {
	return config.KeyringConfig{
		CurrentVersion: version,
		Keys:           map[string][]byte{version: []byte(version + "-key-with-at-least-32-bytes-here")},
	}
}

func newAccessRequestRepository(
	t *testing.T,
	queries generated.Querier,
) *AccessRequestRepository {
	t.Helper()
	platformStore := New(queries, time.Second)
	platformStore.now = func() time.Time { return accessRequestNow }
	platformStore.core = config.CoreConfig{
		RateLimitWindow: time.Minute,
		RateLimitKeys:   accessRequestKeyring("rate-limit-v1"),
		ActorKeys:       accessRequestKeyring("actor-v1"),
		IdempotencyKeys: accessRequestKeyring("idempotency-v1"),
	}
	platformStore.selfService = config.SelfServiceConfig{
		Enabled: true, ChallengeTTL: time.Minute,
		DifficultyBase: 1, DifficultyCeiling: 2, DifficultyRequestsPerBit: 100,
		ProofOfWorkKeys: accessRequestKeyring("pow-v1"),
	}
	repository, err := NewAccessRequestRepository(platformStore)
	if err != nil {
		t.Fatalf("NewAccessRequestRepository() error = %v", err)
	}
	return repository
}

func sampleCreateCommand() accessrequest.CreateCommand {
	return accessrequest.CreateCommand{
		ClientSubmissionID: uuid.MustParse(accessRequestID),
		AccommodationName:  "Pousada do Descobrimento",
		Category:           accommodation.CategoryFormalLodging, Capacity: 12,
		ContactName: "Responsavel", ContactEmail: "contato@pousada.invalid",
		CityLabel: "Prado", StateCode: "BA",
	}
}

// The 30-day deadline is born at creation rather than from a column DEFAULT:
// the worker sweep reads exactly this value to decide what has expired.
func TestCreateParamsCarryTheThirtyDayDeadline(t *testing.T) {
	t.Parallel()

	params := createAccessRequestParams(
		sampleCreateCommand(), uuid.MustParse(accessRequestID), accessRequestNow,
	)
	want := accessRequestNow.Add(30 * 24 * time.Hour)
	if !params.ExpiresAt.Time.Equal(want) {
		t.Fatalf("ExpiresAt = %v, want %v", params.ExpiresAt.Time, want)
	}
	if params.ContactPhone != nil {
		t.Fatalf("an absent phone became %q instead of NULL", *params.ContactPhone)
	}
}

// The second pending request from the same address collides on the partial
// unique index. The collision is a business answer — 409 — and never a 500:
// whoever re-sends because they got no reply needs to understand the request is
// already in the queue.
func TestSecondPendingRequestForTheSameEmailIsAConflict(t *testing.T) {
	t.Parallel()

	duplicate := &pgconn.PgError{Code: "23505"}
	if !errors.Is(accessRequestMutationError(duplicate), accessrequest.ErrConflict) {
		t.Fatal("the duplicate pending e-mail did not become a conflict")
	}
	if errors.Is(accessRequestMutationError(errors.New("boom")), accessrequest.ErrConflict) {
		t.Fatal("an unrelated failure was reported as a conflict")
	}
}

// A forged challenge never reaches the nonce book: spending before verifying
// would let an attacker consume capacity just by sending garbage.
func TestForgedProofOfWorkIsRefusedBeforeTheSpend(t *testing.T) {
	t.Parallel()

	queries := &accessRequestQueriesStub{spendRows: 1}
	repository := newAccessRequestRepository(t, queries)
	err := repository.spendChallenge(context.Background(), queries,
		accessrequest.ProofOfWorkAnswer{Challenge: "forjado", Solution: "x"},
		accessRequestNow,
	)
	if !errors.Is(err, accessrequest.ErrInvalidInput) {
		t.Fatalf("spendChallenge() error = %v", err)
	}
	if queries.spendCalls != 0 {
		t.Fatal("a forged challenge reached the nonce book")
	}
}

// Replaying the same challenge is a conflict on the book's primary key, and it
// answers exactly like an invalid challenge so the route does not become an
// oracle.
func TestSpentProofOfWorkIsRefusedOnReplay(t *testing.T) {
	t.Parallel()

	queries := &accessRequestQueriesStub{spendRows: 0}
	repository := newAccessRequestRepository(t, queries)
	answer := solvedChallenge(t, repository)
	err := repository.spendChallenge(
		context.Background(), queries, answer, accessRequestNow,
	)
	if !errors.Is(err, accessrequest.ErrInvalidInput) {
		t.Fatalf("spendChallenge() on replay error = %v", err)
	}
	if queries.spendCalls != 1 {
		t.Fatalf("spend calls = %d, want 1", queries.spendCalls)
	}
}

// solvedChallenge solves the challenge the way the browser does: SHA-256 over
// the concatenation of challenge and solution, difficulty counted in leading
// zero bits. A disagreement of one bit would make every legitimate submission
// fail.
func solvedChallenge(
	t *testing.T,
	repository *AccessRequestRepository,
) accessrequest.ProofOfWorkAnswer {
	t.Helper()
	challenge, err := repository.challenges.Issue(
		accessRequestChallengeScope, 1, accessRequestNow,
	)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	for attempt := range 1000 {
		solution := strconv.Itoa(attempt)
		digest := sha256.Sum256([]byte(challenge.Value + solution))
		if digest[0]&0x80 == 0 {
			return accessrequest.ProofOfWorkAnswer{
				Challenge: challenge.Value, Solution: solution,
			}
		}
	}
	t.Fatal("no solution found for a one-bit challenge")
	return accessrequest.ProofOfWorkAnswer{}
}

// Absent, already decided and diverging version are three different answers, and
// none of them is a 500. Approving a rejected request and rejecting an approved
// one fail alike.
func TestDecisionLockSeparatesAbsentDecidedAndStale(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		stub *accessRequestQueriesStub
		want error
	}{
		"absent": {
			&accessRequestQueriesStub{lockErr: pgx.ErrNoRows},
			accessrequest.ErrNotFound,
		},
		"already approved": {
			&accessRequestQueriesStub{lockRow: lockedRow("approved", 2)},
			accessrequest.ErrConflict,
		},
		"already rejected": {
			&accessRequestQueriesStub{lockRow: lockedRow("rejected", 2)},
			accessrequest.ErrConflict,
		},
		"already expired": {
			&accessRequestQueriesStub{lockRow: lockedRow("expired", 2)},
			accessrequest.ErrConflict,
		},
		"stale version": {
			&accessRequestQueriesStub{lockRow: lockedRow("pending", 7)},
			accessrequest.ErrPreconditionFailed,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := lockPendingAccessRequest(
				context.Background(), test.stub, uuid.MustParse(accessRequestID), 1,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("lockPendingAccessRequest() error = %v, want %v", err, test.want)
			}
		})
	}
}

func lockedRow(
	state string,
	version int64,
) generated.LockAccommodationAccessRequestForDecisionRow {
	return generated.LockAccommodationAccessRequestForDecisionRow{
		ID: idToPG(uuid.MustParse(accessRequestID)), AccommodationName: "Pousada",
		Category: "formal_lodging", Capacity: 12,
		ApprovalState: state, Version: version,
	}
}

// The row that lost its contact comes back with the three fields null rather
// than with empty strings: that difference is what tells "it was erased" apart
// from "it arrived blank".
func TestRowMappingKeepsTheErasedContactNull(t *testing.T) {
	t.Parallel()

	reason := "abuse"
	row := generated.ListAccommodationAccessRequestsRow{
		ID: idToPG(uuid.MustParse(accessRequestID)), AccommodationName: "Pousada",
		Category: "camping", Capacity: 4, CityLabel: "Prado", StateCode: "BA",
		ApprovalState: "rejected", RejectionReasonCode: &reason, Version: 2,
		ExpiresAt: timeToPG(accessRequestNow), CreatedAt: timeToPG(accessRequestNow),
		UpdatedAt: timeToPG(accessRequestNow),
	}
	got := accessRequestFromRow(row)
	if got.ContactName != nil || got.ContactEmail != nil || got.ContactPhone != nil {
		t.Fatalf("erased contact came back as %#v", got)
	}
	if got.AccommodationID != nil {
		t.Fatal("a rejected request carried an accommodation")
	}
	if got.RejectionReasonCode == nil || *got.RejectionReasonCode != accessrequest.ReasonAbuse {
		t.Fatalf("rejection reason = %#v", got.RejectionReasonCode)
	}
}

// The page asks for one item beyond the limit, and it is the existence of that
// item that produces the cursor.
func TestPageStopsAtTheLimitAndCarriesTheCursor(t *testing.T) {
	t.Parallel()

	rows := []generated.ListAccommodationAccessRequestsRow{
		pageRow("019f0000-0000-7000-8000-0000000000b1"),
		pageRow("019f0000-0000-7000-8000-0000000000b2"),
		pageRow("019f0000-0000-7000-8000-0000000000b3"),
	}
	page := accessRequestPage(rows, 2)
	if len(page.Items) != 2 || page.NextCursor == nil {
		t.Fatalf("page = %#v", page)
	}
	if page.NextCursor.ID != page.Items[1].ID {
		t.Fatalf("cursor = %v, want %v", page.NextCursor.ID, page.Items[1].ID)
	}
	if accessRequestPage(rows[:2], 2).NextCursor != nil {
		t.Fatal("a full last page issued a cursor")
	}
	// The service refuses a limit outside 1..100, so this input never arrives
	// from the queue; the helper still has to answer it, because a limit of
	// zero used to match on the first row and index an empty slice.
	zero := accessRequestPage(rows, 0)
	if len(zero.Items) != len(rows) || zero.NextCursor != nil {
		t.Fatalf("page with a zero limit = %#v", zero)
	}
}

func pageRow(id string) generated.ListAccommodationAccessRequestsRow {
	return generated.ListAccommodationAccessRequestsRow{
		ID: idToPG(uuid.MustParse(id)), AccommodationName: "Pousada",
		Category: "other", Capacity: 2, CityLabel: "Prado", StateCode: "BA",
		ApprovalState: "pending", Version: 1,
		ExpiresAt: timeToPG(accessRequestNow), CreatedAt: timeToPG(accessRequestNow),
		UpdatedAt: timeToPG(accessRequestNow),
	}
}

// An absent filter means "every state" and becomes NULL in the SQL; the limit
// always asks for one extra item.
func TestListParamsTranslateTheOptionalStateFilter(t *testing.T) {
	t.Parallel()

	params := listAccessRequestParams(accessrequest.PageRequest{Limit: 25})
	if params.ApprovalState != nil || params.PageLimit != 26 {
		t.Fatalf("params = %#v", params)
	}
	filtered := listAccessRequestParams(accessrequest.PageRequest{
		Limit: 10, State: accessrequest.StatePending,
	})
	if filtered.ApprovalState == nil || *filtered.ApprovalState != "pending" {
		t.Fatalf("filtered params = %#v", filtered)
	}
}

// The onboarding path speaks a different error vocabulary. The translation
// preserves what the administrator needs to tell apart and lets nothing become a
// 500 by accident.
func TestOnboardingErrorsKeepTheirMeaning(t *testing.T) {
	t.Parallel()

	tests := map[error]error{
		accommodation.ErrForbidden:   accessrequest.ErrForbidden,
		accommodation.ErrConflict:    accessrequest.ErrConflict,
		accommodation.ErrUnavailable: accessrequest.ErrUnavailable,
	}
	for input, want := range tests {
		if got := accessRequestOnboardingError(input); !errors.Is(got, want) {
			t.Fatalf("accessRequestOnboardingError(%v) = %v, want %v", input, got, want)
		}
	}
}

// The sweep refuses an uncapped batch before opening a transaction: a cleanup
// with no bound would hold the whole table.
func TestAccessRequestExpiryRefusesAnUnboundedBatch(t *testing.T) {
	t.Parallel()

	platformStore := New(&accessRequestQueriesStub{}, time.Second)
	for _, batch := range []int32{0, maxExpiredRecordCleanupBatch + 1} {
		_, err := platformStore.ExpireAccommodationAccessRequests(
			context.Background(), accessRequestNow, batch,
		)
		if !errors.Is(err, ErrUnavailable) {
			t.Fatalf("batch %d error = %v", batch, err)
		}
	}
}

// The expiry has no decider: nobody decided, the clock ran out. Stamping an
// actor would invent a decision that never happened.
func TestAccessRequestExpiryEventNamesTheProcessAndNoOrganization(t *testing.T) {
	t.Parallel()

	spec := accessRequestExpiryEvent(uuid.MustParse(accessRequestID), "sweep-000000001")
	if spec.actorIssuer != systemActorIssuer || spec.actorSubject != accessRequestExpiryActor {
		t.Fatalf("actor = %s/%s", spec.actorIssuer, spec.actorSubject)
	}
	if spec.organization != uuid.Nil {
		t.Fatal("the expiry stamped an organization the request never had")
	}
}
