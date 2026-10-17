package report

import (
	"context"
	"errors"
	"net/http"
	"time"
	"uuid"

	"github.com/go-chi/chi/v5"

	"github.com/vasmaae/distributed-computing-and-applications/common/telemetry"
	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/client/dto"
	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/client/file"
	errs "github.com/vasmaae/distributed-computing-and-applications/employees/internal/errors"
	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/handler/response"
)

type Client interface {
	GetEmployeesReport(ctx context.Context) (dto.ReportResponse, error)
}

type FileClient interface {
	GetAsync(ctx context.Context, id uuid.UUID) <-chan file.Result
}

type Handler struct {
	c     Client
	files FileClient
}

func NewHandler(c Client, files FileClient) *Handler {
	return &Handler{c: c, files: files}
}

// GetFile
//
//	@Summary	Download a stored CSV report through a Go future
//	@Tags		reports
//
//	@Security	BearerAuth
//
//	@Produce	text/csv
//	@Param		id	path		string	true	"File ID returned by the report endpoint"
//	@Success	200	{file}		file
//	@Failure	400	{object}	response.ErrorResponse
//	@Failure	404	{object}	response.ErrorResponse
//	@Failure	502	{object}	response.ErrorResponse
//	@Failure	504	{object}	response.ErrorResponse
//	@Router		/reports/files/{id} [get]
func (h *Handler) GetFile(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid file id")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	select {
	case <-ctx.Done():
		response.WriteError(w, http.StatusGatewayTimeout, "file request timed out")
	case result := <-h.files.GetAsync(ctx, id):
		if result.Err != nil {
			switch {
			case errors.Is(result.Err, errs.ErrFileNotFound):
				response.WriteError(w, http.StatusNotFound, "file not found or still queued")
			case errors.Is(result.Err, context.DeadlineExceeded):
				response.WriteError(w, http.StatusGatewayTimeout, "file request timed out")
			default:
				telemetry.LogError(ctx, "fetch stored report", result.Err)
				response.WriteError(w, http.StatusBadGateway, "failed to fetch file")
			}
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+id.String()+`.csv"`)
		if _, err := w.Write(result.Data); err != nil {
			telemetry.LogError(r.Context(), "write CSV", err)
		}
	}
}

// GetEmployeesReport
//
//	@Summary	Get employees reports
//	@Tags		reports
//
//	@Security	BearerAuth
//
//	@Produce	json
//	@Success	200	{object}	dto.ReportResponse
//	@Failure	424	{object}	response.ErrorResponse
//	@Failure	500	{object}	response.ErrorResponse
//	@Router		/reports/employees-report [get]
func (h *Handler) GetEmployeesReport(w http.ResponseWriter, r *http.Request) {
	report, err := h.c.GetEmployeesReport(r.Context())
	if err != nil {
		telemetry.LogError(r.Context(), "fetch report", err)
		if errors.Is(err, errs.ErrReportsClient) {
			response.WriteError(w, http.StatusFailedDependency, "failed to fetch report")
			return
		}
		response.WriteError(w, http.StatusFailedDependency, "internal server error")
		return
	}

	response.WriteResponse(w, http.StatusOK, report)
}
