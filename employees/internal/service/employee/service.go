package employee

import (
	"context"
	"time"
	"uuid"

	handler "github.com/vasmaae/distributed-computing-and-applications/employees/internal/handler/employee"
	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/model"
)

type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (model.Employee, error)
	GetAll(ctx context.Context) ([]model.Employee, error)
	Create(ctx context.Context, employee model.Employee) error
	Update(ctx context.Context, employee model.Employee) error
	Delete(ctx context.Context, id uuid.UUID) error
}

var _ handler.Service = (*Service)(nil)

type Service struct {
	r Repository
}

func NewService(r Repository) *Service {
	return &Service{r: r}
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (model.Employee, error) {
	return s.r.GetByID(ctx, id)
}

func (s *Service) GetAll(ctx context.Context) ([]model.Employee, error) {
	return s.r.GetAll(ctx)
}

func (s *Service) Create(ctx context.Context, fio string, birthDate time.Time) (model.Employee, error) {
	validFio, err := model.NewFIO(fio)
	if err != nil {
		return model.Employee{}, err
	}

	employee, err := model.NewEmployee(validFio, birthDate)
	if err != nil {
		return model.Employee{}, err
	}

	return employee, s.r.Create(ctx, employee)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, fio string, birthDate time.Time) (model.Employee, error) {
	validFio, err := model.NewFIO(fio)
	if err != nil {
		return model.Employee{}, err
	}

	employee, err := model.NewEmployee(validFio, birthDate)
	if err != nil {
		return model.Employee{}, err
	}

	employee.ID = id

	return employee, s.r.Update(ctx, employee)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.r.Delete(ctx, id)
}
