package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	ec "github.com/vasmaae/distributed-computing-and-applications/reports/internal/client/employee"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/config"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/handler"
	rh "github.com/vasmaae/distributed-computing-and-applications/reports/internal/handler/report"
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
	client := ec.NewClient(cfg.EmployeesClientURL())
	svc := rs.NewService(client)
	h := rh.NewHandler(svc)

	log.Println("started")

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	handler.RegisterRoutes(router, h)

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
