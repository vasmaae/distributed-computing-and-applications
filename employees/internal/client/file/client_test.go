package file

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	errs "github.com/vasmaae/distributed-computing-and-applications/employees/internal/errors"
)

func TestGetAsyncDoesNotWaitForResponse(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-release:
			_, _ = w.Write([]byte("id,fio\n"))
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	future := NewClient(server.URL).GetAsync(ctx, uuid.New())
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("future did not start request")
	}
	select {
	case <-future:
		t.Fatal("future completed before the server responded")
	default:
	}
	close(release)
	result := <-future
	if result.Err != nil || string(result.Data) != "id,fio\n" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestGetAsyncMissingFileAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	client := NewClient(server.URL)
	if result := <-client.GetAsync(t.Context(), uuid.New()); !errors.Is(result.Err, errs.ErrFileNotFound) {
		t.Fatalf("404 lost: %+v", result)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	select {
	case result := <-client.GetAsync(ctx, uuid.New()):
		if !errors.Is(result.Err, context.Canceled) {
			t.Fatalf("cancellation lost: %v", result.Err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled future did not complete")
	}
}
