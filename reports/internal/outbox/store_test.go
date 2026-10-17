package outbox

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/migrations"
	"github.com/vasmaae/distributed-computing-and-applications/reports/internal/model"
)

type publishFunc func(context.Context, uuid.UUID, io.Reader) error

func (f publishFunc) Save(ctx context.Context, id uuid.UUID, body io.Reader) error {
	return f(ctx, id, body)
}

func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to run PostgreSQL transaction tests")
	}
	base, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := pgx.Identifier{"outbox_test_" + uuid.New().String()}.Sanitize()
	if _, err := base.Exec(t.Context(), "CREATE SCHEMA "+schema); err != nil {
		base.Close()
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
		defer cancel()
		if _, err := base.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
		base.Close()
	})
	conn, err := db.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	err = migrations.MigratePG(t.Context(), conn.Conn())
	conn.Release()
	if err != nil {
		t.Fatal(err)
	}
	return New(db)
}

func execSQL(t *testing.T, store *Store, sql string) {
	t.Helper()
	if _, err := store.db.Exec(t.Context(), sql); err != nil {
		t.Fatal(err)
	}
}

func TestSaveRollsBackReportWhenOutboxInsertFails(t *testing.T) {
	store := testStore(t)
	execSQL(t, store, "ALTER TABLE report_outbox ADD CONSTRAINT reject_insert CHECK (false) NOT VALID")
	err := store.Save(t.Context(), model.Report{FileID: uuid.New()}, []byte("id,fio,birth_date,is_fired\n"))
	if err == nil {
		t.Fatal("expected outbox insert failure")
	}
	var count int
	if err := store.db.QueryRow(t.Context(), "SELECT count(*) FROM reports").Scan(&count); err != nil || count != 0 {
		t.Fatalf("report must roll back with its outbox row: count=%d err=%v", count, err)
	}
}

func TestRetryRetainsCSVAndTraceThenReleasesPayload(t *testing.T) {
	store := testStore(t)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	traceID, err := trace.TraceIDFromHex("1234567890abcdef1234567890abcdef")
	if err != nil {
		t.Fatal(err)
	}
	spanID, err := trace.SpanIDFromHex("1234567890abcdef")
	if err != nil {
		t.Fatal(err)
	}
	ctx := trace.ContextWithSpanContext(t.Context(), trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled}))
	report := model.Report{FileID: uuid.New()}
	const payload = "id,fio,birth_date,is_fired\n"
	if err := store.Save(ctx, report, []byte(payload)); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("Kafka unavailable")
	calls := 0
	publisher := publishFunc(func(ctx context.Context, id uuid.UUID, body io.Reader) error {
		calls++
		csv, err := io.ReadAll(body)
		if err != nil || string(csv) != payload || id != report.FileID || trace.SpanContextFromContext(ctx).TraceID() != traceID {
			t.Fatalf("retry changed immutable payload, ID or trace: err=%v", err)
		}
		if calls == 1 {
			timer := time.NewTimer(1100 * time.Millisecond)
			defer timer.Stop()
			select {
			case <-timer.C:
				return failure
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})
	if worked, err := store.Dispatch(t.Context(), publisher); !worked || !errors.Is(err, failure) {
		t.Fatalf("worked=%v err=%v", worked, err)
	}
	var attempts int
	var pending, scheduled bool
	var csv, lastError string
	err = store.db.QueryRow(t.Context(), `SELECT attempts, published_at IS NULL, available_at > now(), convert_from(csv, 'UTF8'), last_error FROM report_outbox`).Scan(&attempts, &pending, &scheduled, &csv, &lastError)
	if err != nil || attempts != 1 || !pending || !scheduled || csv != payload || lastError != failure.Error() {
		t.Fatalf("retry state not retained: attempts=%d pending=%v scheduled=%v err=%v", attempts, pending, scheduled, err)
	}
	execSQL(t, store, "UPDATE report_outbox SET available_at = now()")
	if worked, err := store.Dispatch(t.Context(), publisher); !worked || err != nil {
		t.Fatalf("retry: worked=%v err=%v", worked, err)
	}
	err = store.db.QueryRow(t.Context(), "SELECT attempts, published_at IS NOT NULL AND csv IS NULL AND last_error IS NULL FROM report_outbox").Scan(&attempts, &pending)
	if err != nil || attempts != 2 || !pending {
		t.Fatalf("publication state: attempts=%d err=%v", attempts, err)
	}
	if worked, err := store.Dispatch(t.Context(), publisher); worked || err != nil || calls != 2 {
		t.Fatalf("published row must not be selected again: calls=%d worked=%v err=%v", calls, worked, err)
	}
}

func TestPublishThenSQLFailureCanRepeatTheSameFile(t *testing.T) {
	store := testStore(t)
	report := model.Report{FileID: uuid.New()}
	if err := store.Save(t.Context(), report, []byte("id,fio,birth_date,is_fired\n")); err != nil {
		t.Fatal(err)
	}
	execSQL(t, store, "ALTER TABLE report_outbox ADD CONSTRAINT reject_sent CHECK (published_at IS NULL) NOT VALID")
	calls := 0
	publisher := publishFunc(func(_ context.Context, id uuid.UUID, _ io.Reader) error {
		calls++
		if id != report.FileID {
			t.Fatal("file ID changed during redelivery")
		}
		return nil
	})
	if worked, err := store.Dispatch(t.Context(), publisher); !worked || err == nil || calls != 1 {
		t.Fatalf("expected SQL failure after publishing: calls=%d worked=%v err=%v", calls, worked, err)
	}
	execSQL(t, store, "ALTER TABLE report_outbox DROP CONSTRAINT reject_sent")
	if worked, err := store.Dispatch(t.Context(), publisher); !worked || err != nil || calls != 2 {
		t.Fatalf("uncommitted row was not safely redelivered: calls=%d worked=%v err=%v", calls, worked, err)
	}
}

func TestConcurrentWorkersSkipLockedRows(t *testing.T) {
	store := testStore(t)
	for range 2 {
		if err := store.Save(t.Context(), model.Report{FileID: uuid.New()}, []byte("id,fio,birth_date,is_fired\n")); err != nil {
			t.Fatal(err)
		}
	}
	entered := make(chan uuid.UUID, 2)
	release := make(chan struct{})
	defer close(release)
	publisher := publishFunc(func(ctx context.Context, id uuid.UUID, _ io.Reader) error {
		entered <- id
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	done := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := store.Dispatch(t.Context(), publisher)
			done <- err
		}()
	}
	ids := map[uuid.UUID]bool{}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for range 2 {
		select {
		case id := <-entered:
			ids[id] = true
		case <-timer.C:
			t.Fatal("second worker blocked instead of skipping the first worker's row")
		}
	}
	if len(ids) != 2 {
		t.Fatal("workers selected the same row")
	}
	release <- struct{}{}
	release <- struct{}{}
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}
