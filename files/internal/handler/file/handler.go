package file

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"uuid"

	"github.com/go-chi/chi/v5"

	errs "github.com/vasmaae/distributed-computing-and-applications/files/internal/errors"
	"github.com/vasmaae/distributed-computing-and-applications/files/internal/handler/response"
)

type Service interface {
	Save(ctx context.Context, id uuid.UUID, src io.Reader) error
	Get(ctx context.Context, id uuid.UUID, dst io.Writer) error
}

type Handler struct {
	s Service
}

func NewHandler(s Service) *Handler {
	return &Handler{s: s}
}

func (h *Handler) Save(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid file id")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<20))
	if err != nil {
		response.WriteError(w, http.StatusRequestEntityTooLarge, "cannot read file (limit 8 MiB)")
		return
	}
	if len(body) == 0 {
		response.WriteError(w, http.StatusBadRequest, "empty file")
		return
	}
	if err := h.s.Save(r.Context(), id, bytes.NewReader(body)); err != nil {
		response.WriteError(w, http.StatusBadGateway, "failed to save file")
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid file id")
		return
	}
	var buf bytes.Buffer
	if err := h.s.Get(r.Context(), id, &buf); err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			response.WriteError(w, http.StatusNotFound, "file not found")
			return
		}
		response.WriteError(w, http.StatusBadGateway, "failed to read file")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+id.String()+`.csv"`)
	if _, err := w.Write(buf.Bytes()); err != nil {
		log.Printf("failed to write file: %v", err)
	}
}
