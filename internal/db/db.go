// Package db provides the database connection pool and schema management for HiveStack.
package db

import (
    "context"
    "fmt"
    "time"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps a pgxpool connection pool.
type DB struct {
    Pool *pgxpool.Pool
}

// Open creates a connection pool and verifies connectivity.
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

// Close closes the connection pool.
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

// Transaction executes fn within a database transaction.
func (d *DB) Transaction(ctx context.Context, fn func(pgx.Tx) error) error {
    tx, err := d.Pool.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx)
    if err := fn(tx); err != nil {
        return err
    }
    return tx.Commit(ctx)
}

// QueryRowContext executes a query expected to return at most one row.
func (d *DB) QueryRowContext(ctx context.Context, query string, args ...interface{}) pgx.Row {
    return d.Pool.QueryRow(ctx, query, args...)
}

// QueryContext executes a query returning multiple rows.
func (d *DB) QueryContext(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
    return d.Pool.Query(ctx, query, args...)
}

// ExecContext executes a query that doesn't return rows.
func (d *DB) ExecContext(ctx context.Context, query string, args ...interface{}) (any, error) {
    return d.Pool.Exec(ctx, query, args...)
}

// Begin starts a new transaction.
func (d *DB) Begin(ctx context.Context) (pgx.Tx, error) {
    return d.Pool.Begin(ctx)
}

// NewRecord creates a new database record returning its auto-generated ID.
func NewRecord(ctx context.Context, tx pgx.Tx, table string, fields map[string]interface{}) (string, error) {
    if len(fields) == 0 {
        return "", fmt.Errorf("no fields provided")
    }
    cols := make([]string, 0, len(fields))
    vals := make([]string, 0, len(fields))
    args := make([]interface{}, 0, len(fields))
    i := 1
    for col, val := range fields {
        cols = append(cols, col)
        vals = append(vals, fmt.Sprintf("$%d", i))
        args = append(args, val)
        i++
    }
    query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING id",
        table, joinStrings(cols, ","), joinStrings(vals, ","))
    var id string
    err := tx.QueryRow(ctx, query, args...).Scan(&id)
    if err != nil {
        return "", err
    }
    return id, nil
}

// joinStrings joins a slice of strings with a separator.
func joinStrings(parts []string, sep string) string {
    if len(parts) == 0 {
        return ""
    }
    result := parts[0]
    for _, p := range parts[1:] {
        result += sep + p
    }
    return result
}
