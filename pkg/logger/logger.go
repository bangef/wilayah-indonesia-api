package logger

import (
	"log/slog"
	"os"
)

// InitLogger menginisialisasi JSON logger bawaan log/slog ke os.Stdout
// dengan atribut statis ber-kardinalitas rendah: service dan env.
func InitLogger(serviceName, env string) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)

	logger := slog.New(handler).With(
		slog.String("service", serviceName),
		slog.String("env", env),
	)

	slog.SetDefault(logger)

	return logger
}
