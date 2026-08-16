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
		// The reason is logged as a fixed code: the failure path must never echo
		// a value that could carry the bootstrap secret.
		logger.Error(
			"seed failed",
			"error_code",
			"seed_failed",
			"reason",
			"bootstrap_failed",
		)
		os.Exit(1)
	}
}
