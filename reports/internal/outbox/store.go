package outbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/model"
)

type Publisher interface {
	Save(context.Context, uuid.UUID, io.Reader) error
}

type Store struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) Save(ctx context.Context, report model.Report, csv []byte) error {
	if report.FileID == uuid.Nil() || len(csv) == 0 || len(csv) > 8<<20 {
		return fmt.Errorf("report needs a file ID and CSV between 1 byte and 8 MiB")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	headers, err := json.Marshal(carrier)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, `INSERT INTO reports (file_id, total, is_fired) VALUES ($1, $2, $3)`, report.FileID.String(), report.Total, report.IsFired); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO report_outbox (file_id, csv, trace_headers) VALUES ($1, $2, $3)`, report.FileID.String(), csv, headers); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	slog.InfoContext(ctx, "queued report in outbox", "file_id", report.FileID.String())
	return nil
}

// Dispatch holds one row lock during a bounded publish; other instances skip that row.
func (s *Store) Dispatch(ctx context.Context, publisher Publisher) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	var fileID string
	var csv, headers []byte
	var attempts int
	err = tx.QueryRow(ctx, `SELECT file_id::text, csv, trace_headers, attempts FROM report_outbox
		WHERE published_at IS NULL AND available_at <= now()
		ORDER BY available_at, file_id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&fileID, &csv, &headers, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	id, err := uuid.Parse(fileID)
	if err != nil {
		return false, err
	}
	carrier := propagation.MapCarrier{}
	if err := json.Unmarshal(headers, &carrier); err != nil {
		return false, err
	}
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
	ctx, span := otel.Tracer("reports").Start(ctx, "outbox deliver")
	defer span.End()
	if err := publisher.Save(ctx, id, bytes.NewReader(csv)); err != nil {
		span.RecordError(err)
		delay := time.Second << min(attempts, 6)
		if _, updateErr := tx.Exec(ctx, `UPDATE report_outbox SET attempts = attempts + 1,
			last_error = $2, available_at = clock_timestamp() + $3 * interval '1 second' WHERE file_id = $1`, fileID, err.Error(), min(delay.Seconds(), 60)); updateErr != nil {
			return true, updateErr
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return true, commitErr
		}
		slog.ErrorContext(ctx, "outbox publish failed; retry scheduled", "file_id", fileID, "attempt", attempts+1, "error", err)
		return true, err
	}
	// A crash before this commit can repeat delivery; keep the same ID and immutable CSV.
	if _, err := tx.Exec(ctx, `UPDATE report_outbox SET published_at = clock_timestamp(), csv = NULL,
		attempts = attempts + 1, last_error = NULL WHERE file_id = $1`, fileID); err != nil {
		return true, err
	}
	if err := tx.Commit(ctx); err != nil {
		return true, err
	}
	slog.InfoContext(ctx, "published outbox report", "file_id", fileID, "attempt", attempts+1)
	return true, nil
}

func (s *Store) Run(ctx context.Context, publisher Publisher) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		worked, err := s.Dispatch(ctx, publisher)
		if err == nil && worked {
			continue
		}
		if err != nil && ctx.Err() == nil {
			slog.ErrorContext(ctx, "outbox worker will retry", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
