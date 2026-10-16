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
	ec "github.com/vasmaae/distributed-computing-and-applications/reports/internal/client/employee"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/config"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/handler"
	rh "github.com/vasmaae/distributed-computing-and-applications/reports/internal/handler/report"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/queue/kafka"
	rs "github.com/vasmaae/distributed-computing-and-applications/reports/internal/service/report"
)

// main
//
//	@title			Reports API
//	@version		1.0
//	@description	API for report management
//	@BasePath		/api/v1
func main() {
	cfg := config.MustLoad()

	if err := run(cfg); err != nil { //nolint:staticcheck
		log.Fatal(err)
	}
}

func run(cfg *config.Config) error { //nolint:staticcheck
	observer, shutdown, err := telemetry.New(context.Background(), "reports-service")
	if err != nil {
		return err
	}
	defer shutdown()
	authenticator, err := auth.New(cfg.Auth.Secret)
	if err != nil {
		return err
	}
	client := ec.NewClient(cfg.EmployeesClientURL())
	publisher := kafka.NewPublisher(cfg.Queue.Brokers, cfg.Queue.Topic)
	defer publisher.Close() //nolint:errcheck
	svc := rs.NewService(client, publisher)
	h := rh.NewHandler(svc)

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
	return appserver.ListenAndServe(context.Background(), server)
}
