package putrating

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

type Auth interface {
	GetUser(ctx context.Context) (*accounts.User, error)
}

type DB struct {
	Puzzle PuzzleDB
	Rating RatingDB
}

type PuzzleDB interface {
	Find(ctx context.Context, id string) (*puzzles.Puzzle, error)
	SetRatingStats(ctx context.Context, id string, r puzzles.RatingStats) error
}

type RatingDB interface {
	Find(ctx context.Context, puzzleID string, userID int64) (*model.Rating, error)
	Put(ctx context.Context, puzzleID string, userID int64, r model.Rating) error
	Stats(ctx context.Context, puzzleID string) (puzzles.RatingStats, error)
}
