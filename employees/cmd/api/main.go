package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/vasmaae/distributed-computing-and-applications/employees/docs"
	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/config"
	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/handler"
	eh "github.com/vasmaae/distributed-computing-and-applications/employees/internal/handler/employee"
	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/migrations"
	er "github.com/vasmaae/distributed-computing-and-applications/employees/internal/repository/employee"
	es "github.com/vasmaae/distributed-computing-and-applications/employees/internal/service/employee"
)

// main
//
//	@title			Employee API
//	@version		1.0
//	@description	API for employee management
//	@BasePath		/api/v1
func main() {
	cfg := config.MustLoad()
	ctx := context.Background()

	if err := run(ctx, cfg); err != nil { //nolint:staticcheck
		log.Fatal(err)
	}
}

func run(ctx context.Context, cfg *config.Config) error { //nolint:staticcheck
	if err := migrate(ctx, cfg.DSN()); err != nil {
		return fmt.Errorf("failed to migrate: %w", err)
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	repo := er.NewRepository(db)
	svc := es.NewService(repo)
	h := eh.NewHandler(svc)

	log.Println("started")

	router := chi.NewRouter()
	handler.RegisterRoutes(router, h)

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
	log.Fatal(server.ListenAndServe())

	return nil
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
