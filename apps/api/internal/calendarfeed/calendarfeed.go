// Package calendarfeed imports the lodging platform's own calendar so that
// arrival and departure are typed once instead of twice.
//
// The direct route is closed: Booking.com's Connectivity API is reserved for
// homologated partners that manage price, availability and content in real
// time, which is the description of a channel manager and not of an
// observatory. What exists without a partnership is the iCalendar file the host
// exports from the extranet.
//
// That file carries no identity — the platforms stopped exporting it — and no
// guest count, and it does not separate a real booking from a maintenance block
// with any reliability. So the import produces an observation, never a stay:
// the lodging confirms the dates, states how many people came, and only then
// does core.stays exist. Anything else would let a maintenance block reach the
// published presence indicator (ADR-044).
package calendarfeed

import (
	"context"
	"errors"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/google/uuid"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource conflict")
	ErrForbidden    = errors.New("operation not permitted")
	// ErrPreconditionFailed answers a stale If-Match, never the row that
	// vanished between the read and the write: that one is a conflict.
	ErrPreconditionFailed = errors.New("precondition failed")
	ErrUnavailable        = errors.New("repository unavailable")
	// ErrNotCalendar and ErrMalformed are outcomes of the synchronization, not
	// failures of a request: they are written onto the feed so the lodging can
	// see why it stopped working, and they never interrupt manual registration.
	ErrNotCalendar = errors.New("response is not a calendar")
	ErrMalformed   = errors.New("calendar is malformed")
)

// MaxURLLength is generous for a signed extranet address and still finite;
// nothing legitimate approaches it.
const MaxURLLength = 2048

// MaxLabelLength matches the column. The label names the listing — "Chalé 3" —
// and the screen warns against writing anything else there.
const MaxLabelLength = 120

// SyncInterval is how often a feed is re-read. An extranet calendar changes
// when somebody books, not continuously, and every cycle is an outbound request
// against a host that never asked to serve us.
const SyncInterval = 2 * time.Hour

// SuspendAfterFailures stops a feed that has been failing for a whole day at the
// default cadence. Retrying a dead address forever turns a host's mistake into
// our outbound traffic, and the lodging sees a suspended feed instead of a
// silent one.
const SuspendAfterFailures = 12

type Provider string

const ProviderBooking Provider = "booking"

func (p Provider) Valid() bool {
	return p == ProviderBooking
}

type FeedStatus string

const (
	StatusActive    FeedStatus = "active"
	StatusSuspended FeedStatus = "suspended"
	// StatusRemoved instead of a delete: the stays already confirmed from this
	// feed are facts of the lodging and stand on their own, so erasing the
	// origin would erase the explanation of how they got in.
	StatusRemoved FeedStatus = "removed"
)

type SyncOutcome string

const (
	OutcomeOK          SyncOutcome = "ok"
	OutcomeUnreachable SyncOutcome = "unreachable"
	OutcomeNotCalendar SyncOutcome = "not_calendar"
	OutcomeMalformed   SyncOutcome = "malformed"
)

type ReservationKind string

const (
	KindReserved ReservationKind = "reserved"
	KindBlocked  ReservationKind = "blocked"
	KindUnknown  ReservationKind = "unknown"
)

type ReservationState string

const (
	StatePending   ReservationState = "pending"
	StateConfirmed ReservationState = "confirmed"
	StateDismissed ReservationState = "dismissed"
	StateWithdrawn ReservationState = "withdrawn"
)

// Feed is what the API returns. There is no URL field and there will not be
// one: the address is a bearer secret, and echoing it back would turn a screen
// that anyone with the operator role can open into a way to read the listing's
// whole calendar elsewhere.
type Feed struct {
	ID                  uuid.UUID    `json:"id"`
	AccommodationID     uuid.UUID    `json:"accommodation_id"`
	Provider            Provider     `json:"provider"`
	Label               string       `json:"label"`
	Status              FeedStatus   `json:"status"`
	LastSyncedAt        *time.Time   `json:"last_synced_at"`
	LastSyncOutcome     *SyncOutcome `json:"last_sync_outcome"`
	ConsecutiveFailures int32        `json:"consecutive_failures"`
	Version             int64        `json:"version"`
	CreatedAt           time.Time    `json:"created_at"`
	UpdatedAt           time.Time    `json:"updated_at"`
}

type Reservation struct {
	ID          uuid.UUID        `json:"id"`
	FeedID      uuid.UUID        `json:"feed_id"`
	ArrivalOn   string           `json:"arrival_on"`
	DepartureOn string           `json:"departure_on"`
	Kind        ReservationKind  `json:"kind"`
	State       ReservationState `json:"state"`
	StayID      *uuid.UUID       `json:"stay_id"`
	FirstSeenAt time.Time        `json:"first_seen_at"`
	LastSeenAt  time.Time        `json:"last_seen_at"`
	Version     int64            `json:"version"`
}

type CreateFeedCommand struct {
	Actor           access.Principal
	AccommodationID uuid.UUID
	Provider        Provider
	Label           string
	URL             string
	IdempotencyKey  string
	RequestID       string
}

type RemoveFeedCommand struct {
	Actor           access.Principal
	FeedID          uuid.UUID
	ExpectedVersion int64
	IdempotencyKey  string
	RequestID       string
}

type ListFeedsRequest struct {
	Actor           access.Principal
	AccommodationID uuid.UUID
}

// ListReservationsRequest defaults to the pending queue because that is the
// only list with something to do in it. An empty State means every state, for
// the screen that explains what already happened.
type ListReservationsRequest struct {
	Actor           access.Principal
	AccommodationID uuid.UUID
	State           ReservationState
	Limit           int32
}

// ConfirmCommand carries the guest count because the calendar never does. This
// is the moment the observation becomes a stay, and the number comes from the
// person who received the guests.
type ConfirmCommand struct {
	Actor              access.Principal
	ReservationID      uuid.UUID
	ExpectedVersion    int64
	ExpectedGuestCount int32
	ClientSubmissionID uuid.UUID
	IdempotencyKey     string
	RequestID          string
}

type DismissCommand struct {
	Actor           access.Principal
	ReservationID   uuid.UUID
	ExpectedVersion int64
	IdempotencyKey  string
	RequestID       string
}

type Repository interface {
	CreateFeed(context.Context, CreateFeedCommand, SealedURL, Fingerprint) (Feed, bool, error)
	ListFeeds(context.Context, ListFeedsRequest) ([]Feed, error)
	RemoveFeed(context.Context, RemoveFeedCommand) (Feed, bool, error)
	ListReservations(context.Context, ListReservationsRequest) ([]Reservation, error)
	Confirm(context.Context, ConfirmCommand) (Reservation, bool, error)
	Dismiss(context.Context, DismissCommand) (Reservation, bool, error)
}

type Service struct {
	repository Repository
	sealer     *URLSealer
}

func NewService(repository Repository, sealer *URLSealer) (*Service, error) {
	if repository == nil || sealer == nil {
		return nil, ErrInvalidInput
	}
	return &Service{repository: repository, sealer: sealer}, nil
}

func (s *Service) CreateFeed(
	ctx context.Context,
	command CreateFeedCommand,
) (Feed, bool, error) {
	normalized, err := NormalizeFeedURL(command.URL)
	if err != nil {
		return Feed{}, false, err
	}
	if err := validateCreateFeed(command); err != nil {
		return Feed{}, false, err
	}
	sealed, fingerprint, err := s.sealFeedURL(command.AccommodationID, normalized)
	if err != nil {
		return Feed{}, false, err
	}
	command.URL = ""
	return s.repository.CreateFeed(ctx, command, sealed, fingerprint)
}

func (s *Service) sealFeedURL(
	accommodationID uuid.UUID,
	normalized string,
) (SealedURL, Fingerprint, error) {
	sealed, err := s.sealer.Seal(normalized, accommodationID[:])
	if err != nil {
		return SealedURL{}, Fingerprint{}, err
	}
	fingerprint, err := s.sealer.Fingerprint(normalized)
	if err != nil {
		return SealedURL{}, Fingerprint{}, err
	}
	return sealed, fingerprint, nil
}

func (s *Service) ListFeeds(
	ctx context.Context,
	request ListFeedsRequest,
) ([]Feed, error) {
	if request.AccommodationID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	return s.repository.ListFeeds(ctx, request)
}

func (s *Service) RemoveFeed(
	ctx context.Context,
	command RemoveFeedCommand,
) (Feed, bool, error) {
	if command.FeedID == uuid.Nil || command.ExpectedVersion <= 0 {
		return Feed{}, false, ErrInvalidInput
	}
	return s.repository.RemoveFeed(ctx, command)
}

func (s *Service) ListReservations(
	ctx context.Context,
	request ListReservationsRequest,
) ([]Reservation, error) {
	if request.AccommodationID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	request.Limit = boundedLimit(request.Limit)
	return s.repository.ListReservations(ctx, request)
}

func (s *Service) Confirm(
	ctx context.Context,
	command ConfirmCommand,
) (Reservation, bool, error) {
	if err := validateConfirm(command); err != nil {
		return Reservation{}, false, err
	}
	return s.repository.Confirm(ctx, command)
}

func (s *Service) Dismiss(
	ctx context.Context,
	command DismissCommand,
) (Reservation, bool, error) {
	if command.ReservationID == uuid.Nil || command.ExpectedVersion <= 0 {
		return Reservation{}, false, ErrInvalidInput
	}
	return s.repository.Dismiss(ctx, command)
}

func boundedLimit(limit int32) int32 {
	if limit <= 0 || limit > 200 {
		return 50
	}
	return limit
}

func validateCreateFeed(command CreateFeedCommand) error {
	if command.AccommodationID == uuid.Nil || !command.Provider.Valid() {
		return ErrInvalidInput
	}
	label := strings.TrimSpace(command.Label)
	if label == "" || len(label) > MaxLabelLength {
		return ErrInvalidInput
	}
	return nil
}

// validateConfirm bounds the guest count with the same range core.stays
// enforces, so a refusal explains itself instead of arriving as a constraint
// violation the application cannot describe.
func validateConfirm(command ConfirmCommand) error {
	if command.ReservationID == uuid.Nil || command.ExpectedVersion <= 0 {
		return ErrInvalidInput
	}
	if command.ClientSubmissionID == uuid.Nil || !validGuestCount(command.ExpectedGuestCount) {
		return ErrInvalidInput
	}
	return nil
}

func validGuestCount(count int32) bool {
	return count >= 1 && count <= 100
}

// NormalizeFeedURL is the outbound guard. The system is about to fetch a host
// that a user named, which is new surface, so the rules are the ones a genuine
// `.ics` never needs to break: transport encrypted, no credentials smuggled in
// the address, no fragment, and a name that is not the inside of our own
// network.
func NormalizeFeedURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len(trimmed) > MaxURLLength {
		return "", ErrInvalidInput
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", ErrInvalidInput
	}
	if err := checkFeedURLShape(parsed); err != nil {
		return "", err
	}
	parsed.Fragment = ""
	return parsed.String(), nil
}

func checkFeedURLShape(parsed *url.URL) error {
	if !strings.EqualFold(parsed.Scheme, "https") || parsed.User != nil {
		return ErrInvalidInput
	}
	host := parsed.Hostname()
	if host == "" || !publicHost(host) {
		return ErrInvalidInput
	}
	return nil
}

// publicHost is the cheap, early refusal: an address literal pointing inside
// the deployment is rejected before anything is stored. A bare hostname passes
// here because a name is not an address — what it resolves to is decided later,
// can differ between lookups, and is therefore checked at dial time by
// controlDialedAddress, which is the guard that actually holds.
func publicHost(host string) bool {
	address, err := netip.ParseAddr(host)
	if err != nil {
		return !strings.EqualFold(host, "localhost")
	}
	return routableAddress(address)
}

// routableAddress is the single definition of "outside", shared by the address
// typed into the form and by every address the dialer is about to connect to.
// IsGlobalUnicast already excludes loopback, multicast and the unspecified
// address; private and link-local need saying because Go counts them as global
// unicast.
func routableAddress(address netip.Addr) bool {
	return address.IsValid() &&
		address.IsGlobalUnicast() &&
		!address.IsPrivate() &&
		!address.IsLinkLocalUnicast()
}
