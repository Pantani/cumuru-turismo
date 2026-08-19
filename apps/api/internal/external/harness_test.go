package external

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
)

// No gate reaches the public network. The upstream is a local HTTPS stub
// serving fixtures recorded in testdata/, which is also why the https-only rule
// stays under test instead of being disabled for convenience.
const testUserAgent = "CumuruObservatorio/1.0 " +
	"(+https://turismo.prado.ba.gov.br; contato@prado.ba.gov.br)"

type stubUpstream struct {
	server   *httptest.Server
	requests []*http.Request
}

func newStubUpstream(t *testing.T, handler http.HandlerFunc) *stubUpstream {
	t.Helper()
	stub := &stubUpstream{}
	stub.server = httptest.NewTLSServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			stub.requests = append(stub.requests, request.Clone(request.Context()))
			handler(writer, request)
		},
	))
	t.Cleanup(stub.server.Close)
	return stub
}

func fixtureHandler(t *testing.T, name string) http.HandlerFunc {
	t.Helper()
	body := fixture(t, name)
	return func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(body)
	}
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("fixture %s unreadable: %v", name, err)
	}
	return body
}

func testSettings() config.ExternalContextConfig {
	return config.ExternalContextConfig{
		Enabled:           true,
		AllowedHosts:      []string{"127.0.0.1"},
		RequestTimeout:    300 * time.Millisecond,
		BatchBudget:       5 * time.Second,
		MaxResponseBytes:  64 * 1024,
		IngestionInterval: 6 * time.Hour,
		UserAgent:         testUserAgent,
	}
}

func testLogger(writer io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// newTestFetcher trusts the stub's own certificate and nothing else. The
// allowlist, the https requirement, the size ceiling and the redirect rule stay
// exactly as production builds them.
func newTestFetcher(
	t *testing.T,
	stub *stubUpstream,
	settings config.ExternalContextConfig,
	logger *slog.Logger,
) *Fetcher {
	t.Helper()
	fetcher, err := NewFetcher(settings, logger)
	if err != nil {
		t.Fatalf("fetcher not built: %v", err)
	}
	transport, ok := fetcher.client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("egress transport has an unexpected type")
	}
	transport.TLSClientConfig = &tls.Config{
		RootCAs:    stubCertPool(t, stub),
		MinVersion: tls.VersionTLS12,
	}
	return fetcher
}

func stubCertPool(t *testing.T, stub *stubUpstream) *x509.CertPool {
	t.Helper()
	client := stub.server.Client()
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig == nil {
		t.Fatal("stub client has no TLS configuration")
	}
	return transport.TLSClientConfig.RootCAs
}

// stubTarget points a real Target at the local stub. Everything else — the
// parser, the series definition and the fetch path — is what production uses.
func stubTarget(t *testing.T, stub *stubUpstream) Target {
	t.Helper()
	target := forecastTarget()
	address, err := url.Parse(stub.server.URL)
	if err != nil {
		t.Fatalf("stub URL invalid: %v", err)
	}
	target.Host = address.Hostname()
	target.URL = stub.server.URL + "/v1/forecast?daily=temperature_2m_max"
	prepared, ok := prepare(target)
	if !ok {
		t.Fatal("stub target did not prepare")
	}
	return prepared
}

func newTestIngestion(
	repository Repository,
	fetcher *Fetcher,
	settings config.ExternalContextConfig,
	logger *slog.Logger,
	targets []Target,
) *Ingestion {
	ingestion := NewIngestion(repository, fetcher, settings, logger, NewMetrics())
	ingestion.targets = targets
	return ingestion
}
