package analytics

import (
	"regexp"
	"time"
)

// O seletor de armazenamento é a série, não o recorte. Um dia observado existe
// uma única vez em `observed_daily`; `recent_30_days` e um mês civil são duas
// leituras do mesmo dia, e gravar cada janela como seletor próprio duplicaria a
// célula tantas vezes quantas janelas a capa oferecesse.
const (
	PresenceObservedSelector = "observed_daily"
	PresenceForecastSelector = "next_30_days"
)

// Dois anos de histórico diário: é o mínimo que responde "como está comparado
// ao mesmo mês do ano passado", que é a pergunta de sazonalidade.
const (
	PresenceHistoryDays  = 730
	PresenceForecastDays = 30
)

const presenceMonthWindow = "month"

var presenceMonthPattern = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])$`)

// Janelas fechadas em vez de um intervalo livre: cada uma é um documento
// cacheável com ETag próprio, e o catálogo fechado é o que docs/07 exige contra
// consulta por diferença.
var presenceLookbackDays = map[string]int32{
	"recent_30_days":  30,
	"recent_90_days":  90,
	"recent_365_days": 365,
	"recent_730_days": PresenceHistoryDays,
}

// PresenceSlice descreve o recorte que a leitura pública aplica sobre a série
// publicada. Exatamente um entre LookbackDays e Month é preenchido para a série
// observada; a previsão não recorta nada porque já é publicada no horizonte.
type PresenceSlice struct {
	Window       string
	Selector     string
	LookbackDays int32
	Month        string
	MonthStart   string
	MonthEnd     string
}

// ResolvePresenceWindow traduz o par (window, month) do contrato público. Um
// mês fora de `window=month` — ou ausente dentro dela — é requisição inválida,
// nunca um parâmetro ignorado em silêncio.
func ResolvePresenceWindow(window, month string) (PresenceSlice, bool) {
	if window == presenceMonthWindow {
		return monthSlice(month)
	}
	if month != "" {
		return PresenceSlice{}, false
	}
	if window == PresenceForecastSelector {
		return PresenceSlice{Window: window, Selector: PresenceForecastSelector}, true
	}
	lookback, known := presenceLookbackDays[window]
	if !known {
		return PresenceSlice{}, false
	}
	return PresenceSlice{
		Window: window, Selector: PresenceObservedSelector, LookbackDays: lookback,
	}, true
}

func monthSlice(month string) (PresenceSlice, bool) {
	if !presenceMonthPattern.MatchString(month) {
		return PresenceSlice{}, false
	}
	start, err := time.Parse("2006-01", month)
	if err != nil {
		return PresenceSlice{}, false
	}
	return PresenceSlice{
		Window:     presenceMonthWindow,
		Selector:   PresenceObservedSelector,
		Month:      month,
		MonthStart: start.Format(time.DateOnly),
		MonthEnd:   start.AddDate(0, 1, 0).Format(time.DateOnly),
	}, true
}

// CacheKey identifica o documento: a janela sozinha não distingue dois meses.
func (slice PresenceSlice) CacheKey() string {
	if slice.Month == "" {
		return slice.Window
	}
	return slice.Window + ":" + slice.Month
}

// MaxSeriesLength limita quantas células uma janela pode devolver. Um documento
// mais longo que a janela pedida significa seletor corrompido, não excesso a
// truncar.
func (slice PresenceSlice) MaxSeriesLength() int {
	switch {
	case slice.Selector == PresenceForecastSelector:
		return PresenceForecastDays
	case slice.Month != "":
		return 31
	default:
		return int(slice.LookbackDays)
	}
}

// AllowedPresenceWindows nomeia o catálogo publicado na metodologia, na mesma
// ordem em que o contrato o declara.
func AllowedPresenceWindows() []string {
	return []string{
		"recent_30_days",
		"recent_90_days",
		"recent_365_days",
		"recent_730_days",
		PresenceForecastSelector,
		presenceMonthWindow,
	}
}
