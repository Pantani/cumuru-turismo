package calendarfeed

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

// NormalizeFeedURL only judges what was typed, and a hostname is not an address.
// This proves the guard that actually holds: the connection to an address
// inside the deployment is refused after resolution, before any byte is sent.
func TestFetchRefusesToDialAnAddressInsideTheDeployment(t *testing.T) {
	t.Parallel()

	var served atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		served.Store(true)
	}))
	defer server.Close()

	fetcher := NewHTTPFetcher(2*time.Second, DefaultFetchLimit)
	if _, err := fetcher.Fetch(context.Background(), server.URL); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Fetch(loopback) error = %v, want ErrUnavailable", err)
	}
	if served.Load() {
		t.Fatal("the loopback handler ran: the dial guard did not hold")
	}
}

// Blocking the embedded credential only at NormalizeFeedURL would leave a
// redirect free to reintroduce exactly what the guard was written to stop.
func TestCheckRedirectRefusesTheHopThatChangesTheRules(t *testing.T) {
	t.Parallel()

	origin := mustRequest(t, "https://ical.booking.com/v1/export?t=9f2a")
	refused := map[string]string{
		"embedded secret": "https://user:pass@ical.booking.com/v1/export",
		"plain http":      "http://ical.booking.com/v1/export",
		"another host":    "https://attacker.invalid/v1/export",
	}
	for name, target := range refused {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			hop := mustRequest(t, target)
			if err := checkRedirect(hop, []*http.Request{origin}); err == nil {
				t.Fatalf("checkRedirect(%s) error = nil", name)
			}
		})
	}
}

func TestCheckRedirectAllowsTheExtranetOwnHop(t *testing.T) {
	t.Parallel()

	origin := mustRequest(t, "https://ical.booking.com/v1/export?t=9f2a")
	hop := mustRequest(t, "https://ical.booking.com/v1/export/final.ics")
	if err := checkRedirect(hop, []*http.Request{origin}); err != nil {
		t.Fatalf("checkRedirect(same host) error = %v", err)
	}
}

func TestCheckRedirectStopsAChain(t *testing.T) {
	t.Parallel()

	origin := mustRequest(t, "https://ical.booking.com/v1/export?t=9f2a")
	chain := make([]*http.Request, maxRedirects)
	for index := range chain {
		chain[index] = origin
	}
	if err := checkRedirect(origin, chain); err == nil {
		t.Fatal("checkRedirect(chain) error = nil")
	}
}

func mustRequest(t *testing.T, address string) *http.Request {
	t.Helper()
	parsed, err := url.Parse(address)
	if err != nil {
		t.Fatalf("parse %q: %v", address, err)
	}
	return &http.Request{URL: parsed, Host: parsed.Host}
}
