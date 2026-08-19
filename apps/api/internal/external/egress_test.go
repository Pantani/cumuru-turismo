package external

import (
	"bytes"
	"context"
	"net/http"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"strings"
	"testing"
)

// Nothing outside the configured allowlist is fetched, and the allowlist lives
// in configuration precisely so that an UPDATE cannot turn the fetcher into an
// SSRF primitive.
func TestFetcherRefusesHostsOutsideTheAllowlist(t *testing.T) {
	cases := map[string]string{
		"foreign host":  "https://example.invalid/v1/forecast",
		"plain http":    "http://127.0.0.1/v1/forecast",
		"host as path":  "https://attacker.invalid/127.0.0.1",
		"credentials":   "https://127.0.0.1@attacker.invalid/v1",
		"link local":    "https://169.254.169.254/latest/meta-data",
		"unix scheme":   "file:///etc/passwd",
		"missing sched": "//127.0.0.1/v1/forecast",
	}
	fetcher := allowlistFetcher(t)
	for name, candidate := range cases {
		t.Run(name, func(t *testing.T) {
			if fetcher.Allows(candidate) {
				t.Fatalf("fetcher allowed %s", name)
			}
		})
	}
}

func TestFetcherAllowsOnlyTheConfiguredHostOverHTTPS(t *testing.T) {
	fetcher := allowlistFetcher(t)
	if !fetcher.Allows("https://127.0.0.1/v1/forecast") {
		t.Fatal("fetcher refused the allowlisted host over https")
	}
}

// A redirect is a way of leaving the allowlist one hop at a time without ever
// appearing in configuration, so it is refused rather than followed.
func TestFetcherRefusesRedirectOffTheAllowlist(t *testing.T) {
	stub := newStubUpstream(t, func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(
			writer, request,
			"https://example.invalid/v1/forecast", http.StatusFound,
		)
	})
	repository, ingestion := ingestionOver(t, stub)

	ingestion.RunCycle(context.Background())

	if got := repository.lastRun().Outcome; got != OutcomeHTTPError {
		t.Fatalf("redirect outcome = %q, want %q", got, OutcomeHTTPError)
	}
	if repository.count() != 0 {
		t.Fatalf("redirect stored %d observations, want 0", repository.count())
	}
}

// No byte of any HTTP request reaches the external URL. The target is a package
// literal and the fetcher is given no channel through which caller data could
// enter it, which closes SSRF and cadence leakage with the same latch.
func TestExternalURLCarriesOnlyItsConstantParameters(t *testing.T) {
	stub := newStubUpstream(t, fixtureHandler(t, "open_meteo_forecast.json"))
	_, ingestion := ingestionOver(t, stub)

	ingestion.RunCycle(context.Background())

	if len(stub.requests) != 1 {
		t.Fatalf("upstream called %d times, want 1", len(stub.requests))
	}
	query := stub.requests[0].URL.Query()
	if len(query) != 1 || query.Get("daily") != "temperature_2m_max" {
		t.Fatalf("unexpected upstream query: %v", query)
	}
}

// The production literals are checked as literals: constant host, constant
// path, constant parameter set, coordinates fixed at two decimals.
func TestDeclaredTargetsAreConstantAndCoordinatesRounded(t *testing.T) {
	for _, target := range DeclaredTargets() {
		prepared, ok := prepare(target)
		if !ok {
			t.Fatalf("target %s did not prepare", target.SourceCode)
		}
		assertConstantTarget(t, prepared)
	}
}

func assertConstantTarget(t *testing.T, target Target) {
	t.Helper()
	if target.parsed.Scheme != "https" {
		t.Fatalf("%s is not https", target.SourceCode)
	}
	query := target.parsed.Query()
	if query.Get("latitude") != observationLatitude ||
		query.Get("longitude") != observationLongitude {
		t.Fatalf("%s does not use the fixed observation point", target.SourceCode)
	}
	for _, coordinate := range []string{observationLatitude, observationLongitude} {
		if decimals := decimalPlaces(coordinate); decimals != 2 {
			t.Fatalf("coordinate %s has %d decimals, want 2", coordinate, decimals)
		}
	}
}

func decimalPlaces(value string) int {
	_, fraction, found := strings.Cut(value, ".")
	if !found {
		return 0
	}
	return len(fraction)
}

// The User-Agent is institutional and constant. It never carries tenant,
// organization, accommodation, operator, OIDC subject nor a version that varies
// per installation, and no cookie or Referer accompanies it.
func TestUserAgentIsConstantAndCarriesNoIdentifier(t *testing.T) {
	stub := newStubUpstream(t, fixtureHandler(t, "open_meteo_forecast.json"))
	_, ingestion := ingestionOver(t, stub)

	ingestion.RunCycle(context.Background())
	ingestion.RunCycle(context.Background())

	if len(stub.requests) != 2 {
		t.Fatalf("upstream called %d times, want 2", len(stub.requests))
	}
	first := stub.requests[0].Header.Get("User-Agent")
	if first != stub.requests[1].Header.Get("User-Agent") {
		t.Fatal("User-Agent varied between requests")
	}
	assertNoIdentifier(t, first)
	assertNoClientHeaders(t, stub.requests[0])
}

func assertNoIdentifier(t *testing.T, agent string) {
	t.Helper()
	if agent != testUserAgent {
		t.Fatalf("User-Agent = %q, want the institutional constant", agent)
	}
	forbidden := []string{
		"tenant", "organization", "organizacao", "accommodation",
		"acomodacao", "operator", "operador", "subject", "sub=", "oidc",
		"session", "token",
	}
	lowered := strings.ToLower(agent)
	for _, needle := range forbidden {
		if strings.Contains(lowered, needle) {
			t.Fatalf("User-Agent carries %q", needle)
		}
	}
}

func assertNoClientHeaders(t *testing.T, request *http.Request) {
	t.Helper()
	for _, header := range []string{
		"Cookie", "Referer", "X-Forwarded-For", "Authorization", "X-Request-Id",
	} {
		if request.Header.Get(header) != "" {
			t.Fatalf("egress carried the %s header", header)
		}
	}
}

// The egress log records host, status, duration and outcome, and nothing else.
// Never the URL with its query, never a response body, never headers: an echoed
// upstream response is an unreviewed third-party string inside our own trail.
func TestEgressLogOmitsURLBodyAndHeaders(t *testing.T) {
	recorded := &bytes.Buffer{}
	settings := testSettings()
	stub := newStubUpstream(t, fixtureHandler(t, "open_meteo_forecast.json"))
	logger := testLogger(recorded)
	fetcher := newTestFetcher(t, stub, settings, logger)
	ingestion := newTestIngestion(
		&fakeRepository{}, fetcher, settings, logger,
		[]Target{stubTarget(t, stub)},
	)

	ingestion.RunCycle(context.Background())

	logged := recorded.String()
	if !strings.Contains(logged, `"host"`) ||
		!strings.Contains(logged, `"http_status"`) ||
		!strings.Contains(logged, `"duration_ms"`) {
		t.Fatalf("egress log lost its required fields: %s", logged)
	}
	for _, forbidden := range []string{
		"temperature_2m_max", "daily", "26.4", "?", "User-Agent",
		"Content-Type", stub.server.URL,
	} {
		if strings.Contains(logged, forbidden) {
			t.Fatalf("egress log leaked %q: %s", forbidden, logged)
		}
	}
}

func allowlistFetcher(t *testing.T) *Fetcher {
	t.Helper()
	fetcher, err := NewFetcher(testSettings(), testLogger(&bytes.Buffer{}))
	if err != nil {
		t.Fatalf("fetcher not built: %v", err)
	}
	return fetcher
}

// A target whose host is not allowlisted is left out of the cycle, and adding
// the host is all it takes to turn it on. That is the whole cost of the second
// source.
func TestAllowlistDecidesWhichTargetsRun(t *testing.T) {
	logger := testLogger(&bytes.Buffer{})
	defaults, err := NewFetcher(defaultHostSettings(), logger)
	if err != nil {
		t.Fatalf("fetcher not built: %v", err)
	}
	if got := len(allowedTargets(defaults, logger)); got != 1 {
		t.Fatalf("default allowlist enabled %d targets, want 1", got)
	}

	widened := defaultHostSettings()
	widened.AllowedHosts = append(widened.AllowedHosts, archiveHost(t))
	broader, err := NewFetcher(widened, logger)
	if err != nil {
		t.Fatalf("fetcher not built: %v", err)
	}
	if got := len(allowedTargets(broader, logger)); got != len(DeclaredTargets()) {
		t.Fatalf("widened allowlist enabled %d targets, want %d",
			got, len(DeclaredTargets()))
	}
}

// The production default allowlist, verbatim from config/external.go.
func defaultHostSettings() config.ExternalContextConfig {
	settings := testSettings()
	settings.AllowedHosts = []string{"api.open-meteo.com"}
	return settings
}

func archiveHost(t *testing.T) string {
	t.Helper()
	for _, target := range DeclaredTargets() {
		if target.SourceCode == SourceOpenMeteoArchive {
			return target.Host
		}
	}
	t.Fatal("the archive target is not declared")
	return ""
}
