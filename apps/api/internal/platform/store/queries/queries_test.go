package queries

import (
	"os"
	"strings"
	"testing"
)

func TestCoreQueriesKeepAuthorizationAndLocksInSQL(t *testing.T) {
	t.Parallel()

	assertSQLContains(t, "accommodation.sql",
		"-- name: ListAccessibleAccommodations",
		"-- name: GetAccessibleAccommodation",
		"-- name: UpdateAccommodation",
		"-- name: LockMembershipSetForManager",
		"m.oidc_issuer = sqlc.arg(oidc_issuer)",
		"m.oidc_subject = sqlc.arg(oidc_subject)",
		"FOR UPDATE OF target",
	)
	assertSQLContains(t, "own_performance.sql",
		"-- name: ListAccommodationObservedPresence",
		"-- name: SummarizeVillageReporting",
		"m.oidc_issuer = sqlc.arg(oidc_issuer)",
		"m.oidc_subject = sqlc.arg(oidc_subject)",
		"m.active = true",
	)
	assertSQLContains(t, "stay.sql",
		"-- name: CreateStay",
		"-- name: ListAccessibleStays",
		"-- name: GetAccessibleStay",
		"-- name: LockStayForCommand",
		"-- name: ConsumeInvite",
		"m.oidc_issuer = sqlc.arg(oidc_issuer)",
		"m.oidc_subject = sqlc.arg(oidc_subject)",
		"FOR UPDATE OF s",
	)
}

// A rota da lista é aberta: o recorte precisa estar na consulta, e não no
// chamador, senão o primeiro handler com defeito publica o cadastro inteiro.
func TestPublicDirectoryQueryFiltersAndProjectsInSQL(t *testing.T) {
	t.Parallel()

	query := namedQuery(
		t, readSQL(t, "accommodation.sql"), "ListPublicAccommodationDirectory",
	)
	assertContainsAll(t, query,
		"a.public_listing_enabled = true",
		"a.status = 'active'",
		"a.public_contact_phone",
		"ORDER BY a.name, a.id",
	)
	assertSQLDoesNotContain(t, query,
		"cadastur_id",
		"organization_id",
		"oidc_subject",
		"public_listing_consented_at",
	)
}

func TestCorePlatformQueriesAreClosedAndMinimal(t *testing.T) {
	t.Parallel()

	assertSQLContains(t, "idempotency.sql",
		"-- name: ClaimIdempotencyKey",
		"-- name: LockIdempotencyKey",
		"-- name: CompleteIdempotencyKey",
		"-- name: IncrementRateLimit",
	)
	auditQuery := namedQuery(t, readSQL(t, "audit.sql"), "InsertAuditEvent")
	assertContainsAll(t, auditQuery,
		"-- name: InsertAuditEvent :exec",
		"'{}'::jsonb",
	)
	assertSQLDoesNotContain(t, auditQuery, "RETURNING")

	outboxQuery := namedQuery(t, readSQL(t, "outbox.sql"), "InsertOutboxEvent")
	assertContainsAll(t, outboxQuery,
		"-- name: InsertOutboxEvent :exec",
		"aggregate_version",
		"event_type",
	)
	assertSQLDoesNotContain(t, outboxQuery, "RETURNING")
	if strings.Contains(outboxQuery, "payload") {
		t.Fatal("outbox query must not accept a payload")
	}
	backlogQuery := namedQuery(t, readSQL(t, "outbox.sql"), "GetOutboxBacklog")
	assertContainsAll(t, backlogQuery,
		"-- name: GetOutboxBacklog :one",
		"count(*)::bigint AS pending_events",
		"min(occurred_at)::timestamptz AS oldest_pending_at",
		"processed_at IS NULL",
	)
	assertSQLDoesNotContain(t, backlogQuery,
		"aggregate_id",
		"lease_owner",
		"last_error_code",
		"payload",
	)
}

func TestMembershipMutationsRejectClosedAccommodationInSQL(t *testing.T) {
	t.Parallel()

	content := readSQL(t, "accommodation.sql")
	lockQuery := namedQuery(t, content, "LockMembershipSetForManager")
	assertContainsAll(t, lockQuery,
		"JOIN core.accommodations AS accommodation",
		"accommodation.id = target.accommodation_id",
		"accommodation.status <> 'closed'",
	)

	updateQuery := namedQuery(t, content, "UpdateAccommodationMembership")
	assertContainsAll(t, updateQuery,
		"FROM core.accommodations AS accommodation",
		"accommodation.id = target.accommodation_id",
		"accommodation.status <> 'closed'",
	)
}

func TestUpdateStayRequiresActiveAccommodationInSQL(t *testing.T) {
	t.Parallel()

	query := namedQuery(t, readSQL(t, "stay.sql"), "UpdateStay")
	assertContainsAll(t, query,
		"FROM core.accommodations AS accommodation",
		"accommodation.id = s.accommodation_id",
		"accommodation.status = 'active'",
	)
}

func TestFinalizeInviteSubmissionIsCapabilityScoped(t *testing.T) {
	t.Parallel()

	query := namedQuery(t, readSQL(t, "stay.sql"), "FinalizeInviteSubmission")
	assertContainsAll(t, query,
		"UPDATE core.stays AS s",
		"expected_guest_count = sqlc.arg(expected_guest_count)::integer",
		"status = 'pre_registered'",
		"version = s.version + 1",
		"i.id = sqlc.arg(invite_id)",
		"i.stay_id = s.id",
		"i.token_hmac = sqlc.arg(token_hmac)",
		"i.revoked_at IS NULL",
		"i.expires_at > sqlc.arg(finalized_at)",
		"i.use_count = i.max_uses",
		"s.id = sqlc.arg(stay_id)",
		"s.version = sqlc.arg(expected_version)",
		"s.status IN ('draft', 'invited')",
		"RETURNING",
		"s.updated_at",
	)
	for _, forbidden := range []string{"oidc_issuer", "oidc_subject"} {
		if strings.Contains(query, forbidden) {
			t.Errorf("FinalizeInviteSubmission must not require %q", forbidden)
		}
	}
}

func TestGetStayGroupSubmissionIsTenantScopedAndMinimal(t *testing.T) {
	t.Parallel()

	query := namedQuery(t, readSQL(t, "stay.sql"), "GetStayGroupSubmission")
	assertContainsAll(t, query,
		"SELECT",
		"gs.id",
		"gs.stay_id",
		"gs.privacy_notice_version",
		"gs.collection_channel",
		"gs.submitted_at",
		"FROM core.group_submissions AS gs",
		"JOIN core.stays AS s",
		"ON s.id = gs.stay_id",
		"gs.stay_id = sqlc.arg(stay_id)",
		"m.accommodation_id = s.accommodation_id",
		"m.oidc_issuer = sqlc.arg(oidc_issuer)",
		"m.oidc_subject = sqlc.arg(oidc_subject)",
		"m.active = true",
	)
	for _, forbidden := range []string{"ORDER BY", "LIMIT"} {
		if strings.Contains(query, forbidden) {
			t.Errorf("GetStayGroupSubmission must rely on UNIQUE(stay_id), found %q", forbidden)
		}
	}
}

func assertSQLContains(t *testing.T, path string, snippets ...string) {
	t.Helper()
	assertContainsAll(t, readSQL(t, path), snippets...)
}

func assertSQLDoesNotContain(t *testing.T, query string, snippets ...string) {
	t.Helper()
	for _, snippet := range snippets {
		if strings.Contains(query, snippet) {
			t.Errorf("SQL must not contain %q", snippet)
		}
	}
}

func namedQuery(t *testing.T, content, name string) string {
	t.Helper()
	start := strings.Index(content, "-- name: "+name+" ")
	if start < 0 {
		t.Fatalf("SQL query %q is missing", name)
	}
	remaining := content[start+1:]
	end := strings.Index(remaining, "\n-- name: ")
	if end < 0 {
		return content[start:]
	}
	return content[start : start+1+end]
}

func readSQL(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(content)
}

func assertContainsAll(t *testing.T, content string, snippets ...string) {
	t.Helper()
	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Errorf("SQL missing %q", snippet)
		}
	}
}

// The erasure of the contact lives in the SQL itself rather than in an
// application step: the decision constraint refuses 'rejected' and 'expired'
// rows still carrying a name, e-mail or phone, so the purge cannot be forgotten
// by some path through the code (ADR-042).
func TestAccessRequestDecisionsEraseTheContactInSQL(t *testing.T) {
	t.Parallel()

	content := readSQL(t, "access_request.sql")
	for _, name := range []string{
		"RejectAccommodationAccessRequest",
		"ExpireAccommodationAccessRequests",
	} {
		assertContainsAll(t, namedQuery(t, content, name),
			"contact_name = NULL",
			"contact_email = NULL",
			"contact_phone = NULL",
		)
	}
}

// The worker sweep stays inside the column-by-column grant: it does not read the
// contact it erases, does not read version — which is why version is not
// incremented — and returns only the id. Without a batch and without SKIP
// LOCKED, one cycle would hold the whole table.
func TestAccessRequestExpirySweepStaysInsideTheWorkerGrant(t *testing.T) {
	t.Parallel()

	sweep := namedQuery(t, readSQL(t, "access_request.sql"),
		"ExpireAccommodationAccessRequests")
	assertContainsAll(t, sweep,
		"candidate.approval_state = 'pending'",
		"candidate.expires_at < sqlc.arg(cutoff)",
		"LIMIT sqlc.arg(batch_size)",
		"FOR UPDATE SKIP LOCKED",
		"RETURNING request.id",
	)
	assertSQLDoesNotContain(t, sweep,
		"version = request.version + 1",
		"decided_at",
		"decided_by_oidc_issuer",
	)
}

// Every decision is gated by the expected version and by the pending state:
// without both conditions, approving an already rejected request would take
// effect.
func TestAccessRequestDecisionsDemandThePendingStateAndTheVersion(t *testing.T) {
	t.Parallel()

	content := readSQL(t, "access_request.sql")
	for _, name := range []string{
		"ApproveAccommodationAccessRequest",
		"RejectAccommodationAccessRequest",
	} {
		assertContainsAll(t, namedQuery(t, content, name),
			"request.version = sqlc.arg(expected_version)",
			"request.approval_state = 'pending'",
			"version = request.version + 1",
		)
	}
}
