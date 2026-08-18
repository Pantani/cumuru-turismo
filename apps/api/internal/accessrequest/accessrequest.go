// Package accessrequest holds the public request for access that a lodging
// makes on its own behalf to join the platform.
//
// The landing page invited the pousada to sign up and sent whoever clicked to
// the login screen, which only opens from the inside. This channel exists to
// give that invitation an honest destination: an open form that records a
// request, a queue the administration reads, and an approval that creates the
// accommodation.
//
// It is the declared exception to ADR-040, and it is narrow on purpose. There
// the open channel was forbidden from collecting identity because a stranger
// would be describing a third party who would never be contacted. Here the
// author is the person responsible describing their own establishment: without
// the accommodation name there is nothing to approve, and without an address
// there is nobody to hand the access back to. What does not dissolve is that
// the declared address is not verified, which is why approval creates the
// accommodation and nothing else — issuing the access remains the separate act
// of ADR-041 (ADR-042).
package accessrequest

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/google/uuid"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource conflict")
	ErrForbidden    = errors.New("operation not permitted")
	// ErrPreconditionFailed answers a stale If-Match, and never the row that
	// vanished between the read and the write: that one is a conflict.
	ErrPreconditionFailed = errors.New("precondition failed")
	ErrUnavailable        = errors.New("repository unavailable")
	ErrRateLimited        = errors.New("rate limited")
)

var (
	emailPattern          = regexp.MustCompile(`^[^@\s]+@[^@\s.]+(\.[^@\s.]+)+$`)
	statePattern          = regexp.MustCompile(`^[A-Z]{2}$`)
	base64URLPattern      = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{16,128}$`)
)

// PendingTTL is deliberately not the 72 hours of ADR-040: there the clock ran
// against a visitor self-registration made inside the pousada, here it runs
// against the inbox of a municipal administration, which does not operate in
// three-day shifts. What the precedent protects — inaction ceasing to be a form
// of indefinite retention — still holds (ADR-042).
const PendingTTL = 30 * 24 * time.Hour

// PrivacyNoticeVersion is fixed on this surface because there is no prior
// invite to read it from: the pousada's wall poster carries its own, this form
// has no poster. The client receives the version in the context and sends it
// back on submission, and a divergence is a conflict — that is how the server
// knows the notice displayed was the notice in force.
const PrivacyNoticeVersion = "prototype-v1"

// ApprovalState has four values because expired is not a decision: it is the
// request nobody read within the deadline, and therefore has neither a decider
// nor a reason.
type ApprovalState string

const (
	StatePending  ApprovalState = "pending"
	StateApproved ApprovalState = "approved"
	StateRejected ApprovalState = "rejected"
	StateExpired  ApprovalState = "expired"
)

func (s ApprovalState) Valid() bool {
	switch s {
	case StatePending, StateApproved, StateRejected, StateExpired:
		return true
	default:
		return false
	}
}

// RejectionReason is a closed list for the same reason RejectStayRequest is:
// the reason goes into platform.audit_events, which is append-only with no
// UPDATE and no DELETE, and free text there would become permanent personal
// data on the very path designed to erase it.
type RejectionReason string

const (
	ReasonDuplicateRequest        RejectionReason = "duplicate_request"
	ReasonNotALodging             RejectionReason = "not_a_lodging"
	ReasonInsufficientInformation RejectionReason = "insufficient_information"
	ReasonAbuse                   RejectionReason = "abuse"
)

func (r RejectionReason) Valid() bool {
	switch r {
	case ReasonDuplicateRequest, ReasonNotALodging,
		ReasonInsufficientInformation, ReasonAbuse:
		return true
	default:
		return false
	}
}

// Request is the queue item. The three contact fields are pointers because
// rejection and expiry erase them in the same transaction that writes the
// state: what survives is the fact, the reason and the instant, never the
// person.
type Request struct {
	ID                  uuid.UUID              `json:"id"`
	AccommodationName   string                 `json:"accommodation_name"`
	Category            accommodation.Category `json:"category"`
	Capacity            int32                  `json:"capacity"`
	ContactName         *string                `json:"contact_name"`
	ContactEmail        *string                `json:"contact_email"`
	ContactPhone        *string                `json:"contact_phone"`
	CityLabel           string                 `json:"city_label"`
	StateCode           string                 `json:"state_code"`
	ApprovalState       ApprovalState          `json:"approval_state"`
	ExpiresAt           time.Time              `json:"expires_at"`
	AccommodationID     *uuid.UUID             `json:"accommodation_id"`
	RejectionReasonCode *RejectionReason       `json:"rejection_reason_code"`
	Version             int64                  `json:"version"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
}

// Created is a minimal receipt. No submitted field comes back: the route is
// open, and echoing what was stored would turn creation into a lookup of
// somebody else's contact details.
type Created struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

type ProofOfWorkChallenge struct {
	Algorithm      string    `json:"algorithm"`
	Challenge      string    `json:"challenge"`
	DifficultyBits int       `json:"difficulty_bits"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type Context struct {
	ProofOfWork          ProofOfWorkChallenge `json:"proof_of_work"`
	PrivacyNoticeVersion string               `json:"privacy_notice_version"`
}

// ProofOfWorkAnswer is a toll, not proof of a human: it measures electricity,
// not entitlement. The abuse that matters here is volume — flooding the queue
// until it stops being read — and volume is exactly what it makes expensive.
type ProofOfWorkAnswer struct {
	Challenge string
	Solution  string
}

func (p ProofOfWorkAnswer) valid() bool {
	return validOpaqueToken(p.Challenge, 32, 256) &&
		validOpaqueToken(p.Solution, 1, 64)
}

type ContextRequest struct {
	RateSubject string
}

type CreateCommand struct {
	RateSubject          string
	ClientSubmissionID   uuid.UUID
	AccommodationName    string
	Category             accommodation.Category
	Capacity             int32
	ContactName          string
	ContactEmail         string
	ContactPhone         string
	CityLabel            string
	StateCode            string
	PrivacyNoticeVersion string
	ProofOfWork          ProofOfWorkAnswer
	IdempotencyKey       string
	RequestID            string
}

type PageCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// PageRequest takes an empty State as "every state", because a state the
// listing cannot ask for becomes a record the screen cannot explain.
type PageRequest struct {
	CursorCreatedAt time.Time
	CursorID        uuid.UUID
	Limit           int32
	State           ApprovalState
}

type Page struct {
	Items      []Request
	NextCursor *PageCursor
}

// ApprovalCommand and RejectionCommand carry the expected version from
// If-Match, the idempotency key and the request identifier, like every other
// mutation of the repository. The decision always has a named holder: approving
// produces the same effect as creating the record by hand.
type ApprovalCommand struct {
	Actor           access.Principal
	AccessRequestID uuid.UUID
	ExpectedVersion int64
	IdempotencyKey  string
	RequestID       string
}

type RejectionCommand struct {
	Actor           access.Principal
	AccessRequestID uuid.UUID
	ExpectedVersion int64
	ReasonCode      RejectionReason
	IdempotencyKey  string
	RequestID       string
}

type Repository interface {
	Context(context.Context, ContextRequest) (Context, error)
	Create(context.Context, CreateCommand) (Created, bool, error)
	List(context.Context, PageRequest) (Page, error)
	Approve(context.Context, ApprovalCommand) (Request, bool, error)
	Reject(context.Context, RejectionCommand) (Request, bool, error)
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Context(ctx context.Context, request ContextRequest) (Context, error) {
	if strings.TrimSpace(request.RateSubject) == "" {
		return Context{}, ErrInvalidInput
	}
	return s.repository.Context(ctx, request)
}

// Create normalizes the address before validating because the column demands
// the already normalized value: leaving that to the database would trade an
// explainable refusal for a CHECK violation, which reaches the application as a
// technical failure.
func (s *Service) Create(ctx context.Context, command CreateCommand) (Created, bool, error) {
	command.ContactEmail = NormalizeEmail(command.ContactEmail)
	if err := ValidateCreate(command); err != nil {
		return Created{}, false, err
	}
	return s.repository.Create(ctx, command)
}

func (s *Service) List(ctx context.Context, page PageRequest) (Page, error) {
	if !validPage(page) {
		return Page{}, ErrInvalidInput
	}
	return s.repository.List(ctx, page)
}

func (s *Service) Approve(
	ctx context.Context,
	command ApprovalCommand,
) (Request, bool, error) {
	if err := ValidateApproval(command); err != nil {
		return Request{}, false, err
	}
	return s.repository.Approve(ctx, command)
}

func (s *Service) Reject(
	ctx context.Context,
	command RejectionCommand,
) (Request, bool, error) {
	if err := ValidateRejection(command); err != nil {
		return Request{}, false, err
	}
	return s.repository.Reject(ctx, command)
}

// NormalizeEmail mirrors the btrim(lower(...)) of the
// accommodation_access_requests_email_normalized constraint. The partial unique
// index of one pending request per address depends on it: without normalizing,
// "A@x.com" and "a@x.com" would be two rows in the queue instead of a conflict.
func NormalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// ValidateCreate refuses by shape what the migration refuses by CHECK. The
// duplication is deliberate: server-side validation must not depend on a
// client-generated type, and the CHECK must not depend on the application
// having remembered.
func ValidateCreate(command CreateCommand) error {
	for _, valid := range createValidators {
		if !valid(command) {
			return ErrInvalidInput
		}
	}
	return nil
}

// The validation is a list of named predicates rather than one long
// conjunction: each line states which group of fields it covers, and adding a
// new group means adding an entry instead of growing a function.
var createValidators = []func(CreateCommand) bool{
	validSubmissionIdentity,
	validSubmissionEnvelope,
	validAccommodationFields,
	validContactFields,
	validLocationFields,
}

func validSubmissionIdentity(command CreateCommand) bool {
	return strings.TrimSpace(command.RateSubject) != "" &&
		command.ClientSubmissionID.Version() == 7 &&
		command.ClientSubmissionID.Variant() == uuid.RFC4122
}

func validSubmissionEnvelope(command CreateCommand) bool {
	return validRequiredText(command.PrivacyNoticeVersion, 100) &&
		command.ProofOfWork.valid() &&
		validMutationMeta(command.IdempotencyKey, command.RequestID)
}

// ValidInput leaves 'unclassified' out: it is the value the registry uses when
// nobody classified the accommodation, not a choice the form offers.
func validAccommodationFields(command CreateCommand) bool {
	return validRequiredText(command.AccommodationName, 200) &&
		command.Category.ValidInput() &&
		command.Capacity >= 1 && command.Capacity <= 10000
}

func validContactFields(command CreateCommand) bool {
	return validRequiredText(command.ContactName, 120) &&
		validEmail(command.ContactEmail) &&
		validOptionalText(command.ContactPhone, 40)
}

func validLocationFields(command CreateCommand) bool {
	return validRequiredText(command.CityLabel, 120) &&
		statePattern.MatchString(command.StateCode)
}

func ValidateApproval(command ApprovalCommand) error {
	if !validDecision(command.Actor, command.AccessRequestID,
		command.ExpectedVersion, command.IdempotencyKey, command.RequestID) {
		return ErrInvalidInput
	}
	return nil
}

// The reason is refused here and again by the CHECK in the database, because
// neither barrier may depend on the other having worked.
func ValidateRejection(command RejectionCommand) error {
	if !command.ReasonCode.Valid() {
		return ErrInvalidInput
	}
	if !validDecision(command.Actor, command.AccessRequestID,
		command.ExpectedVersion, command.IdempotencyKey, command.RequestID) {
		return ErrInvalidInput
	}
	return nil
}

func validDecision(
	actor access.Principal,
	id uuid.UUID,
	expectedVersion int64,
	idempotencyKey string,
	requestID string,
) bool {
	return validActor(actor) &&
		id != uuid.Nil && expectedVersion > 0 &&
		validMutationMeta(idempotencyKey, requestID)
}

func validActor(actor access.Principal) bool {
	return actor.Issuer != "" && actor.Subject != ""
}

func validPage(page PageRequest) bool {
	return validPageBounds(page) && validPageFilter(page.State) &&
		page.CursorCreatedAt.IsZero() == (page.CursorID == uuid.Nil)
}

func validPageBounds(page PageRequest) bool {
	return page.Limit >= 1 && page.Limit <= 100
}

// An absent filter is legitimate and means "every state"; a value outside the
// enum is not, and becomes a refusal instead of an empty page.
func validPageFilter(state ApprovalState) bool {
	return state == "" || state.Valid()
}

func validMutationMeta(idempotencyKey, requestID string) bool {
	return idempotencyKeyPattern.MatchString(idempotencyKey) && requestID != ""
}

// The bound counts runes, like every other text bound here, because the
// contract declares minLength and maxLength and OpenAPI counts characters.
// emailPattern only forbids '@' and whitespace, so a non-ASCII local part
// reaches this check and would fail a byte count well before 254 characters.
func validEmail(value string) bool {
	return utf8.RuneCountInString(value) >= 3 &&
		validTextLength(value, 254) &&
		emailPattern.MatchString(value)
}

func validRequiredText(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && validTextLength(value, maximum)
}

// The phone is the only optional field of the form; empty means absent, and not
// a blank string, which the not-blank CHECK would refuse in the database.
func validOptionalText(value string, maximum int) bool {
	return value == "" || validRequiredText(value, maximum)
}

func validTextLength(value string, maximum int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximum
}

// The alphabet is the contract's, so a value carrying anything else never
// reaches the verifier, the digest or a log line.
func validOpaqueToken(value string, minimum, maximum int) bool {
	if len(value) < minimum || len(value) > maximum {
		return false
	}
	return base64URLPattern.MatchString(value)
}
