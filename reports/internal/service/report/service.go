package report

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"
	"uuid"

	handler "github.com/vasmaae/distributed-computing-and-applications/reports/internal/handler/report"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/model"
)

type Repository interface {
	GetAll(ctx context.Context) ([]model.Employee, error)
}

type FileStorage interface {
	Save(ctx context.Context, id uuid.UUID, src io.Reader) error
}

var _ handler.Service = (*Service)(nil)

type Service struct {
	r     Repository
	files FileStorage
}

func NewService(r Repository, files FileStorage) *Service {
	return &Service{r: r, files: files}
}

func (s *Service) GetEmployeesReport(ctx context.Context) (model.Report, error) {
	employees, err := s.r.GetAll(ctx)
	if err != nil {
		return model.Report{}, err
	}

	report := model.NewReport(employees)
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write([]string{"id", "fio", "birth_date", "is_fired"}); err != nil {
		return model.Report{}, err
	}
	for _, e := range employees {
		if err := w.Write([]string{e.ID.String(), e.FIO.Value(), e.BirthDate.Format(time.DateOnly), strconv.FormatBool(e.IsFired)}); err != nil {
			return model.Report{}, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return model.Report{}, err
	}
	if buf.Len() > 8<<20 {
		return model.Report{}, fmt.Errorf("report exceeds 8 MiB")
	}
	report.FileID = uuid.New()
	if err := s.files.Save(ctx, report.FileID, &buf); err != nil {
		return model.Report{}, fmt.Errorf("save report: %w", err)
	}
	return report, nil
}
