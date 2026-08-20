package analytics

// OwnSeriesDay é a linha crua que o repositório monta antes da indexação: o dia
// da janela, o valor exato da própria hospedagem e a célula publicada da vila,
// que é nula quando a supressão não a liberou.
type OwnSeriesDay struct {
	Date          string
	OwnPersonDays int32
	VillageValue  *int32
}

const ownSeriesIndexBase = 100

// BuildOwnSeries indexa os dois lados na mesma data-base para que a leitura seja
// "quanto cada um variou desde o início da janela", e não "quanto cada um vale".
// Sem base comum as duas curvas não se comparam; sem comparativo liberado o lado
// da vila simplesmente não existe na resposta.
func BuildOwnSeries(days []OwnSeriesDay, comparable bool) []OwnPresencePoint {
	base, ok := indexBase(days, comparable)
	points := make([]OwnPresencePoint, 0, len(days))
	for _, day := range days {
		points = append(points, seriesPoint(day, base, ok))
	}
	return points
}

type seriesBase struct {
	own     int32
	village int32
}

// A base é o primeiro dia em que os dois lados existem e são positivos. Um dia
// com zero de um dos lados não serve: indexar contra zero não tem valor
// definido, e escolher o primeiro dia do calendário produziria índice infinito
// na primeira noite ocupada.
func indexBase(days []OwnSeriesDay, comparable bool) (seriesBase, bool) {
	if !comparable {
		return seriesBase{}, false
	}
	for _, day := range days {
		if eligibleBase(day) {
			return seriesBase{own: day.OwnPersonDays, village: *day.VillageValue}, true
		}
	}
	return seriesBase{}, false
}

func eligibleBase(day OwnSeriesDay) bool {
	return day.OwnPersonDays > 0 && day.VillageValue != nil && *day.VillageValue > 0
}

func seriesPoint(day OwnSeriesDay, base seriesBase, indexed bool) OwnPresencePoint {
	point := OwnPresencePoint{Date: day.Date, OwnPersonDays: day.OwnPersonDays}
	if !indexed {
		return point
	}
	point.OwnIndex = indexValue(day.OwnPersonDays, base.own)
	if day.VillageValue != nil {
		point.VillageIndex = indexValue(*day.VillageValue, base.village)
	}
	return point
}

func indexValue(value, base int32) *int32 {
	if base <= 0 {
		return nil
	}
	indexed := int32(roundHalfUp(
		float64(value)*ownSeriesIndexBase/float64(base), 1,
	))
	return &indexed
}
