package config

import (
	"strings"
	"testing"
)

// A malformed numeric field used to be answered with a sentinel — zero for a
// duration or an integer, NaN for a decimal — and only a later range check could
// notice. These cases pin the failure to the field that is actually wrong.
func TestEnvReaderRejectsMalformedValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		read  func(*envReader)
	}{
		{"duration", "15", func(r *envReader) { r.duration("FIELD", 0) }},
		{"integer", "ten", func(r *envReader) { r.integer("FIELD", 0) }},
		{"decimal", "zero-eight", func(r *envReader) { r.decimal("FIELD", 0) }},
		{"boolean", "sim", func(r *envReader) { r.boolean("FIELD", false) }},
		{"decimal NaN", "NaN", func(r *envReader) { r.decimal("FIELD", 0) }},
		{"decimal Inf", "Inf", func(r *envReader) { r.decimal("FIELD", 0) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			reader := newEnvReader(lookupMap(map[string]string{"FIELD": test.value}))
			test.read(reader)
			err := reader.Err()
			if err == nil || !strings.Contains(err.Error(), "FIELD") {
				t.Fatalf("Err() = %v, want a failure naming FIELD", err)
			}
		})
	}
}

// An absent or blank field selects the caller's fallback and is never a failure.
func TestEnvReaderFallsBackWithoutFailing(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", "   "} {
		reader := newEnvReader(lookupMap(map[string]string{"FIELD": value}))
		if got := reader.integer("FIELD", 7); got != 7 {
			t.Fatalf("integer() = %d, want 7", got)
		}
		if got := reader.decimal("FIELD", 0.5); got != 0.5 {
			t.Fatalf("decimal() = %v, want 0.5", got)
		}
		if err := reader.Err(); err != nil {
			t.Fatalf("Err() = %v, want nil", err)
		}
	}
}

// The first failing field is the one reported, so the operator is pointed at the
// beginning of the problem rather than at whatever tripped last.
func TestEnvReaderKeepsFirstFailure(t *testing.T) {
	t.Parallel()

	reader := newEnvReader(lookupMap(map[string]string{
		"FIRST":  "not-a-number",
		"SECOND": "also-not-a-number",
	}))
	reader.integer("FIRST", 0)
	reader.integer("SECOND", 0)
	err := reader.Err()
	if err == nil || !strings.Contains(err.Error(), "FIRST") {
		t.Fatalf("Err() = %v, want a failure naming FIRST", err)
	}
}
