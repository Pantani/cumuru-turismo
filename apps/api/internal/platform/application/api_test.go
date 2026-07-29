package application

import (
	"context"
	"testing"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/store"
)

func TestOpenAnalyticsServicesIsDisabledWithoutOpeningPublicPool(t *testing.T) {
	t.Parallel()

	publicReader, qualityReader, closeDatabase, err := openAnalyticsServices(
		context.Background(),
		config.Config{},
		&store.Store{},
	)
	if err != nil {
		t.Fatalf("openAnalyticsServices() error = %v", err)
	}
	if publicReader != nil || qualityReader != nil {
		t.Fatalf("readers = %#v %#v", publicReader, qualityReader)
	}
	closeDatabase()
}
