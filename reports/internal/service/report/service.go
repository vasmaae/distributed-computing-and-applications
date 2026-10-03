package report

import (
	"context"

	handler "github.com/vasmaae/distributed-computing-and-applications/reports/internal/handler/report"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/model"
)

type Repository interface {
	GetAll(ctx context.Context) ([]model.Employee, error)
}

var _ handler.Service = (*Service)(nil)

type Service struct {
	r Repository
}

func NewService(r Repository) *Service {
	return &Service{r: r}
}

func (s *Service) GetEmployeesReport(ctx context.Context) (model.Report, error) {
	employees, err := s.r.GetAll(ctx)
	if err != nil {
		return model.Report{}, err
	}

	return model.NewReport(employees), nil
}
