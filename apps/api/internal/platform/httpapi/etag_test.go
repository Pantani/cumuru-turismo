package httpapi

import (
	"errors"
	"testing"
)

func TestParseIfMatchRequiresStrongPositiveVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  int64
		err   error
	}{
		{"", 0, errIfMatchRequired},
		{`W/"1"`, 0, errInvalidIfMatch},
		{`"0"`, 0, errInvalidIfMatch},
		{`"7"`, 7, nil},
	}
	for _, test := range tests {
		got, err := parseIfMatch(test.value)
		if got != test.want || !errors.Is(err, test.err) {
			t.Errorf("parseIfMatch(%q) = %d, %v", test.value, got, err)
		}
	}
}
