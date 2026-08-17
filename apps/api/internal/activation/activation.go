// Package activation transfers the access of an accommodation to whoever
// operates it, without inventing a provisioning administrator and without
// sending e-mail.
//
// The ADR-035 already implemented self-provisioning: the principal creates the
// accommodation and becomes its manager. This phase does not contradict that.
// It adds a single-use capability, shown on the screen as a link and a QR code
// generated in the browser, so the manager can hand the access over. Delivering
// that link is a human responsibility outside the system, which is a declared
// limitation and not a security control (ADR-041).
package activation

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/google/uuid"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	// ErrNotFound is the single answer for an absent, wrong, expired, consumed
	// or revoked capability. Anything finer would let a probe map the state of
	// somebody else's account.
	ErrNotFound           = errors.New("resource not found")
	ErrConflict           = errors.New("resource conflict")
	ErrForbidden          = errors.New("operation not permitted")
	ErrPreconditionFailed = errors.New("precondition failed")
	ErrUnavailable        = errors.New("repository unavailable")

	emailPattern          = regexp.MustCompile(`^[^@\s]+@[^@\s.]+(\.[^@\s.]+)+$`)
	idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{16,128}$`)
)

// CreateCommand issues the capability. The e-mail identifies the account being
// created; it is never echoed back by the public context, because holding the
// capability is not proof of owning the address.
type CreateCommand struct {
	Actor           access.Principal
	AccommodationID uuid.UUID
	Email           string
	DisplayName     string
	ExpectedVersion int64
	IdempotencyKey  string
	RequestID       string
}

type Created struct {
	ActivationID uuid.UUID `json:"activation_id"`
	AccountID    uuid.UUID `json:"account_id"`
	URL          string    `json:"url"`
	ExpiresAt    time.Time `json:"expires_at"`
	Version      int64     `json:"version"`
}

// PasswordPolicy is echoed so the form states the same rule the server
// enforces, instead of a copy that can drift.
type PasswordPolicy struct {
	MinLength int `json:"min_length"`
	MaxLength int `json:"max_length"`
}

// Context deliberately omits the e-mail. The capability proves possession of a
// link, not ownership of an address.
type Context struct {
	AccommodationName string         `json:"accommodation_name"`
	DisplayName       string         `json:"display_name"`
	ExpiresAt         time.Time      `json:"expires_at"`
	PasswordPolicy    PasswordPolicy `json:"password_policy"`
}

// CompleteCommand sets the password and activates the account. There is no
// idempotency key: the capability is itself the single-use token, and repeating
// the call answers not-found on purpose.
type CompleteCommand struct {
	Token       string
	RateSubject string
	Password    string
	RequestID   string
}

// scopes mirrors the accommodation operator, plus the phase's own approval
// scope. There are two independent layers behind an approval: the scope, which
// the transport checks, and accommodation.allowedOperations plus the manager
// role, which the domain checks. Granting the scope does not grant the
// operation; withholding it makes the operation unreachable. The activated
// account needs the scope because the whole flow exists so the accommodation
// approves its own queue.
//
// platform:read and accommodations:onboard stay out: they belong to other
// roles, and the accommodation already exists when the capability is issued.
var scopes = []string{
	"accommodations:manage", "stays:read:own", "stays:write", "stays:approve",
}

// Scopes hands out a copy. The slice is package state, and a caller that
// appended to it would widen every future account.
func Scopes() []string {
	return append([]string(nil), scopes...)
}

type Repository interface {
	Create(context.Context, CreateCommand) (Created, bool, error)
	Context(context.Context, Request) (Context, error)
	Complete(context.Context, CompleteCommand) error
}

type Request struct {
	Token       string
	RateSubject string
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Create(ctx context.Context, command CreateCommand) (Created, bool, error) {
	if err := validateCreate(command); err != nil {
		return Created{}, false, err
	}
	return s.repository.Create(ctx, command)
}

// A malformed capability answers not-found, never invalid-input: the public
// surface must not tell a probe whether the token was the wrong shape.
func (s *Service) Context(ctx context.Context, request Request) (Context, error) {
	if !validCapability(request.Token, request.RateSubject) {
		return Context{}, ErrNotFound
	}
	return s.repository.Context(ctx, request)
}

func (s *Service) Complete(ctx context.Context, command CompleteCommand) error {
	if !validCapability(command.Token, command.RateSubject) {
		return ErrNotFound
	}
	if access.ValidatePasswordLength(command.Password) != nil || command.RequestID == "" {
		return ErrInvalidInput
	}
	return s.repository.Complete(ctx, command)
}

func validateCreate(command CreateCommand) error {
	if !validActor(command.Actor) || !validTarget(command) || !validMeta(command) {
		return ErrInvalidInput
	}
	return nil
}

func validActor(actor access.Principal) bool {
	return actor.Issuer != "" && actor.Subject != ""
}

func validTarget(command CreateCommand) bool {
	return command.AccommodationID != uuid.Nil &&
		validEmail(command.Email) &&
		validDisplayName(command.DisplayName)
}

func validMeta(command CreateCommand) bool {
	return command.ExpectedVersion >= 1 &&
		idempotencyKeyPattern.MatchString(command.IdempotencyKey) &&
		command.RequestID != ""
}

func validEmail(value string) bool {
	return len(value) >= 3 && len(value) <= 254 && emailPattern.MatchString(value)
}

func validDisplayName(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && len(value) <= 120
}

func validCapability(token, subject string) bool {
	return len(token) >= 64 && len(token) <= 128 && strings.TrimSpace(subject) != ""
}
