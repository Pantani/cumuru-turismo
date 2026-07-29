package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

func Serve(ctx context.Context, listener net.Listener, httpServer *http.Server, shutdownTimeout time.Duration, logger *slog.Logger) error {
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-ctx.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownContext); err != nil && logger != nil {
			logger.Error("http shutdown failed", "error_code", "shutdown_failed")
		}
	}()

	err := httpServer.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	if ctx.Err() != nil {
		<-stopped
	}
	return err
}
