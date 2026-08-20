// Package external carries the context layer of ADR-045: data copied from a
// third party, credited by licence, displayed beside the series the platform
// measures itself and never mixed into it.
//
// The direction is one-way and it is enforced by PostgreSQL ACL, not by this
// package: `external_runtime` writes here and reaches neither `core`, `survey`,
// `analytics` nor `public_data`, and `worker_runtime`, which reconciles the
// protected series, holds no privilege in `external`. Nothing in this package
// imports `internal/analytics`, and nothing here produces an `analytics.Cell`.
package external

import (
	"context"
	"errors"
	"time"
)

// ErrContextUnavailable is the only failure the public reader reports. A dead
// source is never this error: it is a card with status `unavailable` inside a
// 200 (ADR-045 §3). This error means the document itself could not be
// assembled, which is the sole case the contract answers with 503.
var ErrContextUnavailable = errors.New("external context unavailable")

const (
	// LayerCode and DisclaimerCode are `const` in the contract. They exist so a
	// reader never has to infer, from position on a page, that these numbers
	// are not a platform measurement.
	LayerCode      = "external_context"
	DisclaimerCode = "external_context_not_platform_measurement"

	// PublicTimeZone anchors the covered period and the ingestion calendar.
	// Constant by ADR-045 §6: a schedule derived from traffic would turn the
	// upstream into an observer of the panel's cadence.
	PublicTimeZone = "America/Bahia"

	StatusPublished   = "published"
	StatusUnavailable = "unavailable"
)

// The closed reason vocabulary of the contract. No free text: a reason a
// machine cannot enumerate is a reason nobody can test.
const (
	ReasonSourceUnavailable    = "source_unavailable"
	ReasonSourceRateLimited    = "source_rate_limited"
	ReasonSourceNotLicensed    = "source_not_licensed"
	ReasonSourceDataMissing    = "source_data_missing"
	ReasonConstantsNotImported = "constants_not_imported"
	ReasonStaleBeyondLag       = "stale_beyond_declared_lag"
)

// ContextReader is what the HTTP handler depends on. It is deliberately narrow:
// the handler must not be able to reach ingestion, and this interface gives it
// no way to.
type ContextReader interface {
	Context(context.Context) (PublicContext, error)
}

type PublicContext struct {
	GeneratedAt    string           `json:"generated_at"`
	Layer          string           `json:"layer"`
	DisclaimerCode string           `json:"disclaimer_code"`
	Cards          []Card           `json:"cards"`
	Sources        []CreditedSource `json:"sources"`
}

// Card is the wrapper for the contract's `oneOf`. `unit_code` and `series`
// exist only in the published branch and `reason_code` only in the unavailable
// one, which `omitempty` expresses exactly because a published card always has
// at least one point and an unavailable card always has a reason.
type Card struct {
	CardCode   string        `json:"card_code"`
	Status     string        `json:"status"`
	DataMode   string        `json:"data_mode"`
	Provenance Provenance    `json:"provenance"`
	UnitCode   string        `json:"unit_code,omitempty"`
	Series     []SeriesPoint `json:"series,omitempty"`
	ReasonCode string        `json:"reason_code,omitempty"`
}

// Provenance is mandatory in both branches (ADR-045 §7): source, licence and
// attribution exist because the source exists, not because the fetch
// succeeded. AttributionText is copied from the database and is never composed
// in Go — licence text concatenated in code is licence text nobody reviews.
type Provenance struct {
	SourceCode          string        `json:"source_code"`
	Publisher           string        `json:"publisher"`
	LicenseCode         string        `json:"license_code"`
	LicenseURL          string        `json:"license_url"`
	AttributionText     string        `json:"attribution_text"`
	TermsURL            string        `json:"terms_url"`
	RetrievedAt         string        `json:"retrieved_at"`
	ObservedAt          string        `json:"observed_at,omitempty"`
	CoveredPeriod       CoveredPeriod `json:"covered_period"`
	DeclaredLagSeconds  int64         `json:"declared_lag_seconds"`
	Revision            int32         `json:"revision"`
	Derived             bool          `json:"derived"`
	DerivationCode      string        `json:"derivation_code,omitempty"`
	SourceRevisionLabel string        `json:"source_revision_label,omitempty"`
}

type CoveredPeriod struct {
	Start        string `json:"start"`
	End          string `json:"end"`
	EndExclusive bool   `json:"end_exclusive"`
	TimeZone     string `json:"time_zone"`
}

// SeriesPoint carries no rounding. The base-10 rounding of /public/presence is
// disclosure control; copying it here would suggest the suppression pipeline
// ran over a number it never touched.
type SeriesPoint struct {
	PeriodStart string  `json:"period_start"`
	PeriodEnd   string  `json:"period_end"`
	Value       float64 `json:"value"`
}

// CreditedSource is the credit list of the document. Cadastur appears here and
// only here (U-7): attribution and link, with no count computed by the
// platform and no published universe series.
type CreditedSource struct {
	SourceCode      string `json:"source_code"`
	Publisher       string `json:"publisher"`
	LicenseCode     string `json:"license_code"`
	LicenseURL      string `json:"license_url"`
	AttributionText string `json:"attribution_text"`
	TermsURL        string `json:"terms_url"`
}

// ContextRow is the neutral row the store repository produces. The mapping
// lives here, against this type, so that assembling the document — including
// every unavailable branch — is testable without a database.
type ContextRow struct {
	CardCode              string
	SourceCode            string
	SeriesCode            string
	UnitCode              string
	DataMode              string
	Derived               bool
	DerivationCode        string
	UnavailableReasonCode string
	DeclaredLagSeconds    int64
	Publisher             string
	LicenseCode           string
	LicenseURL            string
	AttributionText       string
	TermsURL              string
	PeriodStart           *time.Time
	PeriodEnd             *time.Time
	Value                 *float64
	RetrievedAt           *time.Time
	Revision              int32
	SourceRevisionLabel   string
	LastFetchOutcome      string
	LastFetchFinishedAt   *time.Time
}
