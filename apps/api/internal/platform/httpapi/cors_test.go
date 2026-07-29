package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestRateSubjectSelectsOnlyTrustedForwardedAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		remote    string
		forwarded []string
		trusted   []netip.Prefix
		want      string
		wantErr   bool
	}{
		{
			name: "untrusted ignores spoofed header", remote: "203.0.113.123:4321",
			forwarded: []string{"198.51.100.44"}, want: "203.0.113.0/24",
		},
		{
			name: "untrusted ignores malformed header", remote: "203.0.113.123:4321",
			forwarded: []string{"not-an-ip, 198.51.100.44"}, want: "203.0.113.0/24",
		},
		{
			name: "trusted IPv4", remote: "10.20.30.40:4321",
			forwarded: []string{"198.51.100.44"},
			trusted:   []netip.Prefix{netip.MustParsePrefix("10.20.30.40/32")},
			want:      "198.51.100.0/24",
		},
		{
			name: "trusted IPv6", remote: "[2001:db8::10]:4321",
			forwarded: []string{"2001:db8:abcd:1234::99"},
			trusted:   []netip.Prefix{netip.MustParsePrefix("2001:db8::10/128")},
			want:      "2001:db8:abcd:1200::/56",
		},
		{
			name: "mapped IPv4 is normalized", remote: "10.20.30.40:4321",
			forwarded: []string{"::ffff:192.0.2.129"},
			trusted:   []netip.Prefix{netip.MustParsePrefix("10.20.30.40/32")},
			want:      "192.0.2.0/24",
		},
		{
			name: "trusted missing header", remote: "10.20.30.40:4321",
			trusted: []netip.Prefix{netip.MustParsePrefix("10.20.30.40/32")},
			wantErr: true,
		},
		{
			name: "trusted comma chain", remote: "10.20.30.40:4321",
			forwarded: []string{"198.51.100.44, 203.0.113.9"},
			trusted:   []netip.Prefix{netip.MustParsePrefix("10.20.30.40/32")},
			wantErr:   true,
		},
		{
			name: "trusted repeated header", remote: "10.20.30.40:4321",
			forwarded: []string{"198.51.100.44", "203.0.113.9"},
			trusted:   []netip.Prefix{netip.MustParsePrefix("10.20.30.40/32")},
			wantErr:   true,
		},
		{
			name: "trusted malformed literal", remote: "10.20.30.40:4321",
			forwarded: []string{"198.51.100.44:1234"},
			trusted:   []netip.Prefix{netip.MustParsePrefix("10.20.30.40/32")},
			wantErr:   true,
		},
		{
			name: "trusted zoned IPv6", remote: "10.20.30.40:4321",
			forwarded: []string{"fe80::1%eth0"},
			trusted:   []netip.Prefix{netip.MustParsePrefix("10.20.30.40/32")},
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertRateSubject(t, tt.remote, tt.forwarded, tt.trusted, tt.want, tt.wantErr)
		})
	}
}

func TestRateSubjectsSeparateForwardedClients(t *testing.T) {
	t.Parallel()

	trusted := []netip.Prefix{netip.MustParsePrefix("10.20.30.40/32")}
	first := requestWithForwardedFor("10.20.30.40:4321", []string{"198.51.100.44"})
	second := requestWithForwardedFor("10.20.30.40:4321", []string{"203.0.113.9"})
	firstSubject, firstErr := rateSubject(first, trusted)
	secondSubject, secondErr := rateSubject(second, trusted)
	if firstErr != nil || secondErr != nil {
		t.Fatalf("rateSubject() errors = %v, %v", firstErr, secondErr)
	}
	if firstSubject == secondSubject {
		t.Fatalf("rate subjects collapsed two clients: %q", firstSubject)
	}
}

func assertRateSubject(
	t *testing.T,
	remote string,
	forwarded []string,
	trusted []netip.Prefix,
	want string,
	wantErr bool,
) {
	t.Helper()
	request := requestWithForwardedFor(remote, forwarded)
	got, err := rateSubject(request, trusted)
	if wantErr {
		if err == nil {
			t.Fatalf("rateSubject() = %q, want error", got)
		}
		return
	}
	if err != nil || got != want {
		t.Fatalf("rateSubject() = %q, %v; want %q, nil", got, err, want)
	}
}

func requestWithForwardedFor(remote string, values []string) *http.Request {
	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = remote
	for _, value := range values {
		request.Header.Add("X-Forwarded-For", value)
	}
	return request
}
