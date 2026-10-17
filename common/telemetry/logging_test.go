package telemetry

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/trace"
)

func TestJSONLoggingAndTraceContext(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(logHandler(&output, "test-service")))
	t.Cleanup(func() { slog.SetDefault(previous) })
	span := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1}, SpanID: trace.SpanID{2}, TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(t.Context(), span)
	slog.Default().With("component", "test").ErrorContext(ctx, "first\nsecond", "error", "line one\nline two")
	log.Print("legacy library message")
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected one JSON line per record: %q", output.String())
	}
	for i, line := range lines {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		if record["service"] != "test-service" || record["time"] == nil || record["msg"] == nil {
			t.Fatalf("missing common log fields: %v", record)
		}
		if i == 0 && (record["trace_id"] != span.TraceID().String() || record["span_id"] != span.SpanID().String() || record["level"] != "ERROR") {
			t.Fatalf("trace or level missing: %v", record)
		}
		if i == 1 && record["trace_id"] != nil {
			t.Fatal("startup/library log must not invent a trace ID")
		}
	}
}

func TestHTTPLoggingAndRecovery(t *testing.T) {
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer collector.Close()
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", collector.URL)
	observer, shutdown, err := New(t.Context(), "test-service")
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown()
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(logHandler(&output, "test-service")))
	t.Cleanup(func() { slog.SetDefault(previous) })
	router := chi.NewRouter()
	router.Use(observer.Middleware)
	router.Use(Recoverer)
	router.Get("/panic", func(http.ResponseWriter, *http.Request) { panic("boom\nsecond line") })
	router.Get("/bad", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadRequest) })
	router.Get("/ok", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	for path, status := range map[string]int{"/panic": 500, "/bad": 400, "/ok": 200} {
		r := httptest.NewRequest(http.MethodGet, path+"?password=secret-value", nil)
		r.Header.Set("Authorization", "Bearer secret-value")
		r.Header.Set("traceparent", "00-12345678901234567890123456789012-1234567890123456-01")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s: expected %d, got %d", path, status, w.Code)
		}
	}
	levels := map[string]string{"/panic": "ERROR", "/bad": "WARN", "/ok": "INFO"}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 4 || strings.Contains(output.String(), "secret-value") {
		t.Fatalf("missing log records or leaked credentials: %q", output.String())
	}
	for _, line := range lines {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		if record["trace_id"] != "12345678901234567890123456789012" {
			t.Fatalf("missing trace context: %v", record)
		}
		if route, ok := record["route"].(string); ok {
			if record["level"] != levels[route] {
				t.Fatalf("wrong HTTP log level: %v", record)
			}
		} else if record["stack"] == nil || record["level"] != "ERROR" {
			t.Fatalf("panic details missing: %v", record)
		}
	}
}

func TestRecovererPreservesHTTPAbort(t *testing.T) {
	defer func() {
		err, ok := recover().(error)
		if !ok || !errors.Is(err, http.ErrAbortHandler) {
			t.Fatal("HTTP abort must propagate to net/http")
		}
	}()
	handler := Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) }))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}
