package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Pantani/cumuru/apps/api/internal/platform/application"
	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
)

var (
	version  string
	revision string
	builtAt  string
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	build, err := application.ParseBuild(version, revision, builtAt)
	if err != nil {
		logger.Error("build metadata rejected", "error_code", "build_metadata_invalid")
		os.Exit(1)
	}
	cfg, err := config.Load(config.ProcessWorker, os.LookupEnv)
	if err != nil {
		logger.Error("configuration rejected", "error", err.Error())
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := application.RunWorker(ctx, cfg, build, os.Stdout); err != nil {
		logger.Error("worker stopped", "error_code", "worker_runtime_failed")
		os.Exit(1)
	}
}
