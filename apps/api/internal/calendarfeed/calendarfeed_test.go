package calendarfeed_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/calendarfeed"
	"github.com/google/uuid"
)

// The system is about to fetch a host a user named, so the guard is the part
// worth proving: every rejected shape here is one a genuine `.ics` never needs.
func TestNormalizeFeedURLRefusesUnsafeAddresses(t *testing.T) {
	t.Parallel()

	refused := map[string]string{
		"plain http":       "http://ical.booking.com/v1/export?t=abc",
		"embedded secret":  "https://user:pass@ical.booking.com/v1/export",
		"loopback name":    "https://localhost/export.ics",
		"loopback address": "https://127.0.0.1/export.ics",
		"private range":    "https://10.0.0.5/export.ics",
		"link local":       "https://169.254.169.254/latest/meta-data",
		"empty":            "   ",
		"not a scheme":     "ical.booking.com/v1/export",
	}
	for name, address := range refused {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := calendarfeed.NormalizeFeedURL(address); err == nil {
				t.Fatalf("NormalizeFeedURL(%q) error = nil", address)
			}
		})
	}
}

func TestNormalizeFeedURLAcceptsTheExtranetAddressAndDropsTheFragment(t *testing.T) {
	t.Parallel()

	normalized, err := calendarfeed.NormalizeFeedURL(
		"  https://ical.booking.com/v1/export?t=9f2a#anchor  ",
	)
	if err != nil {
		t.Fatalf("NormalizeFeedURL() error = %v", err)
	}
	if normalized != "https://ical.booking.com/v1/export?t=9f2a" {
		t.Fatalf("NormalizeFeedURL() = %q", normalized)
	}
}

func TestSealerRoundTripsAndBindsTheAddressToItsAccommodation(t *testing.T) {
	t.Parallel()

	sealer := testSealer(t)
	owner := uuid.MustParse("019f0000-0000-7000-8000-000000000001")
	other := uuid.MustParse("019f0000-0000-7000-8000-000000000002")
	address := "https://ical.booking.com/v1/export?t=9f2a"

	sealed, err := sealer.Seal(address, owner[:])
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	opened, err := sealer.Open(sealed, owner[:])
	if err != nil || opened != address {
		t.Fatalf("Open() = %q, %v", opened, err)
	}
	// A row moved to another accommodation must stop decrypting, so a feed
	// cannot be reassigned by writing to the foreign key alone.
	if _, err := sealer.Open(sealed, other[:]); err == nil {
		t.Fatal("Open(other accommodation) error = nil")
	}
}

func TestFingerprintIsStableAndDoesNotRevealTheAddress(t *testing.T) {
	t.Parallel()

	sealer := testSealer(t)
	address := "https://ical.booking.com/v1/export?t=9f2a"
	first, err := sealer.Fingerprint(address)
	if err != nil {
		t.Fatalf("Fingerprint() error = %v", err)
	}
	second, _ := sealer.Fingerprint(address)
	if string(first.Digest) != string(second.Digest) {
		t.Fatal("Fingerprint() is not deterministic")
	}
	if len(first.Digest) != 32 || string(first.Digest) == address {
		t.Fatalf("Fingerprint() digest = %x", first.Digest)
	}
	other, _ := sealer.Fingerprint("https://ical.booking.com/v1/export?t=0000")
	if string(other.Digest) == string(first.Digest) {
		t.Fatal("Fingerprint() collided across addresses")
	}
}

// The same UID under two different feeds must not collide, or one lodging's
// queue would silently absorb another's reservation.
func TestFingerprintUIDIsScopedToItsFeed(t *testing.T) {
	t.Parallel()

	sealer := testSealer(t)
	first, err := sealer.FingerprintUID("feed-a", "booking-0001")
	if err != nil {
		t.Fatalf("FingerprintUID() error = %v", err)
	}
	second, _ := sealer.FingerprintUID("feed-b", "booking-0001")
	if string(first.Digest) == string(second.Digest) {
		t.Fatal("FingerprintUID() collided across feeds")
	}
}

func TestServiceRefusesAFeedItCannotStoreSafely(t *testing.T) {
	t.Parallel()

	service := testService(t, &recordingRepository{})
	base := calendarfeed.CreateFeedCommand{
		AccommodationID: uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		Provider:        calendarfeed.ProviderBooking,
		Label:           "Chalé 3",
		URL:             "https://ical.booking.com/v1/export?t=9f2a",
	}
	refused := map[string]func(calendarfeed.CreateFeedCommand) calendarfeed.CreateFeedCommand{
		"blank label":      func(c calendarfeed.CreateFeedCommand) calendarfeed.CreateFeedCommand { c.Label = "  "; return c },
		"unknown provider": func(c calendarfeed.CreateFeedCommand) calendarfeed.CreateFeedCommand { c.Provider = "airbnb"; return c },
		"unsafe url": func(c calendarfeed.CreateFeedCommand) calendarfeed.CreateFeedCommand {
			c.URL = "http://localhost/x.ics"
			return c
		},
	}
	for name, mutate := range refused {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, _, err := service.CreateFeed(context.Background(), mutate(base))
			if !errors.Is(err, calendarfeed.ErrInvalidInput) {
				t.Fatalf("CreateFeed(%s) error = %v", name, err)
			}
		})
	}
}

// The repository must never receive the address in the clear: it is stored
// sealed, and the plaintext stops at the service boundary.
func TestCreateFeedHandsTheRepositoryOnlySealedMaterial(t *testing.T) {
	t.Parallel()

	repository := &recordingRepository{}
	service := testService(t, repository)
	_, _, err := service.CreateFeed(context.Background(), calendarfeed.CreateFeedCommand{
		AccommodationID: uuid.MustParse("019f0000-0000-7000-8000-000000000001"),
		Provider:        calendarfeed.ProviderBooking,
		Label:           "Chalé 3",
		URL:             "https://ical.booking.com/v1/export?t=9f2a",
	})
	if err != nil {
		t.Fatalf("CreateFeed() error = %v", err)
	}
	if repository.created.URL != "" {
		t.Fatalf("repository received the address in the clear: %q", repository.created.URL)
	}
	if len(repository.sealed.Ciphertext) == 0 || len(repository.fingerprint.Digest) == 0 {
		t.Fatal("repository received no sealed material")
	}
}

func testService(t *testing.T, repository calendarfeed.Repository) *calendarfeed.Service {
	t.Helper()
	service, err := calendarfeed.NewService(repository, testSealer(t))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func testSealer(t *testing.T) *calendarfeed.URLSealer {
	t.Helper()
	sealing := calendarfeed.Keyring{
		CurrentVersion: "feed-v1",
		Keys:           map[string][]byte{"feed-v1": []byte("calendar-feed-sealing-key-32byte")},
	}
	fingerprint := calendarfeed.Keyring{
		CurrentVersion: "print-v1",
		Keys:           map[string][]byte{"print-v1": []byte("calendar-feed-print-key-32bytes!")},
	}
	sealer, err := calendarfeed.NewURLSealer(sealing, fingerprint)
	if err != nil {
		t.Fatalf("NewURLSealer() error = %v", err)
	}
	return sealer
}

type recordingRepository struct {
	created     calendarfeed.CreateFeedCommand
	sealed      calendarfeed.SealedURL
	fingerprint calendarfeed.Fingerprint
}

func (r *recordingRepository) CreateFeed(
	_ context.Context,
	command calendarfeed.CreateFeedCommand,
	sealed calendarfeed.SealedURL,
	fingerprint calendarfeed.Fingerprint,
) (calendarfeed.Feed, bool, error) {
	r.created = command
	r.sealed = sealed
	r.fingerprint = fingerprint
	return calendarfeed.Feed{ID: uuid.New()}, true, nil
}

func (r *recordingRepository) ListFeeds(
	context.Context, calendarfeed.ListFeedsRequest,
) ([]calendarfeed.Feed, error) {
	return nil, nil
}

func (r *recordingRepository) RemoveFeed(
	context.Context, calendarfeed.RemoveFeedCommand,
) (calendarfeed.Feed, bool, error) {
	return calendarfeed.Feed{}, false, nil
}

func (r *recordingRepository) ListReservations(
	context.Context, calendarfeed.ListReservationsRequest,
) ([]calendarfeed.Reservation, error) {
	return nil, nil
}

func (r *recordingRepository) Confirm(
	context.Context, calendarfeed.ConfirmCommand,
) (calendarfeed.Reservation, bool, error) {
	return calendarfeed.Reservation{}, false, nil
}

func (r *recordingRepository) Dismiss(
	context.Context, calendarfeed.DismissCommand,
) (calendarfeed.Reservation, bool, error) {
	return calendarfeed.Reservation{}, false, nil
}
