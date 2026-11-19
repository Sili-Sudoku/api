// Command puzzles runs publish, browse, get, claim, report and delete as their own program. It also runs the background
// validation: see "Deployables" in STYLE.md for what the host must provide for that.
package main

import (
	"context"
	"log"
	"time"

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

	// Puzzles are validated by a background job. This sweep finishes any that a restart or a tool failure left pending.
	go func() {
		for {
			if err := w.ValidatePending(ctx); err != nil {
				log.Printf("validate pending: %v", err)
			}

			time.Sleep(30 * time.Second)
		}
	}()

	boot.Serve("puzzles", routes.New(routes.Deps{
		Config:   server.ConfigFromEnv(),
		Limits:   routes.NewLimits(),
		Puzzles:  puzzles,
		Sessions: accountspg.NewSession(pool),
		Users:    accountspg.NewUser(pool),
		Worker:   w,
	}))
}
