package telemetry

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/vasmaae/distributed-computing-and-applications/common/auth"
)

type Observer struct {
	service  string
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func New(ctx context.Context, service string) (*Observer, func(), error) {
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, nil, err
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(time.Second)),
		sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", service), attribute.String("service.instance.id", instanceID))),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	o := &Observer{
		service:  service,
		registry: prometheus.NewRegistry(),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_requests_total", Help: "HTTP requests by route and status."}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "HTTP request duration by route."}, []string{"method", "route"}),
	}
	o.registry.MustRegister(o.requests, o.duration, collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return o, func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := provider.Shutdown(ctx); err != nil {
			slog.Error("flush traces", "error", err)
		}
	}, nil
}

func (o *Observer) Metrics() http.Handler {
	return promhttp.HandlerFor(o.registry, promhttp.HandlerOpts{})
}

func (o *Observer) Middleware(next http.Handler) http.Handler {
	observed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		writer := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(writer, r)
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		method := r.Method
		switch method {
		case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
		default:
			method = "OTHER"
		}
		status := writer.Status()
		if status == 0 {
			status = http.StatusOK
		}
		elapsed := time.Since(started)
		o.requests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
		o.duration.WithLabelValues(method, route).Observe(elapsed.Seconds())
		span := trace.SpanFromContext(r.Context())
		span.SetName(method + " " + route)
		span.SetAttributes(attribute.String("http.route", route))
		level := slog.LevelInfo
		if status >= http.StatusInternalServerError {
			level = slog.LevelError
		} else if status >= http.StatusBadRequest {
			level = slog.LevelWarn
		}
		slog.Log(r.Context(), level, "http request", "method", method, "route", route, "status", status,
			"duration_ms", elapsed.Milliseconds())
	})
	traced := otelhttp.NewHandler(observed, o.service)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Service-Instance", instanceID)
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		traced.ServeHTTP(w, r)
	})
}

func HTTPClient() *http.Client {
	return &http.Client{
		Timeout:       10 * time.Second,
		Transport:     auth.Transport{Base: otelhttp.NewTransport(http.DefaultTransport)},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func LogError(ctx context.Context, message string, err error) {
	slog.ErrorContext(ctx, message, "error", err)
}
