package accommodation

import (
	"context"
	"errors"
	"net/url"
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
	ErrUnavailable        = errors.New("repository unavailable")
	idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{16,128}$`)
)

type Accommodation struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Name           string    `json:"name"`
	Category       string    `json:"category"`
	Status         Status    `json:"status"`
	CadasturID     *string   `json:"cadastur_id,omitempty"`
	Capacity       *int32    `json:"capacity,omitempty"`
	PublicAreaCode *string   `json:"public_area_code,omitempty"`
	Version        int64     `json:"version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Membership struct {
	ID              uuid.UUID `json:"id"`
	AccommodationID uuid.UUID `json:"accommodation_id"`
	OIDCIssuer      string    `json:"oidc_issuer"`
	OIDCSubject     string    `json:"oidc_subject"`
	Role            Role      `json:"role"`
	Active          bool      `json:"active"`
	Version         int64     `json:"version"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type MembershipCreated struct {
	ID              uuid.UUID `json:"id"`
	AccommodationID uuid.UUID `json:"accommodation_id"`
	Role            Role      `json:"role"`
	Active          bool      `json:"active"`
	Version         int64     `json:"version"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type PageRequest struct {
	CursorCreatedAt time.Time
	CursorID        uuid.UUID
	Limit           int32
}

type AccommodationPage struct {
	Items      []Accommodation
	NextCursor *PageCursor
}

type MembershipPage struct {
	Items      []Membership
	NextCursor *PageCursor
}

type PageCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

type UpdatePatch struct {
	SetName           bool
	Name              string
	SetCategory       bool
	Category          string
	SetCadasturID     bool
	CadasturID        *string
	SetCapacity       bool
	Capacity          *int32
	SetPublicAreaCode bool
	PublicAreaCode    *string
}

type UpdateCommand struct {
	Actor           access.Principal
	AccommodationID uuid.UUID
	ExpectedVersion int64
	Patch           UpdatePatch
	RequestID       string
}

type CreateMembershipCommand struct {
	Actor           access.Principal
	AccommodationID uuid.UUID
	TargetIssuer    string
	TargetSubject   string
	Role            Role
	IdempotencyKey  string
	RequestID       string
}

type UpdateMembershipPatch struct {
	SetRole   bool
	Role      Role
	SetActive bool
	Active    bool
}

type UpdateMembershipCommand struct {
	Actor           access.Principal
	AccommodationID uuid.UUID
	MembershipID    uuid.UUID
	ExpectedVersion int64
	Patch           UpdateMembershipPatch
	RequestID       string
}

type Repository interface {
	List(context.Context, access.Principal, PageRequest) (AccommodationPage, error)
	Get(context.Context, access.Principal, uuid.UUID) (Accommodation, error)
	Update(context.Context, UpdateCommand) (Accommodation, error)
	ListMemberships(context.Context, access.Principal, uuid.UUID, PageRequest) (MembershipPage, error)
	CreateMembership(context.Context, CreateMembershipCommand) (MembershipCreated, bool, error)
	UpdateMembership(context.Context, UpdateMembershipCommand) (Membership, error)
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) List(ctx context.Context, actor access.Principal, page PageRequest) (AccommodationPage, error) {
	if !validActor(actor) || !validPage(page) {
		return AccommodationPage{}, ErrInvalidInput
	}
	return s.repository.List(ctx, actor, page)
}

func (s *Service) Get(ctx context.Context, actor access.Principal, id uuid.UUID) (Accommodation, error) {
	if !validActor(actor) || id == uuid.Nil {
		return Accommodation{}, ErrInvalidInput
	}
	return s.repository.Get(ctx, actor, id)
}

func (s *Service) Update(ctx context.Context, command UpdateCommand) (Accommodation, error) {
	if !validActor(command.Actor) || !validUpdate(command) {
		return Accommodation{}, ErrInvalidInput
	}
	return s.repository.Update(ctx, command)
}

func (s *Service) ListMemberships(
	ctx context.Context,
	actor access.Principal,
	accommodationID uuid.UUID,
	page PageRequest,
) (MembershipPage, error) {
	if !validActor(actor) || accommodationID == uuid.Nil || !validPage(page) {
		return MembershipPage{}, ErrInvalidInput
	}
	return s.repository.ListMemberships(ctx, actor, accommodationID, page)
}

func (s *Service) CreateMembership(
	ctx context.Context,
	command CreateMembershipCommand,
) (MembershipCreated, bool, error) {
	if !validActor(command.Actor) || !validCreateMembership(command) {
		return MembershipCreated{}, false, ErrInvalidInput
	}
	return s.repository.CreateMembership(ctx, command)
}

func (s *Service) UpdateMembership(
	ctx context.Context,
	command UpdateMembershipCommand,
) (Membership, error) {
	if !validActor(command.Actor) || !validUpdateMembership(command) {
		return Membership{}, ErrInvalidInput
	}
	return s.repository.UpdateMembership(ctx, command)
}

func validActor(actor access.Principal) bool {
	return actor.Issuer != "" && actor.Subject != ""
}

func validPage(page PageRequest) bool {
	if page.Limit < 1 || page.Limit > 100 {
		return false
	}
	return page.CursorCreatedAt.IsZero() == (page.CursorID == uuid.Nil)
}

func validUpdate(command UpdateCommand) bool {
	if !validUpdateIdentity(command) {
		return false
	}
	patch := command.Patch
	if !hasAccommodationPatch(patch) {
		return false
	}
	if !validRequiredTextPatch(patch.SetName, patch.Name, 200) {
		return false
	}
	if !validRequiredTextPatch(patch.SetCategory, patch.Category, 100) {
		return false
	}
	return validOptionalFields(patch)
}

func validUpdateIdentity(command UpdateCommand) bool {
	return command.AccommodationID != uuid.Nil &&
		command.ExpectedVersion > 0 &&
		command.RequestID != ""
}

func hasAccommodationPatch(patch UpdatePatch) bool {
	return patch.SetName || patch.SetCategory || patch.SetCadasturID ||
		patch.SetCapacity || patch.SetPublicAreaCode
}

func validRequiredTextPatch(set bool, value string, maximum int) bool {
	return !set || (strings.TrimSpace(value) != "" && len(value) <= maximum)
}

func validOptionalFields(patch UpdatePatch) bool {
	if !validNullableText(patch.SetCadasturID, patch.CadasturID, 100) {
		return false
	}
	if !validNullableCapacity(patch.SetCapacity, patch.Capacity) {
		return false
	}
	return validNullableText(patch.SetPublicAreaCode, patch.PublicAreaCode, 100)
}

func validNullableText(set bool, value *string, maximum int) bool {
	return !set || value == nil || len(*value) <= maximum
}

func validNullableCapacity(set bool, value *int32) bool {
	return !set || value == nil || (*value >= 1 && *value <= 10000)
}

func validCreateMembership(command CreateMembershipCommand) bool {
	if !validMembershipTarget(command) {
		return false
	}
	if !idempotencyKeyPattern.MatchString(command.IdempotencyKey) || command.RequestID == "" {
		return false
	}
	issuer, err := url.Parse(command.TargetIssuer)
	return err == nil && issuer.IsAbs() && issuer.Host != "" && len(command.TargetIssuer) <= 2048
}

func validMembershipTarget(command CreateMembershipCommand) bool {
	return command.AccommodationID != uuid.Nil &&
		command.Role.Valid() &&
		command.TargetSubject != "" &&
		len(command.TargetSubject) <= 255
}

func validUpdateMembership(command UpdateMembershipCommand) bool {
	if command.AccommodationID == uuid.Nil || command.MembershipID == uuid.Nil {
		return false
	}
	if command.ExpectedVersion < 1 || command.RequestID == "" {
		return false
	}
	if !command.Patch.SetRole && !command.Patch.SetActive {
		return false
	}
	return !command.Patch.SetRole || command.Patch.Role.Valid()
}
