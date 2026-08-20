package platform

import (
	"log/slog"
	"os"
)

// NewLogger builds the process-wide slog logger.
// Development uses human-readable text output, production uses JSON.
func NewLogger(env string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if env == "development" {
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}
