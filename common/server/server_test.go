package server

import (
	"context"
	"net/http"
	"testing"
)

func TestCanceledServerShutsDown(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := ListenAndServe(ctx, &http.Server{Addr: "127.0.0.1:0"}); err != nil {
		t.Fatal(err)
	}
}
