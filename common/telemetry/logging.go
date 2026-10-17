package telemetry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"

	"go.opentelemetry.io/otel/trace"
)

var instanceID, _ = os.Hostname()

// InitLogging runs before configuration loading so startup failures are JSON too.
func InitLogging(service string) {
	slog.SetDefault(slog.New(logHandler(os.Stdout, service)))
}

func logHandler(output io.Writer, service string) slog.Handler {
	return traceHandler{slog.NewJSONHandler(output, nil).WithAttrs([]slog.Attr{slog.String("service", service), slog.String("instance_id", instanceID)})}
}

type traceHandler struct {
	slog.Handler
}

func (h traceHandler) Handle(ctx context.Context, record slog.Record) error {
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		record.AddAttrs(slog.String("trace_id", span.TraceID().String()), slog.String("span_id", span.SpanID().String()))
	}
	return h.Handler.Handle(ctx, record)
}

func (h traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceHandler{h.Handler.WithAttrs(attrs)}
}

func (h traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{h.Handler.WithGroup(name)}
}

// Recoverer keeps panic details in one JSON record instead of chi's stderr stack.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func(ctx context.Context) {
			if value := recover(); value != nil {
				if err, ok := value.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(value)
				}
				slog.ErrorContext(ctx, "http handler panic", "error", fmt.Sprint(value), "stack", string(debug.Stack()))
				w.WriteHeader(http.StatusInternalServerError)
			}
		}(r.Context())
		next.ServeHTTP(w, r)
	})
}
