// Package directory publica a lista aberta de hospedagens: o documento que o
// hóspede lê antes de ter qualquer vínculo com o Observatório.
//
// É o oposto de analytics na finalidade e por isso não passa por lá. A leitura
// pública de analytics é estatística, agregada e defendida por supressão, e
// existe justamente para não identificar ninguém. Esta é nominal de propósito —
// nome e telefone de quem quer ser encontrado — e só publica o que a própria
// hospedagem pediu para publicar. Misturar as duas colocaria contato consentido
// atrás das regras de célula mínima, e a política de supressão passaria a
// decidir sobre um dado que não é amostra de nada.
package directory

import (
	"context"
	"errors"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/accommodation"
	"github.com/google/uuid"
)

// ErrUnavailable cobre tanto a falha de leitura quanto a linha incoerente. Uma
// hospedagem publicada sem telefone só existe se a constraint tiver sido
// contornada, e servir meia linha seria publicar defeito como se fosse dado.
var ErrUnavailable = errors.New("directory unavailable")

// MaxEntries é teto de leitura, não página. A lista é municipal e é servida
// inteira, num documento só, cacheável por inteiro; chegar ao teto significa que
// a premissa mudou, e a leitura recusa em vez de truncar em silêncio.
const MaxEntries = 1000

type Entry struct {
	ID       uuid.UUID              `json:"id"`
	Name     string                 `json:"name"`
	Category accommodation.Category `json:"category"`
	Capacity *int32                 `json:"capacity"`
	AreaCode *string                `json:"area_code"`
	Phone    string                 `json:"phone"`
	WhatsApp bool                   `json:"whatsapp"`
	Website  *string                `json:"website"`
}

type Listing struct {
	// UpdatedAt é o instante da alteração mais recente entre as publicadas, não
	// o da consulta: dois pedidos iguais devolvem o mesmo documento, e o ETag
	// deles não muda a cada segundo.
	UpdatedAt time.Time `json:"updated_at"`
	Count     int       `json:"count"`
	Entries   []Entry   `json:"entries"`
}

type PublicReader interface {
	List(context.Context) (Listing, error)
}

// NewListing recusa a lista que não pode ser publicada como está: categoria fora
// do vocabulário, telefone ausente ou nome em branco são defeito de linha, e o
// documento inteiro é negado — servir o resto esconderia a linha quebrada.
func NewListing(entries []Entry, updatedAt time.Time) (Listing, error) {
	if len(entries) > MaxEntries {
		return Listing{}, ErrUnavailable
	}
	for _, entry := range entries {
		if !validEntry(entry) {
			return Listing{}, ErrUnavailable
		}
	}
	return Listing{
		UpdatedAt: updatedAt.UTC(),
		Count:     len(entries),
		Entries:   entries,
	}, nil
}

func validEntry(entry Entry) bool {
	return entry.ID != uuid.Nil &&
		entry.Name != "" &&
		entry.Phone != "" &&
		entry.Category.Valid()
}
