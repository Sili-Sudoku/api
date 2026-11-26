package db

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeTx struct {
	pgx.Tx
	commitErr  error
	rolledBack *int
}

func (f fakeTx) Commit(context.Context) error   { return f.commitErr }
func (f fakeTx) Rollback(context.Context) error { *f.rolledBack++; return nil }

type fakeDB struct {
	commitErrs []error // one per Begin; nil = commit works
	begins     int
	rollbacks  int
}

func (f *fakeDB) Begin(context.Context) (pgx.Tx, error) {
	var err error
	if f.begins < len(f.commitErrs) {
		err = f.commitErrs[f.begins]
	}
	f.begins++

	return fakeTx{commitErr: err, rolledBack: &f.rollbacks}, nil
}

var conflict = fmt.Errorf("%w: %w", ErrConflict, &pgconn.PgError{Code: "40001"})

func TestInTx(t *testing.T) {
	tests := []struct {
		Name       string
		CommitErrs []error
		Fn         func(runs int) error
		WantErr    error
		WantRuns   int
	}{
		{Name: "commits", Fn: func(int) error { return nil }, WantRuns: 1},
		{Name: "fn_error_is_not_retried", Fn: func(int) error { return errors.New("boom") }, WantErr: errors.New("boom"), WantRuns: 1},
		{Name: "conflict_in_fn_is_retried", Fn: func(n int) error {
			if n < 3 {
				return fmt.Errorf("wrapped: %w", conflict)
			}
			return nil
		}, WantRuns: 3},
		{Name: "conflict_on_commit_is_retried", CommitErrs: []error{&pgconn.PgError{Code: "40001"}}, Fn: func(int) error { return nil }, WantRuns: 2},
		{Name: "gives_up", Fn: func(int) error { return conflict }, WantErr: ErrConflict, WantRuns: maxAttempts},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			d, runs := &fakeDB{commitErrs: tt.CommitErrs}, 0
			err := retry(context.Background(), d, func(DBTX) error { runs++; return tt.Fn(runs) })
			if (err == nil) != (tt.WantErr == nil) || (tt.WantErr != nil && !errors.Is(err, tt.WantErr) && err.Error() != tt.WantErr.Error()) {
				t.Fatalf("err = %v, want %v", err, tt.WantErr)
			}
			if runs != tt.WantRuns || d.begins != tt.WantRuns || d.rollbacks != tt.WantRuns {
				t.Fatalf("runs = %d, begins = %d, rollbacks = %d, want %d each", runs, d.begins, d.rollbacks, tt.WantRuns)
			}
		})
	}

	t.Run("stops_when_the_context_ends", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		runs := 0
		err := retry(ctx, &fakeDB{}, func(DBTX) error { runs++; cancel(); return conflict })
		if !errors.Is(err, conflict) || runs != 1 {
			t.Fatalf("err = %v, runs = %d", err, runs)
		}
	})
}

func TestMapErr(t *testing.T) {
	tests := []struct {
		Name string
		In   error
		Want error // nil = returned as it came
	}{
		{Name: "nil"},
		{Name: "no_rows", In: pgx.ErrNoRows, Want: ErrNoRows},
		{Name: "unique", In: &pgconn.PgError{Code: "23505"}, Want: ErrUniqueViolation},
		{Name: "conflict", In: &pgconn.PgError{Code: "40001"}, Want: ErrConflict},
		{Name: "wrapped_conflict", In: fmt.Errorf("x: %w", &pgconn.PgError{Code: "40001"}), Want: ErrConflict},
		{Name: "other", In: errors.New("other")},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			got := mapErr(tt.In)
			if tt.Want == nil && got != tt.In || tt.Want != nil && !errors.Is(got, tt.Want) {
				t.Fatalf("got %v, want %v", got, tt.Want)
			}
			var pgErr *pgconn.PgError
			if tt.Want != nil && tt.Want != ErrNoRows && !errors.As(got, &pgErr) {
				t.Fatal("the driver error must stay reachable")
			}
		})
	}
}
