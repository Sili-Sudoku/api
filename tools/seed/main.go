// Command seed fills a dev database: two users (one admin) and the bundled puzzles, published under "maker".
//
//	SEED_PASSWORD=... seed puzzles.json     (web: npm run -s seed:export > puzzles.json)
//
// It is safe to run again. It needs DATABASE_URL and VALIDATE_CMD like the API.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Sili-Sudoku/api/libs/boot"
	"github.com/Sili-Sudoku/api/libs/ratelimit"
	"github.com/Sili-Sudoku/api/services/accounts/endpoints/signup"
	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	accountspg "github.com/Sili-Sudoku/api/services/accounts/postgres"
	"github.com/Sili-Sudoku/api/services/puzzles/endpoints/publish"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
	puzzlespg "github.com/Sili-Sudoku/api/services/puzzles/postgres"
	"github.com/Sili-Sudoku/api/services/puzzles/worker"
)

type db struct {
	Puzzle  publish.PuzzleDB
	Session signup.SessionDB
	User    userDB
	Worker  validator
}

type userDB interface {
	signup.UserDB
	FindByName(ctx context.Context, name string) (*accounts.User, error)
	SetAdmin(ctx context.Context, id int64, admin bool) error
}

type validator interface {
	Validate(ctx context.Context, id string) error
}

type puzzleReader interface {
	Find(ctx context.Context, id string) (*puzzles.Puzzle, error)
}

// owner is the signed-in user of the publish calls.
type owner struct {
	u *accounts.User
}

func (o owner) FindUser(context.Context) *accounts.User { return o.u }

// skip stands in for the background validation: seed validates in line, so it can count the published puzzles.
type skip struct{}

func (skip) Validate(context.Context, string) error { return nil }

func seed(ctx context.Context, d db, find puzzleReader, password string, raw []byte) (published int, err error) {
	var all []json.RawMessage
	if err := json.Unmarshal(raw, &all); err != nil {
		return 0, fmt.Errorf("puzzle file must be a JSON array: %w", err)
	}

	allow := ratelimit.New(1<<30, time.Hour)
	register := signup.New(&signup.DB{Session: d.Session, User: d.User}, allow, signup.Config{})

	var maker *accounts.User
	for _, name := range []string{"admin", "maker"} {
		u, err := d.User.FindByName(ctx, name)
		if err != nil {
			return 0, fmt.Errorf("user %s: %w", name, err)
		}

		if u == nil {
			if _, err := register.Execute(ctx, &signup.EndpointReq{Username: name, Password: password}); err != nil {
				return 0, fmt.Errorf("user %s: %w", name, err)
			}

			if u, err = d.User.FindByName(ctx, name); err != nil || u == nil {
				return 0, fmt.Errorf("user %s: %v", name, err)
			}
		}
		if name == "admin" {
			if err := d.User.SetAdmin(ctx, u.ID, true); err != nil {
				return 0, err
			}
		} else {
			maker = u
		}
	}
	add := publish.New(owner{maker}, &publish.DB{Puzzle: d.Puzzle}, allow, skip{}, false)
	for _, body := range all {
		resp, err := add.Execute(ctx, &publish.EndpointReq{Body: body})
		if err != nil {
			return published, fmt.Errorf("publish: %w", err)
		}

		if err := d.Worker.Validate(ctx, resp.ID); err != nil {
			return published, fmt.Errorf("validate %s: %w", resp.ID, err)
		}

		if p, err := find.Find(ctx, resp.ID); err == nil && p != nil && p.Status == puzzles.Published {
			published++
		}
	}

	return published, nil
}

func main() {
	ctx := context.Background()
	password := os.Getenv("SEED_PASSWORD")
	if password == "" || len(os.Args) != 2 || os.Getenv("DATABASE_URL") == "" {
		log.Fatal("usage: SEED_PASSWORD=... DATABASE_URL=... seed puzzles.json")
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	pool, err := boot.OpenPool(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer pool.Close()

	puzzleStore := puzzlespg.NewPuzzle(pool)
	d := db{Puzzle: puzzleStore, Session: accountspg.NewSession(pool), User: accountspg.NewUser(pool), Worker: worker.New(puzzleStore, boot.Validator())}
	n, err := seed(ctx, d, puzzleStore, password, raw)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("seeded users admin and maker, %d puzzles published\n", n)
}
