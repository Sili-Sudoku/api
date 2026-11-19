package claim

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
	GetUser(ctx context.Context) (*accounts.User, error)
}

type DB struct {
	Puzzle PuzzleDB
}

type PuzzleDB interface {
	Find(ctx context.Context, id string) (*model.Puzzle, error)
	SetOwner(ctx context.Context, id string, ownerID int64) error
}
