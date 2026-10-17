package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// ListenAndServe drains HTTP requests on termination so deferred trace exporters can flush.
func ListenAndServe(ctx context.Context, s *http.Server) error {
	if s.ErrorLog == nil {
		s.ErrorLog = slog.NewLogLogger(slog.Default().Handler(), slog.LevelError)
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	defer s.Close() //nolint:errcheck
	errors := make(chan error, 1)
	slog.InfoContext(ctx, "starting HTTP server", "address", s.Addr)
	go func() { errors <- s.ListenAndServe() }()
	select {
	case err := <-errors:
		return err
	case <-ctx.Done():
		slog.InfoContext(ctx, "stopping HTTP server")
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return s.Shutdown(ctx)
	}
}
