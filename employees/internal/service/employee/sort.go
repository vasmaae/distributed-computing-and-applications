package employee

import (
	"context"
	"fmt"
	"strings"
	"time"
	"uuid"

	"github.com/vasmaae/distributed-computing-and-applications/mergesort"

	errs "github.com/vasmaae/distributed-computing-and-applications/employees/internal/errors"
	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/model"
)

func (s *Service) GetSorted(ctx context.Context, sortBy, order string, threads int) ([]model.Employee, time.Duration, error) {
	// Bound per-request goroutines even when the database contains many employees.
	if threads < 1 || threads > 256 {
		return nil, 0, fmt.Errorf("%w: threads must be between 1 and 256", errs.ErrInvalidSort)
	}
	if order != "asc" && order != "desc" {
		return nil, 0, fmt.Errorf("%w: order must be asc or desc", errs.ErrInvalidSort)
	}

	var compare func(model.Employee, model.Employee) int
	var fioKeys map[uuid.UUID]string
	switch sortBy {
	case "id":
		compare = func(a, b model.Employee) int { return a.ID.Compare(b.ID) }
	case "birth_date":
		compare = func(a, b model.Employee) int { return a.BirthDate.Compare(b.BirthDate) }
	case "fio":
		compare = func(a, b model.Employee) int { return strings.Compare(fioKeys[a.ID], fioKeys[b.ID]) }
	default:
		return nil, 0, fmt.Errorf("%w: sort_by must be id, birth_date or fio", errs.ErrInvalidSort)
	}
	if order == "desc" {
		ascending := compare
		compare = func(a, b model.Employee) int { return ascending(b, a) }
	}

	employees, err := s.r.GetAll(ctx)
	if err != nil {
		return nil, 0, err
	}
	started := time.Now()
	if sortBy == "fio" {
		fioKeys = make(map[uuid.UUID]string, len(employees))
		for _, employee := range employees {
			fioKeys[employee.ID] = strings.ReplaceAll(strings.ToLower(strings.Join(strings.Fields(employee.FIO.Value()), " ")), "ё", "е")
		}
	}
	err = mergesort.Sort(employees, threads, compare)
	return employees, time.Since(started), err
}
