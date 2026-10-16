package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/vasmaae/distributed-computing-and-applications/common/auth"
	appserver "github.com/vasmaae/distributed-computing-and-applications/common/server"
	"github.com/vasmaae/distributed-computing-and-applications/common/telemetry"
	"github.com/vasmaae/distributed-computing-and-applications/files/internal/config"
	"github.com/vasmaae/distributed-computing-and-applications/files/internal/handler"
	fh "github.com/vasmaae/distributed-computing-and-applications/files/internal/handler/file"
	"github.com/vasmaae/distributed-computing-and-applications/files/internal/queue/kafka"
	"github.com/vasmaae/distributed-computing-and-applications/files/internal/repository/s3"
	fs "github.com/vasmaae/distributed-computing-and-applications/files/internal/service/file"
)

// main
//
//	@title			Files API
//	@version		1.0
//	@description	API for CSV file storage
//	@BasePath		/api/v1
func main() {
	cfg := config.MustLoad()

	if err := run(cfg); err != nil { //nolint:staticcheck
		log.Fatal(err)
	}
}

func run(cfg *config.Config) error { //nolint:staticcheck
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observer, shutdown, err := telemetry.New(ctx, "files-service")
	if err != nil {
		return err
	}
	defer shutdown()
	authenticator, err := auth.New(cfg.Auth.Secret)
	if err != nil {
		return err
	}
	repo := s3.NewRepository(cfg.S3Client(), cfg.S3.Bucket)
	svc := fs.NewService(repo)
	h := fh.NewHandler(svc)

	log.Println("started")

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(observer.Middleware)
	router.Use(middleware.Recoverer)
	router.Handle("/metrics", observer.Metrics())
	router.Group(func(r chi.Router) {
		r.Use(authenticator.Middleware)
		handler.RegisterRoutes(r, h)
	})

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.HTTP.Host, cfg.HTTP.Port),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  10 * time.Second,
	}
	defer server.Close() //nolint:errcheck
	errors := make(chan error, 2)
	go func() { errors <- kafka.Consume(ctx, cfg.Queue.Brokers, cfg.Queue.Topic, cfg.Queue.Group, svc) }()
	go func() { errors <- appserver.ListenAndServe(ctx, server) }()
	err = <-errors
	cancel()
	<-errors
	return err
}
