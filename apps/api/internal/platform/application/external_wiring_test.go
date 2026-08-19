package application

import (
	"os"
	"strings"
	"testing"
)

// Egress belongs to the worker and to nothing else. The API process must not be
// able to build a fetcher, an ingestion cycle or the ingestion pool, because
// the moment it can, a future change can call it from a request path and hand
// the upstream the cadence of the panel (ADR-045 §6).
func TestOnlyTheWorkerBuildsTheExternalEgress(t *testing.T) {
	t.Parallel()

	egress := []string{
		"external.NewFetcher",
		"external.NewIngestion",
		"store.OpenExternalPool",
		"store.NewExternalIngestionRepository",
	}
	assertAbsent(t, "api.go", egress)
	assertPresent(t, "worker.go", egress)
}

// The API reaches the layer through the public pool alone, so the ingestion DSN
// never has to exist in that process.
func TestAPINeverReadsTheIngestionDSN(t *testing.T) {
	t.Parallel()

	assertAbsent(t, "api.go", []string{"ExternalContext.DatabaseURL"})
}

func assertAbsent(t *testing.T, name string, needles []string) {
	t.Helper()
	body := readSource(t, name)
	for _, needle := range needles {
		if strings.Contains(body, needle) {
			t.Fatalf("%s builds %s", name, needle)
		}
	}
}

func assertPresent(t *testing.T, name string, needles []string) {
	t.Helper()
	body := readSource(t, name)
	for _, needle := range needles {
		if !strings.Contains(body, needle) {
			t.Fatalf("%s no longer builds %s", name, needle)
		}
	}
}

func readSource(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("%s not readable: %v", name, err)
	}
	return string(body)
}
