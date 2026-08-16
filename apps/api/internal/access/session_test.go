package access_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/access"
)

type stubLookup struct {
	digest    []byte
	principal access.Principal
}

func (s *stubLookup) LookupSession(_ context.Context, digest []byte) (access.Principal, error) {
	if string(digest) != string(s.digest) {
		return access.Principal{}, access.ErrInvalidToken
	}
	return s.principal, nil
}

type stubVerifier struct {
	token     string
	principal access.Principal
}

func (s *stubVerifier) Verify(_ context.Context, token string) (access.Principal, error) {
	if token != s.token {
		return access.Principal{}, access.ErrInvalidToken
	}
	return s.principal, nil
}

func TestNewSessionTokenIsPrefixedAndDigested(t *testing.T) {
	t.Parallel()
	token, digest, err := access.NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken() error = %v", err)
	}
	if !strings.HasPrefix(token, access.SessionTokenPrefix) {
		t.Fatalf("NewSessionToken() = %q, want the session prefix", token)
	}
	expected := sha256.Sum256([]byte(token))
	if string(digest) != string(expected[:]) {
		t.Fatal("NewSessionToken() digest does not match SHA-256 of the token")
	}
	if len(digest) != sha256.Size {
		t.Fatalf("digest length = %d, want %d", len(digest), sha256.Size)
	}
}

func TestNewSessionTokenIsUnique(t *testing.T) {
	t.Parallel()
	first, _, err := access.NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken() error = %v", err)
	}
	second, _, err := access.NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken() error = %v", err)
	}
	if first == second {
		t.Fatal("NewSessionToken() repeated a token")
	}
}

func TestSessionTokenDigestIgnoresForeignCredentials(t *testing.T) {
	t.Parallel()
	if _, ok := access.SessionTokenDigest("eyJhbGciOiJSUzI1NiJ9.body.sig"); ok {
		t.Fatal("SessionTokenDigest() claimed an OIDC token")
	}
}

func TestSessionVerifierResolvesThroughLookup(t *testing.T) {
	t.Parallel()
	token, digest, err := access.NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken() error = %v", err)
	}
	want := access.NewPrincipal(access.LocalSessionIssuer, "account", []string{"stays:write"})
	verifier, err := access.NewSessionVerifier(&stubLookup{digest: digest, principal: want})
	if err != nil {
		t.Fatalf("NewSessionVerifier() error = %v", err)
	}
	got, err := verifier.Verify(t.Context(), token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if got.Issuer != access.LocalSessionIssuer || !got.HasScope("stays:write") {
		t.Fatalf("Verify() = %+v, want the local session principal", got)
	}
}

func TestSessionVerifierRequiresLookup(t *testing.T) {
	t.Parallel()
	if _, err := access.NewSessionVerifier(nil); err == nil {
		t.Fatal("NewSessionVerifier(nil) accepted a missing lookup")
	}
}

func newRoutingChain(t *testing.T, digest []byte) access.Verifier {
	t.Helper()
	session, err := access.NewSessionVerifier(&stubLookup{
		digest:    digest,
		principal: access.NewPrincipal(access.LocalSessionIssuer, "local", nil),
	})
	if err != nil {
		t.Fatalf("NewSessionVerifier() error = %v", err)
	}
	federated := &stubVerifier{
		token:     "federated-token",
		principal: access.NewPrincipal("https://oidc.invalid/local", "federated", nil),
	}
	chain, err := access.NewChainVerifier(session, federated)
	if err != nil {
		t.Fatalf("NewChainVerifier() error = %v", err)
	}
	return chain
}

func TestChainVerifierRoutesByPrefix(t *testing.T) {
	t.Parallel()
	token, digest, err := access.NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken() error = %v", err)
	}
	chain := newRoutingChain(t, digest)

	tests := map[string]struct {
		token      string
		wantIssuer string
	}{
		"session token":   {token, access.LocalSessionIssuer},
		"federated token": {"federated-token", "https://oidc.invalid/local"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := chain.Verify(t.Context(), test.token)
			if err != nil {
				t.Fatalf("Verify() error = %v", err)
			}
			if got.Issuer != test.wantIssuer {
				t.Fatalf("Verify() issuer = %q, want %q", got.Issuer, test.wantIssuer)
			}
		})
	}
}

// A session token must never fall through to the OIDC verifier, otherwise a
// revoked session could be revalidated by an unrelated credential path.
func TestChainVerifierDoesNotFallThroughForSessionTokens(t *testing.T) {
	t.Parallel()
	_, digest, err := access.NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken() error = %v", err)
	}
	otherToken, _, err := access.NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken() error = %v", err)
	}
	session, err := access.NewSessionVerifier(&stubLookup{digest: digest})
	if err != nil {
		t.Fatalf("NewSessionVerifier() error = %v", err)
	}
	federated := &stubVerifier{
		token:     otherToken,
		principal: access.NewPrincipal("https://oidc.invalid/local", "federated", nil),
	}
	chain, err := access.NewChainVerifier(session, federated)
	if err != nil {
		t.Fatalf("NewChainVerifier() error = %v", err)
	}
	if _, err := chain.Verify(t.Context(), otherToken); !errors.Is(err, access.ErrInvalidToken) {
		t.Fatalf("Verify() error = %v, want ErrInvalidToken", err)
	}
}

func TestChainVerifierRequiresAtLeastOneVerifier(t *testing.T) {
	t.Parallel()
	if _, err := access.NewChainVerifier(nil, nil); err == nil {
		t.Fatal("NewChainVerifier(nil, nil) accepted an empty chain")
	}
}
