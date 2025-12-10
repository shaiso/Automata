package telemetry

import (
	"context"
	"log/slog"
	"os"
)

// LogLevel определяет уровень логирования из переменной окружения.
// Возможные значения: DEBUG, INFO, WARN, ERROR
// По умолчанию: INFO
func LogLevel() slog.Level {
	level := os.Getenv("LOG_LEVEL")
	switch level {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// SetupLogger инициализирует глобальный логгер.
//
// Формат вывода определяется переменной LOG_FORMAT:
//   - "json" (по умолчанию) — JSON формат для production
//   - "text" — человекочитаемый формат для разработки
func SetupLogger() *slog.Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level:     LogLevel(),
		AddSource: LogLevel() == slog.LevelDebug,
	}

	format := os.Getenv("LOG_FORMAT")
	if format == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}

// Ключи контекста для передачи данных в логгер.
type ctxKey string

const (
	// CtxLogger — ключ для логгера в контексте.
	CtxLogger ctxKey = "logger"
)

// WithLogger добавляет логгер в контекст.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, CtxLogger, logger)
}

// FromContext извлекает логгер из контекста.
// Если логгер не найден, возвращает глобальный.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(CtxLogger).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

// WithRunID возвращает логгер с добавленным run_id.
func WithRunID(logger *slog.Logger, runID string) *slog.Logger {
	return logger.With("run_id", runID)
}

// WithTaskID возвращает логгер с добавленным task_id.
func WithTaskID(logger *slog.Logger, taskID string) *slog.Logger {
	return logger.With("task_id", taskID)
}

// WithFlowID возвращает логгер с добавленным flow_id.
func WithFlowID(logger *slog.Logger, flowID string) *slog.Logger {
	return logger.With("flow_id", flowID)
}
