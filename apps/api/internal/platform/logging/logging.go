package logging

import (
	"io"
	"log/slog"
)

func New(writer io.Writer, level string) *slog.Logger {
	var configured slog.Level
	switch level {
	case "debug":
		configured = slog.LevelDebug
	case "warn":
		configured = slog.LevelWarn
	case "error":
		configured = slog.LevelError
	default:
		configured = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: configured}))
}
