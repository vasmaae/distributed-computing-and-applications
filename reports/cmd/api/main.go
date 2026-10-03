package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/vasmaae/distributed-computing-and-applications/reports/docs"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/config"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/handler"
	rh "github.com/vasmaae/distributed-computing-and-applications/reports/internal/handler/report"
	rr "github.com/vasmaae/distributed-computing-and-applications/reports/internal/repository/report"
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
	ctx := context.Background()

	if err := run(ctx, cfg); err != nil { //nolint:staticcheck
		log.Fatal(err)
	}
}

func run(ctx context.Context, cfg *config.Config) error { //nolint:staticcheck
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	repo := rr.NewRepository(db)
	svc := rs.NewService(repo)
	h := rh.NewHandler(svc)

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
