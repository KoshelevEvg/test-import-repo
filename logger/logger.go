package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync/atomic"

	stError "github.com/go-errors/errors"
	"github.com/rs/zerolog"
	slogzerolog "github.com/samber/slog-zerolog/v2"
	"go.opentelemetry.io/otel/trace"
)

type logFieldsKey string

const (
	ctxFieldsKey logFieldsKey = "logFields"

	traceLogKey = "trace_id"
	spanLogKey  = "span_id"

	LevelFatal = slog.Level(12)
)

var logger atomic.Pointer[slog.Logger]

type stacker interface {
	Stack() []byte
}

type logFieldsGetter interface {
	LogFields() []slog.Attr
}

func Init(level string, out ...io.Writer) {
	if logger.Load() != nil {
		return
	}

	if len(out) == 0 {
		out = append(out, os.Stdout)
	}

	multi := io.MultiWriter(out...)
	zerologLogger := zerolog.New(multi)
	l := slog.New(
		slogzerolog.Option{
			Level:  parseLogLevel(level),
			Logger: &zerologLogger,
			AttrFromContext: []func(ctx context.Context) []slog.Attr{
				extractFieldsFromCtx,
			},
		}.NewZerologHandler(),
	)

	logger.Store(l)
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug", "DEBUG":
		return slog.LevelDebug
	case "info", "INFO":
		return slog.LevelInfo
	case "warn", "WARN":
		return slog.LevelWarn
	case "error", "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func extractFieldsFromCtx(ctx context.Context) []slog.Attr {
	attrs := GetFieldsFromCtx(ctx)

	sc := trace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		attrs = append(attrs,
			slog.String(traceLogKey, sc.TraceID().String()),
			slog.String(spanLogKey, sc.SpanID().String()),
		)
	}

	return attrs
}

func GetFieldsFromCtx(ctx context.Context) []slog.Attr {
	if fields, ok := ctx.Value(ctxFieldsKey).([]slog.Attr); ok {
		logAttrs := make([]slog.Attr, 0, len(fields))
		exist := make(map[string]struct{})

		for _, field := range fields {
			if _, ok = exist[field.Key]; ok {
				continue
			}

			exist[field.Key] = struct{}{}

			logAttrs = append(logAttrs, field)
		}

		return logAttrs
	}

	return []slog.Attr{}
}

func AddFieldsToCtx(parent context.Context, fields ...slog.Attr) context.Context {
	if len(fields) == 0 {
		return parent
	}

	old, _ := parent.Value(ctxFieldsKey).([]slog.Attr)
	old = append(old, fields...)

	return context.WithValue(parent, ctxFieldsKey, old)
}

func Debug(ctx context.Context, msg string, args ...any) {
	getLogger().DebugContext(ctx, msg, args...)
}

func Info(ctx context.Context, msg string, args ...any) {
	getLogger().InfoContext(ctx, msg, args...)
}

func Warn(ctx context.Context, msg string, args ...any) {
	getLogger().WarnContext(ctx, msg, args...)
}

func Error(ctx context.Context, msg string, err error, args ...any) {
	l := getLogger().With(args...)

	if err != nil {
		var stErr stacker
		if stError.As(err, &stErr) && stErr != nil {
			l = l.With(slog.String("stacktrace", string(stErr.Stack())))
		}

		var lf logFieldsGetter
		if stError.As(err, &lf) && lf != nil {
			ctx = AddFieldsToCtx(ctx, lf.LogFields()...)
		}

		l = l.With(slog.String("error", err.Error()))
	}

	l.ErrorContext(ctx, msg)
}

func Fatal(ctx context.Context, msg string, err error, args ...any) {
	l := getLogger().With(args...)

	if err != nil {
		var stErr stacker
		if stError.As(err, &stErr) && stErr != nil {
			l = l.With(slog.String("stacktrace", string(stErr.Stack())))
		}

		var lf logFieldsGetter
		if stError.As(err, &lf) && lf != nil {
			ctx = AddFieldsToCtx(ctx, lf.LogFields()...)
		}

		l = l.With(slog.String("error", err.Error()))
	}

	l.Log(ctx, LevelFatal, msg, slog.String("level", "fatal"))

	os.Exit(1)
}

func getLogger() *slog.Logger {
	if logger.Load() == nil {
		level := os.Getenv("SERVICE_LOG_LEVEL")
		if level == "" {
			level = "info"
		}

		Init(level)
	}

	return logger.Load()
}
