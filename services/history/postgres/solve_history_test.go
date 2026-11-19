package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Sili-Sudoku/api/libs/dbtest"
	"github.com/Sili-Sudoku/api/services/history/model"
	"github.com/Sili-Sudoku/api/services/history/postgres"
	"github.com/jackc/pgx/v5"
)

func ptr[T any](v T) *T { return &v }

// These tests need TEST_DATABASE_URL and are skipped without it.
func setup(t *testing.T) (pgx.Tx, *postgres.SolveHistory, func() int64) {
	pool := dbtest.Tx(t)
	var n int
	return pool, postgres.NewSolveHistory(pool), func() int64 {
		n++
		var id int64
		err := pool.QueryRow(context.Background(), `INSERT INTO users (username, password_hash) VALUES ($1, 'x') RETURNING id`,
			fmt.Sprintf("user_%d", n)).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
}

func TestSolveHistory(t *testing.T) {
	ctx := context.Background()
	_, c, newUser := setup(t)
	ann, bob := newUser(), newUser()
	d1, d2 := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)

	// The lower time wins and brings its date; a slower time changes nothing.
	e, err := c.Upsert(ctx, ann, model.Entry{PuzzleID: "p1", SolvedAt: &d2, Seconds: ptr(300)})
	if err != nil || *e.Seconds != 300 {
		t.Fatalf("first: %+v %v", e, err)
	}
	e, _ = c.Upsert(ctx, ann, model.Entry{PuzzleID: "p1", SolvedAt: &d1, Seconds: ptr(400)})
	if *e.Seconds != 300 || !e.SolvedAt.Equal(d2) {
		t.Fatalf("slower: %+v", e)
	}
	e, _ = c.Upsert(ctx, ann, model.Entry{PuzzleID: "p1", SolvedAt: &d1, Seconds: ptr(200)})
	if *e.Seconds != 200 || !e.SolvedAt.Equal(d1) {
		t.Fatalf("faster: %+v", e)
	}
	// An unknown date is filled in later.
	_, _ = c.Upsert(ctx, ann, model.Entry{PuzzleID: "p2", Seconds: ptr(50)})
	e, _ = c.Upsert(ctx, ann, model.Entry{PuzzleID: "p2", SolvedAt: &d2, Seconds: ptr(60)})
	if *e.Seconds != 50 || e.SolvedAt == nil || !e.SolvedAt.Equal(d2) {
		t.Fatalf("fill date: %+v", e)
	}
	_, _ = c.Upsert(ctx, bob, model.Entry{PuzzleID: "p1", Seconds: ptr(100)})

	// Paging: most recently changed first, no overlap.
	page, err := c.List(ctx, ann, nil, 1)
	if err != nil || len(page) != 1 || page[0].PuzzleID != "p2" {
		t.Fatalf("page 1: %+v %v", page, err)
	}
	page, _ = c.List(ctx, ann, &model.Cursor{UpdatedAt: page[0].UpdatedAt, PuzzleID: page[0].PuzzleID}, 5)
	if len(page) != 1 || page[0].PuzzleID != "p1" {
		t.Fatalf("page 2: %+v", page)
	}

	// Times of every user feed the solve statistics.
	secs, _ := c.SolveSeconds(ctx, "p1")
	if len(secs) != 2 {
		t.Fatalf("seconds: %v", secs)
	}

	// Deleting your history leaves other users alone.
	if err := c.DeleteAll(ctx, ann); err != nil {
		t.Fatal(err)
	}
	if page, _ = c.List(ctx, ann, nil, 10); len(page) != 0 {
		t.Fatalf("after delete: %+v", page)
	}
	if secs, _ = c.SolveSeconds(ctx, "p1"); len(secs) != 1 {
		t.Fatalf("bob's time is gone: %v", secs)
	}
}

func TestSolveHistory_Full(t *testing.T) {
	ctx := context.Background()
	pool, c, newUser := setup(t)
	user := newUser()
	// Fill the table in one statement: 5000 round trips would only slow the test down.
	_, err := pool.Exec(ctx, `INSERT INTO solve_history (user_id, puzzle_id) SELECT $1, 'p' || g FROM generate_series(1, $2) AS g`, user, model.MaxEntries)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Upsert(ctx, user, model.Entry{PuzzleID: "one-more"}); !errors.Is(err, model.ErrFull) {
		t.Fatalf("over the limit: %v", err)
	}
	if _, err := c.Upsert(ctx, user, model.Entry{PuzzleID: "p1", Seconds: ptr(9)}); err != nil {
		t.Fatalf("an existing entry still updates: %v", err)
	}
}
