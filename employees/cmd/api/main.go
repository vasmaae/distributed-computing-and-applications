package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/vasmaae/distributed-computing-and-applications/common/auth"
	appserver "github.com/vasmaae/distributed-computing-and-applications/common/server"
	"github.com/vasmaae/distributed-computing-and-applications/common/telemetry"
	"github.com/vasmaae/distributed-computing-and-applications/employees/docs"
	fc "github.com/vasmaae/distributed-computing-and-applications/employees/internal/client/file"
	rc "github.com/vasmaae/distributed-computing-and-applications/employees/internal/client/report"
	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/config"
	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/handler"
	eh "github.com/vasmaae/distributed-computing-and-applications/employees/internal/handler/employee"
	rh "github.com/vasmaae/distributed-computing-and-applications/employees/internal/handler/report"
	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/migrations"
	er "github.com/vasmaae/distributed-computing-and-applications/employees/internal/repository/employee"
	es "github.com/vasmaae/distributed-computing-and-applications/employees/internal/service/employee"
)

// main
//
//	@title						Employee API
//	@version					1.0
//	@description				API for employee management
//	@BasePath					/api/v1
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
func main() {
	telemetry.InitLogging("employees-service")
	cfg := config.MustLoad()
	ctx := context.Background()

	if err := run(ctx, cfg); err != nil { //nolint:staticcheck
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *config.Config) error { //nolint:staticcheck
	observer, shutdown, err := telemetry.New(ctx, "employees-service")
	if err != nil {
		return err
	}
	defer shutdown()
	authenticator, err := auth.New(cfg.Auth.Secret)
	if err != nil {
		return err
	}
	if err := migrate(ctx, cfg.DSN()); err != nil {
		return fmt.Errorf("failed to migrate: %w", err)
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	repo := er.NewRepository(db)
	svc := es.NewService(repo)
	employeeHandler := eh.NewHandler(svc)

	client := rc.NewClient(cfg.ReportsClientURL())
	filesClient := fc.NewClient(cfg.FilesClientURL())
	reportHandler := rh.NewHandler(client, filesClient)

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(observer.Middleware)
	router.Use(telemetry.Recoverer)
	router.Post("/auth/login", authenticator.LoginHandler(cfg.Login.Username, cfg.Login.Password))
	router.Post("/api/v1/auth/login", authenticator.LoginHandler(cfg.Login.Username, cfg.Login.Password))
	router.Handle("/metrics", observer.Metrics())
	router.Group(func(r chi.Router) {
		r.Use(authenticator.Middleware)
		handler.RegisterRoutes(r, employeeHandler, reportHandler)
	})

	docs.SwaggerInfo.BasePath = cfg.HTTP.SwaggerPrefix + docs.SwaggerInfo.BasePath
	router.Handle("/swagger/*",
		httpSwagger.Handler(
			httpSwagger.URL("/swagger/doc.json"),
		),
	)

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.HTTP.Host, cfg.HTTP.Port),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  10 * time.Second,
	}
	return appserver.ListenAndServe(ctx, server)
}

func migrate(ctx context.Context, dsn string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	defer conn.Close(ctx) //nolint:gosec,errcheck

	if err := migrations.MigratePG(ctx, conn); err != nil {
		return err
	}

	return nil
}
