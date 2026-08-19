package calendarfeed

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"syscall"
	"time"
)

// DefaultFetchLimit bounds the response body. A pousada's two-year calendar is
// tens of kilobytes; four megabytes is far past any honest file and still small
// enough that a hostile host cannot spend the worker's memory.
const DefaultFetchLimit int64 = 4 << 20

// maxRedirects allows the extranet's own hop and nothing resembling a chain.
const maxRedirects = 3

// HTTPFetcher is the only outbound caller in the system. Every rule it enforces
// is one a genuine `.ics` never needs broken: encrypted transport, a bounded
// body, a deadline, no redirect that leaves the host the operator named, and no
// connection to an address inside the deployment.
type HTTPFetcher struct {
	client *http.Client
	limit  int64
}

func NewHTTPFetcher(timeout time.Duration, limit int64) *HTTPFetcher {
	if limit <= 0 {
		limit = DefaultFetchLimit
	}
	return &HTTPFetcher{
		client: &http.Client{
			Timeout:       timeout,
			CheckRedirect: checkRedirect,
			Transport:     guardedTransport(timeout),
		},
		limit: limit,
	}
}

// guardedTransport is where the address guard actually holds. NormalizeFeedURL
// can only judge what was typed, and a hostname is not an address: it may
// resolve to 127.0.0.1 or to 10.0.0.0/8, and it may resolve differently on the
// second lookup than on the first. Control runs after resolution, once per
// connection, on the address about to be dialed — so a name that points inside
// the deployment is refused whether it always did or only started to.
//
// The transport is built here instead of cloned from http.DefaultTransport: a
// dependency that replaces the global would turn the clone into a panic at
// startup, and this client wants a shape it declares rather than one it inherits.
func guardedTransport(timeout time.Duration) *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:         timeout,
			ControlContext:  controlDialedAddress,
			FallbackDelay:   -1,
			KeepAliveConfig: net.KeepAliveConfig{Enable: false},
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          8,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   timeout,
		ExpectContinueTimeout: time.Second,
	}
}

func controlDialedAddress(_ context.Context, _, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return ErrUnavailable
	}
	parsed, err := netip.ParseAddr(host)
	if err != nil || !routableAddress(parsed) {
		return ErrUnavailable
	}
	return nil
}

func (f *HTTPFetcher) Fetch(ctx context.Context, address string) (string, error) {
	response, err := f.get(ctx, address)
	if err != nil {
		return "", ErrUnavailable
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", ErrUnavailable
	}
	return f.readBounded(response.Body)
}

func (f *HTTPFetcher) get(ctx context.Context, address string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "text/calendar")
	return f.client.Do(request)
}

// readBounded refuses the file that is one byte past the limit instead of
// truncating it: a half-read calendar parses into a queue missing its tail,
// which looks like reservations that were withdrawn.
func (f *HTTPFetcher) readBounded(body io.Reader) (string, error) {
	content, err := io.ReadAll(io.LimitReader(body, f.limit+1))
	if err != nil {
		return "", ErrUnavailable
	}
	if int64(len(content)) > f.limit {
		return "", ErrMalformed
	}
	return string(content), nil
}

// checkRedirect refuses the credential smuggled through a hop as well as the one
// typed in the address: blocking it only at NormalizeFeedURL would leave a
// redirect free to reintroduce exactly what the guard was written to stop.
func checkRedirect(request *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects || request.URL.User != nil {
		return ErrUnavailable
	}
	if !strings.EqualFold(request.URL.Scheme, "https") {
		return ErrUnavailable
	}
	if !strings.EqualFold(request.URL.Host, via[0].URL.Host) {
		return ErrUnavailable
	}
	return nil
}
