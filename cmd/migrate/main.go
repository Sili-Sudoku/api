// Command migrate applies the SQL files in migrations/ and exits. Run it before the programs start: a Cloud Run job in the
// cloud, one command locally.
package main

import (
	"context"
	"log"

	"github.com/Sili-Sudoku/api/libs/boot"
	"github.com/Sili-Sudoku/api/libs/db"
	"github.com/Sili-Sudoku/api/migrations"
)

func main() {
	ctx := context.Background()

	pool, err := boot.OpenPool(ctx)
	if err != nil {
		log.Fatalf("migrate: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	log.Print("migrations applied")
}
