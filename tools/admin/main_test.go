package main

import (
	"context"
	"strconv"
	"testing"

	"github.com/Sili-Sudoku/api/libs/dbtest"
	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	accountspg "github.com/Sili-Sudoku/api/services/accounts/postgres"
	communitymodel "github.com/Sili-Sudoku/api/services/community/model"
	communitypg "github.com/Sili-Sudoku/api/services/community/postgres"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
	puzzlespg "github.com/Sili-Sudoku/api/services/puzzles/postgres"
)

// The commands change a real database, so the test needs TEST_DATABASE_URL and is skipped without it.
func TestAdminCommands(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Tx(t)
	users, pz, cm := accountspg.NewUser(pool), puzzlespg.NewPuzzle(pool), communitypg.NewComment(pool)

	ann := &accounts.User{Username: "ann", PasswordHash: "x"}
	if err := users.Create(ctx, ann); err != nil {
		t.Fatal(err)
	}
	publish := func(title string) string {
		p := &puzzles.Puzzle{ID: title, Body: []byte(`{}`), Status: puzzles.Published, Moderation: puzzles.ModNone,
			Title: title, Tags: []string{}, Variants: []string{}}
		if _, err := pz.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
		return p.ID
	}
	id := publish("t")
	listed := func() int { items, _ := pz.List(ctx, puzzles.Filter{Limit: 10}); return len(items) }

	steps := []struct {
		args []string
		ok   bool
	}{
		{[]string{"promote", "ann"}, true},
		{[]string{"promote", "nobody"}, false},
		{[]string{"hide", id}, true},
		{[]string{"unhide", id}, true},
		{[]string{"delete", id}, true},
		{[]string{"delete", id}, false},
		{[]string{"explode", id}, false},
		{[]string{"hide"}, false},
	}
	for _, s := range steps {
		if err := run(ctx, users, pz, cm, s.args); (err == nil) != s.ok {
			t.Errorf("%v: %v", s.args, err)
		}
		switch s.args[0] {
		case "hide":
			if listed() != 0 {
				t.Error("hide left the puzzle in lists")
			}
		case "unhide":
			if listed() != 1 {
				t.Error("unhide did not list the puzzle")
			}
		}
	}

	// Comment moderation: hide, unhide, delete; a bad id is an error.
	id2 := publish("u")
	c := &communitymodel.Comment{PuzzleID: id2, UserID: ann.ID, Body: "hello", Moderation: puzzles.ModNone}
	if err := cm.Create(ctx, c); err != nil {
		t.Fatal(err)
	}
	shown := func() int { items, _ := cm.List(ctx, id2, nil, 10); return len(items) }
	cid := strconv.FormatInt(c.ID, 10)
	for _, step := range []struct {
		cmd  string
		want int
	}{{"hide-comment", 0}, {"unhide-comment", 1}, {"delete-comment", 0}} {
		if err := run(ctx, users, pz, cm, []string{step.cmd, cid}); err != nil || shown() != step.want {
			t.Errorf("%s: %v, shown %d", step.cmd, err, shown())
		}
	}
	if err := run(ctx, users, pz, cm, []string{"hide-comment", "x"}); err == nil {
		t.Error("a non-numeric comment id was accepted")
	}

	if u, _ := users.FindByName(ctx, "ann"); !u.IsAdmin {
		t.Error("promote did not set is_admin")
	}
	if err := run(ctx, users, pz, cm, []string{"demote", "ann"}); err != nil {
		t.Fatal(err)
	}
	if u, _ := users.FindByName(ctx, "ann"); u.IsAdmin {
		t.Error("demote did not clear is_admin")
	}
}
