package report

import (
	"context"

	"gorm.io/gorm"

	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/model"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/repository/entity"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/service/report"
)

var _ report.Repository = (*Repository)(nil)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r Repository) GetAll(ctx context.Context) ([]model.Employee, error) {
	var records []entity.EmployeeRecord

	err := r.db.
		WithContext(ctx).
		Find(&records).
		Error
	if err != nil {
		return []model.Employee{}, err
	}

	return entity.ToEmployees(records)
}
