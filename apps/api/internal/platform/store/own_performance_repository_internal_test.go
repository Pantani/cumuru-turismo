package store

import (
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
)

func TestObservedWindowFollowsThePublishedSeries(t *testing.T) {
	t.Parallel()

	// A janela do lado próprio nasce da publicação, não do relógio de quem
	// consulta: o fim é exclusivo, então o último dia publicado exige o dia
	// seguinte para entrar na leitura.
	window, ok := observedWindow([]analytics.PresencePoint{
		{Date: "2026-07-30"},
		{Date: "2026-07-31"},
	})
	if !ok {
		t.Fatal("a published series always yields a window")
	}
	if window.start != "2026-07-30" || window.end != "2026-08-01" {
		t.Fatalf("window = [%s, %s)", window.start, window.end)
	}
}

func TestObservedWindowRejectsAnEmptyOrCorruptSeries(t *testing.T) {
	t.Parallel()

	if _, ok := observedWindow(nil); ok {
		t.Fatal("an empty publication is no window at all")
	}
	if _, ok := observedWindow([]analytics.PresencePoint{{Date: "julho"}}); ok {
		t.Fatal("an unparseable date is a corrupt document, not a window")
	}
}

func TestOwnSeriesDaysKeepsTheDaysOfThePublication(t *testing.T) {
	t.Parallel()

	value := int32(40)
	days := ownSeriesDays(
		[]analytics.PresencePoint{
			{Date: "2026-07-30", Value: &value},
			{Date: "2026-07-31"},
		},
		map[string]int32{"2026-07-31": 6},
	)
	if len(days) != 2 {
		t.Fatalf("days = %d, want one per published day", len(days))
	}
	// Um dia sem estadia própria é zero, não ausência: a curva precisa do dia.
	if days[0].OwnPersonDays != 0 || days[0].VillageValue == nil {
		t.Fatalf("first day = %+v", days[0])
	}
	if days[1].OwnPersonDays != 6 || days[1].VillageValue != nil {
		t.Fatalf("second day = %+v", days[1])
	}
}
