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
