package directory_test

import (
	"errors"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/directory"
	"github.com/google/uuid"
)

func entry() directory.Entry {
	return directory.Entry{
		ID:       uuid.MustParse("0198f000-0000-7000-8000-000000000001"),
		Name:     "Pousada da Vila",
		Category: accommodation.CategoryFormalLodging,
		Phone:    "+5573999990001",
	}
}

func TestNewListingCountsAndNormalizesTheMoment(t *testing.T) {
	t.Parallel()

	moment := time.Date(2026, 8, 18, 12, 0, 0, 0, time.FixedZone("BRT", -3*3600))
	listing, err := directory.NewListing([]directory.Entry{entry()}, moment)
	if err != nil {
		t.Fatalf("NewListing = %v", err)
	}
	if listing.Count != 1 {
		t.Fatalf("count = %d, want 1", listing.Count)
	}
	if listing.UpdatedAt.Location() != time.UTC {
		t.Fatalf("updated_at location = %s, want UTC", listing.UpdatedAt.Location())
	}
}

// Uma linha publicada sem o que a lista promete é defeito, e o documento inteiro
// é negado: servir as demais esconderia a linha quebrada de quem pode corrigi-la.
func TestNewListingRefusesIncompleteEntry(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*directory.Entry){
		"sem identificador": func(e *directory.Entry) { e.ID = uuid.Nil },
		"sem nome":          func(e *directory.Entry) { e.Name = "" },
		"sem telefone":      func(e *directory.Entry) { e.Phone = "" },
		"categoria fora do vocabulário": func(e *directory.Entry) {
			e.Category = accommodation.Category("hotel")
		},
	}
	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			broken := entry()
			corrupt(&broken)
			if _, err := directory.NewListing(
				[]directory.Entry{broken}, time.Now(),
			); !errors.Is(err, directory.ErrUnavailable) {
				t.Fatalf("err = %v, want ErrUnavailable", err)
			}
		})
	}
}

// unclassified é estado de linha herdada e existe no banco; recusá-la sumiria
// com a hospedagem da lista sem que ninguém tivesse despublicado nada.
func TestNewListingAcceptsUnclassifiedCategory(t *testing.T) {
	t.Parallel()

	legacy := entry()
	legacy.Category = accommodation.CategoryUnclassified
	if _, err := directory.NewListing(
		[]directory.Entry{legacy}, time.Now(),
	); err != nil {
		t.Fatalf("NewListing = %v", err)
	}
}

func TestNewListingRefusesMoreThanTheReadCeiling(t *testing.T) {
	t.Parallel()

	entries := make([]directory.Entry, directory.MaxEntries+1)
	for index := range entries {
		entries[index] = entry()
	}
	if _, err := directory.NewListing(
		entries, time.Now(),
	); !errors.Is(err, directory.ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}
