package httpapi

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

type strictFixture struct {
	Name string `json:"name"`
}

func TestDecodeStrictRejectsUnknownDuplicateAndTrailingJSON(t *testing.T) {
	t.Parallel()

	tests := []string{
		`{"name":"one","unknown":true}`,
		`{"name":"one","name":"two"}`,
		`{"name":"one"} {"name":"two"}`,
	}
	for _, body := range tests {
		request := httptest.NewRequest("POST", "/", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		var target strictFixture
		if err := decodeStrict(request, "application/json", &target); !errors.Is(err, errInvalidJSON) {
			t.Errorf("decodeStrict(%q) error = %v", body, err)
		}
	}
}

func TestDecodeStrictAcceptsClosedJSON(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"one"}`))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	var target strictFixture

	if err := decodeStrict(request, "application/json", &target); err != nil {
		t.Fatalf("decodeStrict() error = %v", err)
	}
	if target.Name != "one" {
		t.Fatalf("target = %#v", target)
	}
}
