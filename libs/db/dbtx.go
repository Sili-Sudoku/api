package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The errors of the database, as the Postgres clients see them. The driver's own errors never leave this package: every
// method below turns them into these, and the original stays reachable with errors.Is and errors.As.
var (
	// ErrNoRows is a QueryRow that found nothing.
	ErrNoRows = errors.New("db: no rows")
	// ErrUniqueViolation is a write that breaks a unique constraint (SQLSTATE 23505).
	ErrUniqueViolation = errors.New("db: unique violation")
	// ErrConflict is a serialization failure (SQLSTATE 40001): CockroachDB asks the client to run the transaction again.
	// InTx does that, so a client only sees it when the attempts ran out.
	ErrConflict = errors.New("db: transaction conflict")
)

// mapErr turns a driver error into one of the errors above, keeping the original as the cause.
func mapErr(err error) error {
	if err == nil {
		return nil
	}

	kind := error(nil)
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		kind = ErrNoRows
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		kind = ErrUniqueViolation
	case errors.As(err, &pgErr) && pgErr.Code == "40001":
		kind = ErrConflict
	default:
		return err
	}

	return fmt.Errorf("%w: %w", kind, err)
}

// DBTX is what the Postgres clients need. It has no Begin and no Commit: a transaction is only opened with InTx, which
// retries it when CockroachDB asks (see InTx). The tx that fn gets is a DBTX too, so a client works the same on a pool and
// inside a transaction.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (Result, error)
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
	InTx(ctx context.Context, fn func(tx DBTX) error) error
}

// Result is what a statement without rows reports.
type Result interface {
	RowsAffected() int64
}

// Row is the one row of a QueryRow. Scan returns ErrNoRows when there is none.
type Row interface {
	Scan(dest ...any) error
}

// Rows is the result of a Query. Check Err after Next returns false, and Close when done.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

type row struct{ r pgx.Row }

func (this row) Scan(dest ...any) error { return mapErr(this.r.Scan(dest...)) }

type rows struct{ r pgx.Rows }

func (this rows) Next() bool             { return this.r.Next() }
func (this rows) Scan(dest ...any) error { return mapErr(this.r.Scan(dest...)) }
func (this rows) Err() error             { return mapErr(this.r.Err()) }
func (this rows) Close()                 { this.r.Close() }

// querier is what a pgx pool and a pgx transaction both have.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

func exec(ctx context.Context, q querier, sql string, args []any) (Result, error) {
	tag, err := q.Exec(ctx, sql, args...)
	if err != nil {
		return nil, mapErr(err)
	}

	return tag, nil
}

func query(ctx context.Context, q querier, sql string, args []any) (Rows, error) {
	r, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, mapErr(err)
	}

	return rows{r}, nil
}

var (
	_ DBTX = (*Pool)(nil)
	_ DBTX = Tx{}
)

// Pool is the connection pool of a program. Build the Postgres clients on it.
type Pool struct{ p *pgxpool.Pool }

// Wrap makes a Pool of a pgx pool. Programs use Open; a test that needs its own pool settings uses this.
func Wrap(p *pgxpool.Pool) *Pool { return &Pool{p: p} }

func (this *Pool) Close() { this.p.Close() }

func (this *Pool) Exec(ctx context.Context, sql string, args ...any) (Result, error) {
	return exec(ctx, this.p, sql, args)
}

func (this *Pool) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	return query(ctx, this.p, sql, args)
}

func (this *Pool) QueryRow(ctx context.Context, sql string, args ...any) Row {
	return row{this.p.QueryRow(ctx, sql, args...)}
}

// InTx runs fn in a transaction and commits it. CockroachDB runs at serializable isolation and answers a transaction that
// conflicts with another with SQLSTATE 40001 (on any statement, or on the commit): the whole transaction must then run
// again. InTx does that, up to maxAttempts times with a short pause, so fn must be safe to run again: no effects outside
// the transaction, and its results taken from the last run. Any other error rolls back and is returned as is.
func (this *Pool) InTx(ctx context.Context, fn func(tx DBTX) error) error {
	return retry(ctx, this.p, fn)
}

// Tx is a pgx transaction used as a DBTX. Everything the client does is in it. An InTx on it opens a savepoint and does
// not retry: only the outermost transaction can run again, so a conflict comes out of it for the outer InTx, if there is
// one. A test uses a savepoint to absorb a statement that fails.
type Tx struct{ tx pgx.Tx }

// WrapTx makes a DBTX of a pgx transaction that the caller owns, and commits or rolls back.
func WrapTx(tx pgx.Tx) Tx { return Tx{tx: tx} }

func (this Tx) Exec(ctx context.Context, sql string, args ...any) (Result, error) {
	return exec(ctx, this.tx, sql, args)
}

func (this Tx) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	return query(ctx, this.tx, sql, args)
}

func (this Tx) QueryRow(ctx context.Context, sql string, args ...any) Row {
	return row{this.tx.QueryRow(ctx, sql, args...)}
}

func (this Tx) InTx(ctx context.Context, fn func(tx DBTX) error) error { return run(ctx, this.tx, fn) }

type beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// maxAttempts is how many times InTx runs a transaction that CockroachDB asks to retry.
const maxAttempts = 5

func retry(ctx context.Context, b beginner, fn func(tx DBTX) error) error {
	var err error
	for attempt := 1; ; attempt++ {
		if err = run(ctx, b, fn); !errors.Is(err, ErrConflict) || attempt == maxAttempts {
			return err
		}

		select {
		case <-time.After(time.Duration(attempt) * 20 * time.Millisecond):
		case <-ctx.Done():
			return err
		}
	}
}

func run(ctx context.Context, b beginner, fn func(tx DBTX) error) error {
	tx, err := b.Begin(ctx)
	if err != nil {
		return mapErr(err)
	}

	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := fn(Tx{tx: tx}); err != nil {
		return err
	}

	return mapErr(tx.Commit(ctx))
}
