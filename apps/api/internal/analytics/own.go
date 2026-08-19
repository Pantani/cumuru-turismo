package analytics

import (
	"context"
	"errors"

	"github.com/Pantani/cumuru/apps/api/internal/access"
	"github.com/google/uuid"
)

var (
	// ErrOwnPerformanceNotFound cobre tanto a hospedagem inexistente quanto a
	// existente sem membership ativa de quem pediu. As duas respondem igual de
	// propósito: distinguir transformaria a rota em oráculo de existência.
	ErrOwnPerformanceNotFound = errors.New("accommodation not found")
	ErrInvalidOwnPerformance  = errors.New("invalid own performance query")
)

// O comparativo da hospedagem é mais restrito que a publicação pública, e o
// motivo é o leitor. O público lê uma célula agregada e não sabe qual parcela é
// de quem; a hospedagem lê a mesma célula sabendo exatamente o próprio número,
// então `outros = total - meu` é uma subtração que só ela consegue fazer. A
// política pública garante três estabelecimentos por célula (policy.go), o que
// deixa dois após a subtração; aqui exigimos cinco, para que sobrem quatro, e
// recusamos o comparativo quando a própria capacidade domina o denominador —
// nesse caso o agregado é espelho do próprio dado, não comparação.
type ComparisonPolicy struct {
	MinimumReportingAccommodations int
	MaximumOwnCapacitySharePercent int
}

// DefaultComparisonPolicy é o piso desta camada, não um ajuste fino: cinco
// reportantes e um quarto do denominador. O encarregado ajusta ao risco local,
// como o próprio docs/07 exige do limiar público.
func DefaultComparisonPolicy() ComparisonPolicy {
	return ComparisonPolicy{
		MinimumReportingAccommodations: 5,
		MaximumOwnCapacitySharePercent: 25,
	}
}

func (p ComparisonPolicy) valid() bool {
	return p.MinimumReportingAccommodations >= 5 &&
		p.MaximumOwnCapacitySharePercent > 0 &&
		p.MaximumOwnCapacitySharePercent <= 100
}

// VillageReporting descreve o denominador da vila na janela lida. Os números
// entram na decisão e não saem dela: a resposta carrega apenas o veredito e o
// motivo, porque "somos sete hospedagens com 210 leitos" já é, sozinho, uma
// informação sobre terceiros que a hospedagem não precisa receber.
type VillageReporting struct {
	Accommodations int
	Capacity       int64
	PersonDays     int64
}

const (
	ComparisonAvailable   = "available"
	ComparisonUnavailable = "unavailable"

	ComparisonReasonFewAccommodations = "few_reporting_accommodations"
	ComparisonReasonOwnShareTooHigh   = "own_capacity_share_too_high"
	ComparisonReasonNoPublication     = "no_published_series"
)

type ComparisonAvailability struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

func unavailableComparison(reason string) ComparisonAvailability {
	return ComparisonAvailability{Status: ComparisonUnavailable, Reason: reason}
}

// EvaluateComparison decide se a vila pode ser exibida ao lado do dado próprio.
// A ordem dos testes é a ordem do risco: sem reportantes suficientes nada mais
// importa, e a fatia própria só faz sentido sobre um denominador que existe.
func EvaluateComparison(
	ownCapacity int32,
	reporting VillageReporting,
	policy ComparisonPolicy,
) ComparisonAvailability {
	if !policy.valid() || reporting.Capacity <= 0 {
		return unavailableComparison(ComparisonReasonFewAccommodations)
	}
	if reporting.Accommodations < policy.MinimumReportingAccommodations {
		return unavailableComparison(ComparisonReasonFewAccommodations)
	}
	if dominatesDenominator(ownCapacity, reporting, policy) {
		return unavailableComparison(ComparisonReasonOwnShareTooHigh)
	}
	return ComparisonAvailability{Status: ComparisonAvailable}
}

// A comparação é feita em inteiros para não depender de arredondamento de
// ponto flutuante no limite exato da política.
func dominatesDenominator(
	ownCapacity int32,
	reporting VillageReporting,
	policy ComparisonPolicy,
) bool {
	share := int64(ownCapacity) * 100
	limit := reporting.Capacity * int64(policy.MaximumOwnCapacitySharePercent)
	return share > limit
}

// OwnPresencePoint é um dia da janela. O lado próprio é exato, porque é dado da
// própria hospedagem; o lado da vila é índice, nunca valor absoluto — o valor
// absoluto continua público em /api/v1/public/presence, e repeti-lo aqui apenas
// entregaria a subtração já composta ao lado do número exato de quem lê.
type OwnPresencePoint struct {
	Date          string `json:"date"`
	OwnPersonDays int32  `json:"own_person_days"`
	OwnIndex      *int32 `json:"own_index,omitempty"`
	VillageIndex  *int32 `json:"village_index,omitempty"`
}

type OwnPerformance struct {
	Metadata   PublicMetadata         `json:"metadata"`
	Window     string                 `json:"window"`
	Month      string                 `json:"month,omitempty"`
	Comparison ComparisonAvailability `json:"comparison"`
	Occupancy  Occupancy              `json:"occupancy"`
	Series     []OwnPresencePoint     `json:"series"`
}

type OwnPerformanceQuery struct {
	Actor           access.Principal
	AccommodationID uuid.UUID
	Slice           PresenceSlice
}

type OwnPerformanceReader interface {
	Performance(context.Context, OwnPerformanceQuery) (OwnPerformance, error)
}
