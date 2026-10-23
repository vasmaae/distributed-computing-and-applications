package employee

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	errs "github.com/vasmaae/distributed-computing-and-applications/employees/internal/errors"
	"github.com/vasmaae/distributed-computing-and-applications/employees/internal/model"
)

type sortRepository struct {
	Repository
	employees []model.Employee
	err       error
	calls     int
}

func (r *sortRepository) GetAll(context.Context) ([]model.Employee, error) {
	r.calls++
	return slices.Clone(r.employees), r.err
}

func TestGetSorted(t *testing.T) {
	var employees []model.Employee
	for _, data := range []struct{ id, fio, birth string }{
		{"01900000-0000-7000-8000-000000000003", "Ёлкин  Иван", "1990-01-01"},
		{"01900000-0000-7000-8000-000000000001", "Елкин Иван", "1980-01-01"},
		{"01900000-0001-7000-8000-000000000000", "Яковлев Иван", "1990-01-01"},
		{"01900000-0000-7000-8000-000000000002", "Андреев Иван", "1990-01-01"},
	} {
		fio, err := model.NewFIO(data.fio)
		if err != nil {
			t.Fatal(err)
		}
		birth, err := time.Parse(time.DateOnly, data.birth)
		if err != nil {
			t.Fatal(err)
		}
		employees = append(employees, model.Employee{ID: uuid.MustParse(data.id), FIO: fio, BirthDate: birth})
	}
	for _, tc := range []struct {
		field, order string
		indices      []int
	}{
		{"id", "asc", []int{1, 3, 0, 2}},
		{"id", "desc", []int{2, 0, 3, 1}},
		{"birth_date", "asc", []int{1, 0, 2, 3}},
		{"birth_date", "desc", []int{0, 2, 3, 1}},
		{"fio", "asc", []int{3, 0, 1, 2}},
		{"fio", "desc", []int{2, 0, 1, 3}},
	} {
		t.Run(tc.field+"/"+tc.order, func(t *testing.T) {
			for _, threads := range []int{1, 2, 3, 5, 8, 256} {
				repo := &sortRepository{employees: employees}
				got, elapsed, err := NewService(repo).GetSorted(t.Context(), tc.field, tc.order, threads)
				if err != nil || elapsed <= 0 || repo.calls != 1 {
					t.Fatalf("elapsed=%s calls=%d err=%v", elapsed, repo.calls, err)
				}
				for i, index := range tc.indices {
					if got[i] != employees[index] {
						t.Fatalf("threads=%d index=%d: got %+v, want %+v", threads, i, got[i], employees[index])
					}
				}
			}
		})
	}
}

func TestGetSortedValidation(t *testing.T) {
	for _, tc := range []struct {
		field, order string
		threads      int
	}{
		{"", "asc", 5}, {"wrong", "asc", 5}, {"id", "", 5}, {"id", "up", 5},
		{"id", "asc", 0}, {"id", "asc", -1}, {"id", "asc", 257},
	} {
		repo := &sortRepository{}
		_, elapsed, err := NewService(repo).GetSorted(t.Context(), tc.field, tc.order, tc.threads)
		if !errors.Is(err, errs.ErrInvalidSort) || repo.calls != 0 || elapsed != 0 {
			t.Fatalf("options=%+v: elapsed=%s calls=%d err=%v", tc, elapsed, repo.calls, err)
		}
	}
}

func TestGetSortedEmptyAndRepositoryError(t *testing.T) {
	repo := &sortRepository{}
	got, _, err := NewService(repo).GetSorted(t.Context(), "fio", "asc", 5)
	if err != nil || len(got) != 0 {
		t.Fatalf("empty result=%v err=%v", got, err)
	}
	repo.err = errors.New("database unavailable")
	got, elapsed, err := NewService(repo).GetSorted(t.Context(), "id", "asc", 5)
	if !errors.Is(err, repo.err) || got != nil || elapsed != 0 {
		t.Fatalf("database failure: result=%v elapsed=%s err=%v", got, elapsed, err)
	}
}
