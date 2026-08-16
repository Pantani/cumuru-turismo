package seed

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
	"github.com/google/uuid"
)

// validCategories mirrors the accommodations_category_valid constraint. A
// catalog is authored by hand, so an unknown category is rejected while the
// file is read instead of surfacing as a database error mid-run.
var validCategories = map[string]bool{
	"formal_lodging":  true,
	"seasonal_rental": true,
	"family_hosting":  true,
	"camping":         true,
	"regularizing":    true,
	"other":           true,
	"unclassified":    true,
}

// reservedIDPrefixes are the identifier ranges the local-demo fixtures own:
// 019fae10 for its organization, 019fae11 for its accommodations and 019fae12
// for its memberships. A catalog that reuses one of them overwrites a fixture
// row, and the next local-demo run fails with a conflict it cannot resolve —
// which is exactly the mistake this check turns into a startup error.
var reservedIDPrefixes = []string{"019fae10", "019fae11", "019fae12"}

// catalogFile is the on-disk shape of a versioned establishment catalog. Every
// identifier is declared by the file so a re-run updates the same rows.
type catalogFile struct {
	Organization catalogOrganization `json:"organization"`
}

type catalogOrganization struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Accommodations []catalogAccommodation `json:"accommodations"`
}

type catalogAccommodation struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Category       string  `json:"category"`
	Capacity       *int32  `json:"capacity"`
	PublicAreaCode *string `json:"public_area_code"`
	CadasturID     *string `json:"cadastur_id"`
}

// loadCatalog decodes strictly: an unknown key means the file was written
// against a different contract, and applying it partially would be worse than
// refusing it.
func loadCatalog(path string) (store.SeedOrganization, error) {
	handle, err := os.Open(path)
	if err != nil {
		return store.SeedOrganization{}, fmt.Errorf(
			"accommodation catalog is unreadable: %w", err,
		)
	}
	defer handle.Close()
	decoder := json.NewDecoder(handle)
	decoder.DisallowUnknownFields()
	var file catalogFile
	if err := decoder.Decode(&file); err != nil {
		return store.SeedOrganization{}, fmt.Errorf(
			"accommodation catalog is malformed: %w", err,
		)
	}
	return convertCatalog(file.Organization)
}

func convertCatalog(
	organization catalogOrganization,
) (store.SeedOrganization, error) {
	id, err := parseCatalogID(organization.ID)
	if err != nil {
		return store.SeedOrganization{}, fmt.Errorf("catalog organization id: %w", err)
	}
	if strings.TrimSpace(organization.Name) == "" {
		return store.SeedOrganization{}, errors.New(
			"catalog organization name is required",
		)
	}
	accommodations, err := convertAccommodations(organization.Accommodations)
	if err != nil {
		return store.SeedOrganization{}, err
	}
	return store.SeedOrganization{
		ID: id, Name: organization.Name, Accommodations: accommodations,
	}, nil
}

func convertAccommodations(
	entries []catalogAccommodation,
) ([]store.SeedAccommodation, error) {
	if len(entries) == 0 {
		return nil, errors.New("catalog declares no accommodation")
	}
	seen := make(map[uuid.UUID]bool, len(entries))
	accommodations := make([]store.SeedAccommodation, 0, len(entries))
	for _, entry := range entries {
		accommodation, err := convertAccommodation(entry)
		if err != nil {
			return nil, err
		}
		if seen[accommodation.ID] {
			return nil, fmt.Errorf("catalog repeats accommodation %s", accommodation.ID)
		}
		seen[accommodation.ID] = true
		accommodations = append(accommodations, accommodation)
	}
	return accommodations, nil
}

func convertAccommodation(
	entry catalogAccommodation,
) (store.SeedAccommodation, error) {
	id, err := parseCatalogID(entry.ID)
	if err != nil {
		return store.SeedAccommodation{}, fmt.Errorf(
			"catalog accommodation %q: %w", entry.Name, err,
		)
	}
	if err := validateAccommodationFields(entry); err != nil {
		return store.SeedAccommodation{}, err
	}
	return store.SeedAccommodation{
		ID:             id,
		Name:           strings.TrimSpace(entry.Name),
		Category:       entry.Category,
		CadasturID:     entry.CadasturID,
		Capacity:       entry.Capacity,
		PublicAreaCode: entry.PublicAreaCode,
	}, nil
}

func parseCatalogID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return uuid.Nil, errors.New("not a uuid")
	}
	text := id.String()
	for _, prefix := range reservedIDPrefixes {
		if strings.HasPrefix(text, prefix) {
			return uuid.Nil, fmt.Errorf(
				"%s is reserved for the local-demo fixtures", text,
			)
		}
	}
	return id, nil
}

func validateAccommodationFields(entry catalogAccommodation) error {
	if strings.TrimSpace(entry.Name) == "" {
		return fmt.Errorf("catalog accommodation %s has no name", entry.ID)
	}
	if !validCategories[entry.Category] {
		return fmt.Errorf(
			"catalog accommodation %s has unknown category %q",
			entry.ID, entry.Category,
		)
	}
	if entry.Capacity != nil && *entry.Capacity <= 0 {
		return fmt.Errorf("catalog accommodation %s has non positive capacity", entry.ID)
	}
	return nil
}
