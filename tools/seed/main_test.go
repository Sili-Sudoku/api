package main

import (
	"context"
	"testing"

	"github.com/Sili-Sudoku/api/libs/dbtest"
	"github.com/Sili-Sudoku/api/libs/validate"
	accountspg "github.com/Sili-Sudoku/api/services/accounts/postgres"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
	puzzlespg "github.com/Sili-Sudoku/api/services/puzzles/postgres"
	"github.com/Sili-Sudoku/api/services/puzzles/worker"
)

// The seed fills a real database, so the test needs TEST_DATABASE_URL and is skipped without it.
func TestSeedIsRepeatable(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Tx(t)
	users, pz := accountspg.NewUser(pool), puzzlespg.NewPuzzle(pool)
	d := db{Puzzle: pz, Session: accountspg.NewSession(pool), User: users, Worker: worker.New(pz, validate.Accept{})}
	file := []byte(`[{"metadata":{"title":"a"}},{"metadata":{"title":"b"}}]`)

	for range 2 {
		n, err := seed(ctx, d, pz, "a long password", file)
		if err != nil || n != 2 {
			t.Fatalf("seed: %d %v", n, err)
		}
	}
	admin, _ := users.FindByName(ctx, "admin")
	maker, _ := users.FindByName(ctx, "maker")
	if !admin.IsAdmin || maker.IsAdmin {
		t.Fatalf("admin flags: %v %v", admin.IsAdmin, maker.IsAdmin)
	}
	items, _ := pz.List(ctx, puzzles.Filter{Limit: 10, OwnerIDs: []int64{maker.ID}})
	if len(items) != 2 {
		t.Fatalf("puzzles by maker: %d", len(items))
	}
	if _, err := seed(ctx, d, pz, "a long password", []byte(`{}`)); err == nil {
		t.Fatal("a file that is not an array must fail")
	}
}
