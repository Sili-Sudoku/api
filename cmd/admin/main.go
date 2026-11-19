// Command admin is the moderation tool: there is no admin UI in the MVP.
//
//	admin promote <username>    make a user an admin
//	admin demote <username>
//	admin hide <puzzleId>       remove a puzzle from lists
//	admin unhide <puzzleId>     show it again; reports no longer hide it
//	admin delete <puzzleId>
//	admin hide-comment|unhide-comment|delete-comment <commentId>
//
// It needs DATABASE_URL, like the API.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/Sili-Sudoku/api/libs/boot"
	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	accountspg "github.com/Sili-Sudoku/api/services/accounts/postgres"
	communitypg "github.com/Sili-Sudoku/api/services/community/postgres"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
	puzzlespg "github.com/Sili-Sudoku/api/services/puzzles/postgres"
)

type commentDB interface {
	Delete(ctx context.Context, id int64) error
	SetModeration(ctx context.Context, id int64, m puzzles.Moderation) error
}

type puzzleDB interface {
	Delete(ctx context.Context, id string) error
	SetModeration(ctx context.Context, id string, m puzzles.Moderation) error
}

type userDB interface {
	FindByName(ctx context.Context, name string) (*accounts.User, error)
	SetAdmin(ctx context.Context, id int64, admin bool) error
}

func run(ctx context.Context, users userDB, pz puzzleDB, cm commentDB, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: admin promote|demote <username> | hide|unhide|delete <puzzleId> | hide-comment|unhide-comment|delete-comment <commentId>")
	}
	cmd, target := args[0], args[1]
	switch cmd {
	case "promote", "demote":
		u, err := users.FindByName(ctx, target)
		if err != nil {
			return fmt.Errorf("user %q: %w", target, err)
		}

		if u == nil {
			return fmt.Errorf("user %q: %w", target, accounts.ErrNotFound)
		}

		return users.SetAdmin(ctx, u.ID, cmd == "promote")
	case "hide":
		return pz.SetModeration(ctx, target, puzzles.ModHidden)
	case "unhide":
		return pz.SetModeration(ctx, target, puzzles.ModCleared)
	case "delete":
		return pz.Delete(ctx, target)
	case "hide-comment", "unhide-comment", "delete-comment":
		id, err := strconv.ParseInt(target, 10, 64)
		if err != nil {
			return fmt.Errorf("comment id %q is not a number", target)
		}

		switch cmd {
		case "hide-comment":
			return cm.SetModeration(ctx, id, puzzles.ModHidden)
		case "unhide-comment":
			return cm.SetModeration(ctx, id, puzzles.ModCleared)
		}

		return cm.Delete(ctx, id)
	}

	return fmt.Errorf("unknown command %q", cmd)
}

func main() {
	ctx := context.Background()
	if os.Getenv("DATABASE_URL") == "" {
		log.Fatal("DATABASE_URL is not set: the admin tool changes the real database")
	}
	pool, err := boot.OpenPool(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer pool.Close()
	if err := run(ctx, accountspg.NewUser(pool), puzzlespg.NewPuzzle(pool), communitypg.NewComment(pool), os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	fmt.Println("done")
}
