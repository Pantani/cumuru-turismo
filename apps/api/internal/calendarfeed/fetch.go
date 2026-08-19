package calendarfeed

import (
	"context"
	"io"
	"net/http"
	"strings"
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
// body, a deadline, and no redirect that leaves the host the operator named.
type HTTPFetcher struct {
	client *http.Client
	limit  int64
}

func NewHTTPFetcher(timeout time.Duration, limit int64) *HTTPFetcher {
	if limit <= 0 {
		limit = DefaultFetchLimit
	}
	return &HTTPFetcher{
		client: &http.Client{Timeout: timeout, CheckRedirect: checkRedirect},
		limit:  limit,
	}
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

func checkRedirect(request *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
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
