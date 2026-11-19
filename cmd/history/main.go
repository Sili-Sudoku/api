// Command history runs the solve-history routes of the Sili Sudoku API as their own program.
package main

import (
	"context"
	"log"

	"github.com/Sili-Sudoku/api/libs/boot"
	accountspg "github.com/Sili-Sudoku/api/services/accounts/postgres"
	historypg "github.com/Sili-Sudoku/api/services/history/postgres"
	"github.com/Sili-Sudoku/api/services/history/routes"
	puzzlespg "github.com/Sili-Sudoku/api/services/puzzles/postgres"
)

func main() {
	pool, err := boot.OpenPool(context.Background())
	if err != nil {
		log.Fatalf("start: %v", err)
	}
	defer pool.Close()

	boot.Serve("history", routes.New(routes.Deps{
		Puzzles:      puzzlespg.NewPuzzle(pool),
		Sessions:     accountspg.NewSession(pool),
		SolveHistory: historypg.NewSolveHistory(pool),
	}))
}
