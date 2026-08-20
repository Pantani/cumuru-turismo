package application

import (
	"context"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
)

func TestOpenAnalyticsServicesIsDisabledWithoutOpeningPublicPool(t *testing.T) {
	t.Parallel()

	readers, closeDatabase, err := openAnalyticsServices(
		context.Background(),
		config.Config{},
		&store.Store{},
	)
	if err != nil {
		t.Fatalf("openAnalyticsServices() error = %v", err)
	}
	if readers.analytics != nil || readers.quality != nil {
		t.Fatalf("readers = %#v %#v", readers.analytics, readers.quality)
	}
	// The external context rides the public pool, so it stays absent for the
	// same reason: with analytics off there is no public connection at all, and
	// GET /public/context is simply not registered.
	if readers.context != nil {
		t.Fatalf("context reader = %#v", readers.context)
	}
	closeDatabase()
}
