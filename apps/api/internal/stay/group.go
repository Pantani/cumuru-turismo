package stay

import (
	"errors"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrInvalidGroup     = errors.New("invalid group")
	ErrDuplicateClient  = errors.New("duplicate visitor client ID")
	ErrResponsibleCount = errors.New("group must contain exactly one responsible visitor")
)

var cityCodePattern = regexp.MustCompile(`^[0-9]{7}$`)

type VisitorRole string

const (
	VisitorResponsible VisitorRole = "responsible"
	VisitorCompanion   VisitorRole = "companion"
	VisitorMinor       VisitorRole = "minor"
)

func (r VisitorRole) valid() bool {
	return r == VisitorResponsible || r == VisitorCompanion || r == VisitorMinor
}

type AgeBand string

const (
	Age0To5   AgeBand = "0_5"
	Age6To11  AgeBand = "6_11"
	Age12To17 AgeBand = "12_17"
	Age18To24 AgeBand = "18_24"
	Age25To34 AgeBand = "25_34"
	Age35To44 AgeBand = "35_44"
	Age45To59 AgeBand = "45_59"
	Age60Plus AgeBand = "60_plus"
)

var validAgeBands = map[AgeBand]bool{
	Age0To5: true, Age6To11: true, Age12To17: true, Age18To24: true,
	Age25To34: true, Age35To44: true, Age45To59: true, Age60Plus: true,
}

type Visitor struct {
	ClientID          string
	Role              VisitorRole
	AgeBand           AgeBand
	ResidenceCountry  string
	ResidenceState    string
	ResidenceCityCode string
}

// Exactly one responsible visitor per group: the record needs a single point of
// contact, and more than one would make the group ambiguous.
func ValidateGroup(visitors []Visitor) error {
	if len(visitors) < 1 || len(visitors) > 100 {
		return ErrInvalidGroup
	}
	validation := groupValidation{seen: make(map[string]struct{}, len(visitors))}
	if err := validation.addAll(visitors); err != nil {
		return err
	}
	if validation.responsibleCount != 1 {
		return ErrResponsibleCount
	}
	return nil
}

type groupValidation struct {
	seen             map[string]struct{}
	responsibleCount int
}

func (g *groupValidation) addAll(visitors []Visitor) error {
	for _, visitor := range visitors {
		if err := g.add(visitor); err != nil {
			return err
		}
	}
	return nil
}

func (g *groupValidation) add(visitor Visitor) error {
	if err := visitor.validate(); err != nil {
		return err
	}
	if _, duplicate := g.seen[visitor.ClientID]; duplicate {
		return ErrDuplicateClient
	}
	g.seen[visitor.ClientID] = struct{}{}
	if visitor.Role == VisitorResponsible {
		g.responsibleCount++
	}
	return nil
}

func (v Visitor) validShape() bool {
	identifier, err := uuid.Parse(v.ClientID)
	if err != nil || identifier.Version() != 7 {
		return false
	}
	return v.Role.valid() &&
		validAgeBands[v.AgeBand] &&
		validCountry(v.ResidenceCountry)
}

func (v Visitor) validate() error {
	if !v.validShape() {
		return ErrInvalidGroup
	}
	return v.validateResidence()
}

// A Brazilian residence carries a generalized state and IBGE municipality; any
// other country carries neither, so no sub-national detail leaks abroad.
func (v Visitor) validateResidence() error {
	if v.ResidenceCountry == "BR" {
		return v.validateDomesticResidence()
	}
	if v.ResidenceState != "" || v.ResidenceCityCode != "" {
		return ErrInvalidGroup
	}
	return nil
}

func (v Visitor) validateDomesticResidence() error {
	if !validState(v.ResidenceState) || !cityCodePattern.MatchString(v.ResidenceCityCode) {
		return ErrInvalidGroup
	}
	return nil
}

func validCountry(value string) bool {
	return len(value) == 2 && value == strings.ToUpper(value)
}

func validState(value string) bool {
	return len(value) == 2 && value == strings.ToUpper(value)
}
