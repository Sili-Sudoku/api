// Command accounts runs sign-up, sessions and /v1/me of the Sili Sudoku API as their own program.
package main

import (
	"context"
	"log"

	"github.com/Sili-Sudoku/api/libs/boot"
	"github.com/Sili-Sudoku/api/libs/server"
	accountspg "github.com/Sili-Sudoku/api/services/accounts/postgres"
	"github.com/Sili-Sudoku/api/services/accounts/routes"
)

func main() {
	pool, err := boot.OpenPool(context.Background())
	if err != nil {
		log.Fatalf("start: %v", err)
	}
	defer pool.Close()

	boot.Serve("accounts", routes.New(routes.Deps{
		Config:   server.ConfigFromEnv(),
		Limits:   routes.NewLimits(),
		Sessions: accountspg.NewSession(pool),
		Users:    accountspg.NewUser(pool),
	}))
}
