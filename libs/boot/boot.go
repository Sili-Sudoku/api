// Package boot is the plumbing every program shares: the database pool, the validator and the HTTP server loop. Each
// program builds only the clients it uses.
package boot

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sili-Sudoku/api/libs/db"
	"github.com/Sili-Sudoku/api/libs/validate"
)

// OpenPool opens the database from DATABASE_URL (required). It applies no migrations: run cmd/migrate first.
func OpenPool(ctx context.Context) (*pgxpool.Pool, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return nil, errors.New("DATABASE_URL is not set")
	}

	return db.Open(ctx, url)
}

// Validator reads VALIDATE_CMD (empty = accept every puzzle, dev only).
func Validator() validate.Validator {
	cmd := os.Getenv("VALIDATE_CMD")
	if cmd == "" {
		log.Print("VALIDATE_CMD is empty: accepting every puzzle (dev only)")
		return validate.Accept{}
	}

	return validate.Exec{Path: cmd, Timeout: 20 * time.Second}
}

// Serve listens on API_ADDR (default :8080) and does not return unless the server stops.
func Serve(name string, h http.Handler) {
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("%s listening on %s", name, addr)
	log.Fatal(srv.ListenAndServe())
}
