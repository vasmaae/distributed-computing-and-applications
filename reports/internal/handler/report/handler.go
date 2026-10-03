package report

import (
	"context"
	"errors"
	"net/http"

	errs "github.com/vasmaae/distributed-computing-and-applications/reports/internal/errors"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/handler"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/handler/dto"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/handler/response"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/model"
)

type Service interface {
	GetEmployeesReport(ctx context.Context) (model.Report, error)
}

var _ handler.ReportHandler = (*Handler)(nil)

type Handler struct {
	s Service
}

func NewHandler(s Service) *Handler {
	return &Handler{s: s}
}

// GetEmployeesReport
//
//	@Summary	Get employees report
//	@Tags		reports
//	@Produce	json
//	@Success	200	{array}		dto.ReportResponse
//	@Failure	500	{object}	response.ErrorResponse
//	@Router		/reports/employees-report [get]
func (h *Handler) GetEmployeesReport(w http.ResponseWriter, r *http.Request) {
	report, err := h.s.GetEmployeesReport(r.Context())
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrEmployeesClient):
			response.WriteError(w, http.StatusFailedDependency, "failed to fetch report")
		case errors.Is(err, errs.ErrParseEmployee):
			response.WriteError(w, http.StatusFailedDependency, "failed to parse employees")
		default:
			response.WriteError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	reportResponse := dto.ToReportResponse(report)
	response.WriteResponse(w, http.StatusOK, reportResponse)
}
