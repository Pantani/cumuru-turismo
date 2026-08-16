package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
)

// Authenticator is the local e-mail and password track. It sits beside the OIDC
// verifier so an operator without CNPJ or federal credential can sign in.
type Authenticator interface {
	Authenticate(context.Context, string, string) (store.SessionGrant, error)
	DescribeSession(context.Context, []byte) (store.SessionGrant, error)
	RevokeSession(context.Context, []byte) error
	RotatePassword(context.Context, []byte, string, string) error
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type passwordRotationRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type sessionAccountResponse struct {
	ID                 string   `json:"id"`
	Email              string   `json:"email"`
	DisplayName        string   `json:"display_name"`
	Scopes             []string `json:"scopes"`
	MustChangePassword bool     `json:"must_change_password"`
}

type sessionResponse struct {
	Token     string                 `json:"token"`
	ExpiresAt string                 `json:"expires_at"`
	Account   sessionAccountResponse `json:"account"`
}

func (d Dependencies) registerAuthRoutes(mux *http.ServeMux, metrics *httpMetrics) {
	mux.Handle(
		"POST /api/v1/auth/login",
		d.routeHandler("/api/v1/auth/login", metrics, http.HandlerFunc(d.login)),
	)
	mux.Handle(
		"POST /api/v1/auth/logout",
		d.routeHandler("/api/v1/auth/logout", metrics, http.HandlerFunc(d.logout)),
	)
	mux.Handle(
		"GET /api/v1/auth/session",
		d.routeHandler("/api/v1/auth/session", metrics, http.HandlerFunc(d.currentSession)),
	)
	mux.Handle(
		"POST /api/v1/auth/password",
		d.routeHandler("/api/v1/auth/password", metrics, http.HandlerFunc(d.rotatePassword)),
	)
}

// rotatePassword is reachable by a session whose account still carries the
// provisional flag, and is the only route that is. It answers 204 so no
// response body can echo either secret.
func (d Dependencies) rotatePassword(writer http.ResponseWriter, request *http.Request) {
	token, ok := bearerToken(request.Header.Get("Authorization"))
	if !ok {
		writeProblem(writer, request, http.StatusUnauthorized, "unauthorized", "Credencial inválida ou ausente")
		return
	}
	digest, ok := access.SessionTokenDigest(token)
	if !ok {
		writeProblem(writer, request, http.StatusUnauthorized, "unauthorized", "Credencial inválida ou ausente")
		return
	}
	var payload passwordRotationRequest
	if err := decodeStrict(request, "application/json", &payload); err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "invalid-request", "Requisição inválida")
		return
	}
	err := d.Auth.RotatePassword(
		request.Context(), digest, payload.CurrentPassword, payload.NewPassword,
	)
	if err != nil {
		d.writeRotationProblem(writer, request, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

// A rejected new secret is a client error, while a wrong current secret is an
// authentication failure; the two are answered differently so the operator can
// tell a typo from a weak password.
func (d Dependencies) writeRotationProblem(
	writer http.ResponseWriter,
	request *http.Request,
	err error,
) {
	switch {
	case errors.Is(err, store.ErrPasswordReused), errors.Is(err, access.ErrInvalidPassword):
		writeProblem(
			writer, request, http.StatusUnprocessableEntity,
			"invalid-password", "A nova senha não atende à política",
		)
	default:
		d.writeAuthProblem(writer, request, err)
	}
}

func (d Dependencies) login(writer http.ResponseWriter, request *http.Request) {
	var payload loginRequest
	if err := decodeStrict(request, "application/json", &payload); err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "invalid-request", "Requisição inválida")
		return
	}
	grant, err := d.Auth.Authenticate(request.Context(), payload.Email, payload.Password)
	if err != nil {
		d.writeAuthProblem(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, newSessionResponse(grant))
}

func (d Dependencies) logout(writer http.ResponseWriter, request *http.Request) {
	token, ok := bearerToken(request.Header.Get("Authorization"))
	if !ok {
		writeProblem(writer, request, http.StatusUnauthorized, "unauthorized", "Credencial inválida ou ausente")
		return
	}
	digest, ok := access.SessionTokenDigest(token)
	if !ok {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	if err := d.Auth.RevokeSession(request.Context(), digest); err != nil {
		d.writeAuthProblem(writer, request, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

// currentSession lets the client rehydrate after a reload without persisting
// anything locally. It echoes the presented token, never a new one.
func (d Dependencies) currentSession(writer http.ResponseWriter, request *http.Request) {
	token, ok := bearerToken(request.Header.Get("Authorization"))
	digest, prefixed := access.SessionTokenDigest(token)
	if !ok || !prefixed {
		writeProblem(writer, request, http.StatusUnauthorized, "unauthorized", "Credencial inválida ou ausente")
		return
	}
	grant, err := d.Auth.DescribeSession(request.Context(), digest)
	if err != nil {
		d.writeAuthProblem(writer, request, err)
		return
	}
	grant.Token = token
	writeJSON(writer, http.StatusOK, newSessionResponse(grant))
}

func (d Dependencies) writeAuthProblem(
	writer http.ResponseWriter,
	request *http.Request,
	err error,
) {
	switch {
	case errors.Is(err, store.ErrAuthLocked):
		writer.Header().Set("Retry-After", strconv.Itoa(d.lockoutSeconds()))
		writeProblem(writer, request, http.StatusTooManyRequests, "rate-limited", "Muitas tentativas")
	case errors.Is(err, store.ErrAuthRejected), errors.Is(err, access.ErrInvalidToken):
		writeProblem(writer, request, http.StatusUnauthorized, "unauthorized", "Credencial inválida ou ausente")
	default:
		writeProblem(
			writer, request, http.StatusServiceUnavailable,
			"dependency-unavailable", "Serviço temporariamente indisponível",
		)
	}
}

func (d Dependencies) lockoutSeconds() int {
	seconds := int(d.LockoutDuration / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}

func newSessionResponse(grant store.SessionGrant) sessionResponse {
	return sessionResponse{
		Token:     grant.Token,
		ExpiresAt: grant.ExpiresAt.UTC().Format(time.RFC3339),
		Account: sessionAccountResponse{
			ID:                 grant.AccountID,
			Email:              grant.Email,
			DisplayName:        grant.DisplayName,
			Scopes:             grant.Scopes,
			MustChangePassword: grant.MustChangePassword,
		},
	}
}
