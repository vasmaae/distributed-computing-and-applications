package entity

import (
	"time"
	"uuid"

	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/model"
)

type EmployeeRecord struct {
	ID        uuid.UUID `gorm:"column:id;type:uuid;primaryKey"`
	FIO       string    `gorm:"column:fio;type:text;not null"`
	BirthDate time.Time `gorm:"column:birth_date;not null"`
	IsFired   bool      `gorm:"column:is_fired;not null"`
}

func (EmployeeRecord) TableName() string {
	return "employees"
}

func ToEmployeeRecord(e model.Employee) EmployeeRecord {
	return EmployeeRecord{
		ID:        e.ID,
		FIO:       e.FIO.Value(),
		BirthDate: e.BirthDate,
		IsFired:   e.IsFired,
	}
}

func ToEmployee(er EmployeeRecord) (model.Employee, error) {
	fio, err := model.NewFIO(er.FIO)
	if err != nil {
		return model.Employee{}, err
	}

	employee := model.Employee{
		ID:        er.ID,
		FIO:       fio,
		BirthDate: er.BirthDate,
		IsFired:   er.IsFired,
	}

	return employee, nil
}

func ToEmployees(ers []EmployeeRecord) ([]model.Employee, error) {
	employees := make([]model.Employee, 0, len(ers))
	for _, er := range ers {
		employee, err := ToEmployee(er)
		if err != nil {
			return nil, err
		}
		employees = append(employees, employee)
	}
	return employees, nil
}
