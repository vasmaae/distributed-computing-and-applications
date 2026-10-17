package report

import (
	"context"
	"encoding/csv"
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/model"
)

type employeeRepository struct {
	employees []model.Employee
}

func (r employeeRepository) GetAll(context.Context) ([]model.Employee, error) {
	return r.employees, nil
}

type fileStorage struct {
	id  uuid.UUID
	csv string
	err error
}

func (s *fileStorage) Save(_ context.Context, report model.Report, body []byte) error {
	s.id = report.FileID
	s.csv = string(body)
	return s.err
}

func TestGetEmployeesReport(t *testing.T) {
	fio, err := model.NewFIO("Иванов Иван")
	if err != nil {
		t.Fatal(err)
	}
	employee := model.Employee{ID: uuid.New(), FIO: fio, BirthDate: time.Date(1990, 1, 2, 0, 0, 0, 0, time.UTC), IsFired: true}
	storage := &fileStorage{}
	svc := NewService(employeeRepository{[]model.Employee{employee}}, storage)
	report, err := svc.GetEmployeesReport(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(storage.csv)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if report.Total != 1 || report.IsFired != 1 || report.FileID == uuid.Nil() || report.FileID != storage.id {
		t.Fatalf("invalid report: %+v", report)
	}
	if len(rows) != 2 || rows[1][0] != employee.ID.String() || rows[1][1] != fio.Value() || rows[1][2] != "1990-01-02" || rows[1][3] != "true" {
		t.Fatalf("invalid CSV: %q", rows)
	}
	storage.err = errors.New("storage unavailable")
	if _, err := svc.GetEmployeesReport(t.Context()); !errors.Is(err, storage.err) {
		t.Fatalf("storage failure lost: %v", err)
	}
}

func TestEmptyReportHasCSVHeader(t *testing.T) {
	storage := &fileStorage{}
	report, err := NewService(employeeRepository{}, storage).GetEmployeesReport(t.Context())
	if err != nil || report.Total != 0 || storage.csv != "id,fio,birth_date,is_fired\n" {
		t.Fatalf("report=%+v csv=%q err=%v", report, storage.csv, err)
	}
}
