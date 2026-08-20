package store

import (
	"context"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/analytics"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store/generated"
	"github.com/google/uuid"
)

const ownPerformanceDateLayout = "2006-01-02"

// OwnPerformanceRepository compõe duas leituras que já existem: o documento
// público, lido pelo mesmo caminho e pela mesma role restrita que serve a capa,
// e o dado da própria hospedagem, isolado por membership no SQL. Nenhum agregado
// novo é publicado aqui — o lado da vila é literalmente a publicação corrente.
type OwnPerformanceRepository struct {
	store  *Store
	public analytics.PublicReader
	policy analytics.ComparisonPolicy
}

func NewOwnPerformanceRepository(
	store *Store,
	public analytics.PublicReader,
	policy analytics.ComparisonPolicy,
) *OwnPerformanceRepository {
	return &OwnPerformanceRepository{store: store, public: public, policy: policy}
}

var _ analytics.OwnPerformanceReader = (*OwnPerformanceRepository)(nil)

func (r *OwnPerformanceRepository) Performance(
	ctx context.Context,
	query analytics.OwnPerformanceQuery,
) (analytics.OwnPerformance, error) {
	if !validOwnPerformanceQuery(query) {
		return analytics.OwnPerformance{}, analytics.ErrInvalidOwnPerformance
	}
	village, err := r.public.Presence(ctx, query.Slice)
	if err != nil {
		return analytics.OwnPerformance{}, err
	}
	window, ok := observedWindow(village.Series)
	if !ok {
		return analytics.OwnPerformance{}, analytics.ErrPublicUnavailable
	}
	return r.compose(ctx, query, village, window)
}

// A janela do lado próprio é a janela da publicação, nunca o relógio de quem
// consulta: as duas curvas precisam cobrir exatamente os mesmos dias para que a
// comparação signifique alguma coisa.
type observedRange struct {
	start string
	end   string
}

func (r *OwnPerformanceRepository) compose(
	ctx context.Context,
	query analytics.OwnPerformanceQuery,
	village analytics.PublicPresence,
	window observedRange,
) (analytics.OwnPerformance, error) {
	ctx, cancel := context.WithTimeout(ctx, r.store.timeout)
	defer cancel()
	capacity, err := r.ownCapacity(ctx, query)
	if err != nil {
		return analytics.OwnPerformance{}, err
	}
	own, err := r.ownPresence(ctx, query, window)
	if err != nil {
		return analytics.OwnPerformance{}, err
	}
	reporting, err := r.villageReporting(ctx, window)
	if err != nil {
		return analytics.OwnPerformance{}, err
	}
	comparison := analytics.EvaluateComparison(capacity, reporting, r.policy)
	comparable := comparison.Status == analytics.ComparisonAvailable
	days := ownSeriesDays(village.Series, own)
	return analytics.OwnPerformance{
		Metadata:   village.Metadata,
		Window:     village.Window,
		Month:      village.Month,
		Comparison: comparison,
		Occupancy: analytics.ComputeOccupancy(analytics.OccupancyInput{
			Days:              len(days),
			OwnCapacity:       capacity,
			OwnPersonDays:     totalPersonDays(own),
			VillageCapacity:   reporting.Capacity,
			VillagePersonDays: reporting.PersonDays,
		}, comparable),
		Series: analytics.BuildOwnSeries(days, comparable),
	}, nil
}

// A capacidade sai da mesma consulta que prova a membership. Sem capacidade
// declarada não há denominador para a fatia própria, e o comparativo fecha.
func (r *OwnPerformanceRepository) ownCapacity(
	ctx context.Context,
	query analytics.OwnPerformanceQuery,
) (int32, error) {
	row, err := r.store.queries.GetAccessibleAccommodation(
		ctx,
		generated.GetAccessibleAccommodationParams{
			AccommodationID: pgUUID(query.AccommodationID),
			OidcIssuer:      query.Actor.Issuer,
			OidcSubject:     query.Actor.Subject,
		},
	)
	if err != nil {
		return 0, analytics.ErrOwnPerformanceNotFound
	}
	if row.Capacity == nil {
		return 0, nil
	}
	return *row.Capacity, nil
}

func (r *OwnPerformanceRepository) ownPresence(
	ctx context.Context,
	query analytics.OwnPerformanceQuery,
	window observedRange,
) (map[string]int32, error) {
	rows, err := r.store.queries.ListAccommodationObservedPresence(
		ctx,
		generated.ListAccommodationObservedPresenceParams{
			OidcIssuer:      query.Actor.Issuer,
			OidcSubject:     query.Actor.Subject,
			AccommodationID: pgUUID(query.AccommodationID),
			StartOn:         dateToPG(window.start),
			EndOn:           dateToPG(window.end),
		},
	)
	if err != nil {
		return nil, ErrUnavailable
	}
	own := make(map[string]int32, len(rows))
	for _, row := range rows {
		own[dateString(row.PresenceOn)] = row.PersonDays
	}
	return own, nil
}

func (r *OwnPerformanceRepository) villageReporting(
	ctx context.Context,
	window observedRange,
) (analytics.VillageReporting, error) {
	row, err := r.store.queries.SummarizeVillageReporting(
		ctx,
		generated.SummarizeVillageReportingParams{
			StartOn: dateToPG(window.start),
			EndOn:   dateToPG(window.end),
		},
	)
	if err != nil {
		return analytics.VillageReporting{}, ErrUnavailable
	}
	return analytics.VillageReporting{
		Accommodations: int(row.Accommodations),
		Capacity:       row.Capacity,
		PersonDays:     row.PersonDays,
	}, nil
}

// O total próprio sai do mesmo recorte da série, e não de uma segunda consulta:
// numerador e denominador precisam falar exatamente da mesma janela.
func totalPersonDays(own map[string]int32) int64 {
	var total int64
	for _, value := range own {
		total += int64(value)
	}
	return total
}

func validOwnPerformanceQuery(query analytics.OwnPerformanceQuery) bool {
	return query.Actor.Issuer != "" && query.Actor.Subject != "" &&
		query.AccommodationID != uuid.Nil &&
		query.Slice.Selector == analytics.PresenceObservedSelector
}

// observedWindow deriva o intervalo semiaberto da própria série publicada, em
// vez de recalculá-lo: assim o lado próprio herda o `as_of_on` da publicação.
func observedWindow(series []analytics.PresencePoint) (observedRange, bool) {
	if len(series) == 0 {
		return observedRange{}, false
	}
	last, ok := nextDay(series[len(series)-1].Date)
	if !ok {
		return observedRange{}, false
	}
	return observedRange{start: series[0].Date, end: last}, true
}

// O fim do intervalo é exclusivo, então a última célula publicada precisa do dia
// seguinte para entrar na leitura própria.
func nextDay(date string) (string, bool) {
	parsed, err := time.Parse(ownPerformanceDateLayout, date)
	if err != nil {
		return "", false
	}
	return parsed.AddDate(0, 0, 1).Format(ownPerformanceDateLayout), true
}

func ownSeriesDays(
	series []analytics.PresencePoint,
	own map[string]int32,
) []analytics.OwnSeriesDay {
	days := make([]analytics.OwnSeriesDay, 0, len(series))
	for _, point := range series {
		days = append(days, analytics.OwnSeriesDay{
			Date:          point.Date,
			OwnPersonDays: own[point.Date],
			VillageValue:  point.Value,
		})
	}
	return days
}
