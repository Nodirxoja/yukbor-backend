// Package db owns the Postgres connection pool and the migration runner.
//
// Migrations are applied by the one-shot `migrate` binary rather than by
// postgres' docker-entrypoint-initdb.d, which only ever runs against an empty
// data volume — that would mean `docker compose down -v` after every schema
// change. Here each file is applied exactly once, tracked in
// public.schema_migrations, so `make up` is safe to re-run.
package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a pool, retrying while postgres finishes booting. Services
// start in parallel with the database, so a few seconds of ECONNREFUSED is
// normal and must not crash-loop the container.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	cfg.MaxConns = 8
	cfg.MaxConnLifetime = time.Hour

	var lastErr error
	for attempt := 1; attempt <= 30; attempt++ {
		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				return pool, nil
			}
			pool.Close()
		}
		lastErr = err
		slog.Warn("waiting for postgres", "attempt", attempt, "err", err)
		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, fmt.Errorf("postgres unreachable after 30 attempts: %w", lastErr)
}

// MustConnect is the service-startup form: connect or die with a clear message.
func MustConnect(ctx context.Context, url string) *pgxpool.Pool {
	pool, err := Connect(ctx, url)
	if err != nil {
		panic(err)
	}
	return pool
}

// Migrate applies every *.sql file in fsys exactly once, in filename order.
// Each file runs inside its own transaction: a failing migration leaves the
// database on the last good version instead of half-applied.
func Migrate(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS public.schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)

	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}

	for _, name := range names {
		if applied[name] {
			continue
		}
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if err := applyOne(ctx, pool, name, string(body)); err != nil {
			return err
		}
		slog.Info("migration applied", "version", name)
	}
	return nil
}

func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx, `SELECT version FROM public.schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func applyOne(ctx context.Context, pool *pgxpool.Pool, name, body string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, body); err != nil {
		return fmt.Errorf("migration %s: %w", name, err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO public.schema_migrations (version) VALUES ($1)`, name); err != nil {
		return fmt.Errorf("record migration %s: %w", name, err)
	}
	return tx.Commit(ctx)
}

// ErrNotFound is the shared "no rows" sentinel stores return so handlers can
// map it to the contract's 404 codes without importing pgx.
var ErrNotFound = errors.New("not found")
