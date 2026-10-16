package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// ListenAndServe drains HTTP requests on termination so deferred trace exporters can flush.
func ListenAndServe(ctx context.Context, s *http.Server) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	defer s.Close() //nolint:errcheck
	errors := make(chan error, 1)
	go func() { errors <- s.ListenAndServe() }()
	select {
	case err := <-errors:
		return err
	case <-ctx.Done():
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return s.Shutdown(ctx)
	}
}
