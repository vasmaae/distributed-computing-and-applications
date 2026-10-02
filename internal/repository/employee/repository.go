package employee

import (
	"context"
	"errors"
	"uuid"

	"gorm.io/gorm"

	errs "github.com/vasmaae/distributed-computing-and-applications/internal/errors"
	"github.com/vasmaae/distributed-computing-and-applications/internal/model"
	"github.com/vasmaae/distributed-computing-and-applications/internal/repository/entity"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r Repository) GetByID(ctx context.Context, id uuid.UUID) (model.Employee, error) {
	var record entity.EmployeeRecord

	err := r.db.
		WithContext(ctx).
		Where("id = ?", id).
		First(&record).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Employee{}, errs.ErrEmployeeNotFound
		}

		return model.Employee{}, err
	}

	return entity.ToEmployee(record)
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

func (r Repository) Create(ctx context.Context, employee model.Employee) error {
	return r.db.
		WithContext(ctx).
		Create(new(entity.ToEmployeeRecord(employee))).
		Error
}

func (r Repository) Update(ctx context.Context, employee model.Employee) error {
	record := entity.ToEmployeeRecord(employee)

	result := r.db.
		WithContext(ctx).
		Model(&entity.EmployeeRecord{}).
		Where("id = ?", record.ID).
		Updates(record)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errs.ErrEmployeeNotFound
	}

	return nil
}

func (r Repository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.
		WithContext(ctx).
		Model(&entity.EmployeeRecord{}).
		Where("id = ?", id).
		Update("is_fired", "true")

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errs.ErrEmployeeNotFound
	}

	return nil
}
