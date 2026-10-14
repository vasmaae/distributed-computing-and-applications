package dto

import (
	"time"

	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/model"
)

type ReportResponse struct {
	Employees []EmployeeResponse `json:"employees"`
	Total     int                `json:"total"`
	IsFired   int                `json:"is_fired"`
}

type EmployeeResponse struct {
	ID        string `json:"id"`
	FIO       string `json:"fio"`
	BirthDate string `json:"birth_date"`
	IsFired   bool   `json:"is_fired"`
}

func ToEmployeeResponse(e model.Employee) EmployeeResponse {
	return EmployeeResponse{
		ID:        e.ID.String(),
		FIO:       e.FIO.Value(),
		BirthDate: e.BirthDate.Format(time.RFC3339),
		IsFired:   e.IsFired,
	}
}

func ToEmployeeResponses(es []model.Employee) []EmployeeResponse {
	ers := make([]EmployeeResponse, 0, len(es))

	for _, e := range es {
		ers = append(ers, ToEmployeeResponse(e))
	}

	return ers
}

func ToReportResponse(e model.Report) ReportResponse {
	return ReportResponse{
		Employees: ToEmployeeResponses(e.Employees),
		Total:     e.Total,
		IsFired:   e.IsFired,
	}
}
