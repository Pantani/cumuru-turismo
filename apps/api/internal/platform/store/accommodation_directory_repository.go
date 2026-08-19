package store

import (
	"context"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/Pantani/cumuru/apps/api/internal/directory"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/google/uuid"
)

// AccommodationDirectoryRepository lê a lista pública pelo pool autenticado, e
// não pelo pool de public_runtime. Os dois casos parecem o mesmo — rota aberta
// lendo dado publicado — mas public_data guarda release estatístico imutável,
// e a lista é o cadastro vivo: despublicar precisa sumir da lista na mesma
// transação, sem esperar snapshot. O recorte que public_runtime daria por
// privilégio está inteiro dentro da consulta, que filtra publicação e situação
// no banco e nunca projeta coluna que a lista não publica.
type AccommodationDirectoryRepository struct {
	store *Store
}

func NewAccommodationDirectoryRepository(store *Store) *AccommodationDirectoryRepository {
	return &AccommodationDirectoryRepository{store: store}
}

var _ directory.PublicReader = (*AccommodationDirectoryRepository)(nil)

func (r *AccommodationDirectoryRepository) List(
	ctx context.Context,
) (directory.Listing, error) {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	rows, err := r.store.queries.ListPublicAccommodationDirectory(
		ctx,
		directory.MaxEntries+1,
	)
	if err != nil {
		return directory.Listing{}, directory.ErrUnavailable
	}
	entries, updatedAt, err := directoryEntries(rows)
	if err != nil {
		return directory.Listing{}, err
	}
	return directory.NewListing(entries, updatedAt)
}

func directoryEntries(
	rows []generated.ListPublicAccommodationDirectoryRow,
) ([]directory.Entry, time.Time, error) {
	entries := make([]directory.Entry, 0, len(rows))
	var updatedAt time.Time
	for _, row := range rows {
		if row.PublicContactPhone == nil {
			return nil, time.Time{}, directory.ErrUnavailable
		}
		entries = append(entries, directoryEntry(row))
		if row.UpdatedAt.Time.After(updatedAt) {
			updatedAt = row.UpdatedAt.Time
		}
	}
	return entries, updatedAt, nil
}

func directoryEntry(
	row generated.ListPublicAccommodationDirectoryRow,
) directory.Entry {
	return directory.Entry{
		ID:       uuid.UUID(row.ID.Bytes),
		Name:     row.Name,
		Category: accommodation.Category(row.Category),
		Capacity: row.Capacity,
		AreaCode: row.PublicAreaCode,
		Phone:    *row.PublicContactPhone,
		WhatsApp: row.PublicContactWhatsapp,
		Website:  row.PublicWebsiteUrl,
	}
}
