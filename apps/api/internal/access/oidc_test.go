package access_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
)

func TestOIDCVerifierAcceptsValidTokenAndScopes(t *testing.T) {
	t.Parallel()

	fixture, verifier, now := newVerifierFixture(t)
	validClaims := validOIDCClaims(fixture.server.URL, now)
	valid := fixture.sign(t, "RS256", "fixture-key", validClaims, true)
	principal, err := verifier.Verify(context.Background(), valid)
	if err != nil {
		t.Fatalf("Verify(valid) error = %v", err)
	}
	if principal.Issuer != fixture.server.URL || principal.Subject != "operator-a" || !principal.HasScope("platform:read") {
		t.Fatalf("principal = %#v", principal)
	}
}

func TestOIDCVerifierRejectsInvalidTokensAndClaims(t *testing.T) {
	t.Parallel()

	fixture, verifier, now := newVerifierFixture(t)
	validClaims := validOIDCClaims(fixture.server.URL, now)
	for _, tt := range invalidOIDCCases(t, fixture, validClaims, now) {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			token := tt.token()
			if _, err := verifier.Verify(context.Background(), token); err != access.ErrInvalidToken {
				t.Fatalf("Verify() error = %v, want ErrInvalidToken", err)
			}
		})
	}
}

type invalidOIDCCase struct {
	name  string
	token func() string
}

func invalidOIDCCases(t *testing.T, fixture *oidcFixture, validClaims map[string]any, now time.Time) []invalidOIDCCase {
	t.Helper()
	signedClaims := func(claims map[string]any) func() string {
		return func() string {
			return fixture.sign(t, "RS256", "fixture-key", claims, true)
		}
	}
	return []invalidOIDCCase{
		{name: "malformed", token: func() string { return "not-a-jwt" }},
		{name: "invalid signature", token: func() string { return corruptSignature(fixture.sign(t, "RS256", "fixture-key", validClaims, true)) }},
		{name: "algorithm none", token: func() string { return fixture.sign(t, "none", "fixture-key", validClaims, false) }},
		{name: "unknown key", token: func() string { return fixture.sign(t, "RS256", "unknown-key", validClaims, true) }},
		{name: "wrong issuer", token: signedClaims(mergeClaims(validClaims, map[string]any{"iss": "https://other.invalid"}))},
		{name: "wrong audience", token: signedClaims(mergeClaims(validClaims, map[string]any{"aud": "other-client"}))},
		{name: "expired", token: signedClaims(mergeClaims(validClaims, map[string]any{"exp": now.Add(-time.Minute).Unix()}))},
		{name: "not before in future", token: signedClaims(mergeClaims(validClaims, map[string]any{"nbf": now.Add(time.Minute).Unix()}))},
	}
}

func newVerifierFixture(t *testing.T) (*oidcFixture, *access.OIDCVerifier, time.Time) {
	t.Helper()
	fixture := newOIDCFixture(t)
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	client := fixture.server.Client()
	client.Timeout = time.Second
	verifier, err := access.NewOIDCVerifier(context.Background(), access.OIDCOptions{
		Issuer:     fixture.server.URL,
		Audience:   "cumuru-test",
		HTTPClient: client,
		Now:        func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewOIDCVerifier() error = %v", err)
	}
	return fixture, verifier, now
}

func validOIDCClaims(issuer string, now time.Time) map[string]any {
	return map[string]any{
		"iss":   issuer,
		"sub":   "operator-a",
		"aud":   "cumuru-test",
		"iat":   now.Add(-time.Minute).Unix(),
		"exp":   now.Add(time.Minute).Unix(),
		"nbf":   now.Add(-time.Minute).Unix(),
		"scope": "platform:read stays:read:own",
	}
}

func corruptSignature(token string) string {
	parts := strings.Split(token, ".")
	if parts[2][0] == 'A' {
		parts[2] = "B" + parts[2][1:]
	} else {
		parts[2] = "A" + parts[2][1:]
	}
	return strings.Join(parts, ".")
}

func TestOIDCDiscoveryHonorsHTTPTimeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	client := server.Client()
	client.Timeout = 10 * time.Millisecond
	_, err := access.NewOIDCVerifier(context.Background(), access.OIDCOptions{
		Issuer:     server.URL,
		Audience:   "cumuru-test",
		HTTPClient: client,
	})
	if err == nil || strings.Contains(err.Error(), server.URL) {
		t.Fatalf("NewOIDCVerifier() error = %v, want sanitized timeout", err)
	}
}

type oidcFixture struct {
	server *httptest.Server
	key    *rsa.PrivateKey
}

func newOIDCFixture(t *testing.T) *oidcFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	fixture := &oidcFixture{key: key}
	fixture.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"issuer":                                fixture.server.URL,
				"jwks_uri":                              fixture.server.URL + "/keys",
				"authorization_endpoint":                fixture.server.URL + "/authorize",
				"token_endpoint":                        fixture.server.URL + "/token",
				"id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/keys":
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"keys": []map[string]any{{
					"kty": "RSA",
					"use": "sig",
					"alg": "RS256",
					"kid": "fixture-key",
					"n":   base64.RawURLEncoding.EncodeToString(fixture.key.PublicKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1}),
				}},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (f *oidcFixture) sign(t *testing.T, algorithm, keyID string, claims map[string]any, signed bool) string {
	t.Helper()
	header, err := json.Marshal(map[string]any{"alg": algorithm, "kid": keyID, "typ": "JWT"})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	if !signed {
		return signingInput + "."
	}
	sum := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func mergeClaims(base, override map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(override))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range override {
		out[key] = value
	}
	return out
}
