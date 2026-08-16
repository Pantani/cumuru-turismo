package access

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

const (
	// LocalSessionIssuer identifies principals authenticated by e-mail and
	// password. It occupies the same slot as an OIDC issuer, so a local account
	// reaches core.memberships through the existing tenant resolution without a
	// second authorization path.
	LocalSessionIssuer = "https://auth.cumuru.local"

	// SessionTokenPrefix makes credential dispatch deterministic: a token either
	// addresses the local session store or the OIDC verifier, never both.
	SessionTokenPrefix = "cms_"

	sessionTokenBytes = 32
)

// SessionLookup resolves an opaque session digest into a principal. The store
// owns expiry, revocation and account status; the verifier stays stateless.
type SessionLookup interface {
	LookupSession(context.Context, []byte) (Principal, error)
}

// NewSessionToken returns the token handed to the client and the digest kept in
// the database. The raw token is never persisted.
func NewSessionToken() (string, []byte, error) {
	raw := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, errors.New("session token generation failed")
	}
	token := SessionTokenPrefix + base64.RawURLEncoding.EncodeToString(raw)
	digest := sha256.Sum256([]byte(token))
	return token, digest[:], nil
}

// SessionTokenDigest reports whether the credential addresses the local session
// store and, when it does, the digest to look up.
func SessionTokenDigest(token string) ([]byte, bool) {
	if !strings.HasPrefix(token, SessionTokenPrefix) {
		return nil, false
	}
	digest := sha256.Sum256([]byte(token))
	return digest[:], true
}

type sessionVerifier struct {
	lookup SessionLookup
}

// NewSessionVerifier adapts the session store to the Verifier interface.
func NewSessionVerifier(lookup SessionLookup) (Verifier, error) {
	if lookup == nil {
		return nil, errors.New("session lookup is required")
	}
	return &sessionVerifier{lookup: lookup}, nil
}

func (v *sessionVerifier) Verify(ctx context.Context, token string) (Principal, error) {
	digest, ok := SessionTokenDigest(token)
	if !ok {
		return Principal{}, ErrInvalidToken
	}
	return v.lookup.LookupSession(ctx, digest)
}

type chainVerifier struct {
	session  Verifier
	fallback Verifier
}

// NewChainVerifier routes prefixed credentials to the local session verifier
// and everything else to the configured OIDC verifier. Both remain optional so
// an environment may run with only one of them.
func NewChainVerifier(session, oidc Verifier) (Verifier, error) {
	if session == nil && oidc == nil {
		return nil, errors.New("at least one verifier is required")
	}
	return &chainVerifier{session: session, fallback: oidc}, nil
}

func (v *chainVerifier) Verify(ctx context.Context, token string) (Principal, error) {
	if _, ok := SessionTokenDigest(token); ok {
		return verifyWith(ctx, v.session, token)
	}
	return verifyWith(ctx, v.fallback, token)
}

// IsFixtureCredential keeps the loopback restriction of the development fake
// reachable through the chain.
func (v *chainVerifier) IsFixtureCredential(token string) bool {
	fixture, ok := v.fallback.(FixtureCredentialVerifier)
	return ok && fixture.IsFixtureCredential(token)
}

func verifyWith(ctx context.Context, verifier Verifier, token string) (Principal, error) {
	if verifier == nil {
		return Principal{}, ErrInvalidToken
	}
	return verifier.Verify(ctx, token)
}
