package external

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
)

// The outcome vocabulary of `external.fetch_runs`, closed by CHECK in the
// migration. A value outside this set is rejected by the database, which is
// where a closed vocabulary belongs.
const (
	OutcomeOK            = "ok"
	OutcomeUnchanged     = "unchanged"
	OutcomeHTTPError     = "http_error"
	OutcomeParseError    = "parse_error"
	OutcomeWriteError    = "write_error"
	OutcomeRateLimited   = "rate_limited"
	OutcomeSkippedBudget = "skipped_budget"
)

var (
	errHostNotAllowed  = errors.New("external host is not allowlisted")
	errSchemeNotHTTPS  = errors.New("external egress requires https")
	errResponseTooBig  = errors.New("external response exceeds the size ceiling")
	errTooManyRedirect = errors.New("external redirect chain is too long")
)

// maxRedirects is small on purpose: a chain is a way to walk a fetcher from an
// allowlisted host to somewhere else one hop at a time.
const maxRedirects = 3

// Fetcher is the only outbound HTTP surface of the product. It exists solely
// in the worker: ADR-045 §6 keeps egress off the request path so the upstream
// never observes the panel's cadence, and nothing in `httpapi` can construct
// one, because it takes a worker-only configuration.
type Fetcher struct {
	client    *http.Client
	allowed   map[string]struct{}
	maxBytes  int64
	userAgent string
	logger    *slog.Logger
}

func NewFetcher(
	settings config.ExternalContextConfig,
	logger *slog.Logger,
) (*Fetcher, error) {
	// A zero RequestTimeout disables http.Client.Timeout entirely, and one slow
	// target would then eat the batch budget of every other target in the
	// cycle. A nil logger would panic on the first fetch, which is the worst
	// possible moment to discover the wiring is incomplete.
	if !usableFetcherSettings(settings) || logger == nil {
		return nil, errors.New("external fetcher configuration invalid")
	}
	fetcher := &Fetcher{
		allowed:   hostSet(settings.AllowedHosts),
		maxBytes:  int64(settings.MaxResponseBytes),
		userAgent: settings.UserAgent,
		logger:    logger,
	}
	fetcher.client = &http.Client{
		Timeout:       settings.RequestTimeout,
		Transport:     egressTransport(),
		CheckRedirect: fetcher.checkRedirect,
	}
	return fetcher, nil
}

func usableFetcherSettings(settings config.ExternalContextConfig) bool {
	return len(settings.AllowedHosts) > 0 &&
		settings.MaxResponseBytes > 0 &&
		settings.RequestTimeout > 0
}

func hostSet(hosts []string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		allowed[strings.ToLower(host)] = struct{}{}
	}
	return allowed
}

// The transport ignores proxy environment variables. A fetcher that honoured
// HTTPS_PROXY would route the whole allowlist through whatever host an
// environment variable happens to name, which defeats the allowlist.
func egressTransport() *http.Transport {
	return &http.Transport{
		Proxy:                 nil,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ForceAttemptHTTP2:     true,
		MaxIdleConnsPerHost:   2,
	}
}

// A redirect is followed only to a host that was already allowlisted and only
// over https. Otherwise a 302 is a way to leave the allowlist without ever
// appearing in configuration.
func (f *Fetcher) checkRedirect(request *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return errTooManyRedirect
	}
	return f.allow(request.URL)
}

func (f *Fetcher) allow(target *url.URL) error {
	if target.Scheme != "https" {
		return errSchemeNotHTTPS
	}
	if _, found := f.allowed[strings.ToLower(target.Hostname())]; !found {
		return errHostNotAllowed
	}
	return nil
}

// Allows reports whether a constant target may be fetched at all. The ingestion
// calls it when it assembles the target list, so a target outside the allowlist
// is left out of the cycle instead of failing once per cycle forever.
func (f *Fetcher) Allows(rawURL string) bool {
	target, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return f.allow(target) == nil
}

type fetchResponse struct {
	body     []byte
	status   int
	duration time.Duration
}

// Fetch retrieves one constant URL. It takes the URL from a package-level
// target and never from a caller-supplied string built out of request data:
// ADR-045 §6 keeps SSRF and cadence leakage shut with the same latch.
func (f *Fetcher) Fetch(
	ctx context.Context,
	target Target,
) (fetchResponse, string) {
	started := time.Now()
	response, outcome := f.perform(ctx, target)
	response.duration = time.Since(started)
	f.log(target, response, outcome)
	return response, outcome
}

func (f *Fetcher) perform(
	ctx context.Context,
	target Target,
) (fetchResponse, string) {
	request, err := f.newRequest(ctx, target)
	if err != nil {
		return fetchResponse{}, OutcomeHTTPError
	}
	response, err := f.client.Do(request)
	if err != nil {
		return fetchResponse{}, OutcomeHTTPError
	}
	defer func() { _ = response.Body.Close() }()
	return f.readResponse(response)
}

func (f *Fetcher) newRequest(
	ctx context.Context,
	target Target,
) (*http.Request, error) {
	if err := f.allow(target.parsed); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodGet, target.URL, nil,
	)
	if err != nil {
		return nil, err
	}
	// The header set is fixed and minimal. No cookie, no Referer, no forwarded
	// client address: the outbound face of the same rule ADR-023 imposes on the
	// inbound proxy.
	request.Header.Set("User-Agent", f.userAgent)
	request.Header.Set("Accept", "application/json")
	return request, nil
}

func (f *Fetcher) readResponse(response *http.Response) (fetchResponse, string) {
	body, err := f.readBounded(response.Body)
	result := fetchResponse{status: response.StatusCode, body: body}
	if err != nil {
		return fetchResponse{status: response.StatusCode}, OutcomeHTTPError
	}
	return result, statusOutcome(response.StatusCode)
}

// The ceiling is enforced while reading, not after: a body that only reveals
// its size once buffered has already cost the memory it was supposed to be
// denied.
func (f *Fetcher) readBounded(body io.Reader) ([]byte, error) {
	content, err := io.ReadAll(io.LimitReader(body, f.maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > f.maxBytes {
		return nil, errResponseTooBig
	}
	return content, nil
}

func statusOutcome(status int) string {
	if status == http.StatusTooManyRequests {
		return OutcomeRateLimited
	}
	if status != http.StatusOK {
		return OutcomeHTTPError
	}
	return OutcomeOK
}

// The egress log carries host, status, duration and outcome, and nothing else.
// Never the URL with its query, never a response body, never headers: an echoed
// upstream response is an unreviewed third-party string inside our own trail.
func (f *Fetcher) log(target Target, response fetchResponse, outcome string) {
	f.logger.Info(
		"external fetch finished",
		"source_code", target.SourceCode,
		"host", target.Host,
		"http_status", response.status,
		"duration_ms", response.duration.Milliseconds(),
		"outcome", outcome,
	)
}
