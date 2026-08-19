package store

import (
	"errors"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/directory"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func directoryRow(phone *string) generated.ListPublicAccommodationDirectoryRow {
	capacity := int32(12)
	area := "cumuruxatiba"
	return generated.ListPublicAccommodationDirectoryRow{
		ID: pgtype.UUID{
			Bytes: uuid.MustParse("019fae11-0000-7000-8000-000000000001"),
			Valid: true,
		},
		Name: "Pousada Farol Fictícia", Category: "formal_lodging",
		Capacity: &capacity, PublicAreaCode: &area,
		PublicContactPhone: phone, PublicContactWhatsapp: true,
		UpdatedAt: pgtype.Timestamptz{
			Time:  time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC),
			Valid: true,
		},
	}
}

func TestDirectoryEntriesCarryTheMostRecentPublication(t *testing.T) {
	t.Parallel()

	phone := "+5573999990001"
	older := directoryRow(&phone)
	older.UpdatedAt.Time = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	entries, updatedAt, err := directoryEntries(
		[]generated.ListPublicAccommodationDirectoryRow{older, directoryRow(&phone)},
	)
	if err != nil {
		t.Fatalf("directoryEntries = %v", err)
	}
	if len(entries) != 2 || entries[0].Phone != phone || !entries[0].WhatsApp {
		t.Fatalf("entries = %+v", entries)
	}
	want := time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC)
	if !updatedAt.Equal(want) {
		t.Fatalf("updated_at = %s, want %s", updatedAt, want)
	}
}

// A constraint já impede publicar sem telefone; se a linha chegar assim mesmo, a
// leitura recusa em vez de servir entrada em que não dá para ligar.
func TestDirectoryEntriesRefuseAPublishedRowWithoutPhone(t *testing.T) {
	t.Parallel()

	_, _, err := directoryEntries(
		[]generated.ListPublicAccommodationDirectoryRow{directoryRow(nil)},
	)
	if !errors.Is(err, directory.ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestPublicListingKeepsConsentAbsentWhenNotPublished(t *testing.T) {
	t.Parallel()

	listing := publicListing(false, nil, false, nil, pgtype.Timestamptz{})
	if listing.Enabled || listing.ConsentedAt != nil {
		t.Fatalf("listing = %+v", listing)
	}
	moment := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	phone := "+5573999990001"
	published := publicListing(
		true, &phone, true, nil, pgtype.Timestamptz{Time: moment, Valid: true},
	)
	if published.ConsentedAt == nil || !published.ConsentedAt.Equal(moment) {
		t.Fatalf("published = %+v", published)
	}
}
