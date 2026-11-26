// Command community runs ratings, comments, profiles, follows and the feed as their own program.
package main

import (
	"context"
	"log"

	"github.com/Sili-Sudoku/api/libs/boot"
	accountspg "github.com/Sili-Sudoku/api/services/accounts/postgres"
	communitypg "github.com/Sili-Sudoku/api/services/community/postgres"
	"github.com/Sili-Sudoku/api/services/community/routes"
	historypg "github.com/Sili-Sudoku/api/services/history/postgres"
	puzzlespg "github.com/Sili-Sudoku/api/services/puzzles/postgres"
)

func main() {
	pool, err := boot.OpenPool(context.Background())
	if err != nil {
		log.Fatalf("start: %v", err)
	}
	defer pool.Close()

	boot.Serve("community", routes.New(routes.Deps{
		Comments:     communitypg.NewComment(pool),
		Follows:      communitypg.NewFollow(pool),
		Limits:       routes.NewLimits(),
		Puzzles:      puzzlespg.NewPuzzle(pool),
		Ratings:      communitypg.NewRating(pool),
		Sessions:     accountspg.NewSession(pool),
		SolveHistory: historypg.NewSolveHistory(pool),
		Users:        accountspg.NewUser(pool),
	}))
}
