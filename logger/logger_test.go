package logger

import (
	"bytes"
	"log/slog"
	"testing"

	stError "github.com/go-errors/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
)

type tWriter struct {
	testing.TB
}

func (t tWriter) Write(data []byte) (int, error) {
	t.Helper()

	t.Log(string(bytes.TrimSpace(data)))

	return len(data), nil
}

func TestLogger(t *testing.T) {
	t.Parallel()

	top := t.Context()
	Init("info", tWriter{TB: t})

	ctx := AddFieldsToCtx(top,
		slog.String("key", "value"),
		slog.String("key", "value2"),
		slog.String("key", "value3"))

	Info(ctx, "info message")
	Warn(ctx, "warn message")
}

func TestError(t *testing.T) {
	t.Parallel()

	top := t.Context()
	Init("info", tWriter{TB: t})

	ctx := AddFieldsToCtx(top,
		slog.String("key", "value"),
		slog.String("key", "value2"),
		slog.String("key", "value3"),
	)

	err := stError.New("test error")

	Error(ctx, "error message", err)
}

func TestLoggerWithTrace(t *testing.T) {
	t.Parallel()

	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	spCtx, span := otel.Tracer("").Start(t.Context(), "test")
	defer span.End()

	Init("info", tWriter{TB: t})

	logCtx := AddFieldsToCtx(spCtx,
		slog.String("key", "value"),
		slog.String("key", "value2"),
		slog.String("key", "value3"))

	Info(logCtx, "info message")
	Warn(logCtx, "warn message")
}
