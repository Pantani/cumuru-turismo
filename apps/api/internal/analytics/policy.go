package analytics

import (
	"errors"
	"math"
)

var ErrInvalidPolicy = errors.New("invalid publication policy")

type CellStatus string

const (
	CellPublished   CellStatus = "published"
	CellProtected   CellStatus = "protected"
	CellUnavailable CellStatus = "unavailable"
)

type Policy struct {
	PrimaryThreshold               int
	MinimumReportingAccommodations int
	RoundingBase                   int
}

type Cell struct {
	Key                string
	Family             string
	RawValue           float64
	SampleSize         int
	AccommodationCount int
	Total              bool
}

type ProtectedCell struct {
	Key            string
	Family         string
	Status         CellStatus
	PublishedValue *int
	Total          bool
}

type ForecastValues struct {
	Lower   int
	Central int
	Upper   int
}

func ProtectCells(cells []Cell, policy Policy) ([]ProtectedCell, error) {
	if !validPolicy(policy) {
		return nil, ErrInvalidPolicy
	}
	result, err := primaryProtection(cells, policy)
	if err != nil {
		return nil, err
	}
	applyComplementarySuppression(cells, result)
	publishRounded(cells, result, policy.RoundingBase)
	return result, nil
}

func primaryProtection(cells []Cell, policy Policy) ([]ProtectedCell, error) {
	result := make([]ProtectedCell, 0, len(cells))
	for _, cell := range cells {
		if !validCell(cell) {
			return nil, ErrInvalidPolicy
		}
		result = append(result, ProtectedCell{
			Key: cell.Key, Family: cell.Family,
			Status: primaryStatus(cell, policy), Total: cell.Total,
		})
	}
	return result, nil
}

// Only a cell that survived both suppression passes carries a value, and that
// value is rounded before it ever leaves the process.
func publishRounded(cells []Cell, result []ProtectedCell, base int) {
	for index := range result {
		if result[index].Status != CellPublished {
			continue
		}
		value := roundHalfUp(cells[index].RawValue, base)
		result[index].PublishedValue = &value
	}
}

func orderedForecast(lower, central, upper float64) bool {
	return lower >= 0 && lower <= central && central <= upper
}

// The bounds are rounded outward and the centre to nearest, so rounding can
// never narrow a published interval.
func RoundForecast(lower, central, upper float64, base int) (ForecastValues, error) {
	if base < 1 || !orderedForecast(lower, central, upper) {
		return ForecastValues{}, ErrInvalidPolicy
	}
	result := ForecastValues{
		Lower:   roundFloor(lower, base),
		Central: roundHalfUp(central, base),
		Upper:   roundCeil(upper, base),
	}
	if result.Lower > result.Central || result.Central > result.Upper {
		return ForecastValues{}, ErrInvalidPolicy
	}
	return result, nil
}

func validPolicy(policy Policy) bool {
	return policy.PrimaryThreshold >= 10 &&
		policy.MinimumReportingAccommodations >= 3 &&
		policy.RoundingBase > 0
}

func validCell(cell Cell) bool {
	return cell.Key != "" && cell.Family != "" && cell.RawValue >= 0 &&
		cell.SampleSize >= 0 && cell.AccommodationCount >= 0
}

func primaryStatus(cell Cell, policy Policy) CellStatus {
	if cell.SampleSize < policy.PrimaryThreshold {
		return CellProtected
	}
	if cell.AccommodationCount < policy.MinimumReportingAccommodations {
		return CellProtected
	}
	return CellPublished
}

func applyComplementarySuppression(cells []Cell, result []ProtectedCell) {
	families := cellFamilies(result)
	for _, family := range families {
		if protectedCount(result, family) != 1 {
			continue
		}
		index := complementaryIndex(cells, result, family)
		if index >= 0 {
			result[index].Status = CellProtected
		}
	}
}

func cellFamilies(cells []ProtectedCell) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, cell := range cells {
		if _, exists := seen[cell.Family]; exists {
			continue
		}
		seen[cell.Family] = struct{}{}
		result = append(result, cell.Family)
	}
	return result
}

func protectedCount(cells []ProtectedCell, family string) int {
	count := 0
	for _, cell := range cells {
		if cell.Family == family && cell.Status != CellPublished {
			count++
		}
	}
	return count
}

func complementaryIndex(
	source []Cell,
	protected []ProtectedCell,
	family string,
) int {
	selected := -1
	for index := range source {
		if !eligibleForComplementary(protected[index], family) {
			continue
		}
		if selected < 0 || lessDisclosureRisk(source[index], source[selected]) {
			selected = index
		}
	}
	return selected
}

func eligibleForComplementary(cell ProtectedCell, family string) bool {
	return cell.Family == family && cell.Status == CellPublished
}

func lessDisclosureRisk(candidate, selected Cell) bool {
	if candidate.RawValue != selected.RawValue {
		return candidate.RawValue < selected.RawValue
	}
	return candidate.Key < selected.Key
}

func roundHalfUp(value float64, base int) int {
	return int(math.Floor(value/float64(base)+0.5)) * base
}

func roundFloor(value float64, base int) int {
	return int(math.Floor(value/float64(base))) * base
}

func roundCeil(value float64, base int) int {
	return int(math.Ceil(value/float64(base))) * base
}
