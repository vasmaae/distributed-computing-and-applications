package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type EmployeeHandler interface {
	GetByID(w http.ResponseWriter, r *http.Request)
	GetAll(w http.ResponseWriter, r *http.Request)
	Create(w http.ResponseWriter, r *http.Request)
	Update(w http.ResponseWriter, r *http.Request)
	Delete(w http.ResponseWriter, r *http.Request)
}

type ReportHandler interface {
	GetEmployeesReport(w http.ResponseWriter, r *http.Request)
}

func RegisterRoutes(r chi.Router, employeeHandler EmployeeHandler, reportHandler ReportHandler) {
	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/employees", func(r chi.Router) {
			r.Get("/", employeeHandler.GetAll)
			r.Post("/", employeeHandler.Create)
			r.Put("/{id}", employeeHandler.Update)
			r.Get("/{id}", employeeHandler.GetByID)
			r.Delete("/{id}", employeeHandler.Delete)
		})
		r.Route("/reports", func(r chi.Router) {
			r.Get("/employees-report", reportHandler.GetEmployeesReport)
		})
	})
}
