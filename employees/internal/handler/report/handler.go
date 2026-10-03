package report

import (
	"context"
	"errors"
	"net/http"

	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/client/dto"
	errs "github.com/vasmaae/distributed-computing-and-applications/employees/internal/errors"
	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/handler/response"
)

type Client interface {
	GetEmployeesReport(ctx context.Context) (dto.ReportResponse, error)
}

type Handler struct {
	c Client
}

func NewHandler(c Client) *Handler {
	return &Handler{c: c}
}

// GetEmployeesReport
//
//	@Summary	Get employees reports
//	@Tags		reports
//	@Produce	json
//	@Success	200	{object}	dto.ReportResponse
//	@Failure	424	{object}	response.ErrorResponse
//	@Failure	500	{object}	response.ErrorResponse
//	@Router		/reports/employees-report [get]
func (h *Handler) GetEmployeesReport(w http.ResponseWriter, r *http.Request) {
	report, err := h.c.GetEmployeesReport(r.Context())
	if err != nil {
		if errors.Is(err, errs.ErrReportsClient) {
			response.WriteError(w, http.StatusFailedDependency, "failed to fetch report")
			return
		}
		response.WriteError(w, http.StatusFailedDependency, "internal server error")
		return
	}

	response.WriteResponse(w, http.StatusOK, report)
}
