// Package db provides the PostgreSQL connection pool, schema migrations,
// and repository access for HiveStack Manager.
package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// DB wraps a PostgreSQL connection pool.
type DB struct {
	Pool *pgxpool.Pool
}

// Open opens a PostgreSQL connection pool for dsn and verifies connectivity.
func Open(dsn string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.MaxConnLifetime = time.Hour
	cfg.HealthCheckPeriod = 30 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &DB{Pool: pool}, nil
}

// Close closes the underlying connection pool.
func (d *DB) Close() error {
	d.Pool.Close()
	return nil
}

// Ping verifies the database connection is alive.
func (d *DB) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return d.Pool.Ping(ctx)
}

// Migrate applies all migrations found in dir (numbered *.sql files,
// applied in lexical order) that have not yet been recorded in the
// schema_migrations tracking table. If dir is empty, the migrations
// embedded at build time from internal/db/migrations/ are used.
//
// Each migration runs in its own transaction; a failure stops the run
// without recording that migration as applied, so re-running Migrate
// after fixing the issue resumes where it left off.
func (d *DB) Migrate(dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if _, err := d.Pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version     TEXT PRIMARY KEY,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	names, read, err := migrationFiles(dir)
	if err != nil {
		return err
	}

	applied := map[string]bool{}
	rows, err := d.Pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("query applied migrations: %w", err)
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return fmt.Errorf("scan applied migration: %w", err)
		}
		applied[v] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read applied migrations: %w", err)
	}

	for _, name := range names {
		if applied[name] {
			continue
		}
		sqlBytes, err := read(name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := d.Pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}
	return nil
}

// migrationFiles returns the sorted list of migration filenames and a
// reader function for their contents, sourced from dir on disk if given,
// or from the embedded migrations otherwise.
func migrationFiles(dir string) ([]string, func(name string) ([]byte, error), error) {
	if dir == "" {
		entries, err := fs.ReadDir(embeddedMigrations, "migrations")
		if err != nil {
			return nil, nil, fmt.Errorf("read embedded migrations: %w", err)
		}
		var names []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		read := func(name string) ([]byte, error) {
			return fs.ReadFile(embeddedMigrations, filepath.Join("migrations", name))
		}
		return names, read, nil
	}

	dirFS := os.DirFS(dir)
	entries, err := fs.ReadDir(dirFS, ".")
	if err != nil {
		return nil, nil, fmt.Errorf("read migrations dir %s: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	read := func(name string) ([]byte, error) {
		return fs.ReadFile(dirFS, name)
	}
	return names, read, nil
}
