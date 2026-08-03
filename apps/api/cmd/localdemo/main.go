package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/Pantani/cumuru/apps/api/internal/platform/localdemo"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.LoadLocalDemo(os.LookupEnv)
	if err != nil {
		logger.Error("local demo configuration rejected", "error_code", "local_demo_config_invalid")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()
	if err := localdemo.Run(ctx, cfg, os.Stdout); err != nil {
		logger.Error(
			"local demo failed",
			"error_code",
			"local_demo_failed",
			"stage",
			err.Error(),
		)
		os.Exit(1)
	}
}
