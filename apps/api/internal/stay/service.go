package stay

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
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource conflict")
	// ErrForbidden separates "you are not allowed to do this" from "this does
	// not exist". It is used only where the caller already proved membership,
	// so the distinction discloses nothing new.
	ErrForbidden          = errors.New("operation not permitted")
	ErrPreconditionFailed = errors.New("precondition failed")
	ErrInviteConsumed     = errors.New("invite consumed")
	ErrRateLimited        = errors.New("rate limited")
	ErrUnavailable        = errors.New("repository unavailable")
	idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{16,128}$`)
)

type Record struct {
	ID                     uuid.UUID  `json:"id"`
	AccommodationID        uuid.UUID  `json:"accommodation_id"`
	Status                 Status     `json:"status"`
	PlannedArrivalOn       string     `json:"planned_arrival_on"`
	PlannedDepartureOn     string     `json:"planned_departure_on"`
	ExpectedGuestCount     int32      `json:"expected_guest_count"`
	VisitorCount           int32      `json:"visitor_count"`
	CheckedInAt            *time.Time `json:"checked_in_at,omitempty"`
	CheckedOutAt           *time.Time `json:"checked_out_at,omitempty"`
	CancelledAt            *time.Time `json:"cancelled_at,omitempty"`
	NoShowAt               *time.Time `json:"no_show_at,omitempty"`
	CancellationReasonCode *string    `json:"cancellation_reason_code,omitempty"`
	NoShowReasonCode       *string    `json:"no_show_reason_code,omitempty"`
	// The three approval fields are required by the contract and emitted even
	// when null, so a handler that forgets one breaks the contract test instead
	// of returning a stay whose countability the client has to guess.
	Provenance        Provenance `json:"provenance"`
	ApprovalState     *string    `json:"approval_state"`
	ApprovalExpiresAt *time.Time `json:"approval_expires_at"`
	Version           int64      `json:"version"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type MutationResult struct {
	ID      uuid.UUID `json:"id"`
	Status  Status    `json:"status"`
	Version int64     `json:"version"`
}

type VisitorRecord struct {
	ID                uuid.UUID   `json:"id"`
	Role              VisitorRole `json:"role"`
	AgeBand           AgeBand     `json:"age_band"`
	ResidenceCountry  string      `json:"residence_country"`
	ResidenceState    *string     `json:"residence_state,omitempty"`
	ResidenceCityCode *string     `json:"residence_city_code,omitempty"`
	Version           int64       `json:"version"`
}

type Group struct {
	StayID               uuid.UUID       `json:"stay_id"`
	PrivacyNoticeVersion string          `json:"privacy_notice_version"`
	Visitors             []VisitorRecord `json:"visitors"`
}

type SubmissionAccepted struct {
	SubmissionID     uuid.UUID `json:"submission_id"`
	Status           string    `json:"status"`
	StayStatus       Status    `json:"stay_status"`
	Version          int64     `json:"-"`
	SurveyCapability string    `json:"-"`
}

type InviteCreated struct {
	InviteID  uuid.UUID `json:"invite_id"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
	Version   int64     `json:"-"`
}

type InviteContext struct {
	AccommodationName    string `json:"accommodation_name"`
	PlannedArrivalOn     string `json:"planned_arrival_on"`
	PlannedDepartureOn   string `json:"planned_departure_on"`
	ExpectedGuestCount   int32  `json:"expected_guest_count"`
	PrivacyNoticeVersion string `json:"privacy_notice_version"`
}

type PageRequest struct {
	CursorCreatedAt time.Time
	CursorID        uuid.UUID
	Limit           int32
	AccommodationID uuid.UUID
	Status          Status
	// The approval queue is GET /stays?accommodation_id=…&approval_state=pending.
	// No new listing endpoint: cursor, limit, ordering and membership isolation
	// stay the ones the Fase 2 already proved.
	ApprovalState ApprovalState
	Provenance    Provenance
	ArrivalFrom   string
	ArrivalTo     string
}

type Page struct {
	Items      []Record
	NextCursor *PageCursor
}

type PageCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

type CreateCommand struct {
	Actor              access.Principal
	AccommodationID    uuid.UUID
	ClientSubmissionID uuid.UUID
	PlannedArrivalOn   string
	PlannedDepartureOn string
	ExpectedGuestCount int32
	IdempotencyKey     string
	RequestID          string
}

type UpdatePatch struct {
	SetPlannedArrival     bool
	PlannedArrivalOn      string
	SetPlannedDeparture   bool
	PlannedDepartureOn    string
	SetExpectedGuestCount bool
	ExpectedGuestCount    int32
}

type UpdateCommand struct {
	Actor           access.Principal
	StayID          uuid.UUID
	ExpectedVersion int64
	Patch           UpdatePatch
	RequestID       string
}

type GroupCommand struct {
	Actor                access.Principal
	StayID               uuid.UUID
	ClientSubmissionID   uuid.UUID
	PrivacyNoticeVersion string
	Visitors             []Visitor
	ExpectedVersion      int64
	IdempotencyKey       string
	RequestID            string
}

type InviteCommand struct {
	Actor                access.Principal
	StayID               uuid.UUID
	PrivacyNoticeVersion string
	ExpectedVersion      int64
	IdempotencyKey       string
	RequestID            string
}

type TransitionKind string

const (
	TransitionCheckIn  TransitionKind = "check_in"
	TransitionCheckOut TransitionKind = "check_out"
	TransitionCancel   TransitionKind = "cancel"
	TransitionNoShow   TransitionKind = "no_show"
)

type TransitionCommand struct {
	Actor           access.Principal
	StayID          uuid.UUID
	ExpectedVersion int64
	Kind            TransitionKind
	OccurredAt      time.Time
	ReasonCode      string
	Correction      bool
	IdempotencyKey  string
	RequestID       string
}

type InviteRequest struct {
	Token       string
	RateSubject string
}

type InviteGroupCommand struct {
	Token                string
	RateSubject          string
	ClientSubmissionID   uuid.UUID
	PrivacyNoticeVersion string
	Visitors             []Visitor
	IdempotencyKey       string
	RequestID            string
}

type Repository interface {
	Create(context.Context, CreateCommand) (MutationResult, bool, error)
	List(context.Context, access.Principal, PageRequest) (Page, error)
	Get(context.Context, access.Principal, uuid.UUID) (Record, error)
	Update(context.Context, UpdateCommand) (Record, error)
	GetGroup(context.Context, access.Principal, uuid.UUID) (Group, int64, error)
	SubmitAssistedGroup(context.Context, GroupCommand) (SubmissionAccepted, bool, error)
	CreateInvite(context.Context, InviteCommand) (InviteCreated, bool, error)
	Transition(context.Context, TransitionCommand) (MutationResult, bool, error)
	GetInvite(context.Context, InviteRequest) (InviteContext, error)
	SubmitInviteGroup(context.Context, InviteGroupCommand) (SubmissionAccepted, bool, error)
	CreateAccommodationInvite(
		context.Context, AccommodationInviteCommand,
	) (AccommodationInviteCreated, bool, error)
	GetAccommodationInvite(
		context.Context, access.Principal, uuid.UUID,
	) (AccommodationInviteStatus, error)
	RevokeAccommodationInvite(
		context.Context, AccommodationInviteRevokeCommand,
	) (AccommodationInviteStatus, bool, error)
	GetAccommodationInviteContext(
		context.Context, InviteRequest,
	) (AccommodationInviteContext, error)
	SubmitSelfRegistration(
		context.Context, SelfRegistrationCommand,
	) (SelfRegistrationAccepted, bool, error)
	Approve(context.Context, ApprovalCommand) (MutationResult, bool, error)
	Reject(context.Context, RejectionCommand) (MutationResult, bool, error)
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Create(ctx context.Context, command CreateCommand) (MutationResult, bool, error) {
	if !validActor(command.Actor) || !validCreate(command) {
		return MutationResult{}, false, ErrInvalidInput
	}
	return s.repository.Create(ctx, command)
}

func (s *Service) List(ctx context.Context, actor access.Principal, page PageRequest) (Page, error) {
	if !validActor(actor) || !validPage(page) {
		return Page{}, ErrInvalidInput
	}
	return s.repository.List(ctx, actor, page)
}

func (s *Service) Get(ctx context.Context, actor access.Principal, id uuid.UUID) (Record, error) {
	if !validActor(actor) || id == uuid.Nil {
		return Record{}, ErrInvalidInput
	}
	return s.repository.Get(ctx, actor, id)
}

func (s *Service) Update(ctx context.Context, command UpdateCommand) (Record, error) {
	if !validActor(command.Actor) || !validUpdate(command) {
		return Record{}, ErrInvalidInput
	}
	return s.repository.Update(ctx, command)
}

func (s *Service) GetGroup(ctx context.Context, actor access.Principal, id uuid.UUID) (Group, int64, error) {
	if !validActor(actor) || id == uuid.Nil {
		return Group{}, 0, ErrInvalidInput
	}
	return s.repository.GetGroup(ctx, actor, id)
}

func (s *Service) SubmitAssistedGroup(
	ctx context.Context,
	command GroupCommand,
) (SubmissionAccepted, bool, error) {
	if !validActor(command.Actor) || !validGroupCommand(command) {
		return SubmissionAccepted{}, false, ErrInvalidInput
	}
	return s.repository.SubmitAssistedGroup(ctx, command)
}

func (s *Service) CreateInvite(ctx context.Context, command InviteCommand) (InviteCreated, bool, error) {
	if !validActor(command.Actor) || !validInviteCommand(command) {
		return InviteCreated{}, false, ErrInvalidInput
	}
	return s.repository.CreateInvite(ctx, command)
}

func (s *Service) Transition(ctx context.Context, command TransitionCommand) (MutationResult, bool, error) {
	if !validActor(command.Actor) || !validTransitionCommand(command) {
		return MutationResult{}, false, ErrInvalidInput
	}
	return s.repository.Transition(ctx, command)
}

func (s *Service) GetInvite(ctx context.Context, request InviteRequest) (InviteContext, error) {
	if !validCapabilityRequest(request.Token, request.RateSubject) {
		return InviteContext{}, ErrNotFound
	}
	return s.repository.GetInvite(ctx, request)
}

func (s *Service) SubmitInviteGroup(
	ctx context.Context,
	command InviteGroupCommand,
) (SubmissionAccepted, bool, error) {
	if !validCapabilityRequest(command.Token, command.RateSubject) || !validInviteGroup(command) {
		return SubmissionAccepted{}, false, ErrInvalidInput
	}
	return s.repository.SubmitInviteGroup(ctx, command)
}

func (s *Service) CreateAccommodationInvite(
	ctx context.Context,
	command AccommodationInviteCommand,
) (AccommodationInviteCreated, bool, error) {
	if err := ValidateAccommodationInvite(command); err != nil {
		return AccommodationInviteCreated{}, false, err
	}
	return s.repository.CreateAccommodationInvite(ctx, command)
}

func (s *Service) GetAccommodationInvite(
	ctx context.Context,
	actor access.Principal,
	accommodationID uuid.UUID,
) (AccommodationInviteStatus, error) {
	if !validActor(actor) || accommodationID == uuid.Nil {
		return AccommodationInviteStatus{}, ErrInvalidInput
	}
	return s.repository.GetAccommodationInvite(ctx, actor, accommodationID)
}

func (s *Service) RevokeAccommodationInvite(
	ctx context.Context,
	command AccommodationInviteRevokeCommand,
) (AccommodationInviteStatus, bool, error) {
	if err := ValidateAccommodationInviteRevoke(command); err != nil {
		return AccommodationInviteStatus{}, false, err
	}
	return s.repository.RevokeAccommodationInvite(ctx, command)
}

// A malformed capability answers not-found, never invalid-input: the open
// channel must not tell a probe whether the token was the wrong shape or simply
// unknown.
func (s *Service) GetAccommodationInviteContext(
	ctx context.Context,
	request InviteRequest,
) (AccommodationInviteContext, error) {
	if !validCapabilityRequest(request.Token, request.RateSubject) {
		return AccommodationInviteContext{}, ErrNotFound
	}
	return s.repository.GetAccommodationInviteContext(ctx, request)
}

func (s *Service) SubmitSelfRegistration(
	ctx context.Context,
	command SelfRegistrationCommand,
) (SelfRegistrationAccepted, bool, error) {
	if err := ValidateSelfRegistration(command); err != nil {
		return SelfRegistrationAccepted{}, false, err
	}
	return s.repository.SubmitSelfRegistration(ctx, command)
}

func (s *Service) Approve(
	ctx context.Context,
	command ApprovalCommand,
) (MutationResult, bool, error) {
	if err := ValidateApproval(command); err != nil {
		return MutationResult{}, false, err
	}
	return s.repository.Approve(ctx, command)
}

func (s *Service) Reject(
	ctx context.Context,
	command RejectionCommand,
) (MutationResult, bool, error) {
	if err := ValidateRejection(command); err != nil {
		return MutationResult{}, false, err
	}
	return s.repository.Reject(ctx, command)
}

func validActor(actor access.Principal) bool {
	return actor.Issuer != "" && actor.Subject != ""
}

func validMutationMeta(idempotencyKey, requestID string) bool {
	return validIdempotencyKey(idempotencyKey) && requestID != ""
}

func validCreateShape(command CreateCommand) bool {
	return command.AccommodationID != uuid.Nil &&
		command.ClientSubmissionID.Version() == 7 &&
		command.ExpectedGuestCount >= 1 &&
		command.ExpectedGuestCount <= 100
}

func validCreate(command CreateCommand) bool {
	return validCreateShape(command) &&
		validDateRange(command.PlannedArrivalOn, command.PlannedDepartureOn) &&
		validMutationMeta(command.IdempotencyKey, command.RequestID)
}

func validDateRange(arrivalValue, departureValue string) bool {
	arrival, err := ParseCivilDate(arrivalValue)
	if err != nil {
		return false
	}
	departure, err := ParseCivilDate(departureValue)
	return err == nil && arrival.Before(departure)
}

// A cursor is either fully set or fully absent; a half-set cursor would page
// from an undefined position.
func validCursor(page PageRequest) bool {
	return page.CursorCreatedAt.IsZero() == (page.CursorID == uuid.Nil)
}

func validPageFilter(page PageRequest) bool {
	return optionalFilter(page.Status == "", validStatus(page.Status)) &&
		optionalFilter(page.ApprovalState == "", validApprovalFilter(page.ApprovalState)) &&
		optionalFilter(page.Provenance == "", page.Provenance.Valid())
}

func optionalFilter(absent, valid bool) bool {
	return absent || valid
}

// ApprovalNotRequired has no column value, so it is not a filter: asking for it
// would silently return everything.
func validApprovalFilter(state ApprovalState) bool {
	return state != ApprovalNotRequired && state.Valid()
}

func validPage(page PageRequest) bool {
	if page.Limit < 1 || page.Limit > 100 || !validCursor(page) {
		return false
	}
	return validPageFilter(page) && validOptionalDates(page.ArrivalFrom, page.ArrivalTo)
}

func validOptionalDate(value string) bool {
	if value == "" {
		return true
	}
	_, err := ParseCivilDate(value)
	return err == nil
}

func validOptionalDates(from, to string) bool {
	if !validOptionalDate(from) || !validOptionalDate(to) {
		return false
	}
	return from == "" || to == "" || from <= to
}

func validStatus(status Status) bool {
	allowed := map[Status]bool{
		StatusDraft: true, StatusInvited: true, StatusPreRegistered: true,
		StatusCheckedIn: true, StatusCheckedOut: true,
		StatusCancelled: true, StatusNoShow: true,
	}
	return allowed[status]
}

func validUpdate(command UpdateCommand) bool {
	if !validUpdateIdentity(command) {
		return false
	}
	patch := command.Patch
	if !hasStayPatch(patch) {
		return false
	}
	if !validGuestCountPatch(patch) {
		return false
	}
	return validPatchDates(patch)
}

func validUpdateIdentity(command UpdateCommand) bool {
	return command.StayID != uuid.Nil &&
		command.ExpectedVersion > 0 &&
		command.RequestID != ""
}

func hasStayPatch(patch UpdatePatch) bool {
	return patch.SetPlannedArrival || patch.SetPlannedDeparture || patch.SetExpectedGuestCount
}

func validGuestCountPatch(patch UpdatePatch) bool {
	return !patch.SetExpectedGuestCount ||
		(patch.ExpectedGuestCount >= 1 && patch.ExpectedGuestCount <= 100)
}

func validPatchDates(patch UpdatePatch) bool {
	if patch.SetPlannedArrival {
		if _, err := ParseCivilDate(patch.PlannedArrivalOn); err != nil {
			return false
		}
	}
	if patch.SetPlannedDeparture {
		if _, err := ParseCivilDate(patch.PlannedDepartureOn); err != nil {
			return false
		}
	}
	return true
}

func validGroupCommand(command GroupCommand) bool {
	if command.StayID == uuid.Nil || command.ClientSubmissionID.Version() != 7 {
		return false
	}
	if command.ExpectedVersion < 1 ||
		!validMutationMeta(command.IdempotencyKey, command.RequestID) {
		return false
	}
	return validPrivacyAndVisitors(command.PrivacyNoticeVersion, command.Visitors)
}

func validPrivacyAndVisitors(version string, visitors []Visitor) bool {
	return validNoticeVersion(version) && ValidateGroup(visitors) == nil
}

func validNoticeVersion(value string) bool {
	return strings.TrimSpace(value) != "" && len(value) <= 100
}

func validInviteCommand(command InviteCommand) bool {
	return command.StayID != uuid.Nil &&
		command.ExpectedVersion > 0 &&
		validNoticeVersion(command.PrivacyNoticeVersion) &&
		validMutationMeta(command.IdempotencyKey, command.RequestID)
}

func validTransitionCommand(command TransitionCommand) bool {
	if !validTransitionIdentity(command) {
		return false
	}
	validate, known := transitionReasonRules[command.Kind]
	return known && validate(command)
}

var noShowReasons = map[string]bool{
	"": true, "guest_absent": true, "accommodation_report": true,
}

// Each transition declares which reason codes it accepts. Check-in and
// check-out carry none; cancel requires one from the closed catalogue.
var transitionReasonRules = map[TransitionKind]func(TransitionCommand) bool{
	TransitionCheckIn:  noReasonTransition,
	TransitionCheckOut: noReasonTransition,
	TransitionCancel: func(command TransitionCommand) bool {
		return CancelReason(command.ReasonCode).Valid()
	},
	TransitionNoShow: func(command TransitionCommand) bool {
		return noShowReasons[command.ReasonCode]
	},
}

func noReasonTransition(command TransitionCommand) bool {
	return command.ReasonCode == "" && !command.Correction
}

func validTransitionIdentity(command TransitionCommand) bool {
	return command.StayID != uuid.Nil &&
		command.ExpectedVersion > 0 &&
		validIdempotencyKey(command.IdempotencyKey) &&
		command.RequestID != ""
}

func validCapabilityRequest(token, subject string) bool {
	return len(token) >= 64 && len(token) <= 128 && strings.TrimSpace(subject) != ""
}

func validInviteGroup(command InviteGroupCommand) bool {
	return command.ClientSubmissionID.Version() == 7 &&
		validIdempotencyKey(command.IdempotencyKey) &&
		command.RequestID != "" &&
		validPrivacyAndVisitors(command.PrivacyNoticeVersion, command.Visitors)
}

func validIdempotencyKey(value string) bool {
	return idempotencyKeyPattern.MatchString(value)
}
