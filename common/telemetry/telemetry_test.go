package telemetry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestMetricsAndDistributedHTTPTrace(t *testing.T) {
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer collector.Close()
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", collector.URL)
	observer, shutdown, err := New(t.Context(), "test-service")
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown()
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSyncer(exporter))
	defer func() {
		if err := provider.Shutdown(t.Context()); err != nil {
			t.Error(err)
		}
	}()
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("traceparent"), "12345678901234567890123456789012") {
			t.Error("trace context missing on internal HTTP call")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	router := chi.NewRouter()
	router.Use(observer.Middleware)
	router.Get("/files/{id}", func(w http.ResponseWriter, r *http.Request) {
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstream.URL, nil)
		if err != nil {
			t.Error(err)
			return
		}
		resp, err := HTTPClient().Do(req)
		if err != nil {
			t.Error(err)
			return
		}
		defer resp.Body.Close() //nolint:errcheck
		w.WriteHeader(http.StatusNoContent)
	})
	r := httptest.NewRequest(http.MethodGet, "/files/one", nil)
	r.Header.Set("traceparent", "00-12345678901234567890123456789012-1234567890123456-01")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	metrics := httptest.NewRecorder()
	observer.Metrics().ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusNoContent || !strings.Contains(metrics.Body.String(), `route="/files/{id}",status="204"`) {
		t.Fatalf("status=%d; route metrics missing", w.Code)
	}
	if len(exporter.GetSpans()) != 2 {
		t.Fatalf("expected server and client spans: %d", len(exporter.GetSpans()))
	}
}

func TestHTTPClientDoesNotFollowRedirects(t *testing.T) {
	server := httptest.NewServer(http.RedirectHandler("http://other-service.example", http.StatusFound))
	defer server.Close()
	resp, err := HTTPClient().Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusFound {
		t.Fatal("internal client followed a redirect")
	}
}
