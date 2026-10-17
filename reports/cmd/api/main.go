package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vasmaae/distributed-computing-and-applications/common/auth"
	appserver "github.com/vasmaae/distributed-computing-and-applications/common/server"
	"github.com/vasmaae/distributed-computing-and-applications/common/telemetry"
	ec "github.com/vasmaae/distributed-computing-and-applications/reports/internal/client/employee"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/config"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/handler"
	rh "github.com/vasmaae/distributed-computing-and-applications/reports/internal/handler/report"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/migrations"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/outbox"
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
	telemetry.InitLogging("reports-service")
	cfg := config.MustLoad()

	if err := run(cfg); err != nil { //nolint:staticcheck
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config) error { //nolint:staticcheck
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	observer, shutdown, err := telemetry.New(ctx, "reports-service")
	if err != nil {
		return err
	}
	defer shutdown()
	authenticator, err := auth.New(cfg.Auth.Secret)
	if err != nil {
		return err
	}
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return err
	}
	poolConfig.MaxConns = 4
	db, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return err
	}
	defer db.Close()
	conn, err := db.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("connect reports database: %w", err)
	}
	err = migrations.MigratePG(ctx, conn.Conn())
	conn.Release()
	if err != nil {
		return fmt.Errorf("migrate reports database: %w", err)
	}
	client := ec.NewClient(cfg.EmployeesClientURL())
	publisher := kafka.NewPublisher(cfg.Queue.Brokers, cfg.Queue.Topic)
	defer publisher.Close() //nolint:errcheck
	store := outbox.New(db)
	workerCtx, cancelWorker := context.WithCancel(ctx)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		store.Run(workerCtx, publisher)
	}()
	defer func() {
		cancelWorker()
		<-workerDone
	}()
	svc := rs.NewService(client, store)
	h := rh.NewHandler(svc)

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(observer.Middleware)
	router.Use(telemetry.Recoverer)
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
	return appserver.ListenAndServe(ctx, server)
}
