package external

import (
	"net/url"
	"time"
)

// The source vocabulary is closed by CHECK in migration 000003 and by the
// ExternalSourceCode enum in the contract. These are the four the delivery
// touches.
const (
	SourceOpenMeteoForecast = "open_meteo_forecast"
	SourceOpenMeteoArchive  = "open_meteo_archive"
	SourceCHMHarmonics      = "chm_harmonics"
	SourceCadastur          = "cadastur"
)

const (
	CardWeatherDaily = "weather_daily"
	CardTide         = "tide"
)

// The observation point of the municipality, fixed and rounded to two decimals
// (~1.1 km) by ADR-045 §6. It is a coordinate of a village, not of a reader,
// and it never varies with a request.
const (
	observationLatitude  = "-17.10"
	observationLongitude = "-39.19"
)

// The forecast URL is a literal. Host, path and the whole parameter set are
// constant, `past_days` carries the short declared history and `forecast_days`
// the near horizon, so no byte of any HTTP request ever reaches it.
const forecastURL = "https://api.open-meteo.com/v1/forecast" +
	"?daily=temperature_2m_max" +
	"&forecast_days=7" +
	"&latitude=" + observationLatitude +
	"&longitude=" + observationLongitude +
	"&past_days=7" +
	"&timezone=America%2FBahia"

// The Archive API is served from a different host than the forecast one, and
// the default EXTERNAL_ALLOWED_HOSTS does not carry it, so this target is
// declared and inert. Enabling it is adding the host to the allowlist and
// nothing else.
//
// The window is a literal for the same reason as the URL: a window that rolled
// with the clock would make the URL vary. Its series is not publicly exposable
// because the frozen ExternalCardCode enum has exactly two values, so a second
// weather series could only ever appear as a second `weather_daily` card —
// which the document builder refuses, and which ADR-045 §10 would require an
// amendment to introduce. Delivery 1 excludes long history by plan F-9 anyway.
const archiveURL = "https://archive-api.open-meteo.com/v1/archive" +
	"?daily=temperature_2m_max" +
	"&end_date=2024-12-31" +
	"&latitude=" + observationLatitude +
	"&longitude=" + observationLongitude +
	"&start_date=2024-01-01" +
	"&timezone=America%2FBahia"

// SourceRecord is the publisher row. The attribution text is the exact string
// that reaches the public, and it lives in the database precisely so that the
// public wording is reviewable as data instead of being assembled in Go.
type SourceRecord struct {
	SourceCode           string
	Publisher            string
	LicenseCode          string
	LicenseURL           string
	AttributionText      string
	TermsURL             string
	CommercialUseAllowed bool
	Active               bool
}

// SeriesRecord carries the unit and the period, which never live on the
// observation: changing the unit of a series is a new series_code, otherwise
// the old observations start lying about their own unit.
type SeriesRecord struct {
	SourceCode            string
	SeriesCode            string
	CardCode              string
	UnitCode              string
	PeriodKind            string
	ValueKind             string
	DeclaredLag           time.Duration
	RetentionDays         int32
	PublicExposable       bool
	GeoScope              string
	DefinitionVersion     int32
	DataMode              string
	Derived               bool
	DerivationCode        string
	UnavailableReasonCode string
}

// Target is one constant URL plus the catalogue rows it feeds.
type Target struct {
	SourceCode string
	Host       string
	URL        string
	Source     SourceRecord
	Series     SeriesRecord
	Parse      func([]byte) ([]ObservedPoint, error)
	parsed     *url.URL
}

// Open-Meteo publishes under CC BY 4.0 and its free tier is non-commercial,
// which U-1 makes eligible because the Cumuru is non-commercial. That
// eligibility is recorded as data in `commercial_use_allowed`, not as a comment
// somebody has to find.
var openMeteoLicence = SourceRecord{
	Publisher:   "Open-Meteo",
	LicenseCode: "CC-BY-4.0",
	LicenseURL:  "https://creativecommons.org/licenses/by/4.0/",
	AttributionText: "Dados meteorológicos por Open-Meteo.com, " +
		"sob licença CC BY 4.0",
	TermsURL:             "https://open-meteo.com/en/terms",
	CommercialUseAllowed: false,
	Active:               true,
}

func openMeteoSource(sourceCode string) SourceRecord {
	source := openMeteoLicence
	source.SourceCode = sourceCode
	return source
}

// CreditedOnlySources are published in the credit list and feed no card. U-7
// puts Cadastur here and only here: attribution and link, with no count
// computed by the platform, which removes the differencing risk at its origin
// rather than mitigating it.
func CreditedOnlySources() []SourceRecord {
	return []SourceRecord{
		{
			SourceCode:  SourceCadastur,
			Publisher:   "Ministério do Turismo",
			LicenseCode: "LicenseRef-Cadastur-Termos-de-Uso",
			LicenseURL:  "https://cadastur.turismo.gov.br/",
			AttributionText: "Cadastro dos Prestadores de Serviços " +
				"Turísticos (Cadastur), Ministério do Turismo",
			TermsURL:             "https://cadastur.turismo.gov.br/",
			CommercialUseAllowed: false,
			Active:               true,
		},
	}
}

// TideSeries is the card that is born unavailable and that no code path
// unlocks. It is only lawful to call something a tide, and to publish high and
// low water times, when it derives from the harmonic constants of a named CHM
// station — a rights gate, not an availability one (ADR-045 §8).
func TideSeries() (SourceRecord, SeriesRecord) {
	source := SourceRecord{
		SourceCode:  SourceCHMHarmonics,
		Publisher:   "Centro de Hidrografia da Marinha (CHM)",
		LicenseCode: "LicenseRef-CHM-BNDO-Termo-de-Compromisso",
		LicenseURL:  "https://www.marinha.mil.br/chm/dados-do-bndo",
		AttributionText: "Constantes harmônicas do Centro de Hidrografia " +
			"da Marinha (CHM/BNDO)",
		TermsURL:             "https://www.marinha.mil.br/chm/dados-do-bndo",
		CommercialUseAllowed: false,
		Active:               true,
	}
	series := SeriesRecord{
		SourceCode:            SourceCHMHarmonics,
		SeriesCode:            "tide_extremes",
		CardCode:              CardTide,
		UnitCode:              "metre",
		PeriodKind:            "day",
		ValueKind:             "scalar",
		DeclaredLag:           0,
		RetentionDays:         3650,
		PublicExposable:       true,
		GeoScope:              "station",
		DefinitionVersion:     1,
		DataMode:              "real_source",
		Derived:               true,
		DerivationCode:        "tide_harmonic_prediction",
		UnavailableReasonCode: ReasonConstantsNotImported,
	}
	return source, series
}

// SeededSources and SeededSeries exist in the database by catalogue seeding,
// never as a side effect of a successful fetch. The tide will never have a
// `fetch_run` (U-4) and Cadastur will never have an observation (U-7), so a
// catalogue that only appeared as a consequence of collection would leave both
// permanently absent — no tide card, and no credit to Cadastur at all.
//
// Sources come before series because the series carries a foreign key to its
// source.
func SeededSources() []SourceRecord {
	source, _ := TideSeries()
	return append([]SourceRecord{source}, CreditedOnlySources()...)
}

func SeededSeries() []SeriesRecord {
	_, series := TideSeries()
	return []SeriesRecord{series}
}

// DeclaredTargets is the full list, allowlist aside. AllowedTargets is what a
// cycle actually walks.
func DeclaredTargets() []Target {
	return []Target{forecastTarget(), archiveTarget()}
}

func forecastTarget() Target {
	return Target{
		SourceCode: SourceOpenMeteoForecast,
		Host:       "api.open-meteo.com",
		URL:        forecastURL,
		Source:     openMeteoSource(SourceOpenMeteoForecast),
		Series: SeriesRecord{
			SourceCode:        SourceOpenMeteoForecast,
			SeriesCode:        "temperature_2m_max",
			CardCode:          CardWeatherDaily,
			UnitCode:          "celsius",
			PeriodKind:        "day",
			ValueKind:         "scalar",
			DeclaredLag:       6 * time.Hour,
			RetentionDays:     90,
			PublicExposable:   true,
			GeoScope:          "municipality",
			DefinitionVersion: 1,
			DataMode:          "real_source",
		},
		Parse: parseOpenMeteoDaily,
	}
}

func archiveTarget() Target {
	return Target{
		SourceCode: SourceOpenMeteoArchive,
		Host:       "archive-api.open-meteo.com",
		URL:        archiveURL,
		Source:     openMeteoSource(SourceOpenMeteoArchive),
		Series: SeriesRecord{
			SourceCode:        SourceOpenMeteoArchive,
			SeriesCode:        "temperature_2m_max",
			UnitCode:          "celsius",
			PeriodKind:        "day",
			ValueKind:         "scalar",
			DeclaredLag:       120 * time.Hour,
			RetentionDays:     400,
			PublicExposable:   false,
			GeoScope:          "municipality",
			DefinitionVersion: 1,
			DataMode:          "real_source",
		},
		Parse: parseOpenMeteoDaily,
	}
}

// prepare resolves the URL once, at assembly time. A target whose URL does not
// parse is a programming error in a literal and must not reach a cycle.
func prepare(target Target) (Target, bool) {
	parsed, err := url.Parse(target.URL)
	if err != nil || parsed.Hostname() != target.Host {
		return Target{}, false
	}
	target.parsed = parsed
	return target, true
}
