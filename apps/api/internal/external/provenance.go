package external

// provenance is built identically in both branches. The published card and the
// dead card carry the same publisher, licence, attribution and terms, because
// the CC-BY obligation follows the source, not the outcome of a request.
func provenance(input cardInput) Provenance {
	row := input.head
	return Provenance{
		SourceCode:          row.SourceCode,
		Publisher:           row.Publisher,
		LicenseCode:         row.LicenseCode,
		LicenseURL:          row.LicenseURL,
		AttributionText:     row.AttributionText,
		TermsURL:            row.TermsURL,
		RetrievedAt:         retrievedAt(input),
		ObservedAt:          observedAt(input),
		CoveredPeriod:       coveredPeriod(input),
		DeclaredLagSeconds:  row.DeclaredLagSeconds,
		Revision:            revision(input),
		Derived:             row.Derived,
		DerivationCode:      row.DerivationCode,
		SourceRevisionLabel: sourceRevisionLabel(input),
	}
}

// retrievedAt is the instant of the most recent attempt to obtain the data:
// the observation's retrieval when there is one, the last run otherwise, and
// the assembly instant when the source was never reached — which is when we
// last established that there was nothing to serve.
func retrievedAt(input cardInput) string {
	if input.observed && input.latest.RetrievedAt != nil {
		return instant(*input.latest.RetrievedAt)
	}
	if input.head.LastFetchFinishedAt != nil {
		return instant(*input.head.LastFetchFinishedAt)
	}
	return instant(input.now)
}

// observedAt is the instant the data refers to at the origin, distinct from
// when we collected it. Absent when the source published no period at all.
func observedAt(input cardInput) string {
	if !input.observed || input.latest.PeriodStart == nil {
		return ""
	}
	return instant(*input.latest.PeriodStart)
}

// With observations the covered period is the span they actually cover. With
// none, it is the civil day the card would have covered, so that a dead card
// still states what it is about instead of leaving the reader to guess.
func coveredPeriod(input cardInput) CoveredPeriod {
	start, end := civilDayBounds(input.now)
	if len(input.points) > 0 {
		start = input.points[0].PeriodStart
		end = input.points[len(input.points)-1].PeriodEnd
	}
	return CoveredPeriod{
		Start:        start,
		End:          end,
		EndExclusive: true,
		TimeZone:     PublicTimeZone,
	}
}

func revision(input cardInput) int32 {
	if !input.observed {
		return 0
	}
	return input.latest.Revision
}

func sourceRevisionLabel(input cardInput) string {
	if !input.observed {
		return ""
	}
	return input.latest.SourceRevisionLabel
}
