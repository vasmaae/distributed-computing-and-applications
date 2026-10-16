package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type FileHandler interface {
	Save(w http.ResponseWriter, r *http.Request)
	Get(w http.ResponseWriter, r *http.Request)
}

func RegisterRoutes(r chi.Router, fileHandler FileHandler) {
	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/files", func(r chi.Router) {
			r.Put("/{id}", fileHandler.Save)
			r.Get("/{id}", fileHandler.Get)
		})
	})
}
