package dto

import (
	"time"

	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/model"
)

type EmployeeRequest struct {
	FIO       string `json:"fio"`
	BirthDate string `json:"birth_date"`
}

type EmployeeResponse struct {
	ID        string `json:"id"`
	FIO       string `json:"fio"`
	BirthDate string `json:"birth_date"`
	IsFired   bool   `json:"is_fired"`
}

type SortEmployeesRequest struct {
	SortBy  string `json:"sort_by" enums:"id,birth_date,fio" example:"id" validate:"required"`
	Order   string `json:"order" enums:"asc,desc" example:"asc" validate:"required"`
	Threads int    `json:"threads" minimum:"1" maximum:"256" example:"5" validate:"required"`
}

type SortedEmployeesResponse struct {
	Employees      []EmployeeResponse `json:"employees"`
	SortDurationMS float64            `json:"sort_duration_ms" example:"0.127"`
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
