package migrations

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/tern/v2/migrate"
)

//go:embed sql/*.sql
var migrationsFS embed.FS

func MigratePG(ctx context.Context, conn *pgx.Conn) error {
	migFS, err := fs.Sub(migrationsFS, "sql")
	if err != nil {
		return fmt.Errorf("failed to get migrations fs: %w", err)
	}

	migrator, err := migrate.NewMigratorEx(
		ctx,
		conn,
		"schema_version",
		&migrate.MigratorOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	if err := migrator.LoadMigrations(migFS); err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	if err := migrator.Migrate(ctx); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}
