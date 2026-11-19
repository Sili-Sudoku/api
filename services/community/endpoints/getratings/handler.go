package getratings

import (
	"context"

	accounts "github.com/Sili-Sudoku/api/services/accounts/model"
	"github.com/Sili-Sudoku/api/services/community/model"
	puzzles "github.com/Sili-Sudoku/api/services/puzzles/model"
)

type Endpoint struct {
	auth Auth
	db   *DB
}

func New(auth Auth, db *DB) *Endpoint { return &Endpoint{auth: auth, db: db} }

// Auth answers for signed-out callers too: FindUser returns nil then.
type Auth interface {
	FindUser(ctx context.Context) *accounts.User
}

type DB struct {
	Puzzle PuzzleDB
	Rating RatingDB
}

type PuzzleDB interface {
	Find(ctx context.Context, id string) (*puzzles.Puzzle, error)
}

type RatingDB interface {
	Find(ctx context.Context, puzzleID string, userID int64) (*model.Rating, error)
}
