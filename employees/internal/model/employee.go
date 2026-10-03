package model

import (
	"time"
	"uuid"

	"github.com/vasmaae/distributed-computing-and-applications/internal/errors"
)

type Employee struct {
	ID        uuid.UUID
	FIO       FIO
	BirthDate time.Time
	IsFired   bool
}

func NewEmployee(fio FIO, birthDate time.Time) (Employee, error) {
	if birthDate.IsZero() || birthDate.After(time.Now().AddDate(-14, 0, 0)) {
		return Employee{}, errors.ErrInvalidBirthDate
	}

	return Employee{
		ID:        uuid.NewV7(),
		FIO:       fio,
		BirthDate: birthDate,
	}, nil
}
