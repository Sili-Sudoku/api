// Package db opens Postgres and applies embedded migrations.
package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, url string) (*Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return &Pool{p: pool}, nil
}

// Migrate applies each *.sql file in fsys once, in name order, each in its own transaction.
// Where the database has them, an advisory lock stops two starting instances from migrating at the same time.
func Migrate(ctx context.Context, pool *Pool, fsys fs.FS) error {
	conn, err := pool.p.Acquire(ctx)
	if err != nil {
		return err
	}

	defer conn.Release()

	// CockroachDB has no advisory locks (undefined_function). Without one, two instances starting together can
	// still not corrupt anything: each migration is one transaction and schema_migrations has a primary key,
	// so the loser fails loudly and restarts.
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(727274)`); err != nil {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42883" {
			return err
		}
	} else {
		defer conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(727274)`) //nolint:errcheck
	}

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}

	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return err
	}

	sort.Strings(files)
	for _, name := range files {
		var done bool
		if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE name = $1)`, name).Scan(&done); err != nil {
			return err
		}

		if done {
			continue
		}
		sql, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}

		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", name, err)
		}

		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}

	return nil
}
