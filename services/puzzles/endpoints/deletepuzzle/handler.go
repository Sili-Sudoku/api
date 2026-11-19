package deletepuzzle

import (
	"context"

	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/puzzles/model"
)

type Endpoint struct {
	auth Auth
	db   *DB
}

func New(auth Auth, db *DB) *Endpoint { return &Endpoint{auth: auth, db: db} }

type Auth interface {
	FindUser(ctx context.Context) *accounts.User
}

type DB struct {
	Puzzle PuzzleDB
}

type PuzzleDB interface {
	Delete(ctx context.Context, id string) error
	Find(ctx context.Context, id string) (*model.Puzzle, error)
}
