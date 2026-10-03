package dto

import (
	"time"
	"uuid"

	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/errors"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/model"
)

type EmployeeResponse struct {
	ID        string `json:"id"`
	FIO       string `json:"fio"`
	BirthDate string `json:"birth_date"`
	IsFired   bool   `json:"is_fired"`
}

func ToEmployee(er EmployeeResponse) (model.Employee, error) {
	id, err := uuid.Parse(er.ID)
	if err != nil {
		return model.Employee{}, errors.ErrParseEmployee
	}

	fio, err := model.NewFIO(er.FIO)
	if err != nil {
		return model.Employee{}, errors.ErrParseEmployee
	}

	birthDate, err := time.Parse(time.RFC3339, er.BirthDate)
	if err != nil {
		return model.Employee{}, errors.ErrParseEmployee
	}
	if birthDate.IsZero() || birthDate.After(time.Now().AddDate(-14, 0, 0)) {
		return model.Employee{}, errors.ErrParseEmployee
	}

	return model.Employee{
		ID:        id,
		FIO:       fio,
		BirthDate: birthDate,
		IsFired:   er.IsFired,
	}, nil
}

func ToEmployees(es []EmployeeResponse) ([]model.Employee, error) {
	ers := make([]model.Employee, 0, len(es))

	for _, e := range es {
		emp, err := ToEmployee(e)
		if err != nil {
			return nil, err
		}

		ers = append(ers, emp)
	}

	return ers, nil
}
