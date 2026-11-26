// Package dbtest gives tests a migrated Postgres. Both helpers skip the test when TEST_DATABASE_URL is not set.
//
// Tx is the default: the test runs on a transaction that is always rolled back, so nothing is committed and there is
// nothing to clean up. Pool is for the few tests that need committed data, because a second connection or a
// background goroutine has to see it (the HTTP tests with the publish worker). It uses a database of its own, named
// like TEST_DATABASE_URL's with the suffix _commit, and empties it after each test.
package dbtest

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sili-Sudoku/api/libs/db"
	"github.com/Sili-Sudoku/api/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const lockTable = "test_migrate_lock"

var (
	mu      sync.Mutex
	shared  *pgxpool.Pool // the pool of the rolled-back tests, open until the test binary ends
	commit  *pgxpool.Pool
	commitM sync.Mutex // one committed test at a time: they empty the same tables
)

// Tx begins a transaction that is rolled back when the test ends. Build the client under test on it. A client that
// opens its own transaction gets a savepoint. A statement that fails aborts the whole transaction (25P02), so do
// that last in a test, or inside a savepoint of its own.
func Tx(t *testing.T) db.DBTX {
	t.Helper()
	pool := sharedPool(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	return db.WrapTx(tx)
}

// Pool gives a pool on the _commit database, with the schema and no rows. What the test writes is really committed
// and emptied again when the test ends. Tests that use it do not run at the same time.
func Pool(t *testing.T) *db.Pool {
	t.Helper()
	url := testURL(t)
	ctx := context.Background()

	mu.Lock()
	defer mu.Unlock()
	if commit == nil {
		cfg, err := pgxpool.ParseConfig(url)
		if err != nil {
			t.Fatal(err)
		}

		name := cfg.ConnConfig.Database + "_commit"
		createDatabase(t, url, name)
		cfg.ConnConfig.Database = name
		commit, err = pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}

		migrate(t, commit)
	}
	commitM.Lock()
	empty(t, commit)
	t.Cleanup(func() {
		empty(t, commit)
		commitM.Unlock()
	})

	return db.Wrap(commit)
}

func testURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	return url
}

func sharedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := testURL(t)
	mu.Lock()
	defer mu.Unlock()
	if shared == nil {
		pool, err := pgxpool.New(context.Background(), url)
		if err != nil {
			t.Fatal(err)
		}

		migrate(t, pool)
		shared = pool
	}

	return shared
}

// migrate takes a row lock first. Test binaries of different packages start together, and CockroachDB has no advisory
// lock, so on an empty database they would run the same migration at the same time. The lock goes when the connection
// does, so a crashed run cannot leave it held. The retry covers the race on creating the lock table itself.
func migrate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var err error
	for range 5 {
		if err = migrateLocked(pool); err == nil {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal(err)
}

func migrateLocked(pool *pgxpool.Pool) error {
	ctx := context.Background()
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS ` + lockTable + ` (id INT PRIMARY KEY)`,
		`INSERT INTO ` + lockTable + ` (id) VALUES (1) ON CONFLICT DO NOTHING`,
	} {
		if _, err := pool.Exec(ctx, q); err != nil {
			return err
		}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, `SELECT id FROM `+lockTable+` WHERE id = 1 FOR UPDATE`); err != nil {
		return err
	}

	if err := db.Migrate(ctx, db.Wrap(pool), migrations.FS); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func createDatabase(t *testing.T, url, name string) {
	t.Helper()
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}

	defer pool.Close()
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, name).Scan(&exists); err != nil {
		t.Fatal(err)
	}

	if exists {
		return
	}
	if _, err := pool.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
}

func empty(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename NOT IN ('schema_migrations', '`+lockTable+`')`)
	if err != nil {
		t.Error(err)
		return
	}

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Error(err)
		}

		tables = append(tables, pgx.Identifier{name}.Sanitize())
	}
	rows.Close()
	if len(tables) == 0 {
		return
	}
	if _, err := pool.Exec(ctx, `TRUNCATE `+strings.Join(tables, ", ")+` CASCADE`); err != nil {
		t.Error(err)
	}
}
