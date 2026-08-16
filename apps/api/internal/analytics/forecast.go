package analytics

import (
	"errors"
	"sort"
)

var ErrInvalidForecast = errors.New("invalid forecast input")

type Forecast struct {
	Lower    float64
	Central  float64
	Upper    float64
	Fallback bool
}

// The seasonal baseline is the median of eligible weekdays; a negative sample
// means the history itself is corrupt.
func historicalBaseline(previousWeekdays []float64) (float64, error) {
	history := append([]float64(nil), previousWeekdays...)
	for _, value := range history {
		if value < 0 {
			return 0, ErrInvalidForecast
		}
	}
	sort.Float64s(history)
	return median(history), nil
}

func ExplainableBaseline(
	onBooks float64,
	previousWeekdays []float64,
	leadDays int,
) (Forecast, error) {
	if onBooks < 0 || leadDays < 0 {
		return Forecast{}, ErrInvalidForecast
	}
	if len(previousWeekdays) < 8 {
		return Forecast{
			Lower: onBooks * 0.70, Central: onBooks,
			Upper: onBooks * 1.30, Fallback: true,
		}, nil
	}
	baseline, err := historicalBaseline(previousWeekdays)
	if err != nil {
		return Forecast{}, err
	}
	leadFactor := float64(min(leadDays, 30)) / 30
	projected := max(baseline-onBooks, 0) * leadFactor
	central := onBooks + projected
	return Forecast{
		Lower: central * 0.85, Central: central, Upper: central * 1.15,
	}, nil
}

func median(values []float64) float64 {
	middle := len(values) / 2
	if len(values)%2 == 1 {
		return values[middle]
	}
	return (values[middle-1] + values[middle]) / 2
}
