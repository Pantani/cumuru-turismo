package config

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// envReader parses typed environment values and remembers the first failure.
//
// It exists because the previous parsers answered a malformed value with a
// sentinel — zero for a duration or an integer, NaN for a decimal — and left it
// to a later validator to notice. That worked only while every field happened to
// have a range check: PHASE4_PRE_REGISTERED_WEIGHT did not, because every
// comparison against NaN is false, so a typo there passed validation and reached
// the forecast arithmetic. Recording the failure at the point of parsing removes
// that whole class of bug and names the offending field instead of whichever
// range check happened to trip first.
type envReader struct {
	lookup LookupEnv
	err    error
}

func newEnvReader(lookup LookupEnv) *envReader {
	return &envReader{lookup: resolveLookup(lookup)}
}

// Err reports the first field that failed to parse. Loaders check it once, after
// reading a whole block, so a field read stays a plain assignment.
func (r *envReader) Err() error {
	return r.err
}

// fail keeps the first failure: later fields still parse, but the error the
// operator sees points at the first field that was actually wrong.
func (r *envReader) fail(field string) {
	if r.err == nil {
		r.err = invalid(field)
	}
}

// present reports whether the field carries a value worth parsing. An absent or
// blank field is not an error; it selects the caller's fallback.
func (r *envReader) present(field string) (string, bool) {
	value, ok := r.lookup(field)
	trimmed := strings.TrimSpace(value)
	return trimmed, ok && trimmed != ""
}

func (r *envReader) duration(field string, fallback time.Duration) time.Duration {
	value, ok := r.present(field)
	if !ok {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		r.fail(field)
		return fallback
	}
	return parsed
}

func (r *envReader) integer(field string, fallback int) int {
	value, ok := r.present(field)
	if !ok {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		r.fail(field)
		return fallback
	}
	return parsed
}

// integerInRange refuses a value outside the bounds instead of letting the
// caller narrow it. A field the caller stores as uint8 or int32 is converted
// after this returns, and the conversion is silent: PROOF_OF_WORK_DIFFICULTY_BASE
// set to 268 becomes 12, which then passes the 1..32 range check as a value the
// operator never wrote. Bounding it here is what makes that impossible.
func (r *envReader) integerInRange(field string, fallback, minimum, maximum int) int {
	value := r.integer(field, fallback)
	if value < minimum || value > maximum {
		r.fail(field)
		return fallback
	}
	return value
}

// decimal rejects NaN and the infinities explicitly. ParseFloat accepts "NaN"
// and "Inf" as valid input, and either one would defeat every range check the
// validators apply afterwards.
func (r *envReader) decimal(field string, fallback float64) float64 {
	value, ok := r.present(field)
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		r.fail(field)
		return fallback
	}
	return parsed
}

func (r *envReader) boolean(field string, fallback bool) bool {
	value, ok := r.present(field)
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		r.fail(field)
		return fallback
	}
	return parsed
}
