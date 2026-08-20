package external

import "time"

// fetchOutcomeReasons maps the recorded outcome of the last run onto the closed
// reason vocabulary of the contract. It reads the outcome, never the absence of
// rows: without `fetch_runs`, "the source is unavailable" and "the cycle never
// ran" are the same silence.
//
// `ok` and `unchanged` mean the fetch worked and still produced no observation
// for the series, which is the source not publishing the period. Everything
// else failed on our side of the boundary and the card says so without
// describing how — `write_error` included: the trail names the cause, the
// public card does not, because a reader has no use for whether our database or
// the upstream was the one that broke.
var fetchOutcomeReasons = map[string]string{
	OutcomeOK:            ReasonSourceDataMissing,
	OutcomeUnchanged:     ReasonSourceDataMissing,
	OutcomeRateLimited:   ReasonSourceRateLimited,
	OutcomeHTTPError:     ReasonSourceUnavailable,
	OutcomeParseError:    ReasonSourceUnavailable,
	OutcomeWriteError:    ReasonSourceUnavailable,
	OutcomeSkippedBudget: ReasonSourceUnavailable,
}

// unavailableReason returns the empty string when the card publishes.
func unavailableReason(input cardInput) string {
	// A structural reason outranks any fetch: the tide card is
	// `constants_not_imported` by U-4 and no code path unlocks it.
	if input.head.UnavailableReasonCode != "" {
		return input.head.UnavailableReasonCode
	}
	if len(input.points) == 0 {
		return fetchOutcomeReason(input.head.LastFetchOutcome)
	}
	if staleBeyondDeclaredLag(input) {
		return ReasonStaleBeyondLag
	}
	return ""
}

// An outcome nobody recorded is a run that never happened, and the honest
// answer for a card with no data and no run is that the source is unavailable.
func fetchOutcomeReason(outcome string) string {
	if reason, found := fetchOutcomeReasons[outcome]; found {
		return reason
	}
	return ReasonSourceUnavailable
}

// The declared lag is the delay the source itself announces. Data whose covered
// period ended longer ago than that lag means the source failed to publish on
// time, which is a different fact from a failed fetch and gets its own code.
func staleBeyondDeclaredLag(input cardInput) bool {
	if input.latest.PeriodEnd == nil {
		return false
	}
	lag := time.Duration(input.head.DeclaredLagSeconds) * time.Second
	return input.latest.PeriodEnd.Add(lag).Before(input.now)
}
