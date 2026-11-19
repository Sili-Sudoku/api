package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Sili-Sudoku/api/libs/dbtest"
	"github.com/Sili-Sudoku/api/services/community/model"
	"github.com/Sili-Sudoku/api/services/community/postgres"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
	"github.com/jackc/pgx/v5"
)

// The tests need TEST_DATABASE_URL and are skipped without it.
func newUser(t *testing.T, pool pgx.Tx, name string) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (username, password_hash) VALUES ($1, 'x') RETURNING id`, name).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func newPuzzle(t *testing.T, pool pgx.Tx, id string, owner int64) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO puzzles (id, body, status, delete_token_hash, owner_id) VALUES ($1, '{}', 'published', 'x', $2)`, id, owner)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRating(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Tx(t)
	c := postgres.NewRating(pool)
	maker, ann, bob := newUser(t, pool, "maker"), newUser(t, pool, "ann"), newUser(t, pool, "bob")
	newPuzzle(t, pool, "a", maker)

	if r, err := c.Find(ctx, "a", ann); err != nil || r != nil {
		t.Fatalf("no rating yet: %+v %v", r, err)
	}
	// Stars and felt difficulty are separate; a user has one rating, and a new one replaces it.
	_ = c.Put(ctx, "a", ann, model.Rating{Stars: 5, Difficulty: 4})
	_ = c.Put(ctx, "a", bob, model.Rating{Stars: 3})
	st, err := c.Stats(ctx, "a")
	if err != nil || st != (puzzles.RatingStats{StarsCount: 2, StarsSum: 8, DifficultyCount: 1, DifficultySum: 4}) {
		t.Fatalf("stats: %+v %v", st, err)
	}
	_ = c.Put(ctx, "a", bob, model.Rating{Stars: 4, Difficulty: 2})
	st, _ = c.Stats(ctx, "a")
	if st.StarsSum != 9 || st.DifficultyCount != 2 || st.DifficultySum != 6 {
		t.Fatalf("replace: %+v", st)
	}
	if r, _ := c.Find(ctx, "a", bob); r == nil || *r != (model.Rating{Stars: 4, Difficulty: 2}) {
		t.Fatalf("mine: %+v", r)
	}
	// An empty rating removes the vote.
	_ = c.Put(ctx, "a", bob, model.Rating{})
	if st, _ = c.Stats(ctx, "a"); st.StarsCount != 1 {
		t.Fatalf("removed: %+v", st)
	}
	if r, _ := c.Find(ctx, "a", bob); r != nil {
		t.Fatalf("rating still there: %+v", r)
	}
}

func TestComment(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Tx(t)
	c := postgres.NewComment(pool)
	maker, ann, bob, cy := newUser(t, pool, "maker"), newUser(t, pool, "ann"), newUser(t, pool, "bob"), newUser(t, pool, "cyd")
	newPuzzle(t, pool, "a", maker)

	c1 := &model.Comment{PuzzleID: "a", UserID: ann, Body: "<b>hi</b>"}
	c2 := &model.Comment{PuzzleID: "a", UserID: bob, Body: "second"}
	c3 := &model.Comment{PuzzleID: "a", UserID: cy, Body: "third"}
	for _, cm := range []*model.Comment{c1, c2, c3} {
		if err := c.Create(ctx, cm); err != nil || cm.ID == 0 || cm.CreatedAt.IsZero() {
			t.Fatalf("create: %+v %v", cm, err)
		}
	}
	// The refused insert aborts the transaction, so it runs in a savepoint of its own.
	sp, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := postgres.NewComment(sp).Create(ctx, &model.Comment{PuzzleID: "a", UserID: ann, Body: strings.Repeat("x", model.MaxComment+1)}); err == nil {
		t.Fatal("the table must refuse a long body")
	}
	if err := sp.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if got, err := c.Find(ctx, c1.ID); err != nil || got == nil || got.Body != "<b>hi</b>" || got.Moderation != puzzles.ModNone {
		t.Fatalf("find: %+v %v", got, err)
	}
	if got, err := c.Find(ctx, 9999); err != nil || got != nil {
		t.Fatalf("find missing: %+v %v", got, err)
	}

	// Newest first, no overlap between pages.
	page, err := c.List(ctx, "a", nil, 2)
	if err != nil || len(page) != 2 || page[0].ID != c3.ID || page[1].ID != c2.ID {
		t.Fatalf("page 1: %+v %v", page, err)
	}
	page, _ = c.List(ctx, "a", &model.CommentCursor{CreatedAt: page[1].CreatedAt, ID: page[1].ID}, 2)
	if len(page) != 1 || page[0].ID != c1.ID {
		t.Fatalf("page 2: %+v", page)
	}

	// Reports: one per user, three hide the comment until it is cleared.
	for _, u := range []int64{bob, bob, cy, maker} {
		if _, err := c.AddReport(ctx, c1.ID, u); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := c.Find(ctx, c1.ID); got.Reports != 3 {
		t.Fatalf("one report per user: %d", got.Reports)
	}
	if page, _ = c.List(ctx, "a", nil, 10); len(page) != 2 {
		t.Fatalf("hidden after 3 reports: %d", len(page))
	}
	if err := c.SetModeration(ctx, c1.ID, puzzles.ModCleared); err != nil {
		t.Fatal(err)
	}
	if page, _ = c.List(ctx, "a", nil, 10); len(page) != 3 {
		t.Fatalf("cleared comment is back: %d", len(page))
	}
	if _, err := c.AddReport(ctx, 9999, bob); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("report missing: %v", err)
	}
	if err := c.SetModeration(ctx, 9999, puzzles.ModHidden); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("moderate missing: %v", err)
	}

	// Delete takes the reports with it.
	if err := c.Delete(ctx, c1.ID); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE kind = 'comment' AND target = $1`, fmt.Sprint(c1.ID)).Scan(&n)
	if n != 0 {
		t.Fatalf("reports left behind: %d", n)
	}
	if err := c.Delete(ctx, c1.ID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
}

func TestFollow(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Tx(t)
	c := postgres.NewFollow(pool)
	maker, ann, bob := newUser(t, pool, "maker"), newUser(t, pool, "ann"), newUser(t, pool, "bob")

	// Idempotent, counted, one way, listed newest first.
	for range 2 {
		if err := c.Create(ctx, ann, maker); err != nil {
			t.Fatal(err)
		}
	}
	_ = c.Create(ctx, bob, maker)
	_ = c.Create(ctx, ann, bob)
	if ok, _ := c.Exists(ctx, ann, maker); !ok {
		t.Fatal("not following")
	}
	if ok, _ := c.Exists(ctx, maker, ann); ok {
		t.Fatal("follow is one way")
	}
	if fr, fg, _ := c.Counts(ctx, maker); fr != 2 || fg != 0 {
		t.Fatalf("maker counts: %d %d", fr, fg)
	}
	if fr, fg, _ := c.Counts(ctx, ann); fr != 0 || fg != 2 {
		t.Fatalf("ann counts: %d %d", fr, fg)
	}
	if ids, _ := c.List(ctx, ann, 10); len(ids) != 2 || ids[0] != bob || ids[1] != maker {
		t.Fatalf("following: %v", ids)
	}
	if ids, _ := c.List(ctx, ann, 1); len(ids) != 1 {
		t.Fatalf("limit: %v", ids)
	}
	_ = c.Delete(ctx, ann, maker)
	if err := c.Delete(ctx, ann, maker); err != nil {
		t.Fatal(err)
	}
	if ok, _ := c.Exists(ctx, ann, maker); ok {
		t.Fatal("still following")
	}
}
