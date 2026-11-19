package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Sili-Sudoku/api/libs/dbtest"
	"github.com/Sili-Sudoku/api/libs/validate"
	"github.com/Sili-Sudoku/api/services/puzzles/model"
	"github.com/Sili-Sudoku/api/services/puzzles/postgres"
	"github.com/jackc/pgx/v5"
)

// These tests need TEST_DATABASE_URL and are skipped without it.
func setup(t *testing.T) (*postgres.Puzzle, func() int64) {
	pool := dbtest.Tx(t)
	var n int
	return postgres.NewPuzzle(pool), func() int64 { return newUser(t, pool, &n) }
}

func newUser(t *testing.T, pool pgx.Tx, n *int) int64 {
	*n++
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (username, password_hash) VALUES ($1, 'x') RETURNING id`, fmt.Sprintf("user_%d", *n)).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

var clock = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

// puzzle is a pending puzzle; every call is a minute later than the one before.
func puzzle(id string, owner int64) *model.Puzzle {
	clock = clock.Add(time.Minute)
	return &model.Puzzle{ID: id, Body: []byte(fmt.Sprintf(`{"n":%q}`, id)), Status: model.Pending, Moderation: model.ModNone,
		OwnerID: owner, CreatedAt: clock, TokenHash: [32]byte{1},
		Title: "t-" + id, Author: "Ann", Difficulty: "easy", Tags: []string{"classic", "x"}, Variants: []string{"killer", "thermometer"}}
}

func publish(t *testing.T, c *postgres.Puzzle, p *model.Puzzle) {
	t.Helper()
	ctx := context.Background()
	if created, err := c.Create(ctx, p); err != nil || !created {
		t.Fatalf("create %s: %v %v", p.ID, created, err)
	}
	if err := c.SetVerdict(ctx, p.ID, model.Published, validate.Verdict{Valid: true, Solutions: "one"}); err != nil {
		t.Fatal(err)
	}
}

func TestPuzzle_CreateFindVerdict(t *testing.T) {
	ctx := context.Background()
	c, _ := setup(t)

	if p, err := c.Find(ctx, "missing"); p != nil || err != nil {
		t.Fatalf("find missing: %+v %v", p, err)
	}
	p := puzzle("a", 0)
	if created, err := c.Create(ctx, p); err != nil || !created {
		t.Fatalf("create: %v %v", created, err)
	}
	if created, _ := c.Create(ctx, puzzle("a", 0)); created {
		t.Fatal("a repeat create must change nothing")
	}
	got, err := c.Find(ctx, "a")
	if err != nil || got.Status != model.Pending || got.Verdict != nil || got.OwnerID != 0 {
		t.Fatalf("find pending: %+v %v", got, err)
	}
	if got.Title != "t-a" || got.Author != "Ann" || got.Difficulty != "easy" || len(got.Tags) != 2 ||
		len(got.Variants) != 2 || got.Variants[0] != "killer" || got.Variants[1] != "thermometer" || got.TokenHash != p.TokenHash {
		t.Fatalf("list fields: %+v", got)
	}
	if pending, _ := c.PendingIDs(ctx); len(pending) != 1 || pending[0] != "a" {
		t.Fatalf("pending ids: %v", pending)
	}
	if items, _ := c.List(ctx, model.Filter{Limit: 10}); len(items) != 0 {
		t.Fatal("a pending puzzle must not be listed")
	}

	if err := c.SetVerdict(ctx, "a", model.Published, validate.Verdict{Valid: true, Solutions: "one"}); err != nil {
		t.Fatal(err)
	}
	if got, _ = c.Find(ctx, "a"); got.Status != model.Published || got.Verdict == nil || got.Verdict.Solutions != "one" {
		t.Fatalf("after verdict: %+v", got)
	}
	if pending, _ := c.PendingIDs(ctx); len(pending) != 0 {
		t.Fatalf("pending ids: %v", pending)
	}
	if err := c.SetVerdict(ctx, "missing", model.Published, validate.Verdict{}); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("verdict of a missing puzzle: %v", err)
	}
}

func TestPuzzle_ReportsAndModeration(t *testing.T) {
	ctx := context.Background()
	c, newUser := setup(t)
	publish(t, c, puzzle("a", 0))

	// One report per user, three hide the puzzle from lists, an admin review clears it.
	users := []int64{newUser(), newUser(), newUser()}
	for _, u := range users {
		for range 2 {
			if _, err := c.AddReport(ctx, "a", u); err != nil {
				t.Fatal(err)
			}
		}
		items, _ := c.List(ctx, model.Filter{Limit: 10})
		if wantVisible := u != users[2]; (len(items) == 1) != wantVisible {
			t.Fatalf("after reports by user %d: %d items", u, len(items))
		}
	}
	if p, _ := c.Find(ctx, "a"); p.Reports != 3 {
		t.Fatalf("reports = %d, want 3 (one per user)", p.Reports)
	}
	if err := c.SetModeration(ctx, "a", model.ModCleared); err != nil {
		t.Fatal(err)
	}
	if items, _ := c.List(ctx, model.Filter{Limit: 10}); len(items) != 1 {
		t.Fatal("a cleared puzzle must be listed again")
	}
	if err := c.SetModeration(ctx, "a", model.ModHidden); err != nil {
		t.Fatal(err)
	}
	if items, _ := c.List(ctx, model.Filter{Limit: 10}); len(items) != 0 {
		t.Fatal("a hidden puzzle must not be listed")
	}
	if _, err := c.AddReport(ctx, "missing", users[0]); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("report missing: %v", err)
	}
	if err := c.SetModeration(ctx, "missing", model.ModHidden); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("moderation of a missing puzzle: %v", err)
	}
}

func TestPuzzle_OwnerStatsDelete(t *testing.T) {
	ctx := context.Background()
	c, newUser := setup(t)
	ann := newUser()
	publish(t, c, puzzle("a", 0))

	if err := c.SetOwner(ctx, "a", ann); err != nil {
		t.Fatal(err)
	}
	if p, _ := c.Find(ctx, "a"); p.OwnerID != ann {
		t.Fatalf("owner: %+v", p)
	}
	if err := c.SetOwner(ctx, "missing", ann); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("owner of a missing puzzle: %v", err)
	}

	stats := model.RatingStats{StarsCount: 2, StarsSum: 9, DifficultyCount: 1, DifficultySum: 4}
	if err := c.SetRatingStats(ctx, "a", stats); err != nil {
		t.Fatal(err)
	}
	if p, _ := c.Find(ctx, "a"); p.Ratings != stats {
		t.Fatalf("rating stats: %+v", p.Ratings)
	}
	if err := c.SetRatingStats(ctx, "missing", stats); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("stats of a missing puzzle: %v", err)
	}

	if err := c.Delete(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if p, err := c.Find(ctx, "a"); p != nil || err != nil {
		t.Fatalf("after delete: %+v %v", p, err)
	}
	if err := c.Delete(ctx, "a"); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
}

func TestPuzzle_List(t *testing.T) {
	ctx := context.Background()
	c, newUser := setup(t)
	ann, bob := newUser(), newUser()
	for i := range 5 {
		owner := ann
		if i%2 == 1 {
			owner = bob
		}
		publish(t, c, puzzle(fmt.Sprintf("p%d", i), owner))
	}

	// Pages follow each other with no gap and no repeat, newest first.
	seen := map[string]bool{}
	var after *model.Cursor
	var order []string
	for page := 0; page < 10; page++ {
		items, err := c.List(ctx, model.Filter{Limit: 3, After: after})
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range items {
			if seen[p.ID] {
				t.Fatalf("page %d repeats %s", page, p.ID)
			}
			seen[p.ID] = true
			order = append(order, p.ID)
		}
		if len(items) < 3 {
			break
		}
		last := items[len(items)-1]
		after = &model.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	if fmt.Sprint(order) != "[p4 p3 p2 p1 p0]" {
		t.Fatalf("paging saw %v", order)
	}

	for _, tc := range []struct {
		f    model.Filter
		want int
	}{
		{model.Filter{Limit: 10, OwnerIDs: []int64{ann}}, 3},
		{model.Filter{Limit: 10, Tag: "classic"}, 5},
		{model.Filter{Limit: 10, Tag: "nope"}, 0},
		{model.Filter{Limit: 10, Variant: "killer"}, 5},
		{model.Filter{Limit: 10, Variant: "arrow"}, 0},
		{model.Filter{Limit: 10, Difficulty: "easy"}, 5},
		{model.Filter{Limit: 10, Difficulty: "hard"}, 0},
		{model.Filter{Limit: 10, Tag: "x", OwnerIDs: []int64{bob}}, 2},
		{model.Filter{Limit: 10, OwnerIDs: []int64{ann, bob}}, 5},
		{model.Filter{Limit: 10, OwnerIDs: []int64{}}, 0},
	} {
		if items, _ := c.List(ctx, tc.f); len(items) != tc.want {
			t.Errorf("filter %+v: %d items, want %d", tc.f, len(items), tc.want)
		}
	}
}

func TestPuzzle_ListTop(t *testing.T) {
	ctx := context.Background()
	c, _ := setup(t)
	for _, id := range []string{"low", "high", "none"} {
		publish(t, c, puzzle(id, 0))
	}
	if err := c.SetRatingStats(ctx, "high", model.RatingStats{StarsCount: 3, StarsSum: 15}); err != nil {
		t.Fatal(err)
	}
	if err := c.SetRatingStats(ctx, "low", model.RatingStats{StarsCount: 3, StarsSum: 3}); err != nil {
		t.Fatal(err)
	}

	var order []string
	for offset := 0; offset < 3; offset++ {
		items, err := c.List(ctx, model.Filter{Limit: 1, Top: true, Offset: offset})
		if err != nil || len(items) != 1 {
			t.Fatalf("offset %d: %v %v", offset, items, err)
		}
		order = append(order, items[0].ID)
	}
	if fmt.Sprint(order) != "[high none low]" {
		t.Fatalf("top order: %v", order)
	}
}

func TestPuzzle_PendingIDsOrder(t *testing.T) {
	ctx := context.Background()
	c, _ := setup(t)
	for _, id := range []string{"x", "y"} {
		if _, err := c.Create(ctx, puzzle(id, 0)); err != nil {
			t.Fatal(err)
		}
	}
	if ids, _ := c.PendingIDs(ctx); fmt.Sprint(ids) != "[x y]" {
		t.Fatalf("pending ids: %v", ids)
	}
}
