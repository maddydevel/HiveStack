package db

import (
	"context"
	"embed"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migrate applies all pending SQL migrations to the database.
// Migrations are tracked in a schema_migrations table so each migration
// runs at once. Migration files are read from the embedded filesystem
// and applied in lexicographic order (0001_*.sql, 0002_*.sql, ...).
func (d *DB) Migrate(ctx context.Context) error {
	if _, err := d.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	rows, err := d.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("query applied migrations: %w", err)
	}

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			rows.Close()
			return fmt.Errorf("scan migration version: %w", err)
		}
		applied[version] = true
	}
	rows.Close()

	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasSuffix(name, ".sql") && isMigrationFile(name) {
			files = append(files, name)
		}
	}
	sort.Strings(files)

	for _, file := range files {
		version := strings.TrimSuffix(file, filepath.Ext(file))
		if applied[version] {
			continue
		}

		content, err := migrationFS.ReadFile("migrations/" + file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}

		if err := d.Transaction(ctx, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(content)); err != nil {
				return fmt.Errorf("apply migration %s: %w", file, err)
			}
			if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
				return fmt.Errorf("record migration %s: %w", file, err)
			}
			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}

// isMigrationFile reports whether a filename looks like a valid migration
// (e.g. "0001_tenants_and_auth.sql").
func isMigrationFile(name string) bool {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if len(base) < 5 {
		return false
	}
	for _, c := range base[:4] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return base[4] == '_'
}
