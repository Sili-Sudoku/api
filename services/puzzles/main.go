// Command puzzles runs publish, browse, get, claim, report and delete as their own program. Validation runs inside the
// requests (publish, and a read of a pending puzzle), so nothing runs between requests and the program can scale to zero.
package main

import (
	"context"
	"log"

	"github.com/Sili-Sudoku/api/libs/boot"
	"github.com/Sili-Sudoku/api/libs/server"
	accountspg "github.com/Sili-Sudoku/api/services/accounts/postgres"
	puzzlespg "github.com/Sili-Sudoku/api/services/puzzles/postgres"
	"github.com/Sili-Sudoku/api/services/puzzles/routes"
	"github.com/Sili-Sudoku/api/services/puzzles/worker"
)

func main() {
	ctx := context.Background()

	pool, err := boot.OpenPool(ctx)
	if err != nil {
		log.Fatalf("start: %v", err)
	}
	defer pool.Close()

	puzzles := puzzlespg.NewPuzzle(pool)
	w := worker.New(puzzles, boot.Validator())

	boot.Serve("puzzles", routes.New(routes.Deps{
		Config:   server.ConfigFromEnv(),
		Limits:   routes.NewLimits(),
		Puzzles:  puzzles,
		Sessions: accountspg.NewSession(pool),
		Users:    accountspg.NewUser(pool),
		Worker:   w,
	}))
}
