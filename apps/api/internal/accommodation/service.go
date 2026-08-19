package accommodation

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/google/uuid"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrForbidden          = errors.New("operation forbidden")
	ErrNotFound           = errors.New("resource not found")
	ErrConflict           = errors.New("resource conflict")
	ErrPreconditionFailed = errors.New("precondition failed")
	ErrUnavailable        = errors.New("repository unavailable")
	idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{16,128}$`)
	// O mesmo E.164 da constraint accommodations_public_phone_e164: o número
	// vira link discável e link de WhatsApp, e nenhum dos dois aceita separador.
	publicPhonePattern = regexp.MustCompile(`^\+[1-9][0-9]{9,14}$`)
)

const publicWebsiteMaxLength = 180

type Accommodation struct {
	ID             uuid.UUID     `json:"id"`
	OrganizationID uuid.UUID     `json:"organization_id"`
	Name           string        `json:"name"`
	Category       Category      `json:"category"`
	Status         Status        `json:"status"`
	CadasturID     *string       `json:"cadastur_id,omitempty"`
	Capacity       *int32        `json:"capacity,omitempty"`
	PublicAreaCode *string       `json:"public_area_code,omitempty"`
	PublicListing  PublicListing `json:"public_listing"`
	Version        int64         `json:"version"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

// PublicListing é o consentimento de publicação na lista aberta de hospedagens,
// junto com o contato que ele autoriza. Fica no recurso da acomodação, e não em
// tabela própria, porque é atributo do cadastro e não relação: quem edita a
// acomodação é quem publica, sob a mesma versão otimista.
type PublicListing struct {
	Enabled     bool       `json:"enabled"`
	Phone       *string    `json:"phone"`
	WhatsApp    bool       `json:"whatsapp"`
	Website     *string    `json:"website"`
	ConsentedAt *time.Time `json:"consented_at"`
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
	SetName                  bool
	Name                     string
	SetCategory              bool
	Category                 Category
	SetCapacity              bool
	Capacity                 *int32
	SetPublicAreaCode        bool
	PublicAreaCode           *string
	SetPublicListingEnabled  bool
	PublicListingEnabled     bool
	SetPublicContactPhone    bool
	PublicContactPhone       *string
	SetPublicContactWhatsApp bool
	PublicContactWhatsApp    bool
	SetPublicWebsiteURL      bool
	PublicWebsiteURL         *string
}

type CreateCommand struct {
	Actor              access.Principal
	Name               string
	Category           Category
	Capacity           int32
	ClientSubmissionID uuid.UUID
	IdempotencyKey     string
	RequestID          string
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
	Create(context.Context, CreateCommand) (Accommodation, bool, error)
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

func (s *Service) Create(
	ctx context.Context,
	command CreateCommand,
) (Accommodation, bool, error) {
	if !validActor(command.Actor) || !validCreate(command) {
		return Accommodation{}, false, ErrInvalidInput
	}
	return s.repository.Create(ctx, command)
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

func validAccommodationPatch(patch UpdatePatch) bool {
	if !hasAccommodationPatch(patch) {
		return false
	}
	if !validRequiredTextPatch(patch.SetName, patch.Name, 200) {
		return false
	}
	return !patch.SetCategory || patch.Category.ValidInput()
}

func validUpdate(command UpdateCommand) bool {
	return validUpdateIdentity(command) &&
		validAccommodationPatch(command.Patch) &&
		validOptionalFields(command.Patch)
}

func validUpdateIdentity(command UpdateCommand) bool {
	return command.AccommodationID != uuid.Nil &&
		command.ExpectedVersion > 0 &&
		command.RequestID != ""
}

func hasAccommodationPatch(patch UpdatePatch) bool {
	return patch.SetName || patch.SetCategory ||
		patch.SetCapacity || patch.SetPublicAreaCode ||
		hasPublicListingPatch(patch)
}

func hasPublicListingPatch(patch UpdatePatch) bool {
	return patch.SetPublicListingEnabled || patch.SetPublicContactPhone ||
		patch.SetPublicContactWhatsApp || patch.SetPublicWebsiteURL
}

func validRequiredTextPatch(set bool, value string, maximum int) bool {
	return !set || (strings.TrimSpace(value) != "" && validTextLength(value, maximum))
}

func validOptionalFields(patch UpdatePatch) bool {
	if !validNullableCapacity(patch.SetCapacity, patch.Capacity) {
		return false
	}
	if !validNullableText(patch.SetPublicAreaCode, patch.PublicAreaCode, 100) {
		return false
	}
	return validPublicListingPatch(patch)
}

// Publicar sem telefone não é pedido incompleto, é pedido contraditório: a
// lista existe para o hóspede ligar. A contradição visível no próprio corpo é
// recusada aqui; publicar contando com telefone que a linha não tem é estado, e
// morre na constraint.
func validPublicListingPatch(patch UpdatePatch) bool {
	return validPublicPhonePatch(patch) &&
		validPublicWebsitePatch(patch) &&
		validPublishKeepsPhone(patch)
}

func validPublishKeepsPhone(patch UpdatePatch) bool {
	if !patch.SetPublicListingEnabled || !patch.PublicListingEnabled {
		return true
	}
	return !patch.SetPublicContactPhone || patch.PublicContactPhone != nil
}

func validPublicPhonePatch(patch UpdatePatch) bool {
	if !patch.SetPublicContactPhone || patch.PublicContactPhone == nil {
		return true
	}
	return publicPhonePattern.MatchString(*patch.PublicContactPhone)
}

func validPublicWebsitePatch(patch UpdatePatch) bool {
	if !patch.SetPublicWebsiteURL || patch.PublicWebsiteURL == nil {
		return true
	}
	return validPublicWebsite(*patch.PublicWebsiteURL)
}

func validPublicWebsite(value string) bool {
	if !validTextLength(value, publicWebsiteMaxLength) ||
		strings.ContainsAny(value, " \t\n\r") {
		return false
	}
	address, err := url.Parse(value)
	return err == nil && address.Scheme == "https" && address.Host != ""
}

func validCreateShape(command CreateCommand) bool {
	return strings.TrimSpace(command.Name) != "" &&
		validTextLength(command.Name, 200) &&
		command.Category.ValidInput() &&
		command.Capacity >= 1 && command.Capacity <= 10000
}

func validSubmissionID(id uuid.UUID) bool {
	return id.Version() == 7 && id.Variant() == uuid.RFC4122
}

func validMutationMeta(idempotencyKey, requestID string) bool {
	return idempotencyKeyPattern.MatchString(idempotencyKey) && requestID != ""
}

func validCreate(command CreateCommand) bool {
	return validCreateShape(command) &&
		validSubmissionID(command.ClientSubmissionID) &&
		validMutationMeta(command.IdempotencyKey, command.RequestID)
}

func validNullableText(set bool, value *string, maximum int) bool {
	return !set || value == nil || validTextLength(*value, maximum)
}

func validTextLength(value string, maximum int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximum
}

func validNullableCapacity(set bool, value *int32) bool {
	return !set || value == nil || (*value >= 1 && *value <= 10000)
}

func validIssuerURL(value string) bool {
	issuer, err := url.Parse(value)
	return err == nil && issuer.IsAbs() && issuer.Host != "" && len(value) <= 2048
}

func validCreateMembership(command CreateMembershipCommand) bool {
	return validMembershipTarget(command) &&
		validMutationMeta(command.IdempotencyKey, command.RequestID) &&
		validIssuerURL(command.TargetIssuer)
}

func validMembershipTarget(command CreateMembershipCommand) bool {
	return command.AccommodationID != uuid.Nil &&
		command.Role.Valid() &&
		command.TargetSubject != "" &&
		len(command.TargetSubject) <= 255
}

func validMembershipIdentity(command UpdateMembershipCommand) bool {
	return command.AccommodationID != uuid.Nil &&
		command.MembershipID != uuid.Nil &&
		command.ExpectedVersion >= 1 &&
		command.RequestID != ""
}

func validMembershipPatch(patch UpdateMembershipPatch) bool {
	if !patch.SetRole && !patch.SetActive {
		return false
	}
	return !patch.SetRole || patch.Role.Valid()
}

func validUpdateMembership(command UpdateMembershipCommand) bool {
	return validMembershipIdentity(command) && validMembershipPatch(command.Patch)
}
