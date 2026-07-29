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
	ErrInvalidInput       = errors.New("invalid input")
	ErrNotFound           = errors.New("resource not found")
	ErrConflict           = errors.New("resource conflict")
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
	Version                int64      `json:"version"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
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
	ArrivalFrom     string
	ArrivalTo       string
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

func validActor(actor access.Principal) bool {
	return actor.Issuer != "" && actor.Subject != ""
}

func validCreate(command CreateCommand) bool {
	if command.AccommodationID == uuid.Nil || command.ClientSubmissionID.Version() != 7 {
		return false
	}
	if command.ExpectedGuestCount < 1 || command.ExpectedGuestCount > 100 {
		return false
	}
	return validDateRange(command.PlannedArrivalOn, command.PlannedDepartureOn) &&
		validIdempotencyKey(command.IdempotencyKey) && command.RequestID != ""
}

func validDateRange(arrivalValue, departureValue string) bool {
	arrival, err := ParseCivilDate(arrivalValue)
	if err != nil {
		return false
	}
	departure, err := ParseCivilDate(departureValue)
	return err == nil && arrival.Before(departure)
}

func validPage(page PageRequest) bool {
	if page.Limit < 1 || page.Limit > 100 {
		return false
	}
	if page.CursorCreatedAt.IsZero() != (page.CursorID == uuid.Nil) {
		return false
	}
	if page.Status != "" && !validStatus(page.Status) {
		return false
	}
	return validOptionalDates(page.ArrivalFrom, page.ArrivalTo)
}

func validOptionalDates(from, to string) bool {
	if from != "" {
		if _, err := ParseCivilDate(from); err != nil {
			return false
		}
	}
	if to != "" {
		if _, err := ParseCivilDate(to); err != nil {
			return false
		}
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
	if command.ExpectedVersion < 1 || !validIdempotencyKey(command.IdempotencyKey) || command.RequestID == "" {
		return false
	}
	return validPrivacyAndVisitors(command.PrivacyNoticeVersion, command.Visitors)
}

func validPrivacyAndVisitors(version string, visitors []Visitor) bool {
	if strings.TrimSpace(version) == "" || len(version) > 100 {
		return false
	}
	return ValidateGroup(visitors) == nil
}

func validInviteCommand(command InviteCommand) bool {
	return command.StayID != uuid.Nil &&
		command.ExpectedVersion > 0 &&
		strings.TrimSpace(command.PrivacyNoticeVersion) != "" &&
		len(command.PrivacyNoticeVersion) <= 100 &&
		validIdempotencyKey(command.IdempotencyKey) &&
		command.RequestID != ""
}

func validTransitionCommand(command TransitionCommand) bool {
	if !validTransitionIdentity(command) {
		return false
	}
	switch command.Kind {
	case TransitionCheckIn, TransitionCheckOut:
		return command.ReasonCode == "" && !command.Correction
	case TransitionCancel:
		return CancelReason(command.ReasonCode).Valid()
	case TransitionNoShow:
		return command.ReasonCode == "" ||
			command.ReasonCode == "guest_absent" ||
			command.ReasonCode == "accommodation_report"
	default:
		return false
	}
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
