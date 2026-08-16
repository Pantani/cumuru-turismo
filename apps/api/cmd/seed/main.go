package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/seed"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.LoadSeed(os.LookupEnv)
	if err != nil {
		logger.Error(
			"seed configuration rejected",
			"error_code",
			"seed_config_invalid",
			"reason",
			err.Error(),
		)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()
	if err := seed.Run(ctx, cfg, os.Stdout); err != nil {
		// The reason is safe to log: every error the seeder builds names the step
		// that failed and never carries a credential, and an operator debugging a
		// failed bootstrap needs to know which step it was.
		logger.Error(
			"seed failed",
			"error_code",
			"seed_failed",
			"reason",
			err.Error(),
		)
		os.Exit(1)
	}
}
