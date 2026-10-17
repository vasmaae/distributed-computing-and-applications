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
		return err
	}
	migrator, err := migrate.NewMigratorEx(ctx, conn, "schema_version", &migrate.MigratorOptions{})
	if err != nil {
		return err
	}
	if err := migrator.LoadMigrations(migFS); err != nil {
		return fmt.Errorf("load report migrations: %w", err)
	}
	return migrator.Migrate(ctx)
}
