package config_test

import (
	"strings"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
)

func TestLoadAPIParsesTrustedProxyCIDRs(t *testing.T) {
	t.Parallel()

	values := merge(validLocal(), map[string]string{
		"TRUSTED_PROXY_CIDRS": "10.20.30.41/24,2001:db8:abcd:1234::1/56",
	})
	got, err := config.Load(config.ProcessAPI, lookup(values))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"10.20.30.0/24", "2001:db8:abcd:1200::/56"}
	if len(got.TrustedProxyCIDRs) != len(want) {
		t.Fatalf("TrustedProxyCIDRs length = %d, want %d", len(got.TrustedProxyCIDRs), len(want))
	}
	for index, prefix := range got.TrustedProxyCIDRs {
		if prefix.String() != want[index] {
			t.Errorf("TrustedProxyCIDRs[%d] = %q, want %q", index, prefix, want[index])
		}
	}
}

func TestLoadAPIRejectsInvalidTrustedProxyCIDRs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value *string
	}{
		{name: "missing"},
		{name: "blank", value: stringPointer("")},
		{name: "bare IP", value: stringPointer("127.0.0.1")},
		{name: "invalid member", value: stringPointer("127.0.0.1/32,invalid")},
		{name: "empty member", value: stringPointer("127.0.0.1/32,,::1/128")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertTrustedProxyConfigRejected(t, tt.value)
		})
	}
}

func TestLoadWorkerDoesNotRequireTrustedProxyCIDRs(t *testing.T) {
	t.Parallel()

	values := validLocal()
	delete(values, "TRUSTED_PROXY_CIDRS")
	if _, err := config.Load(config.ProcessWorker, lookup(values)); err != nil {
		t.Fatalf("Load(worker) error = %v", err)
	}
}

func assertTrustedProxyConfigRejected(t *testing.T, value *string) {
	t.Helper()
	values := validLocal()
	if value == nil {
		delete(values, "TRUSTED_PROXY_CIDRS")
	} else {
		values["TRUSTED_PROXY_CIDRS"] = *value
	}
	_, err := config.Load(config.ProcessAPI, lookup(values))
	if err == nil || !strings.Contains(err.Error(), "TRUSTED_PROXY_CIDRS") {
		t.Fatalf("Load() error = %v, want sanitized TRUSTED_PROXY_CIDRS error", err)
	}
	if value != nil && *value != "" && strings.Contains(err.Error(), *value) {
		t.Fatalf("Load() leaked rejected CIDR value: %v", err)
	}
}

func stringPointer(value string) *string {
	return &value
}
