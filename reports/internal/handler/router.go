package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type ReportHandler interface {
	GetEmployeesReport(w http.ResponseWriter, r *http.Request)
}

func RegisterRoutes(r chi.Router, employeeHandler ReportHandler) {
	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/reports", func(r chi.Router) {
			r.Get("/employees-report", employeeHandler.GetEmployeesReport)
		})
	})
}
