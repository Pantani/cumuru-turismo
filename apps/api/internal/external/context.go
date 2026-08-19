package external

import "time"

// BuildDocument assembles the public document from the view rows and the
// credited sources. It never fabricates a value, a zero or a silently served
// last-known value: a card with nothing to show carries a closed reason code
// and its provenance intact.
func BuildDocument(
	rows []ContextRow,
	sources []CreditedSource,
	now time.Time,
) (PublicContext, error) {
	cards, err := buildCards(rows, now)
	if err != nil {
		return PublicContext{}, err
	}
	// The contract requires at least one card and one source. An empty layer is
	// not a dead source, it is a document that cannot be assembled — the only
	// case ADR-045 §3 answers with 503.
	if len(cards) == 0 || len(sources) == 0 {
		return PublicContext{}, ErrContextUnavailable
	}
	return PublicContext{
		GeneratedAt:    instant(now),
		Layer:          LayerCode,
		DisclaimerCode: DisclaimerCode,
		Cards:          cards,
		Sources:        sources,
	}, nil
}

func buildCards(rows []ContextRow, now time.Time) ([]Card, error) {
	groups := groupByCard(rows)
	cards := make([]Card, 0, len(groups))
	for _, group := range groups {
		card, err := buildCard(group, now)
		if err != nil {
			return nil, err
		}
		cards = append(cards, card)
	}
	return cards, nil
}

// The query orders by card_code, source_code, series_code and period_start, so
// the rows of one card arrive contiguous and its points already sorted.
func groupByCard(rows []ContextRow) [][]ContextRow {
	groups := make([][]ContextRow, 0, len(rows))
	for _, row := range rows {
		last := len(groups) - 1
		if last >= 0 && groups[last][0].CardCode == row.CardCode {
			groups[last] = append(groups[last], row)
			continue
		}
		groups = append(groups, []ContextRow{row})
	}
	return groups
}

// A card is one series. Two series sharing a card_code would put two units and
// two provenances behind a single label, so it is refused loudly instead of
// resolved by picking one.
func buildCard(group []ContextRow, now time.Time) (Card, error) {
	if !singleSeries(group) {
		return Card{}, ErrContextUnavailable
	}
	input := cardInput{
		head:   group[0],
		points: seriesPoints(group),
		now:    now,
	}
	input.latest, input.observed = latestObservation(group)
	if reason := unavailableReason(input); reason != "" {
		return unavailableCard(input, reason), nil
	}
	return publishedCard(input), nil
}

type cardInput struct {
	head     ContextRow
	latest   ContextRow
	observed bool
	points   []SeriesPoint
	now      time.Time
}

func singleSeries(group []ContextRow) bool {
	head := group[0]
	for _, row := range group {
		if row.SourceCode != head.SourceCode || row.SeriesCode != head.SeriesCode {
			return false
		}
	}
	return true
}

// A point needs a period and a value. A row whose observation columns are all
// null is the LEFT JOIN reporting that the series has no observation at all,
// which is the unavailable branch and not a point with a missing number.
func seriesPoints(group []ContextRow) []SeriesPoint {
	points := make([]SeriesPoint, 0, len(group))
	for _, row := range group {
		if !completeObservation(row) {
			continue
		}
		points = append(points, SeriesPoint{
			PeriodStart: instant(*row.PeriodStart),
			PeriodEnd:   instant(*row.PeriodEnd),
			Value:       *row.Value,
		})
	}
	return points
}

func completeObservation(row ContextRow) bool {
	return row.PeriodStart != nil && row.PeriodEnd != nil && row.Value != nil
}

// Rows arrive ordered by period_start, so the last one carrying a period is the
// most recent observation and the one whose revision and retrieval instant the
// provenance reports.
func latestObservation(group []ContextRow) (ContextRow, bool) {
	latest := ContextRow{}
	found := false
	for _, row := range group {
		if row.PeriodStart == nil {
			continue
		}
		latest = row
		found = true
	}
	return latest, found
}

func unavailableCard(input cardInput, reason string) Card {
	return Card{
		CardCode:   input.head.CardCode,
		Status:     StatusUnavailable,
		DataMode:   input.head.DataMode,
		Provenance: provenance(input),
		ReasonCode: reason,
	}
}

func publishedCard(input cardInput) Card {
	return Card{
		CardCode:   input.head.CardCode,
		Status:     StatusPublished,
		DataMode:   input.head.DataMode,
		Provenance: provenance(input),
		UnitCode:   input.head.UnitCode,
		Series:     input.points,
	}
}
