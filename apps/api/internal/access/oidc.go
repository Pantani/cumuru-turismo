package access

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type OIDCOptions struct {
	Issuer     string
	Audience   string
	HTTPClient *http.Client
	Now        func() time.Time
}

type OIDCVerifier struct {
	issuer   string
	verifier *oidc.IDTokenVerifier
	now      func() time.Time
}

// Discovery reaches the network, so the client must carry a timeout: a hung
// provider must not be able to stall a request indefinitely.
func (o OIDCOptions) validate() error {
	if strings.TrimSpace(o.Issuer) == "" || strings.TrimSpace(o.Audience) == "" {
		return errors.New("OIDC configuration is incomplete")
	}
	if o.HTTPClient == nil || o.HTTPClient.Timeout <= 0 {
		return errors.New("OIDC HTTP client requires a timeout")
	}
	return nil
}

func NewOIDCVerifier(ctx context.Context, options OIDCOptions) (*OIDCVerifier, error) {
	if err := options.validate(); err != nil {
		return nil, err
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	discoveryContext := context.WithValue(ctx, oauth2.HTTPClient, options.HTTPClient)
	provider, err := oidc.NewProvider(discoveryContext, options.Issuer)
	if err != nil {
		return nil, errors.New("OIDC provider discovery failed")
	}
	verifier := provider.Verifier(&oidc.Config{
		ClientID:             options.Audience,
		SupportedSigningAlgs: []string{oidc.RS256, oidc.ES256},
		Now:                  options.Now,
	})
	return &OIDCVerifier{
		issuer:   strings.TrimSuffix(options.Issuer, "/"),
		verifier: verifier,
		now:      options.Now,
	}, nil
}

func (v *OIDCVerifier) Verify(ctx context.Context, rawToken string) (Principal, error) {
	if strings.TrimSpace(rawToken) == "" {
		return Principal{}, ErrInvalidToken
	}
	token, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return Principal{}, ErrInvalidToken
	}
	scopes, err := v.acceptedScopes(token)
	if err != nil {
		return Principal{}, err
	}
	return NewPrincipal(v.issuer, token.Subject, scopes), nil
}

// A thirty second skew is allowed on nbf, matching the tolerance the library
// applies to the other time claims.
func (v *OIDCVerifier) acceptedScopes(token *oidc.IDToken) ([]string, error) {
	var claims oidcClaims
	if err := token.Claims(&claims); err != nil {
		return nil, ErrInvalidToken
	}
	if token.Subject == "" || claims.NotBefore.After(v.now().Add(30*time.Second)) {
		return nil, ErrInvalidToken
	}
	scopes, err := claims.normalizedScopes()
	if err != nil {
		return nil, ErrInvalidToken
	}
	return scopes, nil
}

type numericDate int64

func (d *numericDate) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var value int64
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*d = numericDate(value)
	return nil
}

func (d numericDate) After(now time.Time) bool {
	if d == 0 {
		return false
	}
	return time.Unix(int64(d), 0).After(now)
}

type scopeClaim []string

func (s *scopeClaim) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		*s = strings.Fields(value)
		return nil
	}
	var values []string
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	*s = values
	return nil
}

type oidcClaims struct {
	Scope     scopeClaim  `json:"scope"`
	SCP       []string    `json:"scp"`
	NotBefore numericDate `json:"nbf"`
}

func (c oidcClaims) normalizedScopes() ([]string, error) {
	scopes := append([]string(nil), c.Scope...)
	scopes = append(scopes, c.SCP...)
	if len(scopes) > 256 {
		return nil, errors.New("too many scopes")
	}
	return scopes, nil
}
